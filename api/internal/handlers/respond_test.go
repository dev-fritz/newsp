package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/dtos"
)

type sampleRequest struct {
	Name  string `json:"name" validate:"required,max=10"`
	Email string `json:"email" validate:"omitempty,email"`
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantErr    bool
		wantFields map[string]string
	}{
		{name: "valid", body: `{"name":"ana","email":"ana@example.com"}`},
		{name: "empty body", body: ``, wantErr: true},
		{name: "malformed", body: `{"name":`, wantErr: true},
		{name: "unknown field", body: `{"name":"ana","role":"admin"}`, wantErr: true},
		{name: "two objects", body: `{"name":"a"}{"name":"b"}`, wantErr: true},
		{name: "validation", body: `{"name":"","email":"nope"}`, wantErr: true, wantFields: map[string]string{"name": "required", "email": "email"}},
		{name: "too large", body: `{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			var dst sampleRequest
			err := decodeJSON(w, r, &dst)
			if !tt.wantErr {
				require.NoError(t, err)
				assert.Equal(t, "ana", dst.Name)
				return
			}
			appErr, ok := apperrors.As(err)
			require.True(t, ok)
			assert.Equal(t, apperrors.KindValidation, appErr.Kind)
			if tt.wantFields != nil {
				assert.Equal(t, tt.wantFields, appErr.Fields)
			}
		})
	}
}

func TestRespondError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "not found", err: apperrors.NotFound("IDEA_NOT_FOUND", "idea not found"), wantStatus: http.StatusNotFound, wantCode: "IDEA_NOT_FOUND"},
		{name: "conflict", err: apperrors.Conflict("EMAIL_TAKEN", "email taken"), wantStatus: http.StatusConflict, wantCode: "EMAIL_TAKEN"},
		{name: "wrapped app error", err: errors.Join(errors.New("ctx"), apperrors.Forbidden("FORBIDDEN", "no")), wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "unknown error hides details", err: errors.New("pq: relation users does not exist"), wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			respondError(w, httptest.NewRequest(http.MethodGet, "/", nil), tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)
			var body dtos.ErrorResponse
			require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
			assert.Equal(t, tt.wantCode, body.Error.Code)
			assert.NotContains(t, body.Error.Message, "relation")
		})
	}
}
