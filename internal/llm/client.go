// Package llm provides a model-agnostic chat client with structured (JSON schema)
// output. It speaks the OpenAI Chat Completions API format, which is supported
// by OpenAI, OpenRouter, Ollama (OpenAI compatibility mode), LocalAI, LM Studio,
// and any provider that implements the /v1/chat/completions endpoint.
//
// This is the key differentiator from the Google reference agent (Gemini-locked):
// okf enrich works with any LLM provider.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ChatMessage is a single message in a chat sequence (OpenAI format).
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body for POST /v1/chat/completions.
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []ChatMessage   `json:"messages"`
	Stream         bool            `json:"stream"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat requests structured JSON output (OpenAI format).
type ResponseFormat struct {
	Type       string         `json:"type"` // "json_schema"
	JSONSchema *JSONSchemaDef `json:"json_schema,omitempty"`
}

// JSONSchemaDef wraps a named JSON schema for the response_format field.
type JSONSchemaDef struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict,omitempty"`
}

// ChatResponse is the response from /v1/chat/completions when stream is false.
type ChatResponse struct {
	Choices []struct {
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Client calls an OpenAI-compatible chat completions endpoint.
type Client struct {
	BaseURL    string        // e.g. "https://api.openai.com/v1", "http://localhost:11434/v1"
	APIKey     string        // bearer token; "ollama" for local Ollama
	Model      string        // e.g. "gpt-4o", "anthropic/claude-sonnet-4", "llama3.2"
	HTTPClient *http.Client  // nil defaults to 120s timeout
}

// Chat sends a chat request and returns the assistant's message content.
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	return c.ChatWithSchema(ctx, messages, nil)
}

// ChatWithSchema sends a chat request with a JSON schema for structured output.
// schema may be nil for free-form responses. The response content is returned
// as a raw string (caller parses into their type).
func (c *Client) ChatWithSchema(ctx context.Context, messages []ChatMessage, schema *JSONSchemaDef) (string, error) {
	reqBody := ChatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false,
	}
	if schema != nil {
		reqBody.ResponseFormat = &ResponseFormat{
			Type:       "json_schema",
			JSONSchema: schema,
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	u.Path = u.Path + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	req.Header.Set("User-Agent", "okf/1.0")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody)
		msg := errBody.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return "", fmt.Errorf("chat API error: %s", msg)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := chatResp.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("empty response content")
	}
	return content, nil
}

// ChatFunc is the injectable chat function signature for testing (mirrors
// protoncli's agent.ChatFunc pattern).
type ChatFunc func(ctx context.Context, messages []ChatMessage, schema *JSONSchemaDef) (string, error)

// ChatFn returns a ChatFunc bound to this client, for injection into enrich.
func (c *Client) ChatFn() ChatFunc {
	return func(ctx context.Context, messages []ChatMessage, schema *JSONSchemaDef) (string, error) {
		return c.ChatWithSchema(ctx, messages, schema)
	}
}
