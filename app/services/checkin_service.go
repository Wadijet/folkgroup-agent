/*
Package services chứa các services hỗ trợ cho agent.
File này quản lý check-in service - thu thập và gửi check-in data lên server.
*/
package services

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"agent_pancake/global"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// CheckInService quản lý check-in với server
type CheckInService struct {
	scheduler          *scheduler.Scheduler
	metricsCollector   *MetricsCollector
	systemInfoCollector *SystemInfoCollector
	configManager      *ConfigManager
	checkInInterval    time.Duration
	stopChan           chan struct{}
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

	return &CheckInService{
		scheduler:           s,
		metricsCollector:   NewMetricsCollector(s),
		systemInfoCollector: NewSystemInfoCollector(),
		checkInInterval:    defaultInterval,
		configManager:      cm,
		stopChan:           make(chan struct{}),
	}
}

// AgentCheckInRequest chứa dữ liệu check-in từ bot
type AgentCheckInRequest struct {
	AgentID       string                 `json:"agentId"`
	Timestamp     int64                  `json:"timestamp"`
	SystemInfo    SystemInfo             `json:"systemInfo"`
	Status        string                 `json:"status"`        // "online", "offline", "error", "maintenance"
	HealthStatus  string                 `json:"healthStatus"`  // "healthy", "degraded", "unhealthy"
	Metrics       AgentMetrics           `json:"metrics"`
	JobStatus     []JobStatus            `json:"jobStatus"`
	ConfigVersion int64                  `json:"configVersion"` // Unix timestamp (server tự động quyết định)
	ConfigHash    string                 `json:"configHash"`
	ConfigData    map[string]interface{} `json:"configData,omitempty"` // Chỉ gửi khi cần submit full config
	Errors        []ErrorReport          `json:"errors,omitempty"`
}

// AgentCheckInResponse chứa response từ server (theo API v3.12)
// Response có cấu trúc: {code, message, data: {commands, configUpdate}, status}
type AgentCheckInResponse struct {
	Code        int            `json:"code"`        // HTTP status code (200, 400, etc.)
	Message     string         `json:"message"`    // Message từ server
	Status      string         `json:"status"`     // "success", "error"
	Data        *CheckInData   `json:"data"`       // Data chứa commands và configUpdate
}

// CheckInData chứa dữ liệu trong response.data
type CheckInData struct {
	Commands    []AgentCommand `json:"commands"`     // Array các commands pending (theo API mới)
	ConfigUpdate *AgentConfig  `json:"configUpdate,omitempty"` // Config update nếu có (theo API mới)
}

