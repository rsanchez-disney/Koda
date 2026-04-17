package autopilot

import (
	"encoding/json"
	"strings"
	"fmt"
	"net/http"
	"time"

	"github.disney.com/SANCR225/koda/internal/pkg"
)

// Client talks to the autopilot dashboard API.
type Client struct {
	baseURL string
	http    *http.Client
}

type Pipeline struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Template string `json:"template"`
	Status   string `json:"status"`
}

type Stage struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Agent      string `json:"agent"`
	TokenUsage int    `json:"token_usage"`
}

type Gate struct {
	ID         string `json:"id"`
	PipelineID string `json:"pipeline_id"`
	StageName  string `json:"stage_name"`
	Status     string `json:"status"`
}

func NewClient(port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://localhost:%d", port),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Available checks if autopilot is installed and the API is reachable.
func (c *Client) Available() bool {
	if !pkg.IsInstalled("autopilot") {
		return false
	}
	resp, err := c.http.Get(c.baseURL + "/api/pipelines")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func (c *Client) ListPipelines() ([]Pipeline, error) {
	return getJSON[[]Pipeline](c, "/api/pipelines")
}

func (c *Client) GetPipeline(id string) (*Pipeline, []Stage, error) {
	var result struct {
		Pipeline Pipeline `json:"pipeline"`
		Stages   []Stage  `json:"stages"`
	}
	resp, err := c.http.Get(c.baseURL + "/api/pipelines/" + id)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&result)
	return &result.Pipeline, result.Stages, nil
}

func (c *Client) ListGates() ([]Gate, error) {
	return getJSON[[]Gate](c, "/api/gates")
}

func (c *Client) ApproveGate(id string) error {
	resp, err := c.http.Post(c.baseURL+"/api/gates/"+id+"/approve", "application/json", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) RejectGate(id, feedback string) error {
	body := fmt.Sprintf(`{"feedback":"%s"}`, feedback)
	resp, err := c.http.Post(c.baseURL+"/api/gates/"+id+"/reject", "application/json",
		strings.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func getJSON[T any](c *Client, path string) (T, error) {
	var result T
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}
