// Package dtos holds the request and response contracts of the HTTP API.
package dtos

// ErrorBody is the payload of every error response.
type ErrorBody struct {
	Code    string            `json:"code" example:"VALIDATION_FAILED"`
	Message string            `json:"message" example:"invalid request"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// ErrorResponse is the envelope of every error response.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// HealthStatus is returned by the health endpoints.
type HealthStatus struct {
	Status string `json:"status" example:"ok"`
}

// HealthEnvelope wraps HealthStatus in the standard data envelope.
type HealthEnvelope struct {
	Data HealthStatus `json:"data"`
}
