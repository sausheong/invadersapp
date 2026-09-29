//go:build jev

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TypeSafe System One endpoint and model (see https://docs.typesafe.ai/api).
const (
	jevEndpoint = "https://api.typesafe.ai/v1/systemone"
	jevModel    = "jev-latest"
)

// jevQuestion is one typed TypeSafe question. Criteria is a map of option
// descriptions for a choice, or {"true":..., "false":...} for a noul.
type jevQuestion struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// jevAnswer holds the fields of a choice or noul answer.
type jevAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
	Noul          float64            `json:"noul"`
}

type jevRequest struct {
	State     any                    `json:"state"`
	Model     string                 `json:"model"`
	Questions map[string]jevQuestion `json:"questions"`
}

type jevResponse struct {
	Model   string               `json:"model"`
	Answers map[string]jevAnswer `json:"answers"`
}

// jevClient is a minimal TypeSafe HTTP client; there's no Go SDK.
type jevClient struct {
	apiKey string
	http   *http.Client
}

func newJevClient(apiKey string) *jevClient {
	return &jevClient{apiKey: apiKey, http: &http.Client{Timeout: 5 * time.Second}}
}

// ask sends one state with its questions and returns the answers by id.
// 429 and 529 are retried once after a short pause.
func (c *jevClient) ask(ctx context.Context, state any, questions map[string]jevQuestion) (map[string]jevAnswer, error) {
	body, err := json.Marshal(jevRequest{State: state, Model: jevModel, Questions: questions})
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, jevEndpoint, bytes.NewReader(body))
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
		var out jevResponse
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out.Answers, nil
	}
}

// jevAPIKey reads the TypeSafe API key from TYPESAFE_API_KEY, then from a
// .env file in the current directory, then from a typesafe-api-key file
// next to the high score file (apps launched from Finder don't inherit
// shell environment variables).
func jevAPIKey() string {
	if k := strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")); k != "" {
		return k
	}
	if data, err := os.ReadFile(".env"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
			if ok && strings.TrimSpace(strings.TrimPrefix(k, "export ")) == "TYPESAFE_API_KEY" {
				return strings.Trim(strings.TrimSpace(v), `"'`)
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(highScorePath()), "typesafe-api-key"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
