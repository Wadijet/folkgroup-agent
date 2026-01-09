/*
Package services chứa các services hỗ trợ cho agent.
File này quản lý check-in service - thu thập và gửi check-in data lên server.
*/
package services

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"agent_pancake/global"
	"agent_pancake/utility/logger"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// CheckInService quản lý check-in với server
type CheckInService struct {
	scheduler           *scheduler.Scheduler
	metricsCollector    *MetricsCollector
	systemInfoCollector *SystemInfoCollector
	configManager       *ConfigManager
	checkInInterval     time.Duration
	stopChan            chan struct{}
	logger              *logrus.Logger // Logger để ghi log vào file
}

// NewCheckInService tạo một instance mới của CheckInService
func NewCheckInService(s *scheduler.Scheduler, cm *ConfigManager) *CheckInService {
	// Default 60 giây (cân bằng giữa realtime và performance)
	defaultInterval := 60 * time.Second

	// Có thể đọc từ config nếu có
	if cm != nil {
		if interval := cm.GetCheckInInterval(); interval > 0 {
			defaultInterval = time.Duration(interval) * time.Second
		}
	}

	// Tạo logger riêng cho check-in service để log vào file
	checkInLogger := logger.GetLogger("check-in-service")

	return &CheckInService{
		scheduler:           s,
		metricsCollector:    NewMetricsCollector(s),
		systemInfoCollector: NewSystemInfoCollector(),
		checkInInterval:     defaultInterval,
		configManager:       cm,
		stopChan:            make(chan struct{}),
		logger:              checkInLogger,
	}
}

// AgentCheckInRequest chứa dữ liệu check-in từ bot
// Theo API v3.14: Hỗ trợ metadata (displayName, icon, color, category, tags) để UI-friendly
type AgentCheckInRequest struct {
	AgentID       string                 `json:"agentId"`
	Timestamp     int64                  `json:"timestamp"`
	SystemInfo    SystemInfo             `json:"systemInfo"`
	Status        string                 `json:"status"`       // "online", "offline", "error", "maintenance"
	HealthStatus  string                 `json:"healthStatus"` // "healthy", "degraded", "unhealthy"
	Metrics       AgentMetrics           `json:"metrics"`
	JobStatus     []JobStatus            `json:"jobStatus"`
	ConfigVersion int64                  `json:"configVersion"` // Unix timestamp (server tự động quyết định)
	ConfigHash    string                 `json:"configHash"`
	ConfigData    map[string]interface{} `json:"configData,omitempty"` // Chỉ gửi khi cần submit full config
	Errors        []ErrorReport          `json:"errors,omitempty"`
	// Metadata fields (theo API v3.14 - Agent UI-Friendly Metadata Updates)
	DisplayName string   `json:"displayName,omitempty"` // Tên hiển thị của agent (ví dụ: "Pancake Sync Agent")
	Icon        string   `json:"icon,omitempty"`        // Icon của agent (ví dụ: "🤖", "sync", "robot")
	Color       string   `json:"color,omitempty"`       // Màu sắc của agent (ví dụ: "#3B82F6", "blue")
	Category    string   `json:"category,omitempty"`    // Danh mục của agent (ví dụ: "sync", "monitoring", "integration")
	Tags        []string `json:"tags,omitempty"`        // Tags của agent (ví dụ: ["pancake", "facebook", "sync"])
}

// AgentCheckInResponse chứa response từ server (theo API v3.12)
// Response có cấu trúc: {code, message, data: {commands, configUpdate}, status}
type AgentCheckInResponse struct {
	Code    int          `json:"code"`    // HTTP status code (200, 400, etc.)
	Message string       `json:"message"` // Message từ server
	Status  string       `json:"status"`  // "success", "error"
	Data    *CheckInData `json:"data"`    // Data chứa commands và configUpdate
}

