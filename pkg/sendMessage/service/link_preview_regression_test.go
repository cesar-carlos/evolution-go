package send_service

import (
	"context"
	"errors"
	"go.mau.fi/whatsmeow"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPreviewRelativeImageAfterRedirect(t *testing.T) {
	r := http.NewServeMux()
	r.HandleFunc("/short", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/shop/item", http.StatusFound) })
	r.HandleFunc("/shop/item", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<title>Fallback</title><meta property="og:title" content="Actual"><meta property="og:image" content="photos/a.jpg">`))
	})
	server := httptest.NewServer(r)
	defer server.Close()
	client := newLinkPreviewHTTPClient(true)
	defer client.CloseIdleConnections()
	title, _, image, err := fetchLinkMetadata(context.Background(), client, server.URL+"/short")
	if err != nil || title != "Actual" || image != server.URL+"/shop/photos/a.jpg" {
		t.Fatalf("bad redirect metadata: %q %q %v", title, image, err)
	}
}

func TestPreviewCancellationAndNetworkPolicy(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	defer server.Close()
	blocked := newLinkPreviewHTTPClient(false)
	defer blocked.CloseIdleConnections()
	if _, _, err := fetchLinkPreviewResource(context.Background(), blocked, server.URL, 1024); err == nil {
		t.Fatal("loopback allowed by default")
	}
	client := newLinkPreviewHTTPClient(true)
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, _, err := fetchLinkPreviewResource(ctx, client, server.URL, 1024); done <- err }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("server not reached")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	for _, address := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.168.0.1", "fc00::1", "fe80::1", "0.0.0.0"} {
		if publicPreviewIP(net.ParseIP(address)) {
			t.Fatalf("private IP allowed: %s", address)
		}
	}
	if !publicPreviewIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IP rejected")
	}
}

func TestPreviewFallbackUploadAndNewsletter(t *testing.T) {
	image := encodePNG(t, 80, 40)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image" {
			w.Write(image)
		} else {
			http.Error(w, "unavailable", 503)
		}
	}))
	defer server.Close()
	client := newLinkPreviewHTTPClient(true)
	defer client.CloseIdleConnections()
	input := &LinkStruct{Text: "original text", Url: server.URL, Title: "Caller", Description: "Description", ImgUrl: server.URL + "/image"}
	calls := 0
	upload := func(ctx context.Context, raw []byte) (whatsmeow.UploadResponse, error) {
		calls++
		return whatsmeow.UploadResponse{}, errors.New("upload failure")
	}
	msg, err := prepareLinkMessage(context.Background(), client, input, upload)
	if err == nil || calls != 1 || len(msg.ExtendedTextMessage.JPEGThumbnail) == 0 {
		t.Fatal("inline fallback lost")
	}
	if msg.ExtendedTextMessage.GetText() != input.Text || msg.ExtendedTextMessage.GetTitle() != "Caller" {
		t.Fatal("caller data lost")
	}
	input.Number = "channel@newsletter"
	calls = 0
	if _, err := prepareLinkMessage(context.Background(), client, input, upload); err != nil || calls != 0 {
		t.Fatal("newsletter encrypted upload attempted")
	}
	msg, err = prepareLinkMessage(context.Background(), client, &LinkStruct{Text: "text", Url: server.URL}, upload)
	if err == nil || msg.ExtendedTextMessage.GetText() != "text" {
		t.Fatal("metadata error lost text")
	}
}

func TestPreviewEXIFAndCancelledSend(t *testing.T) {
	raw := withEXIFOrientation(t, encodeJPEG(t, 80, 40), 6)
	prepared, err := prepareLinkPreviewImage(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertJPEG(t, prepared.HighQuality, 40, 80)
	assertJPEG(t, prepared.Inline, 40, 80)
	if prepared.Width != 40 || prepared.Height != 80 {
		t.Fatal("EXIF lost in preview")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// No dependencies: cancellation must return before session/fetch/send access.
	if _, err := new(sendService).SendLink(ctx, &LinkStruct{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled send continued: %v", err)
	}
	if err := waitSendDelay(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal("delay not cancellable")
	}
}
