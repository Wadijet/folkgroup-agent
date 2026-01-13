/*
Package services chứa các services hỗ trợ cho agent.
File này chứa AI Client Service - service để gọi AI provider APIs (OpenAI, Anthropic, Google, etc.)
*/
package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// AIProviderType định nghĩa các loại AI provider
const (
	AIProviderTypeOpenAI    = "openai"
	AIProviderTypeAnthropic = "anthropic"
	AIProviderTypeGoogle    = "google"
	AIProviderTypeCohere    = "cohere"
	AIProviderTypeCustom    = "custom"
)

// AIProviderProfile là cấu trúc provider profile từ backend
type AIProviderProfile struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Provider          string                 `json:"provider"`
	APIKey            string                 `json:"apiKey"`
	BaseURL           string                 `json:"baseUrl,omitempty"`
	OrganizationID    string                 `json:"organizationId,omitempty"`
	DefaultModel      string                 `json:"defaultModel,omitempty"`
	DefaultTemperature *float64              `json:"defaultTemperature,omitempty"`
	DefaultMaxTokens   *int                  `json:"defaultMaxTokens,omitempty"`
	Config            map[string]interface{} `json:"config,omitempty"`
}

// AICallRequest là request để gọi AI API
type AICallRequest struct {
	ProviderProfile *AIProviderProfile
	Model           string  // Model cụ thể (nếu không có thì dùng DefaultModel)
	Prompt          string  // Prompt text
	Temperature     *float64 // Temperature (nếu không có thì dùng DefaultTemperature)
	MaxTokens       *int    // Max tokens (nếu không có thì dùng DefaultMaxTokens)
	SystemPrompt    string  // System prompt (optional)
	Messages        []AIMessage // Conversation history (optional)
}

// AIMessage là message trong conversation
type AIMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// AICallResponse là response từ AI API
type AICallResponse struct {
	Content      string        // Response text
	Model        string        // Model được sử dụng
	Usage        *AIUsage      // Token usage
	FinishReason string      // Finish reason
	Latency      time.Duration // Thời gian gọi API
	Error        error         // Lỗi nếu có
}

// AIUsage là thông tin token usage
type AIUsage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// AIClient là interface cho AI client
type AIClient interface {
	Call(req AICallRequest) (*AICallResponse, error)
}

// AIClientService là service để gọi AI provider APIs
type AIClientService struct {
	httpClient *http.Client
}

// NewAIClientService tạo một instance mới của AIClientService
func NewAIClientService() *AIClientService {
	return &AIClientService{
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Timeout 2 phút cho AI calls
		},
	}
}

// Call gọi AI provider API dựa trên provider type
func (s *AIClientService) Call(req AICallRequest) (*AICallResponse, error) {
	if req.ProviderProfile == nil {
		return nil, errors.New("provider profile không được để trống")
	}

	startTime := time.Now()

	switch req.ProviderProfile.Provider {
	case AIProviderTypeOpenAI:
		return s.callOpenAI(req, startTime)
	case AIProviderTypeAnthropic:
		return s.callAnthropic(req, startTime)
	case AIProviderTypeGoogle:
		return s.callGoogle(req, startTime)
	case AIProviderTypeCohere:
		return s.callCohere(req, startTime)
	case AIProviderTypeCustom:
		return s.callCustom(req, startTime)
	default:
		return nil, fmt.Errorf("provider type không được hỗ trợ: %s", req.ProviderProfile.Provider)
	}
}

