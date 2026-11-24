package user_handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
)

func TestWriteUserWAError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{
			name:       "rate overlimit",
			err:        fmt.Errorf("usync: %w", whatsmeow.ErrIQRateOverLimit),
			wantStatus: http.StatusTooManyRequests,
			wantError:  "WhatsApp rate limit",
		},
		{
			name:       "private picture",
			err:        fmt.Errorf("picture: %w", whatsmeow.ErrProfilePictureUnauthorized),
			wantStatus: http.StatusForbidden,
			wantError:  "profile picture unavailable",
		},
		{
			name:       "missing picture",
			err:        fmt.Errorf("picture: %w", whatsmeow.ErrProfilePictureNotSet),
			wantStatus: http.StatusNotFound,
			wantError:  "profile picture not set",
		},
		{
			name:       "iq timeout",
			err:        fmt.Errorf("avatar: %w", whatsmeow.ErrIQTimedOut),
			wantStatus: http.StatusGatewayTimeout,
			wantError:  "WhatsApp query timeout or cancellation",
		},
		{
			name:       "context deadline",
			err:        fmt.Errorf("avatar: %w", context.DeadlineExceeded),
			wantStatus: http.StatusGatewayTimeout,
			wantError:  "WhatsApp query timeout or cancellation",
		},
		{
			name:       "context canceled",
			err:        fmt.Errorf("avatar: %w", context.Canceled),
			wantStatus: http.StatusGatewayTimeout,
			wantError:  "WhatsApp query timeout or cancellation",
		},
		{
			name:       "other error",
			err:        errors.New("postgres://user:private-password@database"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "failed to query WhatsApp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			writeUserWAError(c, tt.err)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			if body["error"] != tt.wantError {
				t.Fatalf("error = %q, want %q", body["error"], tt.wantError)
			}
			if strings.Contains(w.Body.String(), "postgres") || strings.Contains(w.Body.String(), "private") {
				t.Fatalf("leaked internal detail: %s", w.Body.String())
			}
			if tt.wantStatus == http.StatusTooManyRequests {
				if got := w.Header().Get("Retry-After"); got == "" {
					t.Fatal("missing Retry-After")
				}
			}
		})
	}
}
