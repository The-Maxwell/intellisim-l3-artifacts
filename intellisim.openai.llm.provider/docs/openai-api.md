# OpenAI API Integration

The OpenAI LLM Provider uses the OpenAI REST API to handle four types of LLM operations. This document describes endpoints, request formats, response parsing, and configuration.

## OpenAI Endpoints

### POST /chat/completions

Used for: Chat requests, Vision analysis, Reasoning requests

**Base URL**: `https://api.openai.com/v1` (configurable via `OPENAI_BASE_URL`)

**Full URL**: `https://api.openai.com/v1/chat/completions`

**Authentication**: Bearer token in Authorization header

### POST /responses

Used for: Simple single-turn requests via OpenAI Responses API

**Base URL**: `https://api.openai.com/v1` (configurable via `OPENAI_BASE_URL`)

**Full URL**: `https://api.openai.com/v1/responses`

**Authentication**: Bearer token in Authorization header

## Request Formats

All requests are JSON-encoded POST bodies with `Content-Type: application/json`.

### Chat Request

Maps to OpenAI Chat Completions API.

**Request Body:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "What is the capital of France?"
    },
    {
      "role": "assistant",
      "content": "The capital of France is Paris."
    },
    {
      "role": "user",
      "content": "Tell me more about it."
    }
  ],
  "temperature": 0.7,
  "max_tokens": 2000
}
```

**Field Descriptions:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| model | string | Yes | Model ID: "gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo" |
| messages | array | Yes | Conversation history with role/content pairs |
| temperature | float | No | Sampling temperature 0-2. Higher = more random. Default: inherited from request |
| max_tokens | int | No | Maximum output tokens. Default: inherited from request |

**Message Roles:**
- `system`: Instructions for the assistant behavior
- `user`: Human messages
- `assistant`: Previous AI responses (for history)

**Provider Implementation:**
```go
// From internal/provider/openai.go Chat() method
messages := make([]map[string]string, 0, len(history)+2)
for _, m := range history {
    messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
}
if systemMessage != "" {
    messages = append([]map[string]string{{"role": "system", "content": systemMessage}}, messages...)
}
messages = append(messages, map[string]string{"role": "user", "content": message})

body := map[string]any{
    "model":       model,
    "messages":    messages,
    "temperature": temperature,
    "max_tokens":  maxTokens,
}
```

### Simple Request

Maps to OpenAI Responses API for single-turn requests.

**Request Body:**
```json
{
  "model": "gpt-4o",
  "input": "Summarize quantum computing in one paragraph.",
  "instructions": "Provide a clear, technical summary.",
  "temperature": 0.7,
  "text": {
    "format": {
      "type": "text"
    }
  },
  "store": false
}
```

**Field Descriptions:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| model | string | Yes | Model ID (typically default) |
| input | string | Yes | The user's request/prompt |
| instructions | string | No | System prompt for behavior guidance |
| temperature | float | No | Sampling temperature. Default: 0.7 |
| text.format.type | string | No | Output format, typically "text" |
| store | bool | No | Whether to store response for training. Default: false |

**Provider Implementation:**
```go
// From internal/provider/openai.go Simple() method
body := map[string]any{
    "model":        c.config.OpenAIDefaultModel,
    "input":        message,
    "instructions": systemPrompt,
    "temperature":  0.7,
    "text":         map[string]any{"format": map[string]string{"type": "text"}},
    "store":        false,
}
```

### Vision Analysis Request

Maps to Chat Completions API with image content.

**Request Body:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "You are a vision expert. Analyze images in detail."
    },
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "What objects are in this image?"
        },
        {
          "type": "image_url",
          "image_url": {
            "url": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEA...",
            "detail": "high"
          }
        }
      ]
    }
  ],
  "temperature": 0.7,
  "max_tokens": 2000
}
```

**Field Descriptions:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| model | string | Yes | Must be "gpt-4o" (only model supporting vision) |
| messages | array | Yes | Array with system + user messages |
| messages[].content | array | Yes (user only) | Array of text and image_url objects |
| messages[].content[].type | string | Yes | "text" or "image_url" |
| messages[].content[].text | string | For text | The text prompt |
| messages[].content[].image_url.url | string | For image | Data URL: `data:image/{format};base64,{data}` |
| messages[].content[].image_url.detail | string | No | "low" (fast), "high" (detailed), "auto" (default). Default: "high" |
| temperature | float | No | Default: 0.7 |
| max_tokens | int | No | Default: 2000 |

**Image Format Support:**
- `jpeg`: `.jpg`, `.jpeg` files
- `png`: `.png` files
- `gif`: `.gif` files
- `webp`: `.webp` files

**Provider Implementation:**
```go
// From internal/provider/openai.go VisionAnalysis() method
imageBase64 := base64.StdEncoding.EncodeToString(imageData)

messages := []any{}
if systemPrompt != "" {
    messages = append(messages, map[string]any{"role": "system", "content": systemPrompt})
}
messages = append(messages, map[string]any{
    "role": "user",
    "content": []map[string]any{
        {"type": "text", "text": message},
        {"type": "image_url", "image_url": map[string]string{
            "url":    fmt.Sprintf("data:image/%s;base64,%s", imageFormat, imageBase64),
            "detail": detail,
        }},
    },
})

body := map[string]any{
    "model":       "gpt-4o",
    "messages":    messages,
    "temperature": temperature,
    "max_tokens":  maxTokens,
}
```

