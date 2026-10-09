package send_handler

import (
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInteractiveValidationBeforeSending(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct{ path, body string }{
		{"/button", "null"}, {"/list", "null"},
		{"/button", `{"number":"1","title":"t","description":"d","footer":"f","buttons":[{"type":"unknown"}]}`},
		{"/list", `{"number":"1","title":"t","description":"d","footerText":"f","sections":[{"rows":[{"rowId":"x"},{"rowId":"x"}]}]}`},
	} {
		t.Run(tt.path+tt.body, func(t *testing.T) {
			h := &sendHandler{}
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set("instance", &instance_model.Instance{Id: "authenticated"}) })
			r.POST("/button", h.SendButton)
			r.POST("/list", h.SendList)
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
