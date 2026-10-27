package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// GenerateJSON asks for JSON matching req.Schema, decodes it into dst and runs
// validate (may be nil). Local models do not always honor the schema, so one
// failed attempt is retried with the error fed back to the model. The usage of
// every attempt is returned so callers can account for it.
func GenerateJSON(ctx context.Context, p Provider, req ChatRequest, dst any, validate func() error) (Usage, error) {
	if req.Schema == nil {
		return Usage{}, errors.New("GenerateJSON needs a schema")
	}
	var total Usage
	for attempt := 1; ; attempt++ {
		res, err := p.Chat(ctx, req)
		total.InputTokens += res.Usage.InputTokens
		total.OutputTokens += res.Usage.OutputTokens
		if err != nil {
			return total, err
		}

		problem := decode(res, dst, validate)
		if problem == nil {
			return total, nil
		}
		if attempt == 2 {
			return total, fmt.Errorf("model output did not match the schema: %w", problem)
		}
		req.Messages = append(append([]Message{}, req.Messages...),
			Message{Role: RoleAssistant, Content: res.Text},
			Message{Role: RoleUser, Content: "Your previous answer was rejected: " + problem.Error() +
				". Reply again with only the corrected JSON object, matching the schema exactly."},
		)
	}
}

func decode(res ChatResponse, dst any, validate func() error) error {
	if res.Truncated {
		return errors.New("the answer was cut off at the token limit; be more concise")
	}
	text := stripFences(res.Text)
	if err := json.Unmarshal([]byte(text), dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if validate != nil {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

// stripFences removes Markdown code fences some models wrap around JSON.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}
