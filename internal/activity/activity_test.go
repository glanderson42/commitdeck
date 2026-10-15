package activity_test

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/database"
	"commitdeck/internal/workspace"
	"path/filepath"
	"testing"
)

func TestTransitionDeduplication(t *testing.T) {
	d, err := database.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w, err := d.CreateWorkspace(workspace.Workspace{Title: "Ticket"})
	if err != nil {
		t.Fatal(err)
	}
	s := activity.Service{Store: d}
	for i := 0; i < 3; i++ {
		if err = s.Transition(w.ID, "ci", "failed", "ci_failed", "CI failed", true); err != nil {
			t.Fatal(err)
		}
	}
	events, err := d.Activity(w.ID, false)
	if err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
	if err = s.Transition(w.ID, "ci", "passed", "ci_passed", "CI passed", true); err != nil {
		t.Fatal(err)
	}
	if err = s.Transition(w.ID, "ci", "failed", "ci_failed", "CI failed again", true); err != nil {
		t.Fatal(err)
	}
	events, _ = d.Activity(w.ID, false)
	if len(events) != 3 {
		t.Fatal("transition reentry was lost")
	}
	if err = d.ReadNotifications(); err != nil {
		t.Fatal(err)
	}
	notifications, _ := d.Activity("", true)
	for _, n := range notifications {
		if !n.Read {
			t.Fatal("notification not acknowledged")
		}
	}
}
