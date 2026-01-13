/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa WorkflowCommandsJob - job xử lý workflow commands từ Module 2 (AI Service).
Job này sẽ:
1. Claim pending workflow commands từ server (atomic operation)
2. Tạo worker (goroutine) để xử lý từng command
3. Worker gọi API Module 2 để start workflow run hoặc execute step
4. Update heartbeat định kỳ (mỗi 30-60 giây) để server biết job đang được thực hiện
5. Update command status sau khi hoàn thành
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"agent_pancake/app/services"
	"agent_pancake/global"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Global variable để track job instance (dùng để track active workers)
var globalWorkflowCommandsJob *WorkflowCommandsJob
var globalWorkflowCommandsJobMu sync.RWMutex

// WorkflowCommandsJob là job xử lý workflow commands từ Module 2
type WorkflowCommandsJob struct {
	*scheduler.BaseJob
	// Map để track các workers đang chạy (tránh xử lý duplicate commands)
	activeWorkers sync.Map // map[string]bool - key là commandID
}

// NewWorkflowCommandsJob tạo một instance mới của WorkflowCommandsJob
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy (ví dụ: "*/30 * * * * *" = mỗi 30 giây)
// Trả về một instance của WorkflowCommandsJob
func NewWorkflowCommandsJob(name, schedule string) *WorkflowCommandsJob {
	job := &WorkflowCommandsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)

	// Lưu job instance vào global variable để có thể truy cập từ worker
	globalWorkflowCommandsJobMu.Lock()
	globalWorkflowCommandsJob = job
	globalWorkflowCommandsJobMu.Unlock()

	return job
}

// ExecuteInternal thực thi logic claim và xử lý workflow commands
func (j *WorkflowCommandsJob) ExecuteInternal(ctx context.Context) error {
	// Logger riêng cho job này
	jobLogger := GetJobLoggerByName("workflow-commands-job")

	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 Workflow commands job bắt đầu")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Gọi hàm logic thực sự
	err := DoProcessWorkflowCommands()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoProcessWorkflowCommands thực thi logic claim và xử lý workflow commands
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface
func DoProcessWorkflowCommands() error {
	// Lấy logger riêng cho job này
	jobLogger := GetJobLoggerByName("workflow-commands-job")

	// Lấy agentId từ config
	agentId := global.GlobalConfig.AgentId
	if agentId == "" {
		jobLogger.Warn("⚠️  AgentId rỗng, không thể claim commands")
		return nil
	}

	// Lấy limit từ config (default: 5, max: 100)
	limit := GetJobConfigInt("workflow-commands-job", "claimLimit", 5)
	if limit > 100 {
		limit = 100
	}

	// Claim commands có status=pending (atomic operation)
	jobLogger.Info("Đang claim workflow commands từ server...")
	commands, err := integrations.FolkForm_ClaimWorkflowCommands(agentId, limit)
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi claim workflow commands")
		return err
	}

	if len(commands) == 0 {
		jobLogger.Debug("Không có command nào cần xử lý")
		return nil
	}

	jobLogger.WithField("count", len(commands)).Info(fmt.Sprintf("📥 Đã claim %d command(s) cần xử lý", len(commands)))

	// Xử lý từng command bằng cách tạo worker (goroutine)
	for _, cmdInterface := range commands {
		cmdMap, ok := cmdInterface.(map[string]interface{})
		if !ok {
			jobLogger.Warn("⚠️  Command không phải là map, bỏ qua")
			continue
		}

		// Lấy commandID
		commandID, ok := cmdMap["id"].(string)
		if !ok || commandID == "" {
			jobLogger.Warn("⚠️  Command không có ID, bỏ qua")
			continue
		}

		// Kiểm tra xem command này đã có worker đang xử lý chưa
		// (tránh xử lý duplicate nếu job chạy lại trước khi worker hoàn thành)
		jobInstance := getWorkflowCommandsJobInstance()
		if jobInstance != nil {
			if _, exists := jobInstance.activeWorkers.Load(commandID); exists {
				jobLogger.WithField("command_id", commandID).Debug("Command đang được xử lý, bỏ qua")
				continue
			}
			// Đánh dấu command đang được xử lý
			jobInstance.activeWorkers.Store(commandID, true)
		}

		// Tạo worker để xử lý command (chạy trong goroutine riêng)
		go processWorkflowCommand(commandID, cmdMap, agentId)
	}

	return nil
}

