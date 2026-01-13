/*
Package services chứa các services hỗ trợ cho agent.
File này chứa WorkflowExecutor - service để thực thi workflow commands
*/
package services

import (
	"agent_pancake/app/integrations"
	"fmt"
	"log"
)

// WorkflowExecutor là service để thực thi workflow
type WorkflowExecutor struct {
	aiClient *AIClientService
}

// NewWorkflowExecutor tạo một instance mới của WorkflowExecutor
func NewWorkflowExecutor() *WorkflowExecutor {
	return &WorkflowExecutor{
		aiClient: NewAIClientService(),
	}
}

// ExecuteWorkflow thực thi một workflow
// Tham số:
// - workflowId: ID của workflow
// - rootRefId: ID của root reference (parent node)
// - rootRefType: Type của root reference
// - params: Additional parameters
// - agentId: ID của agent (để update heartbeat)
// - commandID: ID của command (để update heartbeat)
// Trả về workflowRunID và error
func (e *WorkflowExecutor) ExecuteWorkflow(workflowId, rootRefId, rootRefType string, params map[string]interface{}, agentId, commandID string) (string, error) {
	log.Printf("[WorkflowExecutor] ========================================")
	log.Printf("[WorkflowExecutor] 🚀 BẮT ĐẦU EXECUTE WORKFLOW")
	log.Printf("[WorkflowExecutor] WorkflowId: %s", workflowId)
	log.Printf("[WorkflowExecutor] RootRefId: %s", rootRefId)
	log.Printf("[WorkflowExecutor] RootRefType: %s", rootRefType)
	log.Printf("[WorkflowExecutor] Params: %+v", params)
	log.Printf("[WorkflowExecutor] ========================================")

	// 1. Load workflow definition
	log.Printf("[WorkflowExecutor] [1/5] Đang load workflow definition từ backend...")
	workflowResp, err := integrations.FolkForm_GetWorkflow(workflowId)
	if err != nil {
		log.Printf("[WorkflowExecutor] ❌ Lỗi khi load workflow: %v", err)
		return "", fmt.Errorf("lỗi khi load workflow: %v", err)
	}
	log.Printf("[WorkflowExecutor] ✅ Đã load workflow definition thành công")

	workflowData, ok := workflowResp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("workflow response không hợp lệ")
	}

	// Extract steps từ workflow
	stepsInterface, ok := workflowData["steps"].([]interface{})
	if !ok {
		log.Printf("[WorkflowExecutor] ❌ Workflow không có steps")
		return "", fmt.Errorf("workflow không có steps")
	}
	log.Printf("[WorkflowExecutor] Workflow có %d step(s)", len(stepsInterface))

	// 2. Tạo workflow run record
	log.Printf("[WorkflowExecutor] [2/5] Đang tạo workflow run record trong backend...")
	workflowRunResp, err := integrations.FolkForm_CreateWorkflowRun(workflowId, rootRefId, rootRefType, params)
	if err != nil {
		return "", fmt.Errorf("lỗi khi tạo workflow run: %v", err)
	}

	workflowRunData, ok := workflowRunResp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("workflow run response không hợp lệ")
	}

	workflowRunID, ok := workflowRunData["id"].(string)
	if !ok {
		return "", fmt.Errorf("workflow run không có ID")
	}

	log.Printf("[WorkflowExecutor] ✅ Đã tạo workflow run: %s", workflowRunID)

	// 3. Load root content từ Module 1
	log.Printf("[WorkflowExecutor] [3/5] Đang load root content từ Module 1...")
	log.Printf("[WorkflowExecutor] RootRefId: %s, RootRefType: %s", rootRefId, rootRefType)
	rootContent, err := e.loadRootContent(rootRefId, rootRefType)
	if err != nil {
		log.Printf("[WorkflowExecutor] ❌ Lỗi khi load root content: %v", err)
		_, _ = integrations.FolkForm_UpdateWorkflowRun(workflowRunID, "failed")
		return workflowRunID, fmt.Errorf("lỗi khi load root content: %v", err)
	}
	log.Printf("[WorkflowExecutor] ✅ Đã load root content thành công")

	// 4. Execute từng step theo thứ tự
	totalSteps := len(stepsInterface)
	log.Printf("[WorkflowExecutor] [4/5] Bắt đầu execute %d step(s)...", totalSteps)
	currentParentId := rootRefId
	currentParentType := rootRefType

	for i, stepInterface := range stepsInterface {
		stepMap, ok := stepInterface.(map[string]interface{})
		if !ok {
			log.Printf("[WorkflowExecutor] ⚠️  Step không phải là map, bỏ qua")
			continue
		}

		stepId, ok := stepMap["id"].(string)
		if !ok || stepId == "" {
			log.Printf("[WorkflowExecutor] ⚠️  Step không có ID, bỏ qua")
			continue
		}

		stepNumber := i + 1
		log.Printf("[WorkflowExecutor] ───────────────────────────────────────")
		log.Printf("[WorkflowExecutor] 📍 EXECUTE STEP %d/%d", stepNumber, totalSteps)
		log.Printf("[WorkflowExecutor] StepId: %s", stepId)
		log.Printf("[WorkflowExecutor] CurrentParentId: %s", currentParentId)
		log.Printf("[WorkflowExecutor] CurrentParentType: %s", currentParentType)
		log.Printf("[WorkflowExecutor] ───────────────────────────────────────")

		// Update heartbeat
		percentage := int((float64(stepNumber) / float64(totalSteps)) * 100)
		log.Printf("[WorkflowExecutor] Update heartbeat - Step %d/%d (%d%%)", stepNumber, totalSteps, percentage)
		integrations.FolkForm_UpdateWorkflowCommandHeartbeat(agentId, commandID, map[string]interface{}{
			"step":       "executing_step",
			"percentage": percentage,
			"message":    fmt.Sprintf("Đang execute step %d/%d: %s", stepNumber, totalSteps, stepId),
		})

		// Execute step
		log.Printf("[WorkflowExecutor] Đang gọi StepExecutor để execute step...")
		stepExecutor := NewStepExecutor(e.aiClient)
		stepResult, err := stepExecutor.ExecuteStep(stepId, currentParentId, currentParentType, workflowRunID, rootContent)
		if err != nil {
			log.Printf("[WorkflowExecutor] ❌ Lỗi khi execute step %s: %v", stepId, err)
			integrations.FolkForm_UpdateWorkflowRun(workflowRunID, "failed")
			return workflowRunID, fmt.Errorf("lỗi khi execute step %s: %v", stepId, err)
		}

		log.Printf("[WorkflowExecutor] ✅ Step %d/%d hoàn thành thành công", stepNumber, totalSteps)
		log.Printf("[WorkflowExecutor] StepRunID: %s", stepResult.StepRunID)
		if stepResult.DraftNodeID != "" {
			log.Printf("[WorkflowExecutor] DraftNodeID được tạo: %s", stepResult.DraftNodeID)
		}
		if stepResult.SelectedCandidateID != "" {
			log.Printf("[WorkflowExecutor] SelectedCandidateID: %s", stepResult.SelectedCandidateID)
		}

		// Update parent cho step tiếp theo (nếu có draft node được tạo)
		if stepResult.DraftNodeID != "" {
			log.Printf("[WorkflowExecutor] Update parent cho step tiếp theo: %s", stepResult.DraftNodeID)
			currentParentId = stepResult.DraftNodeID
			// Update rootContent với draft node mới
			rootContent = map[string]interface{}{
				"id":   stepResult.DraftNodeID,
				"type": e.determineNodeType(currentParentType),
				"text": stepResult.Output["content"],
			}
		}

		log.Printf("[WorkflowExecutor] ───────────────────────────────────────")
	}

	// 5. Update workflow run status = "completed"
	log.Printf("[WorkflowExecutor] [5/5] Đang update workflow run status = completed...")
	_, err = integrations.FolkForm_UpdateWorkflowRun(workflowRunID, "completed")
	if err != nil {
		log.Printf("[WorkflowExecutor] ⚠️  Lỗi khi update workflow run status: %v", err)
	} else {
		log.Printf("[WorkflowExecutor] ✅ Đã update workflow run status = completed")
	}

	log.Printf("[WorkflowExecutor] ========================================")
	log.Printf("[WorkflowExecutor] ✅ HOÀN THÀNH WORKFLOW")
	log.Printf("[WorkflowExecutor] WorkflowId: %s", workflowId)
	log.Printf("[WorkflowExecutor] WorkflowRunID: %s", workflowRunID)
	log.Printf("[WorkflowExecutor] Tổng số steps đã execute: %d", totalSteps)
	log.Printf("[WorkflowExecutor] ========================================")
	return workflowRunID, nil
}

