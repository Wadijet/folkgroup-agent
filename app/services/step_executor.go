/*
Package services chứa các services hỗ trợ cho agent.
File này chứa StepExecutor - service để thực thi từng step trong workflow
*/
package services

import (
	"agent_pancake/app/integrations"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// StepResult là kết quả của việc execute step
type StepResult struct {
	StepRunID    string
	DraftNodeID  string
	SelectedCandidateID string
	Output       map[string]interface{}
}

// StepExecutor là service để thực thi step
type StepExecutor struct {
	aiClient *AIClientService
}

// NewStepExecutor tạo một instance mới của StepExecutor
func NewStepExecutor(aiClient *AIClientService) *StepExecutor {
	return &StepExecutor{
		aiClient: aiClient,
	}
}

// ExecuteStep thực thi một step
// Tham số:
// - stepId: ID của step
// - parentId: ID của parent node
// - parentType: Type của parent node
// - workflowRunId: ID của workflow run
// - parentContent: Content của parent node
// Trả về StepResult và error
func (e *StepExecutor) ExecuteStep(stepId, parentId, parentType, workflowRunId string, parentContent map[string]interface{}) (*StepResult, error) {
	log.Printf("[StepExecutor] ========================================")
	log.Printf("[StepExecutor] 🚀 BẮT ĐẦU EXECUTE STEP")
	log.Printf("[StepExecutor] StepId: %s", stepId)
	log.Printf("[StepExecutor] ParentId: %s", parentId)
	log.Printf("[StepExecutor] ParentType: %s", parentType)
	log.Printf("[StepExecutor] WorkflowRunId: %s", workflowRunId)
	log.Printf("[StepExecutor] ========================================")

	// 1. Load step definition (chỉ để lấy stepType, inputSchema, outputSchema)
	log.Printf("[StepExecutor] [1/11] Đang load step definition từ backend...")
	stepResp, err := integrations.FolkForm_GetStep(stepId)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi load step: %v", err)
		return nil, fmt.Errorf("lỗi khi load step: %v", err)
	}

	stepData, ok := stepResp["data"].(map[string]interface{})
	if !ok {
		log.Printf("[StepExecutor] ❌ Step response không hợp lệ")
		return nil, fmt.Errorf("step response không hợp lệ")
	}

	stepType, _ := stepData["type"].(string)
	inputSchema, _ := stepData["inputSchema"].(map[string]interface{})
	outputSchema, _ := stepData["outputSchema"].(map[string]interface{})

	log.Printf("[StepExecutor] ✅ Đã load step definition")
	log.Printf("[StepExecutor] StepType: %s", stepType)

	// 2. Chuẩn bị input data và variables cho render-prompt
	log.Printf("[StepExecutor] [2/11] Đang chuẩn bị input data và variables...")
	stepInput := e.prepareStepInput(parentId, parentType, parentContent, inputSchema)
	
	// Chuẩn bị variables từ stepInput và parentContent để gửi cho render-prompt API
	variables := e.prepareVariablesForRenderPrompt(stepInput, parentContent)
	log.Printf("[StepExecutor] ✅ Đã chuẩn bị variables: %v", getMapKeys(variables))

	// 3. Gọi API render-prompt để lấy prompt đã render và AI config
	log.Printf("[StepExecutor] [3/11] Đang gọi API render-prompt...")
	renderResp, err := integrations.FolkForm_RenderPromptForStep(stepId, variables)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi render prompt: %v", err)
		return nil, fmt.Errorf("lỗi khi render prompt: %v", err)
	}

	renderData, ok := renderResp["data"].(map[string]interface{})
	if !ok {
		log.Printf("[StepExecutor] ❌ Render prompt response không hợp lệ")
		return nil, fmt.Errorf("render prompt response không hợp lệ")
	}

	// Lấy prompt đã render và AI config từ response
	promptText, _ := renderData["renderedPrompt"].(string)
	providerProfileId, _ := renderData["providerProfileId"].(string)
	model, _ := renderData["model"].(string)
	provider, _ := renderData["provider"].(string)
	temperature := getFloat64Ptr(renderData, "temperature")
	maxTokens := getIntPtr(renderData, "maxTokens")
	
	// Lấy promptTemplateId và templateType từ response (nếu có) hoặc từ step
	promptTemplateId := getString(renderData, "promptTemplateId")
	if promptTemplateId == "" {
		promptTemplateId = getString(stepData, "promptTemplateId")
	}
	
	// Lấy templateType từ render response hoặc từ step (nếu có)
	templateType := getString(renderData, "templateType")
	if templateType == "" {
		// Có thể lấy từ step hoặc dùng default
		templateType = "generate" // Default
	}

	log.Printf("[StepExecutor] ✅ Đã render prompt")
	log.Printf("[StepExecutor] Prompt length: %d chars", len(promptText))
	log.Printf("[StepExecutor] ProviderProfileId: %s", providerProfileId)
	log.Printf("[StepExecutor] Provider: %s, Model: %s", provider, model)
	if temperature != nil {
		log.Printf("[StepExecutor] Temperature: %.2f", *temperature)
	}
	if maxTokens != nil {
		log.Printf("[StepExecutor] MaxTokens: %d", *maxTokens)
	}
	log.Printf("[StepExecutor] Prompt preview (first 200 chars): %s", truncateString(promptText, 200))

	// 4. Load provider profile để lấy API key và config
	log.Printf("[StepExecutor] [4/11] Đang load provider profile từ backend...")
	providerResp, err := integrations.FolkForm_GetProviderProfile(providerProfileId)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi load provider profile: %v", err)
		return nil, fmt.Errorf("lỗi khi load provider profile: %v", err)
	}

	providerData, ok := providerResp["data"].(map[string]interface{})
	if !ok {
		log.Printf("[StepExecutor] ❌ Provider profile response không hợp lệ")
		return nil, fmt.Errorf("provider profile response không hợp lệ")
	}

	providerProfile := &AIProviderProfile{
		ID:                getString(providerData, "id"),
		Name:              getString(providerData, "name"),
		Provider:          getString(providerData, "provider"),
		APIKey:            getString(providerData, "apiKey"),
		BaseURL:           getString(providerData, "baseUrl"),
		OrganizationID:    getString(providerData, "organizationId"),
		DefaultModel:      getString(providerData, "defaultModel"),
		DefaultTemperature: getFloat64Ptr(providerData, "defaultTemperature"),
		DefaultMaxTokens:   getIntPtr(providerData, "defaultMaxTokens"),
	}

	log.Printf("[StepExecutor] ✅ Đã load provider profile")
	log.Printf("[StepExecutor] ProviderName: %s, ProviderType: %s", providerProfile.Name, providerProfile.Provider)

	// 5. Tạo step run record
	log.Printf("[StepExecutor] [5/11] Đang tạo step run record trong backend...")
	stepRunResp, err := integrations.FolkForm_CreateStepRun(workflowRunId, stepId, stepInput)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi tạo step run: %v", err)
		return nil, fmt.Errorf("lỗi khi tạo step run: %v", err)
	}

	stepRunData, ok := stepRunResp["data"].(map[string]interface{})
	if !ok {
		log.Printf("[StepExecutor] ❌ Step run response không hợp lệ")
		return nil, fmt.Errorf("step run response không hợp lệ")
	}

	stepRunID, ok := stepRunData["id"].(string)
	if !ok {
		log.Printf("[StepExecutor] ❌ Step run không có ID")
		return nil, fmt.Errorf("step run không có ID")
	}
	log.Printf("[StepExecutor] ✅ Đã tạo step run: %s", stepRunID)

	// 6. Tạo AI run record
	log.Printf("[StepExecutor] [6/11] Đang tạo AI run record trong backend...")
	aiRunResp, err := integrations.FolkForm_CreateAIRun(stepRunID, workflowRunId, promptTemplateId, providerProfileId, promptText)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi tạo AI run: %v", err)
		return nil, fmt.Errorf("lỗi khi tạo AI run: %v", err)
	}

	aiRunData, ok := aiRunResp["data"].(map[string]interface{})
	if !ok {
		log.Printf("[StepExecutor] ❌ AI run response không hợp lệ")
		return nil, fmt.Errorf("AI run response không hợp lệ")
	}

	aiRunID, ok := aiRunData["id"].(string)
	if !ok {
		log.Printf("[StepExecutor] ❌ AI run không có ID")
		return nil, fmt.Errorf("AI run không có ID")
	}
	log.Printf("[StepExecutor] ✅ Đã tạo AI run: %s", aiRunID)

	// 7. Gọi AI Provider API
	log.Printf("[StepExecutor] [7/11] ⚡ ĐANG GỌI AI PROVIDER API...")
	log.Printf("[StepExecutor] Provider: %s (%s)", providerProfile.Name, providerProfile.Provider)
	
	// Sử dụng model, temperature, maxTokens từ render-prompt response
	modelToUse := model
	if modelToUse == "" {
		modelToUse = providerProfile.DefaultModel
	}
	log.Printf("[StepExecutor] Model: %s", modelToUse)
	
	aiReq := AICallRequest{
		ProviderProfile: providerProfile,
		Model:           modelToUse,
		Prompt:          promptText,
		Temperature:     temperature, // Từ render-prompt response
		MaxTokens:       maxTokens,   // Từ render-prompt response
	}

	aiResp, err := e.aiClient.Call(aiReq)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi gọi AI API: %v", err)
		_, _ = integrations.FolkForm_UpdateAIRun(aiRunID, "", 0, 0, "failed")
		return nil, fmt.Errorf("lỗi khi gọi AI API: %v", err)
	}

	if aiResp.Error != nil {
		log.Printf("[StepExecutor] ❌ AI API trả về lỗi: %v", aiResp.Error)
		_, _ = integrations.FolkForm_UpdateAIRun(aiRunID, aiResp.Content, 0, aiResp.Latency.Milliseconds(), "failed")
		return nil, fmt.Errorf("AI API trả về lỗi: %v", aiResp.Error)
	}

	log.Printf("[StepExecutor] ✅ AI API call thành công!")
	log.Printf("[StepExecutor] Latency: %v", aiResp.Latency)
	log.Printf("[StepExecutor] Response length: %d chars", len(aiResp.Content))
	if aiResp.Usage != nil {
		log.Printf("[StepExecutor] Token usage - Prompt: %d, Completion: %d, Total: %d", 
			aiResp.Usage.PromptTokens, aiResp.Usage.CompletionTokens, aiResp.Usage.TotalTokens)
	}
	log.Printf("[StepExecutor] FinishReason: %s", aiResp.FinishReason)
	log.Printf("[StepExecutor] Response preview (first 200 chars): %s", truncateString(aiResp.Content, 200))

	// 9. Tính cost (tạm thời return 0, có thể tính từ pricing config sau)
	cost := 0.0
	log.Printf("[StepExecutor] Cost: $%.4f (tạm thời = 0)", cost)

	// 8. Update AI run record
	log.Printf("[StepExecutor] [8/11] Đang update AI run record với response...")
	_, err = integrations.FolkForm_UpdateAIRun(aiRunID, aiResp.Content, cost, aiResp.Latency.Milliseconds(), "completed")
	if err != nil {
		log.Printf("[StepExecutor] ⚠️  Lỗi khi update AI run: %v", err)
	} else {
		log.Printf("[StepExecutor] ✅ Đã update AI run record")
	}

	// 9. Parse AI response theo output schema
	log.Printf("[StepExecutor] [9/11] Đang parse AI response theo output schema...")
	// Lấy templateType từ render response (đã lấy ở bước 3)
	parsedOutput, err := e.parseAIResponse(aiResp.Content, outputSchema, templateType)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi parse AI response: %v", err)
		return nil, fmt.Errorf("lỗi khi parse AI response: %v", err)
	}
	
	// Thêm model và tokens vào parsed output nếu chưa có
	if _, ok := parsedOutput["model"]; !ok {
		parsedOutput["model"] = aiResp.Model
	}
	if _, ok := parsedOutput["tokens"]; !ok && aiResp.Usage != nil {
		parsedOutput["tokens"] = map[string]interface{}{
			"input":    aiResp.Usage.PromptTokens,
			"output":   aiResp.Usage.CompletionTokens,
			"total":    aiResp.Usage.TotalTokens,
		}
	}
	
	// Thêm generatedAt/judgedAt nếu chưa có (tùy theo step type)
	if stepType == "GENERATE" {
		if _, ok := parsedOutput["generatedAt"]; !ok {
			parsedOutput["generatedAt"] = time.Now().Format(time.RFC3339)
		}
	} else if stepType == "JUDGE" {
		if _, ok := parsedOutput["judgedAt"]; !ok {
			parsedOutput["judgedAt"] = time.Now().Format(time.RFC3339)
		}
	}
	
	log.Printf("[StepExecutor] ✅ Đã parse AI response thành công")
	log.Printf("[StepExecutor] Parsed output keys: %v", getMapKeys(parsedOutput))

	// 10. Xử lý theo step type
	log.Printf("[StepExecutor] [10/11] Đang xử lý theo step type: %s", stepType)
	var draftNodeID string
	var selectedCandidateID string

	if stepType == "GENERATE" {
		log.Printf("[StepExecutor] Xử lý GENERATE step...")
		// Tạo generation batch và candidates
		draftNodeID, selectedCandidateID, err = e.handleGenerateStep(stepRunID, aiRunID, parsedOutput, parentType)
		if err != nil {
			log.Printf("[StepExecutor] ❌ Lỗi khi handle GENERATE step: %v", err)
			return nil, fmt.Errorf("lỗi khi handle GENERATE step: %v", err)
		}
		log.Printf("[StepExecutor] ✅ GENERATE step hoàn thành - DraftNodeID: %s, SelectedCandidateID: %s", draftNodeID, selectedCandidateID)
	} else if stepType == "JUDGE" {
		log.Printf("[StepExecutor] Xử lý JUDGE step...")
		// Judge candidates và select best
		selectedCandidateID, err = e.handleJudgeStep(stepRunID, aiRunID, parsedOutput, parentId)
		if err != nil {
			log.Printf("[StepExecutor] ❌ Lỗi khi handle JUDGE step: %v", err)
			return nil, fmt.Errorf("lỗi khi handle JUDGE step: %v", err)
		}
		log.Printf("[StepExecutor] ✅ JUDGE step hoàn thành - SelectedCandidateID: %s", selectedCandidateID)
	} else {
		log.Printf("[StepExecutor] ⚠️  Step type không được xử lý: %s", stepType)
	}

	// 11. Update step run với output
	log.Printf("[StepExecutor] [11/11] Đang update step run với output...")
	_, err = integrations.FolkForm_UpdateStepRun(stepRunID, parsedOutput, "completed")
	if err != nil {
		log.Printf("[StepExecutor] ⚠️  Lỗi khi update step run: %v", err)
	} else {
		log.Printf("[StepExecutor] ✅ Đã update step run")
	}

	log.Printf("[StepExecutor] ✅ HOÀN THÀNH EXECUTE STEP")
	log.Printf("[StepExecutor] StepRunID: %s", stepRunID)
	if draftNodeID != "" {
		log.Printf("[StepExecutor] DraftNodeID: %s", draftNodeID)
	}
	if selectedCandidateID != "" {
		log.Printf("[StepExecutor] SelectedCandidateID: %s", selectedCandidateID)
	}
	log.Printf("[StepExecutor] ========================================")

	return &StepResult{
		StepRunID:          stepRunID,
		DraftNodeID:        draftNodeID,
		SelectedCandidateID: selectedCandidateID,
		Output:             parsedOutput,
	}, nil
}