// AgentCommand chứa command từ server
type AgentCommand struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`        // "stop", "start", "restart", "reload_config", "shutdown", "run_job", "pause_job", "resume_job", "disable_job", "enable_job", "update_job_schedule"
	Target      string                 `json:"target"`      // "bot" hoặc job name
	Params      map[string]interface{} `json:"params,omitempty"`
	CreatedAt   int64                  `json:"createdAt"`
}

// AgentConfig chứa config từ server
// Có thể là full config (configData) hoặc diff (configDiff) tùy theo backend
type AgentConfig struct {
	ID            string                 `json:"id,omitempty"`            // ID của config (nếu có)
	AgentID       string                 `json:"agentId,omitempty"`       // Agent ID (nếu có)
	Version       int64                  `json:"version"`                 // Unix timestamp (server tự động quyết định)
	ConfigHash    string                 `json:"configHash"`               // Hash của config
	ConfigData    map[string]interface{} `json:"configData,omitempty"`    // Full config data (nếu backend trả về full config)
	ConfigDiff    map[string]interface{} `json:"configDiff,omitempty"`    // Config diff (nếu backend trả về diff)
	NeedFullConfig bool                  `json:"needFullConfig,omitempty"` // true nếu server cần bot gửi full config
	ChangeLog     string                 `json:"changeLog,omitempty"`     // Ghi chú về thay đổi
	HasUpdate     bool                   `json:"hasUpdate"`               // Có update không
	IsActive      bool                   `json:"isActive,omitempty"`      // Config này có active không
	AppliedStatus string                 `json:"appliedStatus,omitempty"`  // "pending", "applied", "failed"
}

// CollectCheckInData thu thập tất cả thông tin cho check-in
func (s *CheckInService) CollectCheckInData() (*AgentCheckInRequest, error) {
	// Thu thập system info
	systemInfo := s.systemInfoCollector.Collect()

	// Thu thập job metrics từ scheduler
	jobStatuses := s.metricsCollector.CollectJobStatuses()

	// Thu thập bot metrics
	metrics := s.metricsCollector.CollectBotMetrics()

	// Thu thập errors (nếu có)
	errors := s.metricsCollector.CollectErrors()

	// Lấy config version và hash (từ config manager)
	configVersion, configHash := s.configManager.GetVersionAndHash()

	// Tối ưu: Chỉ gửi full config khi cần thiết
	var configData map[string]interface{}
	if s.configManager.ShouldSubmitFullConfig() {
		// Lần đầu hoặc config thay đổi → Gửi full config (bao gồm cả metadata)
		configData = s.configManager.CollectCurrentConfig()
	}
	// Nếu không cần → configData = nil (chỉ gửi version và hash)

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
		log.Printf("[CheckInService] ❌ Lỗi marshal response: %v", err)
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	if err := json.Unmarshal(responseBytes, &checkInResponse); err != nil {
		log.Printf("[CheckInService] ❌ Lỗi parse response theo API v3.12: %v", err)
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
						log.Printf("[CheckInService] ⚠️  Version không phải số: %T %v", v, v)
					}
				}
			}
		}
	}

	// Xử lý response (commands, config updates)
	s.handleCheckInResponse(&checkInResponse)

	// Nếu server yêu cầu gửi full config → Đánh dấu để gửi trong check-in tiếp theo
	if checkInResponse.Data != nil && checkInResponse.Data.ConfigUpdate != nil && checkInResponse.Data.ConfigUpdate.NeedFullConfig {
		s.configManager.MarkNeedSubmitFullConfig()
	}

	return &checkInResponse, nil
}

// handleCheckInResponse xử lý response từ server (theo API v3.12)
func (s *CheckInService) handleCheckInResponse(response *AgentCheckInResponse) {
	if response.Data == nil {
		return
	}

	// Xử lý commands (có thể có nhiều commands) - theo API mới
	if len(response.Data.Commands) > 0 {
		log.Printf("[CheckInService] Nhận được %d command(s) từ server", len(response.Data.Commands))
		for _, cmd := range response.Data.Commands {
			log.Printf("[CheckInService] Xử lý command: %s (type: %s, target: %s)", 
				cmd.ID, cmd.Type, cmd.Target)
			
			// Gọi command handler để xử lý từng command
			if s.scheduler != nil {
				// Tạo command handler với scheduler và configManager
				commandHandler := NewCommandHandler(s.scheduler, s.configManager)
				agentCmd := &AgentCommand{
					ID:        cmd.ID,
					Type:      cmd.Type,
					Target:    cmd.Target,
					Params:    cmd.Params,
					CreatedAt: cmd.CreatedAt,
				}
				if err := commandHandler.ExecuteCommand(agentCmd); err != nil {
					log.Printf("[CheckInService] ❌ Lỗi khi thực thi command %s: %v", cmd.ID, err)
				} else {
					log.Printf("[CheckInService] ✅ Đã thực thi command %s thành công", cmd.ID)
				}
			}
		}
	}

	// Xử lý config update nếu có (theo API mới: configUpdate thay vì config)
	if response.Data.ConfigUpdate != nil {
		configUpdate := response.Data.ConfigUpdate
		
		if configUpdate.NeedFullConfig {
			// Server yêu cầu bot gửi full config
			log.Printf("[CheckInService] Server yêu cầu gửi full config")
			s.configManager.MarkNeedSubmitFullConfig()
		} else if configUpdate.HasUpdate {
			// Có config update
			log.Printf("[CheckInService] Nhận được config update: version %d, hash %s", 
				configUpdate.Version, configUpdate.ConfigHash)
			
			// Apply config update thông qua config manager
			if s.configManager != nil {
				var err error
				
				// Backend có thể trả về full config (configData) hoặc diff (configDiff)
				if configUpdate.ConfigData != nil {
					// Backend trả về full config → replace toàn bộ
					log.Printf("[CheckInService] Nhận được full config, đang apply...")
					err = s.configManager.ApplyFullConfig(configUpdate.ConfigData, configUpdate.Version, configUpdate.ConfigHash)
				} else if configUpdate.ConfigDiff != nil {
					// Backend trả về config diff → merge vào config hiện tại
					log.Printf("[CheckInService] Nhận được config diff, đang merge...")
					err = s.configManager.ApplyConfigDiff(configUpdate.ConfigDiff)
					if err == nil {
						// Cập nhật version và hash sau khi apply diff
						s.configManager.SetVersionAndHash(configUpdate.Version, configUpdate.ConfigHash)
					}
				} else {
					log.Printf("[CheckInService] ⚠️  Config update không có configData hoặc configDiff")
				}
				
				if err != nil {
					log.Printf("[CheckInService] ❌ Lỗi khi apply config update: %v", err)
				} else {
					log.Printf("[CheckInService] ✅ Đã apply config update thành công")
					// Nếu apply full config, version và hash đã được set trong ApplyFullConfig
					if configUpdate.ConfigData == nil {
						// Chỉ set version/hash nếu apply diff (full config đã set rồi)
						s.configManager.SetVersionAndHash(configUpdate.Version, configUpdate.ConfigHash)
					}
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

// Start bắt đầu check-in loop
func (s *CheckInService) Start() {
	log.Printf("[CheckInService] 🚀 Bắt đầu check-in service (interval: %v)", s.checkInInterval)

	// Check-in ngay lần đầu
	go func() {
		time.Sleep(5 * time.Second) // Đợi 5 giây để bot khởi động xong
		if _, err := s.SendCheckIn(); err != nil {
			log.Printf("[CheckInService] ❌ Lỗi check-in lần đầu: %v", err)
		}
	}()

	// Check-in định kỳ
	ticker := time.NewTicker(s.checkInInterval)
	defer ticker.Stop()

		for {
		select {
		case <-ticker.C:
			if _, err := s.SendCheckIn(); err != nil {
				log.Printf("[CheckInService] ❌ Lỗi check-in: %v", err)
			}
		case <-s.stopChan:
			log.Printf("[CheckInService] ⏹️  Dừng check-in service")
			return
		}
	}
}

// Stop dừng check-in service
func (s *CheckInService) Stop() {
	close(s.stopChan)
}

