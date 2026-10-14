package whatsmeow_service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_repository "github.com/evolution-foundation/evolution-go/pkg/instance/repository"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/patrickmn/go-cache"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type logoutRepository struct {
	instance_repository.InstanceRepository
	jid    string
	loaded atomic.Bool
}

func (r *logoutRepository) UpdateJid(string, string) error             { r.jid = "cleared"; return nil }
func (r *logoutRepository) UpdateQrcode(string, string) error          { return nil }
func (r *logoutRepository) UpdateConnected(string, bool, string) error { return nil }
func (r *logoutRepository) GetInstanceByIDContext(context.Context, string) (*instance_model.Instance, error) {
	r.loaded.Store(true)
	return &instance_model.Instance{Jid: "revoked"}, nil
}

func TestLoggedOutDiscardsRevokedDeviceAndSkipsReconnect(t *testing.T) {
	cfg := &config.Config{LogDirectory: t.TempDir(), LogMaxSize: 1}
	logs := logger_wrapper.NewLoggerManager(cfg)
	t.Cleanup(func() {
		if err := logs.GetLogger(editTestInstanceID).Close(); err != nil {
			t.Error(err)
		}
	})
	repo := &logoutRepository{}
	sessions, _ := testSessions(t)
	runCtx, cancel := context.WithCancel(context.Background())
	run := &sessionRun{
		ctx: runCtx, cancel: cancel,
		ready: make(chan struct{}), pairingReady: make(chan struct{}), done: make(chan struct{}),
		slot: &sessionSlot{},
	}
	run.paired.Store(true)
	jid := types.NewJID("556699959474", types.DefaultUserServer)
	jid.Device = 60
	client := &MyClient{
		service:  &whatsmeowService{sessions: sessions, instanceRepository: repo, config: cfg, loggerWrapper: logs},
		WAClient: &whatsmeow.Client{Store: &store.Device{Deleted: true, ID: &jid}},
		Instance: &instance_model.Instance{
			Id: editTestInstanceID, Name: "pedidos", Token: "tok", Jid: jid.String(),
		},
		userID: editTestInstanceID, token: "tok",
		instanceRepository: repo,
		userInfoCache:      cache.New(time.Minute, time.Minute),
		config:             cfg, loggerWrapper: logs, run: run,
	}

	client.handleEvent(&events.LoggedOut{Reason: events.ConnectFailureLoggedOut})
	run.workers.Wait()

	if repo.jid != "cleared" || client.Instance.Jid != "" {
		t.Fatal("revoked session kept its JID")
	}
	if !run.loggedOut.Load() || run.paired.Load() || run.ctx.Err() == nil {
		t.Fatal("revoked session stayed eligible to reconnect")
	}

	client.handleEvent(&events.Disconnected{})
	if repo.loaded.Load() {
		t.Fatal("revoked session was reconnected")
	}
}

func TestSessionProxyEnabledIgnoresNullPlaceholder(t *testing.T) {
	if sessionProxyEnabled("null", "") || sessionProxyEnabled("  ", "") || sessionProxyEnabled(`{"host":""}`, "") {
		t.Fatal("placeholder proxy treated as enabled")
	}
	if !sessionProxyEnabled(`{"host":"10.0.0.1","port":"8080"}`, "") || !sessionProxyEnabled("null", "proxy.example") {
		t.Fatal("configured proxy treated as disabled")
	}
}
