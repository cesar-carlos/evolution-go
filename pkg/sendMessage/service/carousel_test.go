package send_service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
)

type carouselUploadFunc func(context.Context, []byte, whatsmeow.MediaType) (whatsmeow.UploadResponse, error)

func (f carouselUploadFunc) Upload(ctx context.Context, b []byte, k whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	return f(ctx, b, k)
}

type carouselSessionStarter struct {
	whatsmeow_service.WhatsmeowService
	cancel     context.CancelFunc
	instanceID string
}

func (*carouselSessionStarter) GetClient(string) *whatsmeow.Client { return nil }

func (s *carouselSessionStarter) StartInstanceContext(ctx context.Context, id string) error {
	s.instanceID = id
	s.cancel()
	return ctx.Err()
}

func TestCarouselSessionStartUsesRequestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sessions := &carouselSessionStarter{cancel: cancel}
	service := &sendService{whatsmeowService: sessions}
	msg, err := service.SendCarousel(ctx, carouselInput(), &instance_model.Instance{Id: "authenticated"})
	if msg != nil || !errors.Is(err, context.Canceled) || sessions.instanceID != "authenticated" {
		t.Fatalf("session cancellation/identity lost: %v, %v, %q", msg, err, sessions.instanceID)
	}
}

func carouselInput() *CarouselStruct {
	return &CarouselStruct{Number: "123", Body: "Main", Footer: "Footer", Cards: []CarouselCardStruct{{Body: CarouselCardBodyStruct{Text: "Card"}, Header: CarouselCardHeaderStruct{Title: "Title", Subtitle: "Subtitle"}}}}
}

func TestCarouselButtonContracts(t *testing.T) {
	for _, tt := range []struct {
		button           CarouselButtonStruct
		name, key, value string
	}{
		{CarouselButtonStruct{Id: "reply\"\\"}, "quick_reply", "id", "reply\"\\"},
		{CarouselButtonStruct{Type: "URL", Id: "https://example.org"}, "cta_url", "url", "https://example.org"},
		{CarouselButtonStruct{Type: "url", URL: "https://example.org"}, "cta_url", "url", "https://example.org"},
		{CarouselButtonStruct{Type: "CALL", Id: "+123"}, "cta_call", "phone_number", "+123"},
		{CarouselButtonStruct{Type: "call", PhoneNumber: "+123"}, "cta_call", "phone_number", "+123"},
		{CarouselButtonStruct{Type: "COPY", CopyCode: "code\"\\"}, "cta_copy", "copy_code", "code\"\\"},
		{CarouselButtonStruct{Type: "COPY_CODE", CopyCode: "code"}, "cta_copy", "copy_code", "code"},
	} {
		t.Run(tt.name+tt.button.Type, func(t *testing.T) {
			data := carouselInput()
			tt.button.DisplayText = "Label\"\\"
			data.Cards[0].Buttons = []CarouselButtonStruct{tt.button}
			msg, err := buildCarouselMessage(context.Background(), data, nil)
			if err != nil {
				t.Fatal(err)
			}
			interactive := msg.GetInteractiveMessage()
			if interactive.GetBody().GetText() != "Main" || interactive.GetFooter().GetText() != "Footer" {
				t.Fatal("main text lost")
			}
			card := interactive.GetCarouselMessage().GetCards()[0]
			b := card.GetNativeFlowMessage().GetButtons()[0]
			var params map[string]string
			if err := json.Unmarshal([]byte(b.GetButtonParamsJSON()), &params); err != nil {
				t.Fatal(err)
			}
			if b.GetName() != tt.name || params[tt.key] != tt.value || params["display_text"] != tt.button.DisplayText {
				t.Fatalf("bad button: %v %v", b, params)
			}
		})
	}
	for _, b := range []CarouselButtonStruct{
		{Type: "PIX"}, {Type: "UNKNOWN"}, {Type: "URL", URL: "https://a.example", Id: "https://b.example"},
		{Type: "CALL", PhoneNumber: "123", Id: "456"}, {Type: "URL", URL: "javascript:bad"}, {Type: "COPY"}, {Type: "REPLY"},
	} {
		b.DisplayText = "Label"
		data := carouselInput()
		data.Cards[0].Buttons = []CarouselButtonStruct{b}
		if ValidateCarousel(data) == nil {
			t.Fatalf("accepted invalid button: %#v", b)
		}
	}
	data := carouselInput()
	data.Cards[0].Buttons = []CarouselButtonStruct{{DisplayText: "A", Id: "a"}, {Type: "URL", DisplayText: "B", URL: "https://example.org"}}
	if ValidateCarousel(data) == nil {
		t.Fatal("mixed buttons accepted")
	}
}

func TestCarouselMediaFailureAndCancellation(t *testing.T) {
	for _, mode := range []string{"http-error", "too-large", "upload-error", "cancel", "success"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "http-error" {
					w.WriteHeader(404)
					return
				}
				if mode == "too-large" {
					_, _ = io.Copy(w, io.LimitReader(strings.NewReader(strings.Repeat("x", (32<<20)+1)), (32<<20)+1))
					return
				}
				_, _ = w.Write([]byte("video data"))
			}))
			defer server.Close()
			data := carouselInput()
			data.Cards[0].Header.VideoUrl = server.URL
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			calls := 0
			uploadErr := errors.New("upload failed")
			uploader := carouselUploadFunc(func(ctx context.Context, b []byte, k whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
				calls++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 30*time.Second {
					t.Error("missing upload deadline")
				}
				if string(b) != "video data" || k != whatsmeow.MediaVideo {
					t.Error("incorrect upload")
				}
				if mode == "upload-error" {
					return whatsmeow.UploadResponse{}, uploadErr
				}
				return whatsmeow.UploadResponse{URL: "https://media.example/video"}, nil
			})
			msg, err := buildCarouselMessage(ctx, data, uploader)
			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}
				h := msg.GetInteractiveMessage().GetCarouselMessage().GetCards()[0].GetHeader()
				if h.GetTitle() != "Title" || h.GetSubtitle() != "Subtitle" || !h.GetHasMediaAttachment() || h.GetVideoMessage().GetURL() != "https://media.example/video" {
					t.Fatal("header lost")
				}
			} else if err == nil || msg != nil {
				t.Fatal("failure produced a message")
			}
			if mode == "upload-error" && !errors.Is(err, uploadErr) {
				t.Fatalf("upload error lost: %v", err)
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			if (mode == "http-error" || mode == "too-large" || mode == "cancel") && calls != 0 {
				t.Fatal("uploaded failed download")
			}
		})
	}
}

func TestCarouselCancelsInFlightMedia(t *testing.T) {
	for _, phase := range []string{"download", "upload"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if phase == "download" {
					cancel()
					<-r.Context().Done()
					return
				}
				_, _ = w.Write([]byte("media"))
			}))
			defer server.Close()
			uploader := carouselUploadFunc(func(ctx context.Context, b []byte, k whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
				if phase != "upload" {
					t.Error("unexpected upload")
				}
				cancel()
				<-ctx.Done()
				return whatsmeow.UploadResponse{}, ctx.Err()
			})
			data := carouselInput()
			data.Cards[0].Header.VideoUrl = server.URL
			msg, err := buildCarouselMessage(ctx, data, uploader)
			if msg != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled %s: %v, %v", phase, msg, err)
			}
		})
	}
}
