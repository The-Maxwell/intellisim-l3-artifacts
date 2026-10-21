package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.uber.org/zap"

	sdk "github.com/The-Maxwell/intellisim.llm.provider.go.sdk"

	"intelli.sim.openai.llm.provider/internal/config"
)

// OpenAIClient handles all HTTP communication with the OpenAI API.
type OpenAIClient struct {
	logger     *zap.Logger
	config     *config.Config
	httpClient *http.Client
}

func NewOpenAIClient(logger *zap.Logger, cfg *config.Config) *OpenAIClient {
	return &OpenAIClient{
		logger: logger.Named("openai-client"),
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxConnsPerHost:     10,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

// Chat calls the Chat Completions API with full message history.
func (c *OpenAIClient) Chat(ctx context.Context, req *sdk.ChatRequest) (*LLMResponse, error) {
	body := buildChatBody(req.ModelUID, req.Effort, req.Message, req.SystemMessage, req.History, req.Config.Params, req.ResponseFormat)
	return c.callChatCompletions(ctx, body, time.Now())
}

// Simple calls the Responses API for single-turn requests.
func (c *OpenAIClient) Simple(ctx context.Context, req *sdk.SimpleRequest) (*LLMResponse, error) {
	body := buildSimpleBody(req.ModelUID, req.Effort, req.Message, req.SystemPrompt, req.Config.Params, req.ResponseFormat)
	return c.callResponsesAPI(ctx, body, time.Now())
}

// Vision calls the Chat Completions API with an image data-URL.
func (c *OpenAIClient) Vision(ctx context.Context, req *sdk.VisionRequest) (*LLMResponse, error) {
	body := buildVisionBody(req.ModelUID, req.Message, req.Images, req.Detail, req.SystemPrompt, req.Config.Params, req.ResponseFormat)
	return c.callChatCompletions(ctx, body, time.Now())
}

// Reasoning calls the Chat Completions API for deep analysis.
func (c *OpenAIClient) Reasoning(ctx context.Context, req *sdk.ReasoningRequest) (*ReasoningResult, error) {
	body := buildReasoningBody(req.ModelUID, req.Effort, req.Prompt, req.Context, req.SystemPrompt, req.Config.Params, req.ResponseFormat)
	resp, err := c.callChatCompletions(ctx, body, time.Now())
	if err != nil {
		return nil, err
	}
	return &ReasoningResult{
		Analysis: resp.Content, Reasoning: resp.Content, Model: resp.Model,
		Usage: resp.Usage, LatencyMs: resp.LatencyMs, FinishReason: resp.FinishReason,
	}, nil
}

// --- internal HTTP helpers ---

func (c *OpenAIClient) callChatCompletions(ctx context.Context, body map[string]any, start time.Time) (*LLMResponse, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq := c.buildHTTPRequest("/chat/completions", jsonData)
	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp chatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode chat response: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in chat response")
	}

	return &LLMResponse{
		Content:      apiResp.Choices[0].Message.Content,
		Model:        apiResp.Model,
		Usage: UsageStats{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			ReasoningTokens:  apiResp.Usage.CompletionTokensDetails.ReasoningTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		},
		LatencyMs:    float64(time.Since(start).Milliseconds()),
		FinishReason: apiResp.Choices[0].FinishReason,
	}, nil
}

func (c *OpenAIClient) callResponsesAPI(ctx context.Context, body map[string]any, start time.Time) (*LLMResponse, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq := c.buildHTTPRequest("/responses", jsonData)
	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp responsesAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode responses response: %w", err)
	}

	text := ""
	if len(apiResp.Output) > 0 && len(apiResp.Output[0].Content) > 0 {
		text = apiResp.Output[0].Content[0].Text
	}

	return &LLMResponse{
		Content:      text,
		Model:        apiResp.Model,
		Usage: UsageStats{
			PromptTokens:     apiResp.Usage.InputTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			ReasoningTokens:  apiResp.Usage.OutputTokensDetails.ReasoningTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		},
		LatencyMs:    float64(time.Since(start).Milliseconds()),
		FinishReason: "complete",
	}, nil
}

func (c *OpenAIClient) buildHTTPRequest(path string, jsonData []byte) *http.Request {
	u, err := url.Parse(c.config.OpenAIBaseURL + path)
	if err != nil {
		panic(fmt.Sprintf("invalid OpenAI URL: %v", err))
	}
	return &http.Request{
		Method: http.MethodPost,
		URL:    u,
		Header: http.Header{
			"Content-Type":  {"application/json"},
			"Authorization": {"Bearer " + c.config.OpenAIAPIKey},
			"User-Agent":    {"IntelliSim-OpenAI-Provider/1.0"},
		},
		Body: io.NopCloser(bytes.NewReader(jsonData)),
	}
}