// CheckInData chứa dữ liệu trong response.data
type CheckInData struct {
	Commands     []AgentCommand `json:"commands"`               // Array các commands pending (theo API mới)
	ConfigUpdate *AgentConfig   `json:"configUpdate,omitempty"` // Config update nếu có (theo API mới)
}

// AgentCommand chứa command từ server
// Theo tài liệu API: Bot nhận commands từ check-in response và update status qua endpoint update
type AgentCommand struct {
	ID          string                 `json:"id"`                    // Command ID (bắt buộc để update status)
	AgentID     string                 `json:"agentId"`               // Agent ID (string, không phải ObjectID)
	Type        string                 `json:"type"`                  // "stop", "start", "restart", "reload_config", "shutdown", "run_job", "pause_job", "resume_job", "disable_job", "enable_job", "update_job_schedule"
	Target      string                 `json:"target"`                // "bot" hoặc job name
	Params      map[string]interface{} `json:"params,omitempty"`      // Parameters cho command
	Status      string                 `json:"status"`                // "pending", "executing", "completed", "failed", "cancelled"
	Result      map[string]interface{} `json:"result,omitempty"`      // Kết quả từ bot sau khi execute
	Error       string                 `json:"error,omitempty"`       // Error message nếu failed
	CreatedBy   string                 `json:"createdBy,omitempty"`   // User ID nếu admin tạo
	CreatedAt   int64                  `json:"createdAt"`             // Timestamp khi command được tạo
	ExecutedAt  int64                  `json:"executedAt,omitempty"`  // Timestamp khi bot bắt đầu execute
	CompletedAt int64                  `json:"completedAt,omitempty"` // Timestamp khi bot hoàn thành
}

// AgentConfig chứa config từ server
// Có thể là full config (configData) hoặc diff (configDiff) tùy theo backend
type AgentConfig struct {
	ID             string                 `json:"id,omitempty"`             // ID của config (nếu có)
	AgentID        string                 `json:"agentId,omitempty"`        // Agent ID (nếu có)
	Version        int64                  `json:"version"`                  // Unix timestamp (server tự động quyết định)
	ConfigHash     string                 `json:"configHash"`               // Hash của config
	ConfigData     map[string]interface{} `json:"configData,omitempty"`     // Full config data (nếu backend trả về full config)
	ConfigDiff     map[string]interface{} `json:"configDiff,omitempty"`     // Config diff (nếu backend trả về diff)
	NeedFullConfig bool                   `json:"needFullConfig,omitempty"` // true nếu server cần bot gửi full config
	ChangeLog      string                 `json:"changeLog,omitempty"`      // Ghi chú về thay đổi
	HasUpdate      bool                   `json:"hasUpdate"`                // Có update không
	IsActive       bool                   `json:"isActive,omitempty"`       // Config này có active không
	AppliedStatus  string                 `json:"appliedStatus,omitempty"`  // "pending", "applied", "failed"
}

