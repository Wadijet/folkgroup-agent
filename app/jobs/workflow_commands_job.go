/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa WorkflowCommandsJob - job xử lý workflow commands từ Module 2 (AI Service).
Theo docs-shared/ai-context/folkform/api-context.md (backend CRUD insert-one, update-by-id):
1. Claim pending: POST /api/v1/ai/workflow-commands/claim-pending
2. Tạo worker (goroutine) để xử lý từng command
3. Worker gọi API Module 2 (workflow-runs/insert-one, step-runs, ...) để start workflow run hoặc execute step
4. Update heartbeat định kỳ: POST /api/v1/ai/workflow-commands/update-heartbeat
5. Update command status: PUT /api/v1/ai/workflow-commands/update-by-id/:id
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
	jobLogger.Debug("Đã có API token, tiếp tục xử lý workflow commands")

	// Gọi hàm logic thực sự
	err := DoProcessWorkflowCommands()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	jobLogger.WithFields(map[string]interface{}{
		"duration":    duration.String(),
		"duration_ms": durationMs,
	}).Debug("DoProcessWorkflowCommands kết thúc thành công")
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
	jobLogger.WithField("agent_id", agentId).Debug("AgentId đã có, chuẩn bị claim commands")

	// Lấy limit từ config (default: 5, max: 100)
	limit := GetJobConfigInt("workflow-commands-job", "claimLimit", 5)
	if limit > 100 {
		limit = 100
	}
	jobLogger.WithFields(map[string]interface{}{
		"agent_id": agentId,
		"limit":    limit,
		"endpoint": "/v1/ai/workflow-commands/claim-pending",
	}).Info("Đang claim workflow commands từ server...")
	// Log chi tiết REQUEST/RESPONSE sẽ ghi qua logToJob → xuất hiện ở đây (console + file workflow-commands-job.log)
	jobLogger.Info("🔍 [Claim] Log chi tiết REQUEST và RESPONSE bên dưới (source=claim_api)")
	logToJob := func(msg string) {
		jobLogger.WithField("source", "claim_api").Info(msg)
	}
	commands, err := integrations.FolkForm_ClaimWorkflowCommands(agentId, limit, logToJob)
	if err != nil {
		jobLogger.WithError(err).WithFields(map[string]interface{}{
			"agent_id": agentId,
			"limit":    limit,
		}).Error("❌ Lỗi khi claim workflow commands")
		return err
	}

	jobLogger.WithFields(map[string]interface{}{
		"agent_id":     agentId,
		"count":        len(commands),
		"has_commands": len(commands) > 0,
	}).Debug("Kết quả claim workflow commands từ API")
	if len(commands) == 0 {
		jobLogger.WithFields(map[string]interface{}{
			"agent_id": agentId,
			"limit":    limit,
		}).Debug("Không có command nào cần xử lý (server trả về 0 commands - xem log [FolkForm] [ClaimWorkflowCommands] để biết cấu trúc response)")
		return nil
	}

	jobLogger.WithField("count", len(commands)).Info(fmt.Sprintf("📥 Đã claim %d command(s) cần xử lý", len(commands)))

	// Xử lý từng command bằng cách tạo worker (goroutine)
	for idx, cmdInterface := range commands {
		cmdMap, ok := cmdInterface.(map[string]interface{})
		if !ok {
			jobLogger.WithFields(map[string]interface{}{
				"index": idx,
				"type":  fmt.Sprintf("%T", cmdInterface),
			}).Warn("⚠️  Command không phải là map, bỏ qua")
			continue
		}

		// Lấy commandID
		commandID, ok := cmdMap["id"].(string)
		if !ok || commandID == "" {
			jobLogger.WithFields(map[string]interface{}{
				"index": idx,
				"id":    cmdMap["id"],
			}).Warn("⚠️  Command không có ID hợp lệ, bỏ qua")
			continue
		}

		// Kiểm tra xem command này đã có worker đang xử lý chưa
		// (tránh xử lý duplicate nếu job chạy lại trước khi worker hoàn thành)
		jobInstance := getWorkflowCommandsJobInstance()
		if jobInstance != nil {
			if _, exists := jobInstance.activeWorkers.Load(commandID); exists {
				jobLogger.WithField("command_id", commandID).Debug("Command đang được xử lý bởi worker khác, bỏ qua (tránh duplicate)")
				continue
			}
			// Đánh dấu command đang được xử lý
			jobInstance.activeWorkers.Store(commandID, true)
			jobLogger.WithField("command_id", commandID).Debug("Đã đánh dấu command vào activeWorkers, spawn worker")
		}

		// Tạo worker để xử lý command (chạy trong goroutine riêng)
		jobLogger.WithFields(map[string]interface{}{
			"command_id": commandID,
			"index":      idx + 1,
			"total":      len(commands),
		}).Debug("Spawning goroutine xử lý command")
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
			jobLogger.WithField("command_id", commandID).Debug("Đã xóa command khỏi activeWorkers (cleanup)")
		}
	}()

	jobLogger.WithField("command_id", commandID).Info("🔄 Bắt đầu xử lý workflow command")

	// Parse command data
	commandType, _ := cmdMap["commandType"].(string)
	workflowId, _ := cmdMap["workflowId"].(string)
	stepId, _ := cmdMap["stepId"].(string)
	rootRefId, _ := cmdMap["rootRefId"].(string)
	rootRefType, _ := cmdMap["rootRefType"].(string)

	jobLogger.WithFields(map[string]interface{}{
		"command_id":    commandID,
		"command_type":  commandType,
		"workflow_id":   workflowId,
		"step_id":       stepId,
		"root_ref_id":   rootRefId,
		"root_ref_type": rootRefType,
	}).Debug("Đã parse command data từ server")

	// Parse params (có thể là map hoặc string JSON)
	var params map[string]interface{}
	if paramsInterface, ok := cmdMap["params"]; ok && paramsInterface != nil {
		if paramsMap, ok := paramsInterface.(map[string]interface{}); ok {
			params = paramsMap
			jobLogger.WithFields(map[string]interface{}{
				"command_id":  commandID,
				"params_keys": getMapKeys(params),
			}).Debug("Params là map, số key")
		} else if paramsStr, ok := paramsInterface.(string); ok {
			// Nếu params là string JSON, parse nó
			if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
				jobLogger.WithError(err).WithField("command_id", commandID).Warn("⚠️  Không thể parse params JSON, dùng nil")
				params = nil
			} else {
				jobLogger.WithFields(map[string]interface{}{
					"command_id":  commandID,
					"params_keys": getMapKeys(params),
				}).Debug("Params đã parse từ JSON string")
			}
		}
	} else {
		jobLogger.WithField("command_id", commandID).Debug("Command không có params hoặc params nil")
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

	jobLogger.WithFields(map[string]interface{}{
		"command_id":           commandID,
		"heartbeat_interval_s": heartbeatInterval,
	}).Debug("Đã tạo heartbeat ticker, bắt đầu goroutine heartbeat")

	// Channel để signal khi worker hoàn thành
	done := make(chan bool, 1)

	// Goroutine để update heartbeat định kỳ
	go func() {
		heartbeatCount := 0
		for {
			select {
			case <-heartbeatTicker.C:
				heartbeatCount++
				// Update heartbeat với progress
				progress := map[string]interface{}{
					"step":       "processing",
					"percentage": 0,
					"message":    fmt.Sprintf("Đang xử lý %s...", commandType),
				}
				_, err := integrations.FolkForm_UpdateWorkflowCommandHeartbeat(agentId, commandID, progress)
				if err != nil {
					jobLogger.WithError(err).WithFields(map[string]interface{}{
						"command_id":      commandID,
						"heartbeat_count": heartbeatCount,
					}).Warn("⚠️  Lỗi khi update heartbeat (tiếp tục xử lý)")
				} else {
					jobLogger.WithFields(map[string]interface{}{
						"command_id":      commandID,
						"heartbeat_count": heartbeatCount,
					}).Debug("Heartbeat gửi thành công")
				}
			case <-done:
				jobLogger.WithField("command_id", commandID).Debug("Heartbeat goroutine nhận done, thoát")
				return
			case <-ctx.Done():
				jobLogger.WithField("command_id", commandID).Debug("Heartbeat goroutine nhận ctx.Done, thoát")
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
			"params_count":  len(params),
		}).Info("🚀 Đang thực thi workflow...")
		jobLogger.WithField("command_id", commandID).Debug("Gọi executor.ExecuteWorkflow...")

		// Tạo workflow executor và thực thi workflow
		executor := services.NewWorkflowExecutor()
		workflowRunID, err = executor.ExecuteWorkflow(workflowId, rootRefId, rootRefType, params, agentId, commandID)
		if err != nil {
			jobLogger.WithError(err).WithField("command_id", commandID).Error("❌ Lỗi khi execute workflow")
			// Update command status = "failed"
			jobLogger.WithField("command_id", commandID).Debug("Gọi FolkForm_UpdateWorkflowCommand status=failed")
			integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			done <- true
			return
		}

		jobLogger.WithFields(map[string]interface{}{
			"command_id":      commandID,
			"workflow_run_id": workflowRunID,
		}).Debug("ExecuteWorkflow trả về thành công, chuẩn bị update heartbeat và command completed")

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
		jobLogger.WithFields(map[string]interface{}{
			"command_id":      commandID,
			"workflow_run_id": workflowRunID,
			"result_data":     resultData,
		}).Debug("Gọi FolkForm_UpdateWorkflowCommand status=completed")

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
		jobLogger.WithFields(map[string]interface{}{
			"command_id":    commandID,
			"root_ref_id":   rootRefId,
			"root_ref_type": rootRefType,
		}).Debug("Gọi loadRootContentForStep...")
		rootContent, err := loadRootContentForStep(commandID, rootRefId, rootRefType)
		if err != nil {
			jobLogger.WithError(err).WithField("command_id", commandID).Error("❌ Lỗi khi load root content")
			jobLogger.WithField("command_id", commandID).Debug("Gọi FolkForm_UpdateWorkflowCommand status=failed (load root content)")
			integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
				"error": fmt.Sprintf("Lỗi khi load root content: %v", err),
			})
			done <- true
			return
		}
		jobLogger.WithFields(map[string]interface{}{
			"command_id":        commandID,
			"root_content_keys": getMapKeys(rootContent),
		}).Debug("loadRootContentForStep thành công, gọi ExecuteStep...")

		// Tạo step executor và thực thi step
		stepExecutor := services.NewStepExecutor(services.NewAIClientService())
		stepResult, err := stepExecutor.ExecuteStep(stepId, rootRefId, rootRefType, "", rootContent)
		if err != nil {
			jobLogger.WithError(err).WithField("command_id", commandID).Error("❌ Lỗi khi execute step")
			jobLogger.WithField("command_id", commandID).Debug("Gọi FolkForm_UpdateWorkflowCommand status=failed (execute step)")
			integrations.FolkForm_UpdateWorkflowCommand(commandID, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			done <- true
			return
		}
		jobLogger.WithFields(map[string]interface{}{
			"command_id":    commandID,
			"step_run_id":   stepResult.StepRunID,
			"draft_node_id": stepResult.DraftNodeID,
		}).Debug("ExecuteStep trả về thành công")

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
		jobLogger.WithFields(map[string]interface{}{
			"command_id":  commandID,
			"result_data": resultData,
		}).Debug("Gọi FolkForm_UpdateWorkflowCommand status=completed (EXECUTE_STEP)")

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

// loadRootContentForStep load root content cho step execution.
// Thử GetContentNode (production) trước, nếu lỗi thì thử GetDraftNode.
// commandID dùng cho log debug.
func loadRootContentForStep(commandID, rootRefId, rootRefType string) (map[string]interface{}, error) {
	jobLogger := GetJobLoggerByName("workflow-commands-job")

	// Thử load từ production trước
	jobLogger.WithFields(map[string]interface{}{
		"command_id":    commandID,
		"root_ref_id":   rootRefId,
		"root_ref_type": rootRefType,
		"source":        "production",
	}).Debug("Gọi FolkForm_GetContentNode (production)...")
	contentResp, err := integrations.FolkForm_GetContentNode(rootRefId)
	if err == nil {
		if data, ok := contentResp["data"].(map[string]interface{}); ok {
			jobLogger.WithFields(map[string]interface{}{
				"command_id":  commandID,
				"root_ref_id": rootRefId,
				"data_keys":   getMapKeys(data),
			}).Debug("Load root content từ production thành công")
			return data, nil
		}
		jobLogger.WithFields(map[string]interface{}{
			"command_id":    commandID,
			"root_ref_id":   rootRefId,
			"response_keys": getMapKeys(contentResp),
		}).Debug("GetContentNode trả về nhưng không có data map, thử draft")
	} else {
		jobLogger.WithError(err).WithFields(map[string]interface{}{
			"command_id":  commandID,
			"root_ref_id": rootRefId,
		}).Debug("GetContentNode (production) lỗi, thử GetDraftNode...")
	}

	// Nếu không có trong production, thử load từ draft
	jobLogger.WithFields(map[string]interface{}{
		"command_id":  commandID,
		"root_ref_id": rootRefId,
		"source":      "draft",
	}).Debug("Gọi FolkForm_GetDraftNode...")
	draftResp, err := integrations.FolkForm_GetDraftNode(rootRefId)
	if err != nil {
		jobLogger.WithError(err).WithFields(map[string]interface{}{
			"command_id":  commandID,
			"root_ref_id": rootRefId,
		}).Error("Cả GetContentNode và GetDraftNode đều lỗi")
		return nil, fmt.Errorf("không tìm thấy content node hoặc draft node: %v", err)
	}

	if data, ok := draftResp["data"].(map[string]interface{}); ok {
		jobLogger.WithFields(map[string]interface{}{
			"command_id":  commandID,
			"root_ref_id": rootRefId,
			"data_keys":   getMapKeys(data),
		}).Debug("Load root content từ draft thành công")
		return data, nil
	}

	jobLogger.WithFields(map[string]interface{}{
		"command_id":      commandID,
		"root_ref_id":     rootRefId,
		"draft_resp_keys": getMapKeys(draftResp),
	}).Warn("GetDraftNode trả về nhưng không có data map")
	return nil, fmt.Errorf("không thể parse content node response")
}

// getWorkflowCommandsJobInstance lấy instance của WorkflowCommandsJob từ global variable
// Hàm này dùng để truy cập activeWorkers map
func getWorkflowCommandsJobInstance() *WorkflowCommandsJob {
	globalWorkflowCommandsJobMu.RLock()
	defer globalWorkflowCommandsJobMu.RUnlock()
	return globalWorkflowCommandsJob
}
