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

	// 1. Load step definition
	log.Printf("[StepExecutor] [1/13] Đang load step definition từ backend...")
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
	promptTemplateId, _ := stepData["promptTemplateId"].(string)
	providerProfileId, _ := stepData["providerProfileId"].(string)
	inputSchema, _ := stepData["inputSchema"].(map[string]interface{})
	outputSchema, _ := stepData["outputSchema"].(map[string]interface{})

	log.Printf("[StepExecutor] ✅ Đã load step definition")
	log.Printf("[StepExecutor] StepType: %s", stepType)
	log.Printf("[StepExecutor] PromptTemplateId: %s", promptTemplateId)
	log.Printf("[StepExecutor] ProviderProfileId: %s", providerProfileId)

	// 2. Load prompt template
	log.Printf("[StepExecutor] [2/13] Đang load prompt template từ backend...")
	promptTemplateResp, err := integrations.FolkForm_GetPromptTemplate(promptTemplateId)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi load prompt template: %v", err)
		return nil, fmt.Errorf("lỗi khi load prompt template: %v", err)
	}

	promptTemplateData, ok := promptTemplateResp["data"].(map[string]interface{})
	if !ok {
		log.Printf("[StepExecutor] ❌ Prompt template response không hợp lệ")
		return nil, fmt.Errorf("prompt template response không hợp lệ")
	}

	templateText, _ := promptTemplateData["template"].(string)
	templateType, _ := promptTemplateData["type"].(string)

	log.Printf("[StepExecutor] ✅ Đã load prompt template")
	log.Printf("[StepExecutor] TemplateType: %s, TemplateLength: %d chars", templateType, len(templateText))

	// 3. Load provider profile
	log.Printf("[StepExecutor] [3/13] Đang load provider profile từ backend...")
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

	// Convert provider data to AIProviderProfile
	providerName := getString(providerData, "name")
	providerType := getString(providerData, "provider")
	log.Printf("[StepExecutor] ✅ Đã load provider profile")
	log.Printf("[StepExecutor] ProviderName: %s, ProviderType: %s", providerName, providerType)

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

	// 4. Chuẩn bị input data cho step
	log.Printf("[StepExecutor] [4/13] Đang chuẩn bị input data cho step...")
	stepInput := e.prepareStepInput(parentId, parentType, parentContent, inputSchema)
	log.Printf("[StepExecutor] ✅ Đã chuẩn bị input data")

	// 5. Generate prompt text từ template
	log.Printf("[StepExecutor] [5/13] Đang generate prompt text từ template...")
	promptText, err := e.generatePrompt(templateText, stepInput, parentContent)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi generate prompt: %v", err)
		return nil, fmt.Errorf("lỗi khi generate prompt: %v", err)
	}
	log.Printf("[StepExecutor] ✅ Đã generate prompt text (length: %d chars)", len(promptText))
	log.Printf("[StepExecutor] Prompt preview (first 200 chars): %s", truncateString(promptText, 200))

	// 6. Tạo step run record
	log.Printf("[StepExecutor] [6/13] Đang tạo step run record trong backend...")
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

	// 7. Tạo AI run record
	log.Printf("[StepExecutor] [7/13] Đang tạo AI run record trong backend...")
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

	// 8. Gọi AI Provider API
	log.Printf("[StepExecutor] [8/13] ⚡ ĐANG GỌI AI PROVIDER API...")
	log.Printf("[StepExecutor] Provider: %s (%s)", providerProfile.Name, providerProfile.Provider)
	log.Printf("[StepExecutor] Model: %s", providerProfile.DefaultModel)
	aiReq := AICallRequest{
		ProviderProfile: providerProfile,
		Prompt:          promptText,
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

	// 10. Update AI run record
	log.Printf("[StepExecutor] [9/13] Đang update AI run record với response...")
	_, err = integrations.FolkForm_UpdateAIRun(aiRunID, aiResp.Content, cost, aiResp.Latency.Milliseconds(), "completed")
	if err != nil {
		log.Printf("[StepExecutor] ⚠️  Lỗi khi update AI run: %v", err)
	} else {
		log.Printf("[StepExecutor] ✅ Đã update AI run record")
	}

	// 11. Parse AI response theo output schema
	log.Printf("[StepExecutor] [10/13] Đang parse AI response theo output schema...")
	parsedOutput, err := e.parseAIResponse(aiResp.Content, outputSchema, templateType)
	if err != nil {
		log.Printf("[StepExecutor] ❌ Lỗi khi parse AI response: %v", err)
		return nil, fmt.Errorf("lỗi khi parse AI response: %v", err)
	}
	log.Printf("[StepExecutor] ✅ Đã parse AI response thành công")
	log.Printf("[StepExecutor] Parsed output keys: %v", getMapKeys(parsedOutput))

	// 12. Xử lý theo step type
	log.Printf("[StepExecutor] [11/13] Đang xử lý theo step type: %s", stepType)
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

	// 13. Update step run với output
	log.Printf("[StepExecutor] [12/13] Đang update step run với output...")
	_, err = integrations.FolkForm_UpdateStepRun(stepRunID, parsedOutput, "completed")
	if err != nil {
		log.Printf("[StepExecutor] ⚠️  Lỗi khi update step run: %v", err)
	} else {
		log.Printf("[StepExecutor] ✅ Đã update step run")
	}

	log.Printf("[StepExecutor] [13/13] ✅ HOÀN THÀNH EXECUTE STEP")
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
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(responseText), &parsed); err != nil {
			// Nếu không parse được JSON, thử parse như plain text
			return map[string]interface{}{
				"content": responseText,
			}, nil
		}
		return parsed, nil
	}

	// Default: return as text
	return map[string]interface{}{
		"content": responseText,
	}, nil
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
	for i, candidateInterface := range candidatesInterface {
		candidateMap, ok := candidateInterface.(map[string]interface{})
		if !ok {
			continue
		}

		text, ok := candidateMap["content"].(string)
		if !ok {
			// Thử "text" field
			text, ok = candidateMap["text"].(string)
			if !ok {
				continue
			}
		}

		candidateResp, err := integrations.FolkForm_CreateCandidate(batchID, aiRunID, text)
		if err != nil {
			log.Printf("[StepExecutor] [handleGenerateStep] ⚠️  Lỗi khi tạo candidate %d: %v", i+1, err)
			continue
		}

		if candidateData, ok := candidateResp["data"].(map[string]interface{}); ok {
			if candidateID, ok := candidateData["id"].(string); ok {
				candidateIDs = append(candidateIDs, candidateID)
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

	// Get candidate text
	selectedCandidateText := ""
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
	// TODO: Implement judge logic
	// Tạm thời return empty
	log.Printf("[StepExecutor] ⚠️  JUDGE step chưa được implement đầy đủ")
	return "", nil
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
