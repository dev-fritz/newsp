// Package handlers is the HTTP layer: it decodes requests, calls services
// and maps their results and errors to responses.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/dtos"
)

// maxBodyBytes caps every JSON request body.
const maxBodyBytes = 1 << 20

// validate is stateless after setup and safe for concurrent use.
var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Report field errors with their JSON names, which is what clients send.
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			return ""
		}
		return name
	})
	return v
}

type dataEnvelope struct {
	Data any `json:"data"`
}

type pageEnvelope struct {
	Data any           `json:"data"`
	Meta dtos.PageMeta `json:"meta"`
}

// respondJSON writes data wrapped in the {"data": ...} envelope.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dataEnvelope{Data: data})
}

// respondPage writes a paginated list with its metadata.
func respondPage(w http.ResponseWriter, data any, meta dtos.PageMeta) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(pageEnvelope{Data: data, Meta: meta})
}

// respondNoContent writes a 204.
func respondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// respondAccepted writes a 202 without a body.
func respondAccepted(w http.ResponseWriter) {
	w.WriteHeader(http.StatusAccepted)
}

// respondError maps err to a status code and a safe error body. Unknown
// errors are logged and answered with a generic 500.
func respondError(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := apperrors.As(err)
	if !ok {
		if errors.Is(err, context.Canceled) {
			slog.DebugContext(r.Context(), "request canceled by client", "request_id", middleware.GetReqID(r.Context()))
			return
		}
		appErr = &apperrors.Error{Kind: apperrors.KindInternal, Code: "INTERNAL_ERROR", Message: "internal server error", Err: err}
	}

	status := statusFor(appErr.Kind)
	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed",
			"request_id", middleware.GetReqID(r.Context()),
			"code", appErr.Code,
			"error", err,
		)
	}

	if appErr.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(appErr.RetryAfter.Seconds()))))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dtos.ErrorResponse{Error: dtos.ErrorBody{
		Code:    appErr.Code,
		Message: appErr.Message,
		Fields:  appErr.Fields,
	}})
}

func statusFor(kind apperrors.Kind) int {
	switch kind {
	case apperrors.KindValidation:
		return http.StatusBadRequest
	case apperrors.KindUnauthorized:
		return http.StatusUnauthorized
	case apperrors.KindForbidden:
		return http.StatusForbidden
	case apperrors.KindNotFound:
		return http.StatusNotFound
	case apperrors.KindConflict:
		return http.StatusConflict
	case apperrors.KindUnprocessable:
		return http.StatusUnprocessableEntity
	case apperrors.KindRateLimited:
		return http.StatusTooManyRequests
	case apperrors.KindUnavailable:
		return http.StatusServiceUnavailable
	case apperrors.KindExternal:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// decodeJSON reads a size-limited, strict JSON body into dst and validates it.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return apperrors.Validation("request body too large", nil)
		case errors.Is(err, io.EOF):
			return apperrors.Validation("request body is empty", nil)
		default:
			return apperrors.Validation("malformed JSON body", nil)
		}
	}
	if dec.More() {
		return apperrors.Validation("request body must contain a single JSON object", nil)
	}
	return validateStruct(dst)
}

// validateStruct runs the validate tags and converts failures to field errors.
func validateStruct(v any) error {
	err := validate.Struct(v)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return apperrors.Validation("invalid request", nil)
	}
	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		fields[fieldPath(fe)] = fe.Tag()
	}
	return apperrors.Validation("invalid request", fields)
}

// fieldPath drops the top-level struct name from the namespace ("Req.items[0].url" -> "items[0].url").
func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if _, rest, ok := strings.Cut(ns, "."); ok {
		return rest
	}
	return ns
}