// callOpenAI gọi OpenAI API
func (s *AIClientService) callOpenAI(req AICallRequest, startTime time.Time) (*AICallResponse, error) {
	profile := req.ProviderProfile
	model := req.Model
	if model == "" {
		model = profile.DefaultModel
	}
	if model == "" {
		model = "gpt-4" // Default model
	}

	temperature := 0.7
	if req.Temperature != nil {
		temperature = *req.Temperature
	} else if profile.DefaultTemperature != nil {
		temperature = *profile.DefaultTemperature
	}

	maxTokens := 2000
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	} else if profile.DefaultMaxTokens != nil {
		maxTokens = *profile.DefaultMaxTokens
	}

	// Chuẩn bị messages
	messages := []map[string]interface{}{}
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	
	// Thêm conversation history nếu có
	if len(req.Messages) > 0 {
		for _, msg := range req.Messages {
			messages = append(messages, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
	}
	
	// Thêm user prompt
	messages = append(messages, map[string]interface{}{
		"role":    "user",
		"content": req.Prompt,
	})

	// Chuẩn bị request body
	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi marshal request body: %v", err)
	}

	// Tạo HTTP request
	url := "https://api.openai.com/v1/chat/completions"
	if profile.BaseURL != "" {
		url = profile.BaseURL + "/v1/chat/completions"
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tạo HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+profile.APIKey)
	if profile.OrganizationID != "" {
		httpReq.Header.Set("OpenAI-Organization", profile.OrganizationID)
	}

	// Gọi API
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi gọi OpenAI API: %v", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)

	// Đọc response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi đọc response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &AICallResponse{
			Latency: latency,
			Error:   fmt.Errorf("OpenAI API trả về lỗi (status %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// Parse response
	var openAIResp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
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

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("lỗi khi parse OpenAI response: %v", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, errors.New("OpenAI response không có choices")
	}

	content := openAIResp.Choices[0].Message.Content
	finishReason := openAIResp.Choices[0].FinishReason

	return &AICallResponse{
		Content:       content,
		Model:         openAIResp.Model,
		Usage:         &AIUsage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
		FinishReason: finishReason,
		Latency:      latency,
	}, nil
}

// callAnthropic gọi Anthropic (Claude) API
func (s *AIClientService) callAnthropic(req AICallRequest, startTime time.Time) (*AICallResponse, error) {
	profile := req.ProviderProfile
	model := req.Model
	if model == "" {
		model = profile.DefaultModel
	}
	if model == "" {
		model = "claude-3-opus-20240229" // Default model
	}

	maxTokens := 2000
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	} else if profile.DefaultMaxTokens != nil {
		maxTokens = *profile.DefaultMaxTokens
	}

	// Chuẩn bị messages (Anthropic dùng format khác)
	messages := []map[string]interface{}{}
	
	// Thêm conversation history nếu có
	if len(req.Messages) > 0 {
		for _, msg := range req.Messages {
			messages = append(messages, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
	}
	
	// Thêm user prompt
	messages = append(messages, map[string]interface{}{
		"role":    "user",
		"content": req.Prompt,
	})

	// Chuẩn bị request body
	requestBody := map[string]interface{}{
		"model":     model,
		"max_tokens": maxTokens,
		"messages":  messages,
	}

	if req.SystemPrompt != "" {
		requestBody["system"] = req.SystemPrompt
	}

	if req.Temperature != nil {
		requestBody["temperature"] = *req.Temperature
	} else if profile.DefaultTemperature != nil {
		requestBody["temperature"] = *profile.DefaultTemperature
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi marshal request body: %v", err)
	}

	// Tạo HTTP request
	url := "https://api.anthropic.com/v1/messages"
	if profile.BaseURL != "" {
		url = profile.BaseURL + "/v1/messages"
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tạo HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", profile.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Gọi API
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi gọi Anthropic API: %v", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)

	// Đọc response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi đọc response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &AICallResponse{
			Latency: latency,
			Error:   fmt.Errorf("Anthropic API trả về lỗi (status %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// Parse response
	var anthropicResp struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Model string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("lỗi khi parse Anthropic response: %v", err)
	}

	if len(anthropicResp.Content) == 0 {
		return nil, errors.New("Anthropic response không có content")
	}

	content := anthropicResp.Content[0].Text

	return &AICallResponse{
		Content:       content,
		Model:         anthropicResp.Model,
		Usage:         &AIUsage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
		FinishReason: anthropicResp.StopReason,
		Latency:      latency,
	}, nil
}

// callGoogle gọi Google (Gemini) API
func (s *AIClientService) callGoogle(req AICallRequest, startTime time.Time) (*AICallResponse, error) {
	profile := req.ProviderProfile
	model := req.Model
	if model == "" {
		model = profile.DefaultModel
	}
	if model == "" {
		model = "gemini-1.5-pro" // Default model
	}

	temperature := 0.7
	if req.Temperature != nil {
		temperature = *req.Temperature
	} else if profile.DefaultTemperature != nil {
		temperature = *profile.DefaultTemperature
	}

	maxTokens := 2000
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	} else if profile.DefaultMaxTokens != nil {
		maxTokens = *profile.DefaultMaxTokens
	}

	log.Printf("[AIClient] [Google] Bắt đầu gọi Google Gemini API - Model: %s, Temperature: %.2f, MaxTokens: %d", model, temperature, maxTokens)

	// Chuẩn bị contents (Gemini dùng contents thay vì messages)
	contents := []map[string]interface{}{}
	
	// Thêm system prompt nếu có (Gemini không có system role riêng, thêm vào user message)
	userPrompt := req.Prompt
	if req.SystemPrompt != "" {
		userPrompt = req.SystemPrompt + "\n\n" + req.Prompt
	}
	
	// Thêm conversation history nếu có
	if len(req.Messages) > 0 {
		for _, msg := range req.Messages {
			role := "user"
			if msg.Role == "assistant" {
				role = "model"
			}
			contents = append(contents, map[string]interface{}{
				"role": role,
				"parts": []map[string]interface{}{
					{"text": msg.Content},
				},
			})
		}
	}
	
	// Thêm user prompt
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{"text": userPrompt},
		},
	})

	// Chuẩn bị request body
	requestBody := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":    temperature,
			"maxOutputTokens": maxTokens,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("[AIClient] [Google] ❌ Lỗi khi marshal request body: %v", err)
		return nil, fmt.Errorf("lỗi khi marshal request body: %v", err)
	}

	log.Printf("[AIClient] [Google] Request body size: %d bytes", len(bodyBytes))

	// Tạo HTTP request
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	if profile.BaseURL != "" {
		url = profile.BaseURL + fmt.Sprintf("/v1beta/models/%s:generateContent", model)
	}

	log.Printf("[AIClient] [Google] URL: %s", url)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		log.Printf("[AIClient] [Google] ❌ Lỗi khi tạo HTTP request: %v", err)
		return nil, fmt.Errorf("lỗi khi tạo HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", profile.APIKey)

	log.Printf("[AIClient] [Google] Đang gửi request...")

	// Gọi API
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("[AIClient] [Google] ❌ Lỗi khi gọi API: %v", err)
		return nil, fmt.Errorf("lỗi khi gọi Google API: %v", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)
	log.Printf("[AIClient] [Google] Response status: %d, Latency: %v", resp.StatusCode, latency)

	// Đọc response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[AIClient] [Google] ❌ Lỗi khi đọc response: %v", err)
		return nil, fmt.Errorf("lỗi khi đọc response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[AIClient] [Google] ❌ API trả về lỗi (status %d): %s", resp.StatusCode, string(respBody))
		return &AICallResponse{
			Latency: latency,
			Error:   fmt.Errorf("Google API trả về lỗi (status %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// Parse response
	var googleResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respBody, &googleResp); err != nil {
		log.Printf("[AIClient] [Google] ❌ Lỗi khi parse response: %v", err)
		return nil, fmt.Errorf("lỗi khi parse Google response: %v", err)
	}

	if len(googleResp.Candidates) == 0 {
		log.Printf("[AIClient] [Google] ❌ Response không có candidates")
		return nil, errors.New("Google response không có candidates")
	}

	content := googleResp.Candidates[0].Content.Parts[0].Text
	finishReason := googleResp.Candidates[0].FinishReason

	log.Printf("[AIClient] [Google] ✅ Thành công - Content length: %d chars, FinishReason: %s", len(content), finishReason)
	log.Printf("[AIClient] [Google] Usage - Prompt: %d, Completion: %d, Total: %d", 
		googleResp.UsageMetadata.PromptTokenCount, 
		googleResp.UsageMetadata.CandidatesTokenCount,
		googleResp.UsageMetadata.TotalTokenCount)

	return &AICallResponse{
		Content:       content,
		Model:         model,
		Usage:         &AIUsage{
			PromptTokens:     googleResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: googleResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      googleResp.UsageMetadata.TotalTokenCount,
		},
		FinishReason: finishReason,
		Latency:      latency,
	}, nil
}

