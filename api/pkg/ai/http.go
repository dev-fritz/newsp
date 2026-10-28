package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxResponseBytes bounds provider responses (embedding batches are the largest).
const maxResponseBytes = 64 << 20

// doJSON sends body as JSON and decodes a 2xx JSON response into dst.
func doJSON(ctx context.Context, client *http.Client, kind Kind, method, url string, header http.Header, body, dst any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range header {
		req.Header[k] = v
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s request: %w", kind, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("%s read response: %w", kind, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return &APIError{Provider: kind, Status: res.StatusCode, Message: errorMessage(raw), Reason: errorReason(raw)}
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%s decode response: %w", kind, err)
	}
	return nil
}

// errorReason extracts Google's machine-readable reason
// ({"error":{"details":[{"reason":...}]}}): the first non-empty one, else "".
func errorReason(raw []byte) string {
	var shaped struct {
		Error struct {
			Details []struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &shaped) != nil {
		return ""
	}
	for _, d := range shaped.Error.Details {
		if d.Reason != "" {
			return d.Reason
		}
	}
	return ""
}

// errorMessage extracts a readable message from the common error shapes
// ({"error":{"message":...}} or {"error":"..."}), never echoing huge bodies.
func errorMessage(raw []byte) string {
	var shaped struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(raw, &shaped) == nil && len(shaped.Error) > 0 {
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(shaped.Error, &obj) == nil && obj.Message != "" {
			return obj.Message
		}
		var s string
		if json.Unmarshal(shaped.Error, &s) == nil && s != "" {
			return s
		}
	}
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return msg
}