// processWorkflowCommand xử lý một workflow command cụ thể
// Hàm này chạy trong goroutine riêng để không block job chính
func processWorkflowCommand(commandID string, cmdMap map[string]interface{}, agentId string) {
	jobLogger := GetJobLoggerByName("workflow-commands-job")

	// Đảm bảo cleanup activeWorkers khi xong
	defer func() {
		jobInstance := getWorkflowCommandsJobInstance()
		if jobInstance != nil {
			jobInstance.activeWorkers.Delete(commandID)
		}
	}()

	jobLogger.WithField("command_id", commandID).Info("🔄 Bắt đầu xử lý workflow command")

	// Parse command data
	commandType, _ := cmdMap["commandType"].(string)
	workflowId, _ := cmdMap["workflowId"].(string)
	stepId, _ := cmdMap["stepId"].(string)
	rootRefId, _ := cmdMap["rootRefId"].(string)
	rootRefType, _ := cmdMap["rootRefType"].(string)

	// Parse params (có thể là map hoặc string JSON)
	var params map[string]interface{}
	if paramsInterface, ok := cmdMap["params"]; ok && paramsInterface != nil {
		if paramsMap, ok := paramsInterface.(map[string]interface{}); ok {
			params = paramsMap
		} else if paramsStr, ok := paramsInterface.(string); ok {
			// Nếu params là string JSON, parse nó
			if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
				jobLogger.WithError(err).WithField("command_id", commandID).Warn("⚠️  Không thể parse params JSON, dùng nil")
				params = nil
			}
		}
	}

	// Validate command type
	if commandType != "START_WORKFLOW" && commandType != "EXECUTE_STEP" {
		jobLogger.WithFields(map[string]interface{}{
			"command_id":   commandID,
			"command_type": commandType,
		}).Warn("⚠️  Command type không được hỗ trợ, chỉ hỗ trợ START_WORKFLOW hoặc EXECUTE_STEP")
		integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
			"error": fmt.Sprintf("Command type không được hỗ trợ: %s", commandType),
		})
		return
	}

	// Validate required fields theo command type
	if commandType == "START_WORKFLOW" {
		if workflowId == "" || rootRefId == "" || rootRefType == "" {
			jobLogger.WithFields(map[string]interface{}{
				"command_id":    commandID,
				"workflow_id":   workflowId,
				"root_ref_id":   rootRefId,
				"root_ref_type": rootRefType,
			}).Error("❌ START_WORKFLOW command thiếu thông tin bắt buộc")
			integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
				"error": "START_WORKFLOW command thiếu thông tin bắt buộc (workflowId, rootRefId, rootRefType)",
			})
			return
		}
	} else if commandType == "EXECUTE_STEP" {
		if stepId == "" || rootRefId == "" || rootRefType == "" {
			jobLogger.WithFields(map[string]interface{}{
				"command_id":    commandID,
				"step_id":       stepId,
				"root_ref_id":   rootRefId,
				"root_ref_type": rootRefType,
			}).Error("❌ EXECUTE_STEP command thiếu thông tin bắt buộc")
			integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
				"error": "EXECUTE_STEP command thiếu thông tin bắt buộc (stepId, rootRefId, rootRefType)",
			})
			return
		}
	}

	// Tạo context với timeout để có thể cancel heartbeat khi xong
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Tạo heartbeat ticker (update mỗi 45 giây - giữa 30-60 giây)
	heartbeatInterval := GetJobConfigInt("workflow-commands-job", "heartbeatInterval", 45)
	if heartbeatInterval < 30 {
		heartbeatInterval = 30
	}
	if heartbeatInterval > 60 {
		heartbeatInterval = 60
	}
	heartbeatTicker := time.NewTicker(time.Duration(heartbeatInterval) * time.Second)
	defer heartbeatTicker.Stop()

	// Channel để signal khi worker hoàn thành
	done := make(chan bool, 1)

	// Goroutine để update heartbeat định kỳ
	go func() {
		for {
			select {
			case <-heartbeatTicker.C:
				// Update heartbeat với progress
				progress := map[string]interface{}{
					"step":       "processing",
					"percentage": 0,
					"message":    fmt.Sprintf("Đang xử lý %s...", commandType),
				}
				_, err := integrations.FolkForm_UpdateWorkflowCommandHeartbeat(agentId, commandID, progress)
				if err != nil {
					jobLogger.WithError(err).WithField("command_id", commandID).Warn("⚠️  Lỗi khi update heartbeat (tiếp tục xử lý)")
				}
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// Xử lý command
	var workflowRunID string
	var err error

	if commandType == "START_WORKFLOW" {
		// Update progress: starting workflow
		integrations.FolkForm_UpdateWorkflowCommandHeartbeat(agentId, commandID, map[string]interface{}{
			"step":       "starting_workflow",
			"percentage": 10,
			"message":    fmt.Sprintf("Đang khởi động workflow: %s", workflowId),
		})

		jobLogger.WithFields(map[string]interface{}{
			"command_id":    commandID,
			"workflow_id":   workflowId,
			"root_ref_id":   rootRefId,
			"root_ref_type": rootRefType,
		}).Info("🚀 Đang thực thi workflow...")

		// Tạo workflow executor và thực thi workflow
		executor := services.NewWorkflowExecutor()
		workflowRunID, err = executor.ExecuteWorkflow(workflowId, rootRefId, rootRefType, params, agentId, commandID)
		if err != nil {
			jobLogger.WithError(err).WithField("command_id", commandID).Error("❌ Lỗi khi execute workflow")
			// Update command status = "failed"
			integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			done <- true
			return
		}

		// Update progress: completed
		integrations.FolkForm_UpdateWorkflowCommandHeartbeat(agentId, commandID, map[string]interface{}{
			"step":       "completed",
			"percentage": 100,
			"message":    fmt.Sprintf("Workflow đã hoàn thành: %s", workflowRunID),
		})

		// Update command status = "completed"
		resultData := map[string]interface{}{
			"workflowRunId": workflowRunID,
		}

		_, err = integrations.FolkForm_UpdateWorkflowCommand(commandID, "completed", resultData)
		if err != nil {
			jobLogger.WithError(err).WithField("command_id", commandID).Error("❌ Lỗi khi update command status = completed")
			done <- true
			return
		}

		jobLogger.WithFields(map[string]interface{}{
			"command_id":      commandID,
			"workflow_run_id": workflowRunID,
		}).Info("✅ Hoàn thành xử lý workflow command")

	} else if commandType == "EXECUTE_STEP" {
		// Update progress: starting step
		integrations.FolkForm_UpdateWorkflowCommandHeartbeat(agentId, commandID, map[string]interface{}{
			"step":       "starting_step",
			"percentage": 10,
			"message":    fmt.Sprintf("Đang khởi động step: %s", stepId),
		})

		jobLogger.WithFields(map[string]interface{}{
			"command_id":    commandID,
			"step_id":       stepId,
			"root_ref_id":   rootRefId,
			"root_ref_type": rootRefType,
		}).Info("🚀 Đang thực thi step...")

		// Load root content
		rootContent, err := loadRootContentForStep(rootRefId, rootRefType)
		if err != nil {
			jobLogger.WithError(err).WithField("command_id", commandID).Error("❌ Lỗi khi load root content")
			integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
				"error": fmt.Sprintf("Lỗi khi load root content: %v", err),
			})
			done <- true
			return
		}

		// Tạo step executor và thực thi step
		stepExecutor := services.NewStepExecutor(services.NewAIClientService())
		stepResult, err := stepExecutor.ExecuteStep(stepId, rootRefId, rootRefType, "", rootContent)
		if err != nil {
			jobLogger.WithError(err).WithField("command_id", commandID).Error("❌ Lỗi khi execute step")
			integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			done <- true
			return
		}

		// Update progress: completed
		integrations.FolkForm_UpdateWorkflowCommandHeartbeat(agentId, commandID, map[string]interface{}{
			"step":       "completed",
			"percentage": 100,
			"message":    fmt.Sprintf("Step đã hoàn thành: %s", stepId),
		})

		// Update command status = "completed"
		resultData := map[string]interface{}{
			"stepRunId": stepResult.StepRunID,
		}
		if stepResult.DraftNodeID != "" {
			resultData["draftNodeId"] = stepResult.DraftNodeID
		}
		if stepResult.SelectedCandidateID != "" {
			resultData["selectedCandidateId"] = stepResult.SelectedCandidateID
		}

		_, err = integrations.FolkForm_UpdateWorkflowCommand(commandID, "completed", resultData)
		if err != nil {
			jobLogger.WithError(err).WithField("command_id", commandID).Error("❌ Lỗi khi update command status = completed")
			done <- true
			return
		}

		jobLogger.WithFields(map[string]interface{}{
			"command_id":    commandID,
			"step_run_id":   stepResult.StepRunID,
			"draft_node_id": stepResult.DraftNodeID,
		}).Info("✅ Hoàn thành xử lý step command")
	}

	// Signal heartbeat goroutine dừng
	done <- true
}

// loadRootContentForStep load root content cho step execution
func loadRootContentForStep(rootRefId, rootRefType string) (map[string]interface{}, error) {
	// Thử load từ production trước
	contentResp, err := integrations.FolkForm_GetContentNode(rootRefId)
	if err == nil {
		if data, ok := contentResp["data"].(map[string]interface{}); ok {
			return data, nil
		}
	}

	// Nếu không có trong production, thử load từ draft
	draftResp, err := integrations.FolkForm_GetDraftNode(rootRefId)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy content node hoặc draft node: %v", err)
	}

	if data, ok := draftResp["data"].(map[string]interface{}); ok {
		return data, nil
	}

	return nil, fmt.Errorf("không thể parse content node response")
}

// getWorkflowCommandsJobInstance lấy instance của WorkflowCommandsJob từ global variable
// Hàm này dùng để truy cập activeWorkers map
func getWorkflowCommandsJobInstance() *WorkflowCommandsJob {
	globalWorkflowCommandsJobMu.RLock()
	defer globalWorkflowCommandsJobMu.RUnlock()
	return globalWorkflowCommandsJob
}