// callCohere gọi Cohere API
func (s *AIClientService) callCohere(req AICallRequest, startTime time.Time) (*AICallResponse, error) {
	profile := req.ProviderProfile
	model := req.Model
	if model == "" {
		model = profile.DefaultModel
	}
	if model == "" {
		model = "command-nightly" // Default model
	}

	temperature := 0.7
	if req.Temperature != nil {
		temperature = *req.Temperature
	} else if profile.DefaultTemperature != nil {
		temperature = *profile.DefaultTemperature
	}

	maxTokens := 2000
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	} else if profile.DefaultMaxTokens != nil {
		maxTokens = *profile.DefaultMaxTokens
	}

	log.Printf("[AIClient] [Cohere] Bắt đầu gọi Cohere API - Model: %s, Temperature: %.2f, MaxTokens: %d", model, temperature, maxTokens)

	// Chuẩn bị chat history (Cohere dùng chat_history)
	chatHistory := []map[string]interface{}{}
	
	// Thêm conversation history nếu có
	if len(req.Messages) > 0 {
		for _, msg := range req.Messages {
			role := "USER"
			if msg.Role == "assistant" {
				role = "CHATBOT"
			} else if msg.Role == "system" {
				// Cohere không có system role, thêm vào message
				continue
			}
			chatHistory = append(chatHistory, map[string]interface{}{
				"role":    role,
				"message": msg.Content,
			})
		}
	}

	// Chuẩn bị message (kết hợp system prompt và user prompt)
	message := req.Prompt
	if req.SystemPrompt != "" {
		message = req.SystemPrompt + "\n\n" + req.Prompt
	}

	// Chuẩn bị request body
	requestBody := map[string]interface{}{
		"model":        model,
		"message":      message,
		"temperature":  temperature,
		"max_tokens":   maxTokens,
	}

	if len(chatHistory) > 0 {
		requestBody["chat_history"] = chatHistory
	}

	// Thêm response_format nếu cần JSON
	requestBody["response_format"] = map[string]interface{}{
		"type": "json_object",
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("[AIClient] [Cohere] ❌ Lỗi khi marshal request body: %v", err)
		return nil, fmt.Errorf("lỗi khi marshal request body: %v", err)
	}

	log.Printf("[AIClient] [Cohere] Request body size: %d bytes", len(bodyBytes))

	// Tạo HTTP request
	url := "https://api.cohere.ai/v1/chat"
	if profile.BaseURL != "" {
		url = profile.BaseURL + "/v1/chat"
	}

	log.Printf("[AIClient] [Cohere] URL: %s", url)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		log.Printf("[AIClient] [Cohere] ❌ Lỗi khi tạo HTTP request: %v", err)
		return nil, fmt.Errorf("lỗi khi tạo HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+profile.APIKey)
	httpReq.Header.Set("Accept", "application/json")

	log.Printf("[AIClient] [Cohere] Đang gửi request...")

	// Gọi API
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("[AIClient] [Cohere] ❌ Lỗi khi gọi API: %v", err)
		return nil, fmt.Errorf("lỗi khi gọi Cohere API: %v", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)
	log.Printf("[AIClient] [Cohere] Response status: %d, Latency: %v", resp.StatusCode, latency)

	// Đọc response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[AIClient] [Cohere] ❌ Lỗi khi đọc response: %v", err)
		return nil, fmt.Errorf("lỗi khi đọc response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[AIClient] [Cohere] ❌ API trả về lỗi (status %d): %s", resp.StatusCode, string(respBody))
		return &AICallResponse{
			Latency: latency,
			Error:   fmt.Errorf("Cohere API trả về lỗi (status %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// Parse response
	var cohereResp struct {
		Text         string `json:"text"`
		GenerationID string `json:"generation_id"`
		FinishReason string `json:"finish_reason"`
		Meta         struct {
			APIVersion struct {
				Version string `json:"version"`
			} `json:"api_version"`
			BilledUnits struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"billed_units"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(respBody, &cohereResp); err != nil {
		log.Printf("[AIClient] [Cohere] ❌ Lỗi khi parse response: %v", err)
		return nil, fmt.Errorf("lỗi khi parse Cohere response: %v", err)
	}

	log.Printf("[AIClient] [Cohere] ✅ Thành công - Content length: %d chars, FinishReason: %s", len(cohereResp.Text), cohereResp.FinishReason)
	log.Printf("[AIClient] [Cohere] Usage - Input: %d, Output: %d", 
		cohereResp.Meta.BilledUnits.InputTokens, 
		cohereResp.Meta.BilledUnits.OutputTokens)

	return &AICallResponse{
		Content:       cohereResp.Text,
		Model:         model,
		Usage:         &AIUsage{
			PromptTokens:     cohereResp.Meta.BilledUnits.InputTokens,
			CompletionTokens: cohereResp.Meta.BilledUnits.OutputTokens,
			TotalTokens:      cohereResp.Meta.BilledUnits.InputTokens + cohereResp.Meta.BilledUnits.OutputTokens,
		},
		FinishReason: cohereResp.FinishReason,
		Latency:      latency,
	}, nil
}

// callCustom gọi Custom provider API
// Custom provider có thể có format riêng, tạm thời dùng OpenAI-compatible format
func (s *AIClientService) callCustom(req AICallRequest, startTime time.Time) (*AICallResponse, error) {
	profile := req.ProviderProfile
	
	if profile.BaseURL == "" {
		log.Printf("[AIClient] [Custom] ❌ Custom provider cần BaseURL")
		return nil, errors.New("Custom provider cần BaseURL trong config")
	}

	model := req.Model
	if model == "" {
		model = profile.DefaultModel
	}
	if model == "" {
		model = "default" // Default model
	}

	temperature := 0.7
	if req.Temperature != nil {
		temperature = *req.Temperature
	} else if profile.DefaultTemperature != nil {
		temperature = *profile.DefaultTemperature
	}

	maxTokens := 2000
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	} else if profile.DefaultMaxTokens != nil {
		maxTokens = *profile.DefaultMaxTokens
	}

	log.Printf("[AIClient] [Custom] Bắt đầu gọi Custom provider API - BaseURL: %s, Model: %s, Temperature: %.2f, MaxTokens: %d", 
		profile.BaseURL, model, temperature, maxTokens)

	// Chuẩn bị messages (dùng OpenAI-compatible format)
	messages := []map[string]interface{}{}
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	
	// Thêm conversation history nếu có
	if len(req.Messages) > 0 {
		for _, msg := range req.Messages {
			messages = append(messages, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
	}
	
	// Thêm user prompt
	messages = append(messages, map[string]interface{}{
		"role":    "user",
		"content": req.Prompt,
	})

	// Chuẩn bị request body (OpenAI-compatible format)
	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}

	// Thêm custom config nếu có
	if profile.Config != nil {
		for k, v := range profile.Config {
			requestBody[k] = v
		}
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("[AIClient] [Custom] ❌ Lỗi khi marshal request body: %v", err)
		return nil, fmt.Errorf("lỗi khi marshal request body: %v", err)
	}

	log.Printf("[AIClient] [Custom] Request body size: %d bytes", len(bodyBytes))

	// Tạo HTTP request
	// Custom provider có thể dùng endpoint khác, mặc định dùng /v1/chat/completions
	endpoint := "/v1/chat/completions"
	if profile.Config != nil {
		if customEndpoint, ok := profile.Config["endpoint"].(string); ok && customEndpoint != "" {
			endpoint = customEndpoint
		}
	}

	url := profile.BaseURL + endpoint
	log.Printf("[AIClient] [Custom] URL: %s", url)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		log.Printf("[AIClient] [Custom] ❌ Lỗi khi tạo HTTP request: %v", err)
		return nil, fmt.Errorf("lỗi khi tạo HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if profile.APIKey != "" {
		// Custom provider có thể dùng header khác, mặc định dùng Authorization
		authHeader := "Bearer " + profile.APIKey
		if profile.Config != nil {
			if authHeaderFormat, ok := profile.Config["authHeaderFormat"].(string); ok && authHeaderFormat != "" {
				authHeader = authHeaderFormat
			}
			if authHeaderName, ok := profile.Config["authHeaderName"].(string); ok && authHeaderName != "" {
				httpReq.Header.Set(authHeaderName, authHeader)
			} else {
				httpReq.Header.Set("Authorization", authHeader)
			}
		} else {
			httpReq.Header.Set("Authorization", authHeader)
		}
	}

	log.Printf("[AIClient] [Custom] Đang gửi request...")

	// Gọi API
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("[AIClient] [Custom] ❌ Lỗi khi gọi API: %v", err)
		return nil, fmt.Errorf("lỗi khi gọi Custom API: %v", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)
	log.Printf("[AIClient] [Custom] Response status: %d, Latency: %v", resp.StatusCode, latency)

	// Đọc response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[AIClient] [Custom] ❌ Lỗi khi đọc response: %v", err)
		return nil, fmt.Errorf("lỗi khi đọc response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[AIClient] [Custom] ❌ API trả về lỗi (status %d): %s", resp.StatusCode, string(respBody))
		return &AICallResponse{
			Latency: latency,
			Error:   fmt.Errorf("Custom API trả về lỗi (status %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// Parse response (OpenAI-compatible format)
	var customResp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
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

	if err := json.Unmarshal(respBody, &customResp); err != nil {
		log.Printf("[AIClient] [Custom] ❌ Lỗi khi parse response: %v", err)
		return nil, fmt.Errorf("lỗi khi parse Custom response: %v", err)
	}

	if len(customResp.Choices) == 0 {
		log.Printf("[AIClient] [Custom] ❌ Response không có choices")
		return nil, errors.New("Custom response không có choices")
	}

	content := customResp.Choices[0].Message.Content
	finishReason := customResp.Choices[0].FinishReason

	log.Printf("[AIClient] [Custom] ✅ Thành công - Content length: %d chars, FinishReason: %s", len(content), finishReason)
	log.Printf("[AIClient] [Custom] Usage - Prompt: %d, Completion: %d, Total: %d", 
		customResp.Usage.PromptTokens, 
		customResp.Usage.CompletionTokens,
		customResp.Usage.TotalTokens)

	return &AICallResponse{
		Content:       content,
		Model:         customResp.Model,
		Usage:         &AIUsage{
			PromptTokens:     customResp.Usage.PromptTokens,
			CompletionTokens: customResp.Usage.CompletionTokens,
			TotalTokens:      customResp.Usage.TotalTokens,
		},
		FinishReason: finishReason,
		Latency:      latency,
	}, nil
}