### Reasoning Request

Maps to Chat Completions API with structured reasoning.

**Request Body:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "You are an expert analyst tasked with providing deep, thoughtful analysis. Think step-by-step and provide detailed reasoning."
    },
    {
      "role": "user",
      "content": "Context:\nThe pandemic disrupted supply chains.\n\nAnalysis Request:\nHow did this affect manufacturing?"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 4000
}
```

**Field Descriptions:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| model | string | Yes | Must be "gpt-4o" |
| messages | array | Yes | System prompt + context + analysis request |
| temperature | float | No | Default: 0.7 |
| max_tokens | int | No | Default: 4000 (reasoning can be lengthy) |

**Provider Implementation:**
```go
// From internal/provider/openai.go Reasoning() method
userContent := prompt
if contextStr != "" {
    userContent = fmt.Sprintf("Context:\n%s\n\nAnalysis Request:\n%s", contextStr, prompt)
}

messages := []map[string]string{
    {"role": "system", "content": systemPrompt},
    {"role": "user", "content": userContent},
}

body := map[string]any{
    "model":       "gpt-4o",
    "messages":    messages,
    "temperature": temperature,
    "max_tokens":  maxTokens,
}
```

## Response Parsing

### Chat Completions Response

**HTTP Status**: 200 OK

**JSON Structure:**
```json
{
  "id": "chatcmpl-abc123...",
  "object": "chat.completion",
  "created": 1699564789,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Paris is the capital of France and is known for..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 42,
    "total_tokens": 57
  }
}
```

**Go Struct Definition:**
```go
// From internal/provider/openai.go
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
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage"`
}
```

**Extraction Logic:**
```go
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
    Usage:        UsageStats{...},
    LatencyMs:    float64(time.Since(start).Milliseconds()),
    FinishReason: apiResp.Choices[0].FinishReason,
}, nil
```

### Responses API Response

**HTTP Status**: 200 OK

**JSON Structure:**
```json
{
  "id": "resp_abc123...",
  "object": "response",
  "model": "gpt-4o",
  "output": [
    {
      "type": "text",
      "content": [
        {
          "type": "text",
          "text": "Quantum computing uses quantum bits..."
        }
      ]
    }
  ]
}
```

**Go Struct Definition:**
```go
// From internal/provider/openai.go
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
}
```

**Extraction Logic:**
```go
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
    LatencyMs:    float64(time.Since(start).Milliseconds()),
    FinishReason: "complete",
}, nil
```

## Authentication

**Method**: Bearer Token in Authorization Header

**Header Format**:
```
Authorization: Bearer sk-...
```

**Key Source**: `OPENAI_API_KEY` environment variable (required)

**Provider Implementation:**
```go
// From internal/provider/openai.go buildHTTPRequest()
Header: http.Header{
    "Content-Type":  {"application/json"},
    "Authorization": {"Bearer " + c.config.OpenAIAPIKey},
    "User-Agent":    {"IntelliSim-OpenAI-Provider/1.0"},
}
```

**Key Safety**:
- Key is injected from environment variable (not hardcoded)
- Key is not logged (careful logging avoidance in error messages)
- Key is per-request, created fresh for each HTTP request

## Retry Logic

**Exponential Backoff on Failure**

All HTTP requests go through `doRequest()` method with automatic retry:

```go
func (c *OpenAIClient) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
    for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
        httpReq := req.WithContext(ctx)
        resp, err := c.httpClient.Do(httpReq)
        
        if err != nil {
            if attempt == c.config.MaxRetries {
                return nil, fmt.Errorf("request failed after %d attempts: %w", c.config.MaxRetries+1, err)
            }
            time.Sleep(time.Duration(attempt*2+1) * 100 * time.Millisecond)
            continue
        }

        if resp.StatusCode >= 200 && resp.StatusCode < 300 {
            return resp, nil
        }

        // Non-2xx response: log and retry
        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()

        c.logger.Warn("OpenAI API error",
            zap.Int("status", resp.StatusCode),
            zap.String("body", string(body)),
            zap.Int("attempt", attempt+1),
        )

        if attempt == c.config.MaxRetries {
            return nil, fmt.Errorf("API error %d after %d attempts: %s", resp.StatusCode, c.config.MaxRetries+1, string(body))
        }
        time.Sleep(time.Duration(attempt*2+1) * 100 * time.Millisecond)
    }
    return nil, fmt.Errorf("max retries exceeded")
}
```

**Backoff Schedule** (default MaxRetries=3):

| Attempt | Backoff | Cumulative |
|---------|---------|-----------|
| 1 | 100 ms | 100 ms |
| 2 | 300 ms | 400 ms |
| 3 | 500 ms | 900 ms |
| 4 | 700 ms | 1600 ms |

**Retry Triggers:**

- **Network Error** (connection refused, timeout, etc.) — Retry
- **Non-2xx HTTP Status** (4xx, 5xx) — Retry up to MaxRetries
- **JSON Decode Error** — No retry (malformed response)
- **Empty Response** (no choices) — No retry (likely invalid request)

**Configuration:**
- `OPENAI_MAX_RETRIES=3` — Maximum attempts after initial request (default)

## Timeout Configuration

**HTTP Client Timeout**: Set at client creation

```go
// From internal/provider/openai.go NewOpenAIClient()
httpClient: &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
```

**Configuration Source**: `OPENAI_TIMEOUT_SECONDS` environment variable

**Default**: 30 seconds

**Timeout Scope**: Entire HTTP request-response cycle (connection + request + response)

**Exceeded Timeout**: 
- Returns network error
- Triggers retry logic
- After MaxRetries, returns to caller as "request failed"

## Vision Parameters

### Image Format

Supports multiple image formats as data URLs:

| Format | Mime Type | Extension | Quality |
|--------|-----------|-----------|---------|
| JPEG | image/jpeg | .jpg, .jpeg | Compressed, good for photos |
| PNG | image/png | .png | Lossless, good for diagrams |
| GIF | image/gif | .gif | Animated or simple graphics |
| WebP | image/webp | .webp | Modern, efficient compression |

**Provider Encoding:**
```go
imageBase64 := base64.StdEncoding.EncodeToString(imageData)
url := fmt.Sprintf("data:image/%s;base64,%s", imageFormat, imageBase64)
```

Where `imageFormat` is passed from VisionAnalysisRequest (e.g., "jpeg", "png").

### Detail Parameter

Controls how deeply the model analyzes the image:

| Value | Behavior | Use Case | Cost Impact |
|-------|----------|----------|-------------|
| "low" | Quick analysis, lower token cost | Thumbnails, simple checks | Lower tokens |
| "high" | Detailed analysis, higher token cost | Complex diagrams, documents | Higher tokens |
| "auto" | Model chooses based on image size | Balanced (default recommendation) | Variable |

**Provider Default**: "high" if not specified

**Provider Implementation:**
```go
if detail == "" {
    detail = "high"
}