// prepareStepInput chuẩn bị input data cho step
func (e *StepExecutor) prepareStepInput(parentId, parentType string, parentContent map[string]interface{}, inputSchema map[string]interface{}) map[string]interface{} {
	input := map[string]interface{}{
		"parentId":   parentId,
		"parentType": parentType,
	}

	// Thêm parent content vào input
	if parentContent != nil {
		input["parentContent"] = parentContent
	}

	return input
}

// prepareVariablesForRenderPrompt chuẩn bị variables từ stepInput và parentContent để gửi cho render-prompt API
func (e *StepExecutor) prepareVariablesForRenderPrompt(stepInput map[string]interface{}, parentContent map[string]interface{}) map[string]interface{} {
	variables := make(map[string]interface{})
	
	// Copy tất cả từ stepInput vào variables
	for k, v := range stepInput {
		variables[k] = v
	}
	
	// Thêm các fields từ parentContent nếu có
	if parentContent != nil {
		// Thêm parentContent text nếu có
		if text, ok := parentContent["text"].(string); ok {
			variables["parentContent"] = text
		} else if content, ok := parentContent["content"].(string); ok {
			variables["parentContent"] = content
		}
		
		// Thêm parentName nếu có
		if name, ok := parentContent["name"].(string); ok {
			variables["parentName"] = name
		}
		
		// Thêm layerName nếu có
		if layerName, ok := parentContent["layerName"].(string); ok {
			variables["layerName"] = layerName
		}
		
		// Thêm layerDescription nếu có
		if layerDesc, ok := parentContent["layerDescription"].(string); ok {
			variables["layerDescription"] = layerDesc
		}
		
		// Thêm targetAudience nếu có
		if targetAudience, ok := parentContent["targetAudience"].(string); ok {
			variables["targetAudience"] = targetAudience
		}
		
		// Thêm context nếu có
		if context, ok := parentContent["context"].(map[string]interface{}); ok {
			variables["context"] = context
		}
	}
	
	return variables
}

