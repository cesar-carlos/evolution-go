package logger

import "testing"

func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "webhook query token",
			in:   "webhook sent successfully - url: https://chat.example.com/webhooks/evolution_go/X?token=56d83faecdbdd3756de957d5d68d57e5, status: 200",
			want: "webhook sent successfully - url: https://chat.example.com/webhooks/evolution_go/X?token=***, status: 200",
		},
		{
			name: "postgres dsn",
			in:   "Connecting to database on: postgresql://evogo:PgEv0g0_secret@postgres:5432/evogo_users?sslmode=disable",
			want: "Connecting to database on: postgresql://evogo:***@postgres:5432/evogo_users?sslmode=disable",
		},
		{
			name: "amqp url",
			in:   "Starting RabbitMQ reconnection process with URL: amqp://admin:admin@localhost:5672/default",
			want: "Starting RabbitMQ reconnection process with URL: amqp://admin:***@localhost:5672/default",
		},
		{
			name: "apikey header style",
			in:   "Headers: apikey=e69e464b-bcb5-446a-a22c-1f39dc7a5e1d",
			want: "Headers: apikey=***",
		},
		{
			name: "bearer",
			in:   "Authorization: Bearer abcdef.GHI.123",
			want: "Authorization: Bearer ***",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "plain message untouched",
			in:   "[id] Client successfully validated - Connected: true",
			want: "[id] Client successfully validated - Connected: true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSensitive(tt.in)
			if got != tt.want {
				t.Fatalf("MaskSensitive()\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}
