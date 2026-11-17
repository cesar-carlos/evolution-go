package user_handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	user_service "github.com/evolution-foundation/evolution-go/pkg/user/service"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow/types"
)

type blocklistServiceFake struct {
	user_service.UserService
	get func(*instance_model.Instance) (*types.Blocklist, error)
}

func (f blocklistServiceFake) BlockContact(_ *user_service.BlockStruct, instance *instance_model.Instance) (*types.Blocklist, error) {
	return f.get(instance)
}

func (f blocklistServiceFake) UnlockContact(_ *user_service.BlockStruct, instance *instance_model.Instance) (*types.Blocklist, error) {
	return f.get(instance)
}

func (f blocklistServiceFake) GetBlockList(instance *instance_model.Instance) (*types.Blocklist, error) {
	return f.get(instance)
}

func TestBlocklistResponsePreservesLIDPhoneAndActive(t *testing.T) {
	instance := &instance_model.Instance{Id: "authenticated"}
	fake := blocklistServiceFake{get: func(got *instance_model.Instance) (*types.Blocklist, error) {
		if got != instance {
			t.Fatal("blocklist used another instance")
		}
		return &types.Blocklist{
			AddressingMode: types.AddressingModeLID, DHash: "hash",
			Items: []types.BlocklistItem{{LID: types.NewJID("123", types.HiddenUserServer), PN: types.NewJID("5511999999999", types.DefaultUserServer), Active: true}},
		}, nil
	}}
	h := &userHandler{userService: fake}
	r := gin.New()
	r.Use(func(ctx *gin.Context) { ctx.Set("instance", instance) })
	r.POST("/user/block", h.BlockContact)
	r.POST("/user/unblock", h.UnblockContact)
	r.GET("/user/blocklist", h.GetBlockList)
	for _, path := range []string{"/user/block", "/user/unblock", "/user/blocklist"} {
		t.Run(path, func(t *testing.T) {
			method := http.MethodPost
			if path == "/user/blocklist" {
				method = http.MethodGet
			}
			req := httptest.NewRequest(method, path, strings.NewReader(`{"number":"5511999999999"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d: %s", w.Code, w.Body.String())
			}
			var got map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			data := got["data"].(map[string]any)
			items := data["Items"].([]any)
			item := items[0].(map[string]any)
			if got["message"] != "success" || data["DHash"] != "hash" || data["AddressingMode"] != "lid" || len(items) != 1 || item["LID"] != "123@lid" || item["PN"] != "5511999999999@s.whatsapp.net" || item["Active"] != true {
				t.Fatalf("lost blocklist metadata: %s", w.Body.String())
			}
			if _, exists := data["JIDs"]; exists {
				t.Fatal("obsolete JIDs field present")
			}
		})
	}
}