// Later in message construction:
"image_url": map[string]string{
    "url":    fmt.Sprintf("data:image/%s;base64,%s", imageFormat, imageBase64),
    "detail": detail,
}
```

## Model Capability Matrix

Supported models and their capabilities:

| Model | Chat | Simple | Vision | Reasoning | Max Output | Context | Pricing Tier |
|-------|------|--------|--------|-----------|-----------|---------|---|
| gpt-4o | ✓ | ✓ | ✓ | ✓ | 16,384 | 128,000 | Premium |
| gpt-4o-mini | ✓ | ✓ | ✗ | ✗ | 16,384 | 128,000 | Standard |
| gpt-3.5-turbo | ✓ | ✓ | ✗ | ✗ | 4,096 | 16,385 | Economy |

### Model Details

**gpt-4o** (Recommended for production)
- Latest OpenAI vision and reasoning model
- Best quality across all task types
- Supports all capabilities (chat, simple, vision, reasoning)
- 128k context window — excellent for long-form analysis
- Suitable for complex agent decision-making

**gpt-4o-mini** (Cost-effective multi-capability)
- Smaller, faster, cheaper variant of gpt-4o
- Supports chat and simple requests
- Still 128k context window — good for structured data
- Recommended when vision/reasoning not needed

**gpt-3.5-turbo** (Legacy/budget option)
- Older model, lower quality but faster and cheaper
- Supports chat and simple only
- Smaller 16k context window — suitable for short conversations
- Consider upgrading to gpt-4o for better results

### Capability Validation

Provider validates capabilities before API call:

```go
func (s *ProviderService) requireModelCapability(model, capability string) error {
    if !IsModelSupported(model) {
        return &capabilityError{...}  // Returns 501 Not Implemented
    }
    if !ModelSupportsCapability(model, capability) {
        return &capabilityError{...}  // Returns 501 Not Implemented
    }
    return nil
}
```

Core catches unsupported capability errors and can:
- Log for debugging
- Retry with different model
- Return error to user
- Fall back to rule-based simulation

## Error Responses

**Format**: Standard OpenAI error response (when API returns error)

```json
{
  "error": {
    "message": "Rate limit exceeded",
    "type": "rate_limit_error",
    "param": null,
    "code": "rate_limit_exceeded"
  }
}
```

**Provider Handling**:
- Logs with Zap logger
- Retries on transient errors (5xx, rate limits)
- Returns error message to Core in ErrorResponse proto
- Core logs and fails the operation

## Summary

The OpenAI API integration layer provides:
- ✓ Two endpoints (/chat/completions, /responses) for different request types
- ✓ Four operation types (Chat, Simple, Vision, Reasoning) with distinct request formats
- ✓ Automatic retry with exponential backoff
- ✓ Configurable timeouts and retry limits
- ✓ Vision support with base64 image encoding and detail control
- ✓ Model capability validation before API calls
- ✓ Structured error handling and logging
- ✓ Bearer token authentication from environment
