package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	URL  string
	HTTP *http.Client
}

type Task struct {
	ProblemID string  `json:"problem_id"`
	Language  string  `json:"language"`
	Source    string  `json:"source"`
	Problem   Problem `json:"problem"`
	OutputDir string  `json:"output_dir"`
}

type Problem struct {
	ID      string                   `json:"id"`
	Limits  map[string]ResourceLimit `json:"limits"`
	Compare CompareOption            `json:"compare"`
	Cases   []Case                   `json:"cases"`
}

type ResourceLimit struct {
	TimeLimitMS   int64  `json:"time_limit_ms"`
	MemoryLimitMB uint64 `json:"memory_limit_mb"`
}

type CompareOption struct {
	FloatingPoint bool    `json:"floating_point"`
	Epsilon       float64 `json:"epsilon"`
}

type Case struct {
	ID           string `json:"id"`
	InputPath    string `json:"input_path"`
	ExpectedPath string `json:"expected_path"`
}

type Response struct {
	Verdict string        `json:"verdict"`
	Compile CompileResult `json:"compile"`
	Cases   []CaseResult  `json:"cases"`
}

type CompileResult struct {
	Verdict string `json:"verdict"`
	Stderr  string `json:"stderr"`
	Error   string `json:"error"`
}

type CaseResult struct {
	ID            string `json:"id"`
	Verdict       string `json:"verdict"`
	SandboxStatus string `json:"sandbox_status"`
	TimeMS        int64  `json:"time_ms"`
	WallTimeMS    int64  `json:"wall_time_ms"`
	Memory        uint64 `json:"memory"`
	ProcessPeak   uint64 `json:"process_peak"`
	OutputPath    string `json:"output_path,omitempty"`
	Stderr        string `json:"stderr"`
	Error         string `json:"error"`
}

func (c Client) Execute(ctx context.Context, task Task) (Response, bool, error) {
	body, err := json.Marshal(task)
	if err != nil {
		return Response{}, false, err
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.URL, "/")+"/api/execute", bytes.NewReader(body))
	if err != nil {
		return Response{}, false, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return Response{}, false, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return Response{}, false, err
	}
	if response.StatusCode == http.StatusTooManyRequests {
		return Response{}, true, nil
	}
	if response.StatusCode != http.StatusOK {
		return Response{}, false, fmt.Errorf("judge engine returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	var result Response
	if err := json.Unmarshal(data, &result); err != nil {
		return Response{}, false, fmt.Errorf("decode judge response: %w", err)
	}
	return result, false, nil
}