// CollectCheckInData thu thập tất cả thông tin cho check-in
func (s *CheckInService) CollectCheckInData() (*AgentCheckInRequest, error) {
	// Thu thập system info
	systemInfo := s.systemInfoCollector.Collect()

	// Thu thập job metrics từ scheduler
	jobStatuses := s.metricsCollector.CollectJobStatuses()

	// Thu thập bot metrics
	metrics := s.metricsCollector.CollectBotMetrics()

	// Lưu ý: Errors của từng job đã được gửi trực tiếp trong JobStatus.LastError
	// Chỉ thu thập system errors (nếu có) - tạm thời để trống vì chưa có system error tracking
	errors := []ErrorReport{}

	// Lấy config version và hash (từ config manager)
	configVersion, configHash := s.configManager.GetVersionAndHash()

	// Tối ưu: Chỉ gửi full config khi cần thiết
	var configData map[string]interface{}
	shouldSubmit := s.configManager.ShouldSubmitFullConfig()
	if shouldSubmit {
		// Lần đầu hoặc config thay đổi → Gửi full config (theo API v3.14: không có metadata chung của job)
		configData = s.configManager.CollectCurrentConfig()
		s.logger.WithField("config_size", len(configData)).Info("📤 Sẽ gửi full config trong check-in request")
	} else {
		s.logger.Info("📤 Chỉ gửi config version và hash (config không thay đổi hoặc đã có trên server)")
	}
	// Nếu không cần → configData = nil (chỉ gửi version và hash)

	// Thu thập metadata cho agent (theo API v3.14 - Agent UI-Friendly Metadata Updates)
	// Metadata có thể được set từ config hoặc default values
	metadata := s.collectAgentMetadata()

	return &AgentCheckInRequest{
		AgentID:       global.GlobalConfig.AgentId,
		Timestamp:     time.Now().Unix(),
		SystemInfo:    systemInfo,
		Status:        s.getBotStatus(),
		HealthStatus:  s.calculateHealthStatus(),
		Metrics:       metrics,
		JobStatus:     jobStatuses,
		ConfigVersion: configVersion,
		ConfigHash:    configHash,
		ConfigData:    configData, // Chỉ có khi cần submit full config
		Errors:        errors,
		// Metadata fields (theo API v3.14)
		DisplayName: metadata.DisplayName,
		Icon:        metadata.Icon,
		Color:       metadata.Color,
		Category:    metadata.Category,
		Tags:        metadata.Tags,
	}, nil
}