// generatePrompt generate prompt text từ template và variables
func (e *StepExecutor) generatePrompt(template string, stepInput map[string]interface{}, parentContent map[string]interface{}) (string, error) {
	// Simple variable substitution: {{variableName}}
	result := template

	// Replace variables từ stepInput
	for key, value := range stepInput {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	// Replace variables từ parentContent
	if parentContent != nil {
		if text, ok := parentContent["text"].(string); ok {
			result = strings.ReplaceAll(result, "{{parentText}}", text)
		}
		if name, ok := parentContent["name"].(string); ok {
			result = strings.ReplaceAll(result, "{{parentName}}", name)
		}
	}

	return result, nil
}

// parseAIResponse parse AI response theo output schema
func (e *StepExecutor) parseAIResponse(responseText string, outputSchema map[string]interface{}, templateType string) (map[string]interface{}, error) {
	// Nếu template type là "generate" hoặc "judge", response thường là JSON
	if templateType == "generate" || templateType == "judge" {
		// Thử parse JSON trực tiếp
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(responseText), &parsed); err != nil {
			// Nếu không parse được, thử extract JSON từ markdown code block
			jsonText := e.extractJSONFromMarkdown(responseText)
			if jsonText != "" {
				if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
					log.Printf("[StepExecutor] [parseAIResponse] ⚠️  Không thể parse JSON từ markdown: %v", err)
					return map[string]interface{}{
						"content": responseText,
					}, nil
				}
			} else {
				// Nếu không có JSON, return như plain text
				log.Printf("[StepExecutor] [parseAIResponse] ⚠️  Response không phải JSON, trả về như plain text")
				return map[string]interface{}{
					"content": responseText,
				}, nil
			}
		}
		
		// Thêm generatedAt nếu chưa có
		if _, ok := parsed["generatedAt"]; !ok {
			parsed["generatedAt"] = time.Now().Format(time.RFC3339)
		}
		
		return parsed, nil
	}

	// Default: return as text
	return map[string]interface{}{
		"content": responseText,
	}, nil
}

