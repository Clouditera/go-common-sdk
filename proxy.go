package aiproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	API_PING = "/api/v1/ping"
)

type AIProxy struct {
	Url     string        `json:"url"`     // URL of the AI proxy service
	Servers []Server      `json:"server"`  // Server configuration
	Timeout time.Duration `json:"timeout"` // Timeout for requests
}

type ChatRequest struct {
	SystemPrompt string `json:"system_prompt"` // System prompt, optional
	UserPrompt   string `json:"user_prompt"`   // User prompt
}

func (proxy AIProxy) Ping() error {
	reqBody := RequestBody{
		LlmServer: proxy.Servers,
	}

	reqJson, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encoding JSON: %v", err)
	}

	client := &http.Client{
		Timeout: proxy.Timeout,
	}

	req, err := http.NewRequest("POST", proxy.Url+API_PING, bytes.NewBuffer(reqJson))
	if err != nil {
		return fmt.Errorf("creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %v", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %v", err)
	}

	var jsonResp ResponseBody
	if err := json.Unmarshal(data, &jsonResp); err != nil {
		return fmt.Errorf("decoding JSON: %v", err)
	}

	if jsonResp.Status != STATUS_OK {
		return fmt.Errorf("response failed: %s", jsonResp.Reason)
	}

	return nil
}
