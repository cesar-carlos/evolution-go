package sender_service

import (
	"context"
	"errors"
	"strings"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ErrInvalidLIDs identifies malformed or oversized LID batches.
var ErrInvalidLIDs = errors.New("provide 1 to 100 numeric LIDs, each at most 32 digits")

// ClientProvider retrieves an already owned session through its synchronized index.
type ClientProvider interface {
	GetClient(string) *whatsmeow.Client
}

// Service exposes the authenticated session's metadata and existing LID store.
type Service struct{ clients ClientProvider }

// NewService reuses the application's session provider.
func NewService(clients ClientProvider) *Service { return &Service{clients: clients} }

// Session contains browser-visible metadata, without credentials.
type Session struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Connected        bool     `json:"connected"`
	WebsocketEnabled bool     `json:"websocketEnabled"`
	Events           []string `json:"events"`
}

func (s *Service) Session(instance *instance_model.Instance) Session {
	client := s.clients.GetClient(instance.Id)
	return Session{ID: instance.Id, Name: instance.Name,
		Connected:        client != nil && client.IsConnected() && client.IsLoggedIn(),
		WebsocketEnabled: (instance.WebSocketEnable == "true" || instance.WebSocketEnable == "enabled"), Events: strings.Split(instance.Events, ",")}
}

// ResolveLIDs uses the authenticated session's existing store, for both sqlstore
// dialects. No database pool is opened by the Sender. Missing mappings stay absent.
func (s *Service) ResolveLIDs(ctx context.Context, instanceID, raw string) (map[string]string, error) {
	parts := strings.Split(raw, ",")
	if len(parts) > 100 {
		return nil, ErrInvalidLIDs
	}
	lids := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) == 0 || len(part) > 32 || strings.Trim(part, "0123456789") != "" {
			return nil, ErrInvalidLIDs
		}
		if !seen[part] {
			lids = append(lids, part)
			seen[part] = true
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	client := s.clients.GetClient(instanceID)
	if client == nil || client.Store == nil || client.Store.LIDs == nil {
		return out, nil
	}
	for _, lid := range lids {
		pn, err := client.Store.LIDs.GetPNForLID(ctx, types.NewJID(lid, types.HiddenUserServer))
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if pn.Server == types.DefaultUserServer && pn.User != "" {
			out[lid] = pn.User
		}
	}
	return out, nil
}
