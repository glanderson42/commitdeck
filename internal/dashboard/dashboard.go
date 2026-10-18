package dashboard

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/pullrequest"
	"commitdeck/internal/redact"
	"commitdeck/internal/workspace"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

var Priority = map[string]int{"AGENT_WAITING": 1, "CHANGES_REQUESTED": 2, "CI_FAILED": 3, "MERGE_CONFLICT": 4, "BLOCKED": 5, "AGENT_RUNNING": 6, "CODING": 7, "READY_TO_MERGE": 8, "WAITING_FOR_REVIEW": 9, "NO_PR": 10, "IDLE": 11}

type Row struct {
	WorkflowWaiting int `json:"workflowWaiting"`
	WorkflowFailed  int `json:"workflowFailed"`
	workspace.Workspace
	Attention          string                   `json:"attention"`
	Group              string                   `json:"group"`
	Sessions           []protocol.Session       `json:"sessions"`
	PR                 *pullrequest.PullRequest `json:"pr"`
	TasksTotal         int                      `json:"tasksTotal"`
	TasksCompleted     int                      `json:"tasksCompleted"`
	PendingPermissions int                      `json:"pendingPermissions"`
	SearchText         string                   `json:"searchText"`
	LastActivity       string                   `json:"lastActivity"`
}

func Group(state string) string {
	if Priority[state] <= 5 && Priority[state] > 0 {
		return "Needs attention"
	}
	switch state {
	case "AGENT_RUNNING", "CODING":
		return "Active work"
	case "READY_TO_MERGE":
		return "Ready"
	default:
		return "Waiting"
	}
}
func State(w workspace.Workspace, s []protocol.Session, p *pullrequest.PullRequest, pending int) string {
	a := workspace.Attention(s, p)
	if pending > 0 {
		return "AGENT_WAITING"
	}
	if (w.Lifecycle == "BLOCKED" || strings.EqualFold(w.Status, "blocked")) && (Priority[a] > 5) {
		return "BLOCKED"
	}
	if a == "NO_PR" && (strings.EqualFold(w.Status, "coding") || (strings.EqualFold(w.Status, "in progress") && w.WorktreePath != "")) {
		return "CODING"
	}
	return a
}
func StateWithWorkflows(w workspace.Workspace, s []protocol.Session, p *pullrequest.PullRequest, pending, waiting, failed int) string {
	result := State(w, s, p, pending)
	if w.Archived {
		return result
	}
	if waiting > 0 {
		return "AGENT_WAITING"
	}
	if failed > 0 && Priority[result] > 5 {
		return "BLOCKED"
	}
	return result
}
func Sort(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if Priority[a.Attention] != Priority[b.Attention] {
			return Priority[a.Attention] < Priority[b.Attention]
		}
		if a.LastActivity != b.LastActivity {
			return a.LastActivity > b.LastActivity
		}
		return a.ID < b.ID
	})
}

// Load uses a fixed number of local bulk queries regardless of workspace count. No remote I/O occurs here.
func Load(db *database.DB) ([]Row, error) {
	ws, err := db.Workspaces()
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(ws))
	indices := map[string]int{}
	for _, w := range ws {
		indices[w.ID] = len(rows)
		rows = append(rows, Row{Workspace: w, Sessions: []protocol.Session{}, LastActivity: w.UpdatedAt})
	}
	sessions, err := db.Query("SELECT " + "id,workspace_id,provider,external_session_id,title,purpose,status,pid,created_at,updated_at,imported,resume_supported,source_cwd FROM agent_sessions ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	for sessions.Next() {
		var s protocol.Session
		if err = sessions.Scan(&s.ID, &s.WorkspaceID, &s.Provider, &s.ExternalSessionID, &s.Title, &s.Purpose, &s.Status, &s.PID, &s.CreatedAt, &s.UpdatedAt, &s.Imported, &s.ResumeSupported, &s.SourceCWD); err != nil {
			sessions.Close()
			return nil, err
		}
		i, ok := indices[s.WorkspaceID]
		if ok {
			rows[i].Sessions = append(rows[i].Sessions, s)
			if s.UpdatedAt > rows[i].LastActivity {
				rows[i].LastActivity = s.UpdatedAt
			}
		}
	}
	err = sessions.Err()
	sessions.Close()
	if err != nil {
		return nil, err
	}
	data, err := db.Query(`SELECT w.id,COALESCE(n.content,''),COALESCE(n.updated_at,''),COALESCE(t.total,0),COALESCE(t.completed,0),p.payload,COALESCE(q.pending,0),e.last_activity,COALESCE(f.waiting,0),COALESCE(f.failed,0) FROM workspaces w LEFT JOIN notes n ON n.workspace_id=w.id LEFT JOIN (SELECT workspace_id,COUNT(*) total,SUM(completed) completed FROM todos GROUP BY workspace_id)t ON t.workspace_id=w.id LEFT JOIN pull_request_cache p ON p.workspace_id=w.id LEFT JOIN (SELECT workspace_id,COUNT(*) pending FROM permission_requests WHERE status='pending' GROUP BY workspace_id)q ON q.workspace_id=w.id LEFT JOIN (SELECT workspace_id,MAX(created_at) last_activity FROM workspace_events GROUP BY workspace_id)e ON e.workspace_id=w.id LEFT JOIN (SELECT workspace_id,SUM(CASE WHEN status='awaiting_approval' THEN 1 ELSE 0 END) waiting,SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) failed FROM agent_workflows GROUP BY workspace_id)f ON f.workspace_id=w.id`)
	if err != nil {
		return nil, err
	}
	defer data.Close()
	for data.Next() {
		var id, notes, noteUpdated string
		var total, completed, pending, waiting, failed int
		var raw, last *string
		if err = data.Scan(&id, &notes, &noteUpdated, &total, &completed, &raw, &pending, &last, &waiting, &failed); err != nil {
			return nil, err
		}
		i := indices[id]
		r := &rows[i]
		if noteUpdated > r.LastActivity {
			r.LastActivity = noteUpdated
		}
		r.TasksTotal = total
		r.TasksCompleted = completed
		r.PendingPermissions = pending
		if raw != nil {
			_ = json.Unmarshal([]byte(*raw), &r.PR)
		}
		if last != nil && *last > r.LastActivity {
			r.LastActivity = *last
		}
		r.WorkflowWaiting = waiting
		r.WorkflowFailed = failed
		r.Attention = StateWithWorkflows(r.Workspace, r.Sessions, r.PR, pending, waiting, failed)
		r.Group = Group(r.Attention)
		pr := ""
		if r.PR != nil {
			pr = r.PR.Title + " #" + strconv.Itoa(r.PR.Number)
		}
		r.SearchText = strings.ToLower(redact.Text(r.ExternalKey + " " + r.Title + " " + r.Branch + " " + pr + " " + notes))
		if len(r.SearchText) > 64000 {
			r.SearchText = r.SearchText[:64000]
		}
	}
	Sort(rows)
	return rows, data.Err()
}
