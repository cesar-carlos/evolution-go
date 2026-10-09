// Sender is inspired by Evolution Go PR #182 (prakash-dev-code). Authentication,
// store access and resource ownership are implemented independently for this fork.
package sender_handler

import (
	"errors"
	"net/http"
	"net/url"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	sender_service "github.com/evolution-foundation/evolution-go/pkg/sender/service"
	"github.com/gin-gonic/gin"
)

type SenderHandler struct {
	service *sender_service.Service
	serveWS func(http.ResponseWriter, *http.Request, string)
}

func NewSenderHandler(service *sender_service.Service, serveWS func(http.ResponseWriter, *http.Request, string)) *SenderHandler {
	return &SenderHandler{service: service, serveWS: serveWS}
}

func (h *SenderHandler) Page(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' blob:; media-src 'self' blob:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	c.File("web/sender/index.html")
}

func authenticatedInstance(c *gin.Context) *instance_model.Instance {
	value, _ := c.Get("instance")
	instance, ok := value.(*instance_model.Instance)
	if !ok || instance == nil || instance.Id == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return nil
	}
	return instance
}

// Session returns only the authenticated instance, without credentials.
// @Summary Sender session
// @Description Uses the instance apikey header; never returns a token or global key.
// @Tags Sender
// @Produce json
// @Success 200 {object} sender_service.Session
// @Failure 401 {object} gin.H
// @Router /sender/session [get]
func (h *SenderHandler) Session(c *gin.Context) {
	if instance := authenticatedInstance(c); instance != nil {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, h.service.Session(instance))
	}
}

// ResolveLIDs maps identifiers using the authenticated session store.
// @Summary Resolve WhatsApp LIDs
// @Description Requires the instance apikey; uses its existing PostgreSQL or SQLite store. Missing mappings are omitted.
// @Tags Sender
// @Produce json
// @Param lids query string true "1-100 comma-separated numeric LIDs (up to 32 digits each)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 503 {object} gin.H
// @Router /sender/resolve-lids [get]
func (h *SenderHandler) ResolveLIDs(c *gin.Context) {
	instance := authenticatedInstance(c)
	if instance == nil {
		return
	}
	c.Header("Cache-Control", "no-store")
	result, err := h.service.ResolveLIDs(c.Request.Context(), instance.Id, c.Query("lids"))
	if errors.Is(err, sender_service.ErrInvalidLIDs) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LID mapping unavailable"})
	} else {
		c.JSON(http.StatusOK, result)
	}
}

// WebsocketAuth adapts the browser query token to the existing instance middleware.
// The selected instance comes exclusively from authentication, never a query ID.
func (h *SenderHandler) WebsocketAuth(auth gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin, err := url.Parse(c.GetHeader("Origin"))
		if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host != c.Request.Host {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid origin"})
			return
		}
		if c.Query("instanceId") != "" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Request.Header.Set("apikey", c.Query("token"))
		auth(c)
	}
}

// Websocket sends events for the authenticated instance only.
// @Summary Sender instance event stream
// @Description Same-origin WebSocket. Token is the instance key; query instanceId is rejected. Access logs must redact query tokens.
// @Tags Sender
// @Param token query string true "Instance API key"
// @Success 101 {string} string "WebSocket upgrade"
// @Failure 401 {object} gin.H
// @Failure 403 {object} gin.H
// @Router /sender/ws [get]
func (h *SenderHandler) Websocket(c *gin.Context) {
	if instance := authenticatedInstance(c); instance != nil {
		h.serveWS(c.Writer, c.Request, instance.Id)
	}
}
