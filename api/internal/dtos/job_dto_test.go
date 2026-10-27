package dtos

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/models"
)

func TestNewJobResponse_ResultDetail(t *testing.T) {
	const withDetail = `{"detail":"dial tcp 192.168.0.10:11434: refused","report_id":"abc"}`
	tests := []struct {
		name      string
		result    string
		showError bool
		want      string
	}{
		{"member loses detail, keeps other keys", withDetail, false, `{"report_id":"abc"}`},
		{"admin keeps detail", withDetail, true, withDetail},
		{"member without detail unchanged", `{"report_id":"abc"}`, false, `{"report_id":"abc"}`},
		{"non-object unchanged", `[1,2]`, false, `[1,2]`},
		{"empty unchanged", ``, false, ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewJobResponse(models.Job{Result: json.RawMessage(tt.result)}, tt.showError)
			if tt.want == "" {
				assert.Empty(t, resp.Result)
				return
			}
			require.JSONEq(t, tt.want, string(resp.Result))
		})
	}
}