// extractJSONFromMarkdown extract JSON từ markdown code block (```json ... ```)
func (e *StepExecutor) extractJSONFromMarkdown(text string) string {
	// Tìm code block với ```json hoặc ```
	startMarker := "```json"
	endMarker := "```"
	
	startIdx := strings.Index(text, startMarker)
	if startIdx == -1 {
		// Thử tìm với ``` thông thường
		startMarker = "```"
		startIdx = strings.Index(text, startMarker)
		if startIdx == -1 {
			return ""
		}
		startIdx += len(startMarker)
	} else {
		startIdx += len(startMarker)
	}
	
	// Bỏ qua whitespace sau start marker
	for startIdx < len(text) && (text[startIdx] == ' ' || text[startIdx] == '\n' || text[startIdx] == '\r') {
		startIdx++
	}
	
	// Tìm end marker
	endIdx := strings.Index(text[startIdx:], endMarker)
	if endIdx == -1 {
		return ""
	}
	endIdx += startIdx
	
	jsonText := strings.TrimSpace(text[startIdx:endIdx])
	return jsonText
}

// handleGenerateStep xử lý GENERATE step: tạo candidates và draft node
func (e *StepExecutor) handleGenerateStep(stepRunID, aiRunID string, parsedOutput map[string]interface{}, parentType string) (string, string, error) {
	log.Printf("[StepExecutor] [handleGenerateStep] Bắt đầu xử lý GENERATE step...")
	
	// Extract candidates từ parsed output
	candidatesInterface, ok := parsedOutput["candidates"].([]interface{})
	if !ok {
		log.Printf("[StepExecutor] [handleGenerateStep] ❌ Parsed output không có candidates")
		return "", "", fmt.Errorf("parsed output không có candidates")
	}
	log.Printf("[StepExecutor] [handleGenerateStep] Tìm thấy %d candidate(s)", len(candidatesInterface))

	// Tạo generation batch
	log.Printf("[StepExecutor] [handleGenerateStep] Đang tạo generation batch...")
	batchResp, err := integrations.FolkForm_CreateGenerationBatch(stepRunID, len(candidatesInterface))
	if err != nil {
		log.Printf("[StepExecutor] [handleGenerateStep] ❌ Lỗi khi tạo generation batch: %v", err)
		return "", "", fmt.Errorf("lỗi khi tạo generation batch: %v", err)
	}

	batchData, ok := batchResp["data"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("generation batch response không hợp lệ")
	}

	batchID, ok := batchData["id"].(string)
	if !ok {
		log.Printf("[StepExecutor] [handleGenerateStep] ❌ Generation batch không có ID")
		return "", "", fmt.Errorf("generation batch không có ID")
	}
	log.Printf("[StepExecutor] [handleGenerateStep] ✅ Đã tạo generation batch: %s", batchID)

	// Tạo candidates
	log.Printf("[StepExecutor] [handleGenerateStep] Đang tạo %d candidate(s)...", len(candidatesInterface))
	var candidateIDs []string
	var candidateDataList []map[string]interface{} // Lưu candidate data để dùng sau
	
	for i, candidateInterface := range candidatesInterface {
		candidateMap, ok := candidateInterface.(map[string]interface{})
		if !ok {
			log.Printf("[StepExecutor] [handleGenerateStep] ⚠️  Candidate %d không phải object, bỏ qua", i+1)
			continue
		}

		// Lấy content/text từ candidate
		text, ok := candidateMap["content"].(string)
		if !ok {
			// Thử "text" field
			text, ok = candidateMap["text"].(string)
			if !ok {
				log.Printf("[StepExecutor] [handleGenerateStep] ⚠️  Candidate %d không có content/text, bỏ qua", i+1)
				continue
			}
		}

		// Tạo candidate trong backend
		candidateResp, err := integrations.FolkForm_CreateCandidate(batchID, aiRunID, text)
		if err != nil {
			log.Printf("[StepExecutor] [handleGenerateStep] ⚠️  Lỗi khi tạo candidate %d: %v", i+1, err)
			continue
		}

		if candidateData, ok := candidateResp["data"].(map[string]interface{}); ok {
			if candidateID, ok := candidateData["id"].(string); ok {
				candidateIDs = append(candidateIDs, candidateID)
				
				// Lưu candidate data với đầy đủ thông tin (title, summary, metadata)
				candidateInfo := map[string]interface{}{
					"candidateId": candidateID,
					"content":    text,
				}
				
				// Thêm title nếu có
				if title, ok := candidateMap["title"].(string); ok && title != "" {
					candidateInfo["title"] = title
				}
				
				// Thêm summary nếu có
				if summary, ok := candidateMap["summary"].(string); ok && summary != "" {
					candidateInfo["summary"] = summary
				}
				
				// Thêm metadata nếu có
				if metadata, ok := candidateMap["metadata"].(map[string]interface{}); ok {
					candidateInfo["metadata"] = metadata
				}
				
				candidateDataList = append(candidateDataList, candidateInfo)
				
				log.Printf("[StepExecutor] [handleGenerateStep] ✅ Đã tạo candidate %d: %s (text length: %d)", i+1, candidateID, len(text))
			}
		}
	}

	if len(candidateIDs) == 0 {
		log.Printf("[StepExecutor] [handleGenerateStep] ❌ Không tạo được candidate nào")
		return "", "", fmt.Errorf("không tạo được candidate nào")
	}
	log.Printf("[StepExecutor] [handleGenerateStep] ✅ Đã tạo %d candidate(s) thành công", len(candidateIDs))

	// Select candidate đầu tiên (sẽ được judge sau)
	selectedCandidateID := candidateIDs[0]
	log.Printf("[StepExecutor] [handleGenerateStep] Selected candidate: %s", selectedCandidateID)

	// Tạo draft node từ selected candidate
	// Determine node type từ parentType (ví dụ: layer -> stp, stp -> insight, etc.)
	nodeType := e.determineNodeType(parentType)
	log.Printf("[StepExecutor] [handleGenerateStep] Node type: %s (từ parentType: %s)", nodeType, parentType)

	// Get candidate text từ candidate đầu tiên
	selectedCandidateText := ""
	if len(candidateDataList) > 0 {
		if content, ok := candidateDataList[0]["content"].(string); ok {
			selectedCandidateText = content
		}
	}
	
	// Fallback: lấy từ candidatesInterface nếu không có trong candidateDataList
	if selectedCandidateText == "" {
		for _, candidateInterface := range candidatesInterface {
			candidateMap, ok := candidateInterface.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := candidateMap["content"].(string); ok {
				selectedCandidateText = text
				break
			}
		}
	}

	log.Printf("[StepExecutor] [handleGenerateStep] Đang tạo draft node...")
	log.Printf("[StepExecutor] [handleGenerateStep] NodeType: %s, TextLength: %d", nodeType, len(selectedCandidateText))
	draftResp, err := integrations.FolkForm_CreateDraftNode(nodeType, selectedCandidateText, "", "", selectedCandidateID)
	if err != nil {
		log.Printf("[StepExecutor] [handleGenerateStep] ❌ Lỗi khi tạo draft node: %v", err)
		return "", selectedCandidateID, fmt.Errorf("lỗi khi tạo draft node: %v", err)
	}

	draftData, ok := draftResp["data"].(map[string]interface{})
	if !ok {
		log.Printf("[StepExecutor] [handleGenerateStep] ❌ Draft node response không hợp lệ")
		return "", selectedCandidateID, fmt.Errorf("draft node response không hợp lệ")
	}

	draftNodeID, ok := draftData["id"].(string)
	if !ok {
		log.Printf("[StepExecutor] [handleGenerateStep] ❌ Draft node không có ID")
		return "", selectedCandidateID, fmt.Errorf("draft node không có ID")
	}

	log.Printf("[StepExecutor] [handleGenerateStep] ✅ Đã tạo draft node: %s", draftNodeID)
	return draftNodeID, selectedCandidateID, nil
}