func (c *OpenAIClient) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		httpReq := req.WithContext(ctx)
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			// Network error — retry with exponential backoff.
			c.logger.Warn("OpenAI network error",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
			)
			if attempt == c.config.MaxRetries {
				return nil, fmt.Errorf("request failed after %d attempts: %w", c.config.MaxRetries+1, err)
			}
			time.Sleep(exponentialBackoff(attempt))
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		c.logger.Warn("OpenAI API error",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
			zap.Int("attempt", attempt+1),
		)

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			// 429 — rate limited. Honour Retry-After if present.
			if attempt == c.config.MaxRetries {
				return nil, fmt.Errorf("rate limited by OpenAI after %d attempts: %s", c.config.MaxRetries+1, string(body))
			}
			wait := parseRetryAfter(resp.Header.Get("Retry-After"))
			if wait <= 0 {
				wait = exponentialBackoff(attempt)
			}
			c.logger.Info("Rate limited — waiting before retry",
				zap.Duration("wait", wait),
				zap.Int("attempt", attempt+1),
			)
			time.Sleep(wait)

		case resp.StatusCode >= 500:
			// 5xx — server error, retry with exponential backoff.
			if attempt == c.config.MaxRetries {
				return nil, fmt.Errorf("OpenAI server error %d after %d attempts: %s", resp.StatusCode, c.config.MaxRetries+1, string(body))
			}
			time.Sleep(exponentialBackoff(attempt))

		default:
			// 4xx (except 429) — client error, do not retry.
			var oaiErr openaiError
			if jsonErr := json.Unmarshal(body, &oaiErr); jsonErr == nil && oaiErr.Error.Message != "" {
				return nil, fmt.Errorf("OpenAI API error %d [%s/%s]: %s",
					resp.StatusCode,
					oaiErr.Error.Type,
					oaiErr.Error.Code,
					oaiErr.Error.Message,
				)
			}
			return nil, fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, string(body))
		}
	}
	return nil, fmt.Errorf("max retries exceeded")
}

// exponentialBackoff returns 1s, 2s, 4s, ... capped at 30s for the given attempt (0-based).
func exponentialBackoff(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// parseRetryAfter parses the Retry-After header value (seconds integer or RFC1123 date).
// Returns 0 if the header is absent or unparseable.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, header); err == nil {
		return time.Until(t)
	}
	return 0
}

// openaiError represents the error envelope returned by the OpenAI API.
type openaiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// --- API response types ---

type LLMResponse struct {
	Content      string
	Model        string
	Usage        UsageStats
	LatencyMs    float64
	FinishReason string
}

type ReasoningResult struct {
	Analysis     string
	Reasoning    string
	Model        string
	Usage        UsageStats
	LatencyMs    float64
	FinishReason string
}

type UsageStats struct {
	PromptTokens     int
	CompletionTokens int
	// ReasoningTokens is the reasoning portion of CompletionTokens (a subset,
	// already counted in CompletionTokens/TotalTokens); 0 for non-reasoning models.
	ReasoningTokens int
	TotalTokens     int
}

type chatCompletionsResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens            int `json:"prompt_tokens"`
		CompletionTokens        int `json:"completion_tokens"`
		TotalTokens             int `json:"total_tokens"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

type responsesAPIResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		TotalTokens         int `json:"total_tokens"`
		OutputTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
}

// --- body builders ---

// applyParams writes the dynamic config map into a chat/completions body.
// Reasoning models reject sampling params and use max_completion_tokens.
// Absent keys are OMITTED — the API default applies.
func applyParams(body map[string]any, params map[string]sdk.ConfigValue, model string) {
	reasoning := IsReasoningModel(model)
	for key, v := range params {
		switch key {
		case "temperature":
			if !reasoning && v.Double != nil {
				body["temperature"] = *v.Double
			}
		case "top_p":
			if !reasoning && v.Double != nil {
				body["top_p"] = *v.Double
			}
		case "frequency_penalty":
			if !reasoning && v.Double != nil {
				body["frequency_penalty"] = *v.Double
			}
		case "presence_penalty":
			if !reasoning && v.Double != nil {
				body["presence_penalty"] = *v.Double
			}
		case "max_tokens":
			if v.Uint != nil {
				if reasoning {
					body["max_completion_tokens"] = *v.Uint
				} else {
					body["max_tokens"] = *v.Uint
				}
			}
		case "seed":
			if v.Uint != nil {
				body["seed"] = *v.Uint
			}
		case "stop":
			if len(v.List) > 0 {
				body["stop"] = v.List
			}
		}
	}
}

// applyResponseFormat writes the core-controlled response format (chat/completions shape).
func applyResponseFormat(body map[string]any, rf sdk.ResponseFormat) {
	if rf == sdk.ResponseFormatJSON {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
}

// applyEffort sets reasoning_effort on chat/completions bodies for reasoning models.
func applyEffort(body map[string]any, model string, effort sdk.Effort) {
	if effort != "" && IsReasoningModel(model) {
		body["reasoning_effort"] = string(effort)
	}
}

func buildChatBody(model string, effort sdk.Effort, message, systemMessage string, history []sdk.ChatMessage, params map[string]sdk.ConfigValue, rf sdk.ResponseFormat) map[string]any {
	messages := make([]map[string]string, 0, len(history)+2)
	if systemMessage != "" {
		messages = append(messages, map[string]string{"role": "system", "content": systemMessage})
	}
	for _, m := range history {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": message})

	body := map[string]any{"model": model, "messages": messages}
	applyParams(body, params, model)
	applyResponseFormat(body, rf)
	applyEffort(body, model, effort)
	return body
}

// buildVisionBody builds a Chat Completions body with one user message whose
// content is the prompt text followed by every reference image (1..N). When an
// image carries a description it is emitted as a text part immediately before
// that image, so the model can tell multiple images apart.
func buildVisionBody(model, message string, images []sdk.VisionImage, detail, systemPrompt string, params map[string]sdk.ConfigValue, rf sdk.ResponseFormat) map[string]any {
	if detail == "" {
		detail = "high"
	}

	messages := []any{}
	if systemPrompt != "" {
		messages = append(messages, map[string]any{"role": "system", "content": systemPrompt})
	}

	content := []map[string]any{{"type": "text", "text": message}}
	for _, img := range images {
		if img.Description != "" {
			content = append(content, map[string]any{"type": "text", "text": img.Description})
		}
		imageBase64 := base64.StdEncoding.EncodeToString(img.ImageData)
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{
			"url":    fmt.Sprintf("data:image/%s;base64,%s", img.ImageFormat, imageBase64),
			"detail": detail,
		}})
	}
	messages = append(messages, map[string]any{"role": "user", "content": content})

	body := map[string]any{"model": model, "messages": messages}
	applyParams(body, params, model)
	applyResponseFormat(body, rf)
	return body
}

func buildSimpleBody(model string, effort sdk.Effort, message, systemPrompt string, params map[string]sdk.ConfigValue, rf sdk.ResponseFormat) map[string]any {
	body := map[string]any{"model": model, "input": message, "store": false}
	if systemPrompt != "" {
		body["instructions"] = systemPrompt
	}
	reasoning := IsReasoningModel(model)
	// The Responses API (/responses) accepts only a subset of the advertised
	// params: temperature, top_p, and max_output_tokens. It does NOT support
	// frequency_penalty/presence_penalty/seed/stop (Chat Completions-only), so
	// those keys are intentionally ignored on the Simple path even though the
	// model advertises them — sending them would be rejected by the API.
	for key, v := range params {
		switch key {
		case "temperature":
			if !reasoning && v.Double != nil {
				body["temperature"] = *v.Double
			}
		case "top_p":
			if !reasoning && v.Double != nil {
				body["top_p"] = *v.Double
			}
		case "max_tokens":
			if v.Uint != nil {
				body["max_output_tokens"] = *v.Uint
			}
		}
	}
	if rf == sdk.ResponseFormatJSON {
		body["text"] = map[string]any{"format": map[string]string{"type": "json_object"}}
	}
	if effort != "" && IsReasoningModel(model) {
		body["reasoning"] = map[string]string{"effort": string(effort)}
	}
	return body
}

func buildReasoningBody(model string, effort sdk.Effort, prompt, contextStr, systemPrompt string, params map[string]sdk.ConfigValue, rf sdk.ResponseFormat) map[string]any {
	if systemPrompt == "" {
		systemPrompt = "You are an expert analyst tasked with providing deep, thoughtful analysis. Think step-by-step and provide detailed reasoning."
	}
	userContent := prompt
	if contextStr != "" {
		userContent = fmt.Sprintf("Context:\n%s\n\nAnalysis Request:\n%s", contextStr, prompt)
	}
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userContent},
	}
	body := map[string]any{"model": model, "messages": messages}
	applyParams(body, params, model)
	applyResponseFormat(body, rf)
	applyEffort(body, model, effort)
	return body
}
