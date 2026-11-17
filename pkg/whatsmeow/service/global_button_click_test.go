package whatsmeow_service

import (
	"errors"
	"github.com/evolution-foundation/evolution-go/pkg/config"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"testing"
)

func TestGlobalNATSButtonClick(t *testing.T) {
	for _, tt := range []struct {
		name                      string
		enabled, subscribed, fail bool
		want                      int
	}{
		{"enabled", true, true, false, 1}, {"disabled", false, true, false, 0},
		{"unsubscribed", true, false, false, 0}, {"producer failure", true, true, true, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{LogDirectory: t.TempDir(), LogMaxSize: 1, NatsGlobalEnabled: tt.enabled}
			if tt.subscribed {
				cfg.NatsGlobalEvents = []string{"BUTTON_CLICK"}
			} else {
				cfg.NatsGlobalEvents = []string{"MESSAGE"}
			}
			logs := logger_wrapper.NewLoggerManager(cfg)
			t.Cleanup(func() {
				if err := logs.GetLogger("owner").Close(); err != nil {
					t.Error(err)
				}
			})
			producer := &recordingEventProducer{}
			if tt.fail {
				producer.err = errors.New("broker unavailable")
			}
			service := &whatsmeowService{config: cfg, loggerWrapper: logs, natsProducer: producer}
			payload := []byte(`{"event":"ButtonClick","buttonId":"choice"}`)
			service.SendToGlobalQueues("ButtonClick", payload, "owner")
			if len(producer.deliveries) != tt.want {
				t.Fatalf("deliveries = %v", producer.deliveries)
			}
			if tt.want == 1 && producer.deliveries[0] != (recordedEventDelivery{"buttonclick", string(payload), "global", "owner"}) {
				t.Fatal("event contract changed")
			}
		})
	}
}
