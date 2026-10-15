package activity

import (
	"commitdeck/internal/identity"
	"commitdeck/internal/redact"
	"encoding/json"
)

type Event struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspaceId"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Metadata    json.RawMessage `json:"metadata"`
	DedupeKey   string          `json:"-"`
	Notify      bool            `json:"notify"`
	Read        bool            `json:"read"`
	CreatedAt   string          `json:"createdAt"`
}
type Store interface {
	AppendActivity(Event) (bool, error)
	TransitionActivity(string, string, string, Event) (bool, error)
}
type Service struct {
	Store Store
	Emit  func(any)
}

func (s *Service) Record(id, kind, title, key string, notify bool) error {
	e := Event{ID: identity.ID(), WorkspaceID: id, Type: kind, Title: redact.Text(title), Metadata: json.RawMessage(`{}`), DedupeKey: key, Notify: notify, CreatedAt: identity.Now()}
	added, err := s.Store.AppendActivity(e)
	if err == nil && added && s.Emit != nil {
		s.Emit(map[string]any{"type": "activity", "workspaceId": id})
	}
	return err
}
func (s *Service) Transition(id, source, value, kind, title string, notify bool) error {
	e := Event{ID: identity.ID(), WorkspaceID: id, Type: kind, Title: redact.Text(title), Metadata: json.RawMessage(`{}`), Notify: notify, CreatedAt: identity.Now()}
	added, err := s.Store.TransitionActivity(id, source, value, e)
	if err == nil && added && s.Emit != nil {
		s.Emit(map[string]any{"type": "activity", "workspaceId": id})
	}
	return err
}