// SendCheckIn gửi check-in lên server
func (s *CheckInService) SendCheckIn() (*AgentCheckInResponse, error) {
	data, err := s.CollectCheckInData()
	if err != nil {
		return nil, err
	}

	// Gửi lên server
	response, err := integrations.FolkForm_EnhancedCheckIn(global.GlobalConfig.AgentId, data)
	if err != nil {
		return nil, err
	}

	// Parse response theo API v3.12: {code, message, data: {commands: [], configUpdate: {}}, status}
	// Version trong configUpdate là Unix timestamp (int64), không phải string
	var checkInResponse AgentCheckInResponse
	responseBytes, err := json.Marshal(response)
	if err != nil {
		s.logger.WithError(err).Error("❌ Lỗi marshal response")
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	if err := json.Unmarshal(responseBytes, &checkInResponse); err != nil {
		s.logger.WithError(err).Error("❌ Lỗi parse response theo API v3.12")
		return nil, fmt.Errorf("failed to parse check-in response: %w", err)
	}

	// Đảm bảo version trong configUpdate là int64 (Unix timestamp)
	// JSON unmarshal có thể trả về float64 cho số, nên cần convert
	if checkInResponse.Data != nil && checkInResponse.Data.ConfigUpdate != nil {
		// Parse version từ raw response nếu cần (vì JSON có thể trả về float64)
		if dataMap, ok := response["data"].(map[string]interface{}); ok {
			if configUpdateRaw, ok := dataMap["configUpdate"].(map[string]interface{}); ok {
				if versionRaw, exists := configUpdateRaw["version"]; exists {
					// Convert version sang int64 (Unix timestamp)
					switch v := versionRaw.(type) {
					case int64:
						checkInResponse.Data.ConfigUpdate.Version = v
					case float64:
						checkInResponse.Data.ConfigUpdate.Version = int64(v)
					case int:
						checkInResponse.Data.ConfigUpdate.Version = int64(v)
					default:
						s.logger.WithFields(logrus.Fields{
							"version_type":  fmt.Sprintf("%T", v),
							"version_value": v,
						}).Warn("⚠️  Version không phải số")
					}
				}
			}
		}
	}

	// Log response để debug (dùng logger để ghi vào file)
	s.logger.WithFields(logrus.Fields{
		"code":    checkInResponse.Code,
		"status":  checkInResponse.Status,
		"message": checkInResponse.Message,
	}).Info("📥 Check-in response từ server")

	if checkInResponse.Data != nil {
		commandCount := len(checkInResponse.Data.Commands)
		s.logger.WithField("commands_count", commandCount).Info("📥 Số lượng commands nhận được")

		if commandCount > 0 {
			for i, cmd := range checkInResponse.Data.Commands {
				s.logger.WithFields(logrus.Fields{
					"command_index":  i,
					"command_id":     cmd.ID,
					"command_type":   cmd.Type,
					"command_target": cmd.Target,
				}).Info("📥 Command nhận được từ server")
			}
		}

		if checkInResponse.Data.ConfigUpdate != nil {
			s.logger.WithFields(logrus.Fields{
				"has_update": checkInResponse.Data.ConfigUpdate.HasUpdate,
				"version":    checkInResponse.Data.ConfigUpdate.Version,
			}).Info("📥 ConfigUpdate từ server")
		} else {
			s.logger.Info("📥 ConfigUpdate: nil")
		}
	} else {
		s.logger.Info("📥 Response.Data: nil")
	}

	// Xử lý response (commands, config updates)
	s.handleCheckInResponse(&checkInResponse)

	// Nếu server yêu cầu gửi full config → Đánh dấu để gửi trong check-in tiếp theo
	// Theo tài liệu: Bot tự submit config qua check-in endpoint, không cần submit riêng
	if checkInResponse.Data != nil && checkInResponse.Data.ConfigUpdate != nil && checkInResponse.Data.ConfigUpdate.NeedFullConfig {
		s.logger.Info("📥 Server yêu cầu gửi full config trong check-in tiếp theo")
		s.configManager.MarkNeedSubmitFullConfig()
	}

	return &checkInResponse, nil
}

// handleCheckInResponse xử lý response từ server (theo API v3.12)
func (s *CheckInService) handleCheckInResponse(response *AgentCheckInResponse) {
	if response.Data == nil {
		s.logger.Warn("⚠️  Response.Data là nil, không có commands hoặc config update")
		return
	}

	// Log số lượng commands nhận được (dùng logger để ghi vào file)
	commandCount := len(response.Data.Commands)
	if commandCount > 0 {
		s.logger.WithField("command_count", commandCount).Info("📥 Nhận được commands từ server")
	} else {
		s.logger.Info("ℹ️  Không có command nào từ server trong check-in response")
	}

	// Xử lý commands (có thể có nhiều commands) - theo API mới
	if len(response.Data.Commands) > 0 {
		for _, cmd := range response.Data.Commands {
			// Gọi command handler để xử lý từng command
			// Theo tài liệu: Bot nên execute commands theo thứ tự và update status qua endpoint update
			if s.scheduler != nil {
				// Tạo command handler với scheduler và configManager
				commandHandler := NewCommandHandler(s.scheduler, s.configManager)
				agentCmd := &AgentCommand{
					ID:        cmd.ID,
					AgentID:   cmd.AgentID,
					Type:      cmd.Type,
					Target:    cmd.Target,
					Params:    cmd.Params,
					Status:    cmd.Status, // Thường là "pending" khi nhận từ server
					CreatedAt: cmd.CreatedAt,
				}

				// Thực thi command và báo kết quả về server
				// Theo tài liệu: Bot update status khi execute command và trả về result/error
				executedAt := time.Now().Unix()

				// Update command status thành "executing" trước khi thực thi
				// Endpoint: PUT /api/v1/agent-management/command/update-by-id/:id
				s.updateCommandStatus(cmd.ID, "executing", nil, executedAt, 0)

				// Thực thi command
				err := commandHandler.ExecuteCommand(agentCmd)
				completedAt := time.Now().Unix()

				// Thu thập thông tin về job execution nếu là command run_job
				var resultData map[string]interface{}
				if cmd.Type == "run_job" && err == nil {
					// Lấy job object để lấy metrics (nếu job implement MetricsProvider)
					jobObj := s.scheduler.GetJobObject(cmd.Target)
					if jobObj != nil {
						// Type assertion để lấy metrics nếu job implement MetricsProvider
						if metricsProvider, ok := jobObj.(scheduler.MetricsProvider); ok {
							metrics := metricsProvider.GetMetrics()
							resultData = map[string]interface{}{
								"success":         true,
								"type":            cmd.Type,
								"target":          cmd.Target,
								"jobRunCount":     metrics.RunCount,
								"lastRunStatus":   metrics.LastRunStatus,
								"lastRunDuration": metrics.LastRunDuration,
								"lastRunAt":       metrics.LastRunAt.Unix(),
							}
							if metrics.LastError != "" {
								resultData["lastError"] = metrics.LastError
							}
						} else {
							// Job không implement MetricsProvider
							resultData = map[string]interface{}{
								"success": true,
								"type":    cmd.Type,
								"target":  cmd.Target,
							}
						}
					} else {
						resultData = map[string]interface{}{
							"success": true,
							"type":    cmd.Type,
							"target":  cmd.Target,
						}
					}
				} else if err == nil {
					// Command khác (không phải run_job)
					resultData = map[string]interface{}{
						"success": true,
						"type":    cmd.Type,
						"target":  cmd.Target,
					}
				}

				// Update command status và kết quả về server sau khi execute xong
				if err != nil {
					s.logger.WithFields(logrus.Fields{
						"command_id":   cmd.ID,
						"command_type": cmd.Type,
						"error":        err.Error(),
					}).Error("❌ Lỗi khi thực thi command")
					// Update status = "failed" và gửi error message
					s.updateCommandStatus(cmd.ID, "failed", map[string]interface{}{
						"error": err.Error(),
					}, executedAt, completedAt)
				} else {
					s.logger.WithFields(logrus.Fields{
						"command_id":   cmd.ID,
						"command_type": cmd.Type,
						"target":       cmd.Target,
					}).Info("✅ Đã thực thi command thành công")
					// Update status = "completed" và gửi result (có thông tin về job nếu là run_job)
					s.updateCommandStatus(cmd.ID, "completed", resultData, executedAt, completedAt)
				}
			}
		}
	}

	// Xử lý config update nếu có (theo API mới: configUpdate thay vì config)
	if response.Data.ConfigUpdate != nil {
		configUpdate := response.Data.ConfigUpdate

		if configUpdate.NeedFullConfig {
			// Server yêu cầu bot gửi full config
			s.configManager.MarkNeedSubmitFullConfig()
		} else if configUpdate.HasUpdate {
			// Apply config update thông qua config manager
			if s.configManager != nil {
				var err error

				// Backend có thể trả về full config (configData) hoặc diff (configDiff)
				if configUpdate.ConfigData != nil {
					// Backend trả về full config → replace toàn bộ
					err = s.configManager.ApplyFullConfig(configUpdate.ConfigData, configUpdate.Version, configUpdate.ConfigHash)
				} else if configUpdate.ConfigDiff != nil {
					// Backend trả về config diff → merge vào config hiện tại
					err = s.configManager.ApplyConfigDiff(configUpdate.ConfigDiff)
					if err == nil {
						// Cập nhật version và hash sau khi apply diff
						s.configManager.SetVersionAndHash(configUpdate.Version, configUpdate.ConfigHash)
					}
				}

				if err != nil {
					s.logger.WithError(err).Error("❌ Lỗi khi apply config update")
				} else {
					s.logger.WithField("version", configUpdate.Version).Info("✅ Đã apply config update thành công từ server")
					s.logger.Info("💡 Các jobs sẽ đọc config mới khi chạy lần tiếp theo")
				}
			}
		}
	}
}

// getBotStatus trả về trạng thái bot
func (s *CheckInService) getBotStatus() string {
	// TODO: Implement logic kiểm tra trạng thái bot
	return "online"
}

// calculateHealthStatus tính toán health status
func (s *CheckInService) calculateHealthStatus() string {
	// TODO: Implement logic tính toán health status
	return "healthy"
}

// AgentMetadata chứa metadata của agent (theo API v3.14)
type AgentMetadata struct {
	DisplayName string
	Icon        string
	Color       string
	Category    string
	Tags        []string
}

// collectAgentMetadata thu thập metadata của agent từ config hoặc default values
// Theo API v3.14: Bot có thể set metadata khi check-in, admin có thể update sau
func (s *CheckInService) collectAgentMetadata() AgentMetadata {
	metadata := AgentMetadata{
		DisplayName: "Agent Đồng Bộ Pancake",
		Icon:        "🤖",
		Color:       "#3B82F6",
		Category:    "sync",
		Tags:        []string{"pancake", "facebook", "sync", "integration"},
	}

	// Metadata có thể được cập nhật từ server hoặc admin sau khi check-in
	// Hiện tại sử dụng default values, server có thể update metadata qua AgentRegistry
	// Theo API v3.14: Bot có thể set metadata khi check-in, admin có thể update sau

	return metadata
}

// Start bắt đầu check-in loop
// DEPRECATED: Không còn sử dụng nữa. Check-in được thực hiện bởi CheckInJob.
// Method này được giữ lại để tương thích ngược, nhưng không nên được gọi.
func (s *CheckInService) Start() {
	s.logger.Warn("⚠️  DEPRECATED: Start() không còn được sử dụng. Check-in được thực hiện bởi CheckInJob.")
	// Không làm gì cả - CheckInJob sẽ gọi SendCheckIn() trực tiếp
}

// Stop dừng check-in service
func (s *CheckInService) Stop() {
	close(s.stopChan)
}

// updateCommandStatus cập nhật trạng thái command lên server
// Theo tài liệu API: PUT /api/v1/agent-management/command/update-by-id/:id
// Bot update status khi execute command và trả về result hoặc error sau khi execute xong
func (s *CheckInService) updateCommandStatus(commandID string, status string, result map[string]interface{}, executedAt int64, completedAt int64) {
	if commandID == "" {
		s.logger.Warn("⚠️  Command ID rỗng, không thể update status")
		return
	}

	s.logger.WithFields(logrus.Fields{
		"command_id": commandID,
		"status":     status,
	}).Info("📤 Bắt đầu update command status")

	// Build update data theo cấu trúc AgentCommand trong tài liệu
	// Fields: status, result, error, executedAt, completedAt
	updateData := map[string]interface{}{
		"status": status, // "pending", "executing", "completed", "failed", "cancelled"
	}

	// Set executedAt khi bắt đầu execute
	if executedAt > 0 {
		updateData["executedAt"] = executedAt
		// Không log Debug để giảm log
	}

	// Set completedAt khi hoàn thành
	if completedAt > 0 {
		updateData["completedAt"] = completedAt
		// Không log Debug để giảm log
	}

	// Set result hoặc error tùy theo status
	if result != nil {
		if status == "failed" {
			// Nếu failed, lưu error message (theo tài liệu: error?: string)
			if errorMsg, ok := result["error"].(string); ok {
				updateData["error"] = errorMsg
				// Không log Debug để giảm log
			}
		} else if status == "completed" {
			// Nếu completed, lưu result (theo tài liệu: result?: Record<string, any>)
			updateData["result"] = result
			// Không log Debug để giảm log
		}
	}

	// Không log Debug để giảm log

	// Gọi API update command
	// Endpoint: PUT /api/v1/agent-management/command/update-by-id/:id
	resultData, err := integrations.FolkForm_UpdateCommand(commandID, updateData)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"command_id": commandID,
			"status":     status,
			"error":      err.Error(),
		}).Error("❌ Lỗi khi update command status")
	} else {
		s.logger.WithFields(logrus.Fields{
			"command_id": commandID,
			"status":     status,
		}).Info("✅ Đã update command status thành công")
		if resultData != nil {
			// Không log Debug để giảm log
		}
	}
}