// handleJudgeStep xử lý JUDGE step: judge candidates và select best
func (e *StepExecutor) handleJudgeStep(stepRunID, aiRunID string, parsedOutput map[string]interface{}, parentId string) (string, error) {
	log.Printf("[StepExecutor] [handleJudgeStep] Bắt đầu xử lý JUDGE step...")
	
	// Thêm judgedAt nếu chưa có
	if _, ok := parsedOutput["judgedAt"]; !ok {
		parsedOutput["judgedAt"] = time.Now().Format(time.RFC3339)
	}
	
	// Lấy bestCandidate từ parsed output
	var bestCandidateID string
	
	// Ưu tiên 1: Lấy từ bestCandidate
	if bestCandidate, ok := parsedOutput["bestCandidate"].(map[string]interface{}); ok {
		if candidateId, ok := bestCandidate["candidateId"].(string); ok && candidateId != "" {
			bestCandidateID = candidateId
			if score, ok := bestCandidate["score"].(float64); ok {
				log.Printf("[StepExecutor] [handleJudgeStep] ✅ Tìm thấy bestCandidate: %s (score: %.2f)", bestCandidateID, score)
			} else {
				log.Printf("[StepExecutor] [handleJudgeStep] ✅ Tìm thấy bestCandidate: %s", bestCandidateID)
			}
		}
	}
	
	// Ưu tiên 2: Lấy từ rankings (candidate có rank = 1 hoặc score cao nhất)
	if bestCandidateID == "" {
		if rankings, ok := parsedOutput["rankings"].([]interface{}); ok && len(rankings) > 0 {
			// Lấy candidate đầu tiên trong rankings (đã được sắp xếp theo score)
			firstRanking, ok := rankings[0].(map[string]interface{})
			if ok {
				if candidateId, ok := firstRanking["candidateId"].(string); ok && candidateId != "" {
					bestCandidateID = candidateId
					if score, ok := firstRanking["score"].(float64); ok {
						log.Printf("[StepExecutor] [handleJudgeStep] ✅ Tìm thấy bestCandidate từ rankings: %s (score: %.2f, rank: 1)", bestCandidateID, score)
					} else {
						log.Printf("[StepExecutor] [handleJudgeStep] ✅ Tìm thấy bestCandidate từ rankings: %s (rank: 1)", bestCandidateID)
					}
				}
			}
		}
	}
	
	// Ưu tiên 3: Lấy từ scores (candidate có overallScore cao nhất)
	if bestCandidateID == "" {
		if scores, ok := parsedOutput["scores"].([]interface{}); ok && len(scores) > 0 {
			var bestScore float64 = -1
			var bestCandidateFromScores string
			
			for _, scoreItem := range scores {
				scoreMap, ok := scoreItem.(map[string]interface{})
				if !ok {
					continue
				}
				
				if candidateId, ok := scoreMap["candidateId"].(string); ok && candidateId != "" {
					if overallScore, ok := scoreMap["overallScore"].(float64); ok {
						if overallScore > bestScore {
							bestScore = overallScore
							bestCandidateFromScores = candidateId
						}
					}
				}
			}
			
			if bestCandidateFromScores != "" {
				bestCandidateID = bestCandidateFromScores
				log.Printf("[StepExecutor] [handleJudgeStep] ✅ Tìm thấy bestCandidate từ scores: %s (overallScore: %.2f)", bestCandidateID, bestScore)
			}
		}
	}
	
	// Nếu vẫn không tìm thấy
	if bestCandidateID == "" {
		log.Printf("[StepExecutor] [handleJudgeStep] ❌ Không tìm thấy bestCandidate trong parsed output")
		log.Printf("[StepExecutor] [handleJudgeStep] Parsed output keys: %v", getMapKeys(parsedOutput))
		return "", fmt.Errorf("không tìm thấy bestCandidate trong parsed output")
	}
	
	log.Printf("[StepExecutor] [handleJudgeStep] ✅ Đã chọn bestCandidate: %s", bestCandidateID)
	return bestCandidateID, nil
}

