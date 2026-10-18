package permissions

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/identity"
	"errors"
	"path/filepath"
	"strings"
	"sync"
)

type Store interface {
	SavePermission(Request) error
	Permission(string) (Request, error)
	FinishPermission(string, string) error
	PermissionRules(string) ([]Rule, error)
	SavePermissionRule(Rule) error
	ExpirePermissions(string) error
	PermissionRoot(string) (string, error)
}
type Service struct {
	Store    Store
	Activity *activity.Service
	Emit     func(any)
	mu       sync.Mutex
	live     map[string]func(bool) error
}

func New(store Store, a *activity.Service, emit func(any)) *Service {
	return &Service{Store: store, Activity: a, Emit: emit, live: map[string]func(bool) error{}}
}
func (s *Service) Request(r Request, reply func(bool) error) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = identity.ID()
	r.CreatedAt = identity.Now()
	r.Status = "pending"
	r.CanRespond = reply != nil
	Classify(&r)
	if r.UnverifiedCwd {
		r.HighRisk = true
		r.Description += "\nThe provider did not report the tool working directory. The displayed directory is the session start directory; remembered permission is unavailable."
	}
	root, rootErr := s.Store.PermissionRoot(r.WorkspaceID)
	canonicalRoot, e1 := filepath.EvalSymlinks(root)
	cwd, e2 := filepath.EvalSymlinks(r.Cwd)
	rel, e3 := filepath.Rel(canonicalRoot, cwd)
	if rootErr != nil || e1 != nil || e2 != nil || e3 != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		r.HighRisk = true
	}
	// Secret-bearing commands cannot be remembered. The original wire input stays only in the provider transport.
	r = r.Redacted()
	if err := s.Store.SavePermission(r); err != nil {
		return "", err
	}
	if reply != nil {
		s.live[r.ID] = reply
		rules, err := s.Store.PermissionRules(r.WorkspaceID)
		if err != nil {
			return "", err
		}
		for _, rule := range rules {
			if Matches(rule, r) {
				go func() { _ = s.Resolve(r.ID, Decision{Allow: true, Scope: "once"}) }()
				s.changed(r.WorkspaceID)
				return r.ID, nil
			}
		}
	}
	_ = s.Activity.Record(r.WorkspaceID, "permission_requested", r.Provider+" needs approval: "+r.Title, r.ID, true)
	s.changed(r.WorkspaceID)
	return r.ID, nil
}
func (s *Service) Resolve(id string, d Decision) error {
	s.mu.Lock()
	r, err := s.Store.Permission(id)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if r.Status != "pending" {
		s.mu.Unlock()
		return errors.New("request is no longer pending")
	}
	reply := s.live[id]
	if reply == nil {
		s.mu.Unlock()
		return errors.New("approval channel is no longer available; resume the provider to obtain a fresh request")
	}
	if d.Scope == "" {
		d.Scope = "once"
	}
	if d.Scope != "once" && d.Scope != "session" && d.Scope != "workspace" {
		s.mu.Unlock()
		return errors.New("invalid permission scope")
	}
	if d.Allow && d.Scope != "once" && r.HighRisk {
		s.mu.Unlock()
		return errors.New("this action requires individual approval; reusable permission refused")
	}
	delete(s.live, id)
	s.mu.Unlock()
	// Never hold the queue mutex while a provider acknowledges a decision: other requests and ACKs must keep flowing.
	if err = reply(d.Allow); err != nil {
		_ = s.Store.FinishPermission(id, "expired")
		s.changed(r.WorkspaceID)
		return err
	}
	status := "denied"
	if d.Allow {
		status = "approved"
	}
	if err = s.Store.FinishPermission(id, status); err != nil {
		return err
	}
	if d.Allow && d.Scope != "once" {
		rule := Rule{ID: identity.ID(), WorkspaceID: r.WorkspaceID, SessionID: r.SessionID, Scope: d.Scope, Executable: r.Executable, ArgPrefix: r.Arguments, Cwd: r.Cwd, CreatedAt: identity.Now()}
		if err = s.Store.SavePermissionRule(rule); err != nil {
			s.changed(r.WorkspaceID)
			return err
		}
	}
	_ = s.Activity.Record(r.WorkspaceID, "permission_"+status, r.Provider+" permission "+status+": "+r.Title, r.ID+":"+status, false)
	s.changed(r.WorkspaceID)
	return nil
}
func (s *Service) ExpireSession(id, workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.live {
		r, err := s.Store.Permission(key)
		if err == nil && r.SessionID == id {
			delete(s.live, key)
		}
	}
	_ = s.Store.ExpirePermissions(id)
	s.changed(workspaceID)
}
func (s *Service) changed(id string) {
	if s.Emit != nil {
		s.Emit(map[string]any{"type": "permissions_changed", "workspaceId": id})
	}
}
