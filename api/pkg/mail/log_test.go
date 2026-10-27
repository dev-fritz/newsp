package mail

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogSend(t *testing.T) {
	t.Parallel()
	msg := Message{To: "ana@example.org", Subject: "Verify", Text: "link: https://x.test/verify?token=abc"}

	tests := []struct {
		name        string
		includeBody bool
	}{
		{"without body", false},
		{"with body", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

			require.NoError(t, NewLog(logger, tt.includeBody).Send(t.Context(), msg))

			out := buf.String()
			assert.Contains(t, out, "level=INFO")
			assert.Contains(t, out, "to=ana@example.org")
			assert.Contains(t, out, "subject=Verify")
			if tt.includeBody {
				assert.Contains(t, out, "token=abc")
			} else {
				assert.NotContains(t, out, "token=abc")
			}
		})
	}
}
