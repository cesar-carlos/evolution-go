package sender_handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	auth_middleware "github.com/evolution-foundation/evolution-go/pkg/middleware"
	sender_service "github.com/evolution-foundation/evolution-go/pkg/sender/service"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
)

type testInstances struct {
	instance_service.InstanceService
}

func (testInstances) GetInstanceByToken(token string) (*instance_model.Instance, error) {
	if token != "a" && token != "b" {
		return nil, errors.New("invalid")
	}
	return &instance_model.Instance{Id: token, Name: "instance-" + token, Token: token}, nil
}

type testClients struct{}

func (testClients) GetClient(string) *whatsmeow.Client { return nil }

func TestAuthenticationIsolationAndOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSenderHandler(sender_service.NewService(testClients{}), func(w http.ResponseWriter, r *http.Request, id string) { w.Write([]byte(id)) })
	auth := auth_middleware.NewMiddleware(&config.Config{GlobalApiKey: "global-secret"}, testInstances{})
	r := gin.New()
	r.GET("/sender/session", auth.Auth, h.Session)
	r.GET("/sender/resolve-lids", auth.Auth, h.ResolveLIDs)
	r.GET("/sender/ws", h.WebsocketAuth(auth.Auth), h.Websocket)
	for _, tt := range []struct {
		path, token, origin, body string
		code                      int
	}{
		{"/sender/session", "", "", "", 401},
		{"/sender/session", "global-secret", "", "", 401},
		{"/sender/session?instanceId=b", "a", "", "instance-a", 200},
		{"/sender/session", "b", "", "instance-b", 200},
		{"/sender/resolve-lids?lids=abc", "a", "", "", 400},
		{"/sender/resolve-lids?lids=12345", "a", "", "{}", 200},
		{"/sender/ws?token=a", "", "http://example.com", "a", 200},
		{"/sender/ws?token=a&instanceId=b", "", "http://example.com", "", 400},
		{"/sender/ws?token=global-secret", "", "http://example.com", "", 401},
		{"/sender/ws?token=a", "", "https://evil.example", "", 403},
		{"/sender/ws?token=a", "", "", "", 403},
	} {
		req := httptest.NewRequest("GET", tt.path, nil)
		req.Header.Set("apikey", tt.token)
		req.Header.Set("Origin", tt.origin)
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, req)
		if recorder.Code != tt.code || !strings.Contains(recorder.Body.String(), tt.body) {
			t.Errorf("%s: %d %s", tt.path, recorder.Code, recorder.Body)
		}
		if strings.Contains(recorder.Body.String(), "global-secret") || strings.Contains(recorder.Body.String(), `"token"`) {
			t.Fatal("credential disclosed")
		}
	}
}

func TestPageNeverEmbedsCredentialsForLoopbackOrProxy(t *testing.T) {
	t.Chdir("../../..")
	t.Setenv("GLOBAL_API_KEY", "must-never-appear-in-html")
	r := gin.New()
	r.GET("/sender", (&SenderHandler{}).Page)
	for _, remote := range []string{"127.0.0.1:4000", "[::1]:4000", "192.0.2.1:4000"} {
		req := httptest.NewRequest("GET", "/sender", nil)
		req.RemoteAddr = remote
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		req.Header.Set("apikey", "must-never-appear-in-html")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 || strings.Contains(rec.Body.String(), "must-never-appear-in-html") || strings.Contains(rec.Body.String(), "__SENDER_BOOTSTRAP__") || strings.Contains(rec.Body.String(), "__EVO_BOOTSTRAP__") {
			t.Fatal(rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("missing browser policy")
		}
	}
}
