// Package typesafe is a minimal client for TypeSafe's System One API
// (https://docs.typesafe.ai/api). There is no official Go SDK.
package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// System One endpoint and default model.
const (
	Endpoint = "https://api.typesafe.ai/v1/systemone"
	Model    = "jev-latest"
)

// Question is one typed TypeSafe question. Criteria is a map of option
// descriptions for a choice, or {"true":..., "false":...} for a noul.
type Question struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// Answer holds the fields of a choice or noul answer.
type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
	Noul          float64            `json:"noul"`
}

type request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

type response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
}

// Client calls the System One endpoint.
type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: &http.Client{Timeout: 5 * time.Second}}
}

// Ask sends one state with its questions and returns the answers by id.
// 429 and 529 are retried once after a short pause.
func (c *Client) Ask(ctx context.Context, state any, questions map[string]Question) (map[string]Answer, error) {
	body, err := json.Marshal(request{State: state, Model: Model, Questions: questions})
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, Endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 529) && attempt == 0 {
			select {
			case <-time.After(500 * time.Millisecond):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("typesafe: %s: %s", resp.Status, strings.TrimSpace(string(data)))
		}
		var out response
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out.Answers, nil
	}
}

// APIKey reads the API key from TYPESAFE_API_KEY, or from a
// TYPESAFE_API_KEY line in a .env file in the current directory.
func APIKey() string {
	if k := strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")); k != "" {
		return k
	}
	data, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && strings.TrimSpace(strings.TrimPrefix(k, "export ")) == "TYPESAFE_API_KEY" {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}