// determineNodeType xác định node type từ parent type (helper function)
func (e *WorkflowExecutor) determineNodeType(parentType string) string {
	mapping := map[string]string{
		"layer":        "stp",
		"stp":          "insight",
		"insight":      "content_line",
		"content_line": "gene",
		"gene":         "script",
		"script":       "video",
		"video":        "publication",
	}
	if nodeType, ok := mapping[parentType]; ok {
		return nodeType
	}
	return parentType
}

// loadRootContent load root content từ Module 1
func (e *WorkflowExecutor) loadRootContent(rootRefId, rootRefType string) (map[string]interface{}, error) {
	log.Printf("[WorkflowExecutor] [loadRootContent] Đang thử load từ production...")
	// Thử load từ production trước
	contentResp, err := integrations.FolkForm_GetContentNode(rootRefId)
	if err == nil {
		if data, ok := contentResp["data"].(map[string]interface{}); ok {
			log.Printf("[WorkflowExecutor] [loadRootContent] ✅ Đã load từ production")
			return data, nil
		}
		log.Printf("[WorkflowExecutor] [loadRootContent] ⚠️  Production response không hợp lệ, thử draft...")
	} else {
		log.Printf("[WorkflowExecutor] [loadRootContent] ⚠️  Không tìm thấy trong production: %v, thử draft...", err)
	}

	// Nếu không có trong production, thử load từ draft
	log.Printf("[WorkflowExecutor] [loadRootContent] Đang thử load từ draft...")
	draftResp, err := integrations.FolkForm_GetDraftNode(rootRefId)
	if err != nil {
		log.Printf("[WorkflowExecutor] [loadRootContent] ❌ Không tìm thấy trong draft: %v", err)
		return nil, fmt.Errorf("không tìm thấy content node hoặc draft node: %v", err)
	}

	if data, ok := draftResp["data"].(map[string]interface{}); ok {
		log.Printf("[WorkflowExecutor] [loadRootContent] ✅ Đã load từ draft")
		return data, nil
	}

	log.Printf("[WorkflowExecutor] [loadRootContent] ❌ Không thể parse draft response")
	return nil, fmt.Errorf("không thể parse content node response")
}
