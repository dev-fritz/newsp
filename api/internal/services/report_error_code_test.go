package services

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dev-fritz/newsp/api/pkg/ai"
)

func TestAIErrorCodeAndRetry(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantCode  string
		wantRetry bool
	}{
		{"401 rejects key", &ai.APIError{Status: 401, Message: "bad key"}, ReportErrKeyRejected, false},
		{"403 rejects key", fmt.Errorf("wrap: %w", &ai.APIError{Status: 403}), ReportErrKeyRejected, false},
		{"gemini invalid key is a 400 with a reason", &ai.APIError{Status: 400, Reason: "API_KEY_INVALID"}, ReportErrKeyRejected, false},
		{"plain 400 is failure", &ai.APIError{Status: 400, Message: "bad request"}, ReportErrAIFailure, false},
		{"500 is failure", &ai.APIError{Status: 500}, ReportErrAIFailure, true},
		{"no model", ErrModelUnavailable, ReportErrNoModel, false},
		{"plain error", errors.New("boom"), ReportErrAIFailure, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := aiErrorCode(tt.err)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantRetry, retryableReportError(code, tt.err))
		})
	}
}
