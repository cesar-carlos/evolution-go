package whatsmeow_service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"

	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestParseWhatsAppWebVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    int
		ok      bool
	}{
		{
			name:    "escaped service worker",
			content: `\"SiteData\":{\"server_revision\":111,\"client_revision\":1052855845,\"push_phase\":\"C3\"}`,
			want:    1052855845,
			ok:      true,
		},
		{
			name:    "plain json",
			content: `{"client_revision": 1052556849}`,
			want:    1052556849,
			ok:      true,
		},
		{
			name:    "missing revision",
			content: `{"server_revision": 1052855845}`,
			ok:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseWhatsAppWebVersion(tt.content)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got.Major != 2 || got.Minor != 3000 || got.Patch != tt.want {
				t.Fatalf("version = %d.%d.%d, want 2.3000.%d", got.Major, got.Minor, got.Patch, tt.want)
			}
		})
	}
}

func TestWhatsAppWebVersionFromErrorPageHeader(t *testing.T) {
	t.Parallel()
	header := make(http.Header)
	header.Add("Reporting-Endpoints", `default="https://web.whatsapp.com/ajax/whatsapp_error_reports/?cpp=C3&cv=1052855845&st=1"`)
	version, ok := whatsAppWebVersionFromFetch(header, "<!DOCTYPE html><title>Error</title>")
	if !ok {
		t.Fatal("header revision was ignored")
	}
	if version.Major != 2 || version.Minor != 3000 || version.Patch != 1052855845 {
		t.Fatalf("version = %d.%d.%d", version.Major, version.Minor, version.Patch)
	}
}

func TestApplyWhatsAppClientVersionUpdatesHandshake(t *testing.T) {
	previous := store.GetWAVersion()
	t.Cleanup(func() {
		store.SetWAVersion(previous)
	})

	deviceProps := proto.Clone(store.DeviceProps).(*waCompanionReg.DeviceProps)
	applyWhatsAppClientVersion(deviceProps, clientVersion{Major: 2, Minor: 3000, Patch: 1052855845})

	if got := store.GetWAVersion().String(); got != "2.3000.1052855845" {
		t.Fatalf("handshake version = %s", got)
	}
	appVersion := store.BaseClientPayload.GetUserAgent().GetAppVersion()
	if appVersion.GetPrimary() != 2 || appVersion.GetSecondary() != 3000 || appVersion.GetTertiary() != 1052855845 {
		t.Fatalf("payload version = %d.%d.%d", appVersion.GetPrimary(), appVersion.GetSecondary(), appVersion.GetTertiary())
	}
	if deviceProps.GetVersion().GetPrimary() != 2 || deviceProps.GetVersion().GetTertiary() != 1052855845 {
		t.Fatalf("device props version = %d.%d.%d", deviceProps.GetVersion().GetPrimary(), deviceProps.GetVersion().GetSecondary(), deviceProps.GetVersion().GetTertiary())
	}
}

func TestArchiveEventDataIsJSONSafe(t *testing.T) {
	t.Parallel()
	jid, err := types.ParseJID("5511999999999@s.whatsapp.net")
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	payload := archiveEventData(&events.Archive{
		JID:          jid,
		Timestamp:    when,
		FromFullSync: true,
	})
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"JID":"5511999999999@s.whatsapp.net"`)) {
		t.Fatalf("payload = %s", body)
	}
	if !bytes.Contains(body, []byte(`"FromFullSync":true`)) {
		t.Fatalf("payload = %s", body)
	}
}

func TestEnsureWhatsAppClientVersionPublishesOnce(t *testing.T) {
	previous := store.GetWAVersion()
	resetPublishedWhatsAppClientVersion()
	t.Cleanup(func() {
		resetPublishedWhatsAppClientVersion()
		store.SetWAVersion(previous)
	})

	service := whatsmeowService{config: &config.Config{
		WhatsappVersionMajor: 2,
		WhatsappVersionMinor: 3000,
		WhatsappVersionPatch: 1052556849,
	}}
	version, source, err := service.ensureWhatsAppClientVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if source != "config" || version.Patch != 1052556849 {
		t.Fatalf("version %d from %s", version.Patch, source)
	}
	if got := store.GetWAVersion().String(); got != "2.3000.1052556849" {
		t.Fatalf("handshake = %s", got)
	}

	service.config.WhatsappVersionPatch = 1
	version, source, err = service.ensureWhatsAppClientVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version.Patch != 1052556849 || source != "config" {
		t.Fatalf("second call changed the published version to %d from %s", version.Patch, source)
	}
}

func TestApplyWhatsAppClientVersionIgnoresZero(t *testing.T) {
	if (clientVersion{}).isZero() != true {
		t.Fatal("zero version was not recognized")
	}
	if (clientVersion{Patch: 1}).isZero() {
		t.Fatal("non-zero version was treated as zero")
	}
}