// determineNodeType xác định node type từ parent type
func (e *StepExecutor) determineNodeType(parentType string) string {
	// Mapping: layer -> stp, stp -> insight, insight -> content_line, etc.
	mapping := map[string]string{
		"layer":       "stp",
		"stp":         "insight",
		"insight":     "content_line",
		"content_line": "gene",
		"gene":        "script",
		"script":      "video",
		"video":       "publication",
	}

	if nodeType, ok := mapping[parentType]; ok {
		return nodeType
	}

	// Default: return parentType
	return parentType
}

// Helper functions
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFloat64Ptr(data map[string]interface{}, key string) *float64 {
	if val, ok := data[key]; ok {
		if f, ok := val.(float64); ok {
			return &f
		}
		if str, ok := val.(string); ok {
			if f, err := strconv.ParseFloat(str, 64); err == nil {
				return &f
			}
		}
	}
	return nil
}

func getIntPtr(data map[string]interface{}, key string) *int {
	if val, ok := data[key]; ok {
		if i, ok := val.(int); ok {
			return &i
		}
		if f, ok := val.(float64); ok {
			i := int(f)
			return &i
		}
		if str, ok := val.(string); ok {
			if i, err := strconv.Atoi(str); err == nil {
				return &i
			}
		}
	}
	return nil
}

// Helper functions cho logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
