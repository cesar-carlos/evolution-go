package whatsmeow_service

import (
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	nats_producer "github.com/evolution-foundation/evolution-go/pkg/events/nats"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
)

func TestNATSUnavailableDoesNotSuppressOtherDestinations(t *testing.T) {
	for _, mode := range []string{"enabled", "true"} {
		t.Run(mode, func(t *testing.T) {
			logs := logger_wrapper.NewLoggerManager(&config.Config{LogDirectory: t.TempDir(), LogMaxSize: 1})
			t.Cleanup(func() {
				if err := logs.GetLogger("owner").Close(); err != nil {
					t.Error(err)
				}
			})
			producer := nats_producer.NewNatsProducer("", false, nil, logs)
			t.Cleanup(producer.Close)
			ws, webhook := &recordingEventProducer{}, &recordingEventProducer{}
			service := &whatsmeowService{natsProducer: producer, websocketProducer: ws, webhookProducer: webhook, loggerWrapper: logs}
			instance := &instance_model.Instance{Id: "owner", NatsEnable: mode, WebSocketEnable: "enabled", Webhook: "https://example.invalid", Events: "BUTTON_CLICK"}
			payload := []byte(`{"event":"ButtonClick","data":{"buttonId":"choice"}}`)
			service.CallWebhook(instance, "owner.buttonclick", payload)
			if len(ws.deliveries) != 1 || len(webhook.deliveries) != 1 {
				t.Fatalf("independent destinations suppressed: ws=%d webhook=%d", len(ws.deliveries), len(webhook.deliveries))
			}
			for _, got := range []recordedEventDelivery{ws.deliveries[0], webhook.deliveries[0]} {
				if got.queue != "owner.buttonclick" || got.payload != string(payload) {
					t.Fatalf("event changed: %+v", got)
				}
			}
		})
	}
}
