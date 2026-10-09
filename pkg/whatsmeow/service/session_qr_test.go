package whatsmeow_service

import (
	"testing"

	instance_repository "github.com/evolution-foundation/evolution-go/pkg/instance/repository"
	"github.com/evolution-foundation/evolution-go/pkg/passkey/ceremony"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
)

type qrLifecycleRepository struct {
	instance_repository.InstanceRepository
	updates chan string
}

func (r *qrLifecycleRepository) UpdateQrcode(_ string, code string) error {
	r.updates <- code
	return nil
}
func (r *qrLifecycleRepository) UpdateConnected(_ string, _ bool, _ string) error { return nil }

func TestQRRotationStopsWithExecution(t *testing.T) {
	s, ctx := testSessions(t)
	m, capture := editHandlerFixture(t, &whatsmeow.Client{Store: &store.Device{}})
	m.config.LogType = "json"
	repo := &qrLifecycleRepository{updates: make(chan string, 4)}
	m.instanceRepository = repo
	r, err := s.start(ctx, sessionData(editTestInstanceID), func(r *sessionRun) {
		m.run = r
		m.handleQRCodes([]string{"first", "second"})
		liveSession(r)
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-repo.updates:
		if code == "" {
			t.Fatal("missing QR")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-capture.done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := s.stop(ctx, editTestInstanceID, StopManual, "", nil); err != nil {
		t.Fatal(err)
	}
	if r.qrCount.Load() != 1 {
		t.Fatal("QR rotation survived cancellation")
	}
	select {
	case extra := <-repo.updates:
		t.Fatal("stale QR update", extra)
	default:
	}
}

func TestQRExhaustionDoesNotRestartOrInterruptPasskey(t *testing.T) {
	for _, activePasskey := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired", true: "passkey"}[activePasskey], func(t *testing.T) {
			s, ctx := testSessions(t)
			m, _ := editHandlerFixture(t, &whatsmeow.Client{Store: &store.Device{}})
			m.instanceRepository = &qrLifecycleRepository{updates: make(chan string, 4)}
			m.passkeyCeremony = ceremony.NewStore()
			if activePasskey {
				m.passkeyCeremony.Start(editTestInstanceID, []byte(`{}`))
			}
			r, err := s.start(ctx, sessionData(editTestInstanceID), func(r *sessionRun) {
				m.run = r
				m.handleQRCodes(nil)
				liveSession(r)
			})
			if err != nil {
				t.Fatal(err)
			}
			<-r.ready
			// Startup has finished adding workers; QR has no rotation timer here.
			r.workers.Wait()
			if activePasskey {
				if r.ctx.Err() != nil {
					t.Fatal("active passkey was interrupted")
				}
				if err := s.stop(ctx, editTestInstanceID, StopManual, "", nil); err != nil {
					t.Fatal(err)
				}
			} else {
				select {
				case <-r.done:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				w := whatsmeowService{sessions: s}
				if !w.PairingExpired(editTestInstanceID) || r.current() {
					t.Fatal("expired session remained active")
				}
			}
		})
	}
}
