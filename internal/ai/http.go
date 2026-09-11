package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var sharedHTTPClient = &http.Client{Timeout: 60 * time.Second}

func SetHTTPClient(client *http.Client) {
	if client != nil {
		sharedHTTPClient = client
	}
}

type httpClient struct {
	apiKey  string
	baseURL string
	model   string
	headers map[string]string
}

func doChatRequest(ctx context.Context, url string, headers map[string]string, body any) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := sharedHTTPClient
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

func extractOpenAIContent(respBody string) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("openai error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from openai")
	}
	return resp.Choices[0].Message.Content, nil
}

func extractClaudeContent(respBody string) (string, error) {
	var resp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("claude error: %s", resp.Error.Message)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no response from claude")
	}
	return resp.Content[0].Text, nil
}

func extractYandexContent(respBody string) (string, error) {
	var resp struct {
		Result struct {
			Alternatives []struct {
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
			} `json:"alternatives"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
		return "", err
	}
	if len(resp.Result.Alternatives) == 0 {
		return "", fmt.Errorf("no response from yandex")
	}
	return resp.Result.Alternatives[0].Message.Text, nil
}
