package nats_producer

import (
	"errors"
	"fmt"
	"strings"
	"time"

	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/gomessguii/logger"
	"github.com/nats-io/nats.go"
)

const connectionTimeout = 2 * time.Second

// ErrUnavailable means this attempt was not published because no live connection exists.
var ErrUnavailable = errors.New("NATS connection unavailable")

type natsProducer struct {
	conn              *nats.Conn
	natsGlobalEnabled bool
}

// NewNatsProducer owns a bounded reconnect lifecycle. The caller must Close it.
// An empty URL is deliberately inert: never fall back to localhost.
func NewNatsProducer(url string, globalEnabled bool, _ []string, _ *logger_wrapper.LoggerManager) *natsProducer {
	p, err := newNatsProducer(url, globalEnabled)
	if err != nil {
		// Connection errors may contain a credential-bearing URL. Never log it.
		logger.LogError("NATS initialization failed; check connection configuration")
	}
	return p
}

func newNatsProducer(url string, globalEnabled bool, options ...nats.Option) (*natsProducer, error) {
	p := &natsProducer{natsGlobalEnabled: globalEnabled}
	if strings.TrimSpace(url) == "" {
		return p, nil
	}
	opts := []nats.Option{
		nats.Timeout(connectionTimeout),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(60),
		nats.ReconnectWait(2 * time.Second),
		// Do not queue failed attempts for delayed delivery during reconnect.
		nats.ReconnectBufSize(-1),
	}
	opts = append(opts, options...)
	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return p, err
	}
	p.conn = conn
	return p, nil
}

func (p *natsProducer) Produce(subject string, payload []byte, mode string, _ string) error {
	switch mode {
	case "global":
		if !p.natsGlobalEnabled {
			return nil
		}
	case "enabled", "true":
	default:
		return nil
	}
	if p.conn == nil || !p.conn.IsConnected() {
		return ErrUnavailable
	}
	if err := p.conn.Publish(subject, payload); err != nil {
		return fmt.Errorf("NATS publish: %w", err)
	}
	// Confirm a server round trip, not subscriber processing or durable storage.
	// A timeout after Publish is uncertain: never retry this payload automatically.
	if err := p.conn.FlushTimeout(connectionTimeout); err != nil {
		return fmt.Errorf("NATS flush (delivery uncertain): %w", err)
	}
	return nil
}

// Close stops retries and releases the connection; it is safe to call repeatedly.
func (p *natsProducer) Close() {
	if p.conn != nil {
		p.conn.Close()
	}
}

// CreateGlobalQueues is a no-op: Core NATS subjects need no provisioning.
func (p *natsProducer) CreateGlobalQueues() error { return nil }
