/*
Package services chứa các services hỗ trợ cho agent.
File này quản lý config động - load, save, submit, pull config từ server.
*/
package services

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"agent_pancake/global"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

// ConfigManager quản lý config động của agent
type ConfigManager struct {
	localConfigPath      string
	currentVersion       int64 // Unix timestamp (server tự động quyết định)
	currentHash          string
	configData           map[string]interface{}
	scheduler            *scheduler.Scheduler
	needSubmitFullConfig bool       // Flag: Server yêu cầu gửi full config
	submitMutex          sync.Mutex // Mutex để tránh submit config trùng lặp
	isSubmitting         bool       // Flag: Đang trong quá trình submit
}

// ========================================
// GLOBAL CONFIGMANAGER INSTANCE
// ========================================
// Để jobs có thể truy cập ConfigManager mà không cần truyền qua parameter
// Sử dụng global instance với mutex để đảm bảo thread-safe

// globalConfigManager là instance toàn cục của ConfigManager
// Được set trong main() sau khi khởi tạo ConfigManager
// Jobs có thể truy cập thông qua GetGlobalConfigManager()
var globalConfigManager *ConfigManager

// globalConfigManagerMu là mutex để bảo vệ globalConfigManager khỏi race condition
// Sử dụng RWMutex để cho phép nhiều goroutines đọc đồng thời
var globalConfigManagerMu sync.RWMutex

// SetGlobalConfigManager set global ConfigManager instance
// Hàm này được gọi trong main() sau khi khởi tạo ConfigManager
// Tham số:
//   - cm: Instance của ConfigManager đã được khởi tạo
//
// Lưu ý: Hàm này thread-safe, sử dụng mutex để bảo vệ
func SetGlobalConfigManager(cm *ConfigManager) {
	globalConfigManagerMu.Lock()
	defer globalConfigManagerMu.Unlock()
	globalConfigManager = cm
}

// GetGlobalConfigManager trả về global ConfigManager instance
// Hàm này được sử dụng bởi jobs để truy cập ConfigManager
// Trả về:
//   - *ConfigManager: Instance của ConfigManager, hoặc nil nếu chưa được set
//
// Lưu ý:
//   - Hàm này thread-safe, sử dụng RWMutex để cho phép đọc đồng thời
//   - Nếu ConfigManager chưa được set (nil), jobs sẽ sử dụng default values
//   - Nên kiểm tra nil trước khi sử dụng (hoặc dùng helper functions trong jobs package)
func GetGlobalConfigManager() *ConfigManager {
	globalConfigManagerMu.RLock()
	defer globalConfigManagerMu.RUnlock()
	return globalConfigManager
}

// NewConfigManager tạo một instance mới của ConfigManager
func NewConfigManager(s *scheduler.Scheduler) *ConfigManager {
	return &ConfigManager{
		localConfigPath:      "./config/agent-config.json",
		configData:           make(map[string]interface{}),
		scheduler:            s,
		needSubmitFullConfig: false,
	}
}

// LoadLocalConfig đọc config từ file local (ưu tiên khi khởi động)
func (cm *ConfigManager) LoadLocalConfig() error {
	data, err := os.ReadFile(cm.localConfigPath)
	if err != nil {
		// File chưa tồn tại → return error để caller biết cần initialize default
		return fmt.Errorf("local config file not found: %w", err)
	}

	var localConfig struct {
		Version       int64                  `json:"version"` // Unix timestamp (int64) - theo API v3.12
		ConfigHash    string                 `json:"configHash"`
		LastUpdatedAt int64                  `json:"lastUpdatedAt"`
		ConfigData    map[string]interface{} `json:"configData"`
	}

	if err := json.Unmarshal(data, &localConfig); err != nil {
		return fmt.Errorf("failed to parse local config: %w", err)
	}

	// Version là Unix timestamp (int64) - theo API v3.12
	// JSON unmarshal có thể trả về float64, nên cần convert
	version := localConfig.Version
	if version == 0 {
		// Nếu version là 0, có thể do JSON unmarshal trả về float64
		// Thử parse lại từ raw data
		var rawConfig map[string]interface{}
		if err := json.Unmarshal(data, &rawConfig); err == nil {
			if vRaw, exists := rawConfig["version"]; exists {
				switch v := vRaw.(type) {
				case float64:
					version = int64(v)
				case int64:
					version = v
				case int:
					version = int64(v)
				}
			}
		}
	}

	// Validate config
	if localConfig.ConfigData == nil {
		return fmt.Errorf("invalid local config: missing configData")
	}

	cm.currentVersion = version
	cm.currentHash = localConfig.ConfigHash
	cm.configData = localConfig.ConfigData

	// Apply config ngay khi load (để bot có thể chạy với config cũ nếu server offline)
	cm.applyConfig()

	return nil
}

// InitializeDefaultConfig tạo default config từ code (khi chưa có local/server config)
func (cm *ConfigManager) InitializeDefaultConfig() error {
	// Nếu đã có config → không làm gì
	if cm.currentVersion != 0 && cm.configData != nil && len(cm.configData) > 0 {
		return nil
	}

	// Tạo default config từ code
	cm.configData = make(map[string]interface{})

	// Agent-level default config - với metadata đầy đủ
	// Lưu ý: Chỉ giữ lại các config thực sự được sử dụng và hợp logic cho agent-level
	agentConfig := make(map[string]interface{})

	// Check-In Config (HOẠT ĐỘNG - được dùng trong main.go và checkin_service.go)
	checkInConfig := make(map[string]interface{})
	checkInConfig["interval"] = cm.createConfigField(
		60,
		"interval",
		"Khoảng thời gian giữa các lần check-in với server (giây). Giảm giá trị để monitoring realtime hơn nhưng tốn tài nguyên hơn.",
	)
	checkInConfig["enabled"] = cm.createConfigField(
		true,
		"enabled",
		"Bật/tắt check-in service. Nếu tắt, server sẽ không nhận được thông tin trạng thái của bot.",
	)
	checkInConfig["systemMetricsCacheInterval"] = cm.createConfigField(
		300,
		"systemMetricsCacheInterval",
		"Khoảng thời gian cache system metrics (CPU, Memory, Disk) - giây. Giảm tải hệ thống bằng cách không thu thập metrics mỗi check-in.",
	)
	agentConfig["checkIn"] = checkInConfig

	// Health Check Config (Đề xuất: Config cho health status calculation)
	healthCheckConfig := make(map[string]interface{})
	healthCheckConfig["cpuThreshold"] = cm.createConfigField(
		90.0,
		"cpuThreshold",
		"Ngưỡng CPU usage (%) để đánh giá health. Nếu CPU > threshold → 'degraded' hoặc 'unhealthy'.",
	)
	healthCheckConfig["memoryThreshold"] = cm.createConfigField(
		90.0,
		"memoryThreshold",
		"Ngưỡng Memory usage (%) để đánh giá health. Nếu Memory > threshold → 'degraded' hoặc 'unhealthy'.",
	)
	healthCheckConfig["diskThreshold"] = cm.createConfigField(
		90.0,
		"diskThreshold",
		"Ngưỡng Disk usage (%) để đánh giá health. Nếu Disk > threshold → 'degraded' hoặc 'unhealthy'.",
	)
	agentConfig["healthCheck"] = healthCheckConfig

	// Error Reporting Config (Đề xuất: Config cho error reporting trong check-in)
	errorReportingConfig := make(map[string]interface{})
	errorReportingConfig["maxErrorsPerCheckIn"] = cm.createConfigField(
		10,
		"maxErrorsPerCheckIn",
		"Số lượng errors tối đa được gửi trong mỗi check-in. Giảm để tránh payload quá lớn.",
	)
	errorReportingConfig["errorRetentionHours"] = cm.createConfigField(
		24,
		"errorRetentionHours",
		"Thời gian giữ lại errors để báo cáo (giờ). Chỉ báo cáo errors xảy ra trong khoảng thời gian này.",
	)
	agentConfig["errorReporting"] = errorReportingConfig

	cm.configData["agent"] = agentConfig

	// Job-level default config (từ scheduler) - với metadata đầy đủ
	// QUAN TRỌNG: Theo API v3.14, jobs phải là array, không phải object
	// Mỗi field trong job config giữ nguyên metadata (name, displayName, description, type, value)
	jobsArray := make([]interface{}, 0)
	if cm.scheduler != nil {
		for jobName := range cm.scheduler.GetJobs() {
			jobConfig := cm.createJobConfigWithMetadata(jobName)
			// Thêm field "name" vào job config (theo API v3.14)
			jobConfig["name"] = jobName
			jobsArray = append(jobsArray, jobConfig)
		}
	}
	cm.configData["jobs"] = jobsArray

	// Set version và hash
	cm.currentVersion = 0 // Chưa có version từ server
	cm.currentHash = cm.calculateHash(cm.configData)

	// Apply config vào runtime
	cm.applyConfig()

	// Lưu local để lần sau dùng
	if err := cm.SaveLocalConfig(); err != nil {
		// Log warning nhưng không fail
		log.Printf("[ConfigManager] Warning: Failed to save default config: %v", err)
	}

	return nil
}

// LoadLocalConfigWithFallback: Load local → Nếu không có → Initialize default
func (cm *ConfigManager) LoadLocalConfigWithFallback() error {
	// Bước 1: Ưu tiên load từ local
	err := cm.LoadLocalConfig()
	if err == nil {
		// Có config local → dùng luôn
		return nil
	}

	// Bước 2: Không có local config → Initialize default từ code
	log.Printf("[ConfigManager] Local config not found, initializing default config...")
	return cm.InitializeDefaultConfig()
}

// SaveLocalConfig lưu config vào file local
func (cm *ConfigManager) SaveLocalConfig() error {
	config := map[string]interface{}{
		"version":       cm.currentVersion,
		"configHash":    cm.currentHash,
		"lastUpdatedAt": time.Now().Unix(),
		"configData":    cm.configData,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Tạo thư mục nếu chưa có
	dir := filepath.Dir(cm.localConfigPath)
	os.MkdirAll(dir, 0755)

	return os.WriteFile(cm.localConfigPath, data, 0644)
}

// ShouldSubmitFullConfig kiểm tra xem có cần gửi full config không
func (cm *ConfigManager) ShouldSubmitFullConfig() bool {
	// Lần đầu (chưa có version trên server)
	if cm.currentVersion == 0 {
		return true
	}

	// Config thay đổi (hash khác)
	currentConfigData := cm.collectCurrentConfig()
	currentHash := cm.calculateHash(currentConfigData)
	if currentHash != cm.currentHash {
		return true
	}

	// Server yêu cầu (needFullConfig flag)
	if cm.needSubmitFullConfig {
		return true
	}

	// Không cần gửi full config
	return false
}

// MarkNeedSubmitFullConfig đánh dấu cần gửi full config (từ server response)
func (cm *ConfigManager) MarkNeedSubmitFullConfig() {
	cm.needSubmitFullConfig = true
}

// GetVersionAndHash trả về version và hash hiện tại (để gửi trong check-in)
func (cm *ConfigManager) GetVersionAndHash() (int64, string) {
	version := cm.currentVersion
	if version == 0 && cm.configData != nil {
		// Chưa có version từ server, nhưng có config → tính hash
		hash := cm.calculateHash(cm.configData)
		return 0, hash
	}

	hash := cm.currentHash
	if hash == "" && cm.configData != nil {
		// Tính hash từ config hiện tại
		hash = cm.calculateHash(cm.configData)
	}

	return version, hash
}

// SetVersionAndHash cập nhật version và hash sau khi apply config update
func (cm *ConfigManager) SetVersionAndHash(version int64, hash string) {
	cm.currentVersion = version
	cm.currentHash = hash

	// Lưu local để lần sau dùng
	if err := cm.SaveLocalConfig(); err != nil {
		log.Printf("[ConfigManager] Warning: Failed to save local config after update: %v", err)
	}
}

// GetConfigData trả về config data hiện tại (để đọc metadata của jobs)
func (cm *ConfigManager) GetConfigData() map[string]interface{} {
	return cm.configData
}

// ApplyConfigDiff áp dụng config diff từ server vào config hiện tại
// ConfigDiff có thể chứa:
// - agent: map[string]interface{} - các field thay đổi trong agent config
// - jobs: map[string]interface{} - các jobs có thay đổi (key: jobName, value: chỉ các field thay đổi)
// - deletedJobs: []string - các jobs bị xóa/disable
func (cm *ConfigManager) ApplyConfigDiff(configDiff map[string]interface{}) error {
	if configDiff == nil || len(configDiff) == 0 {
		return fmt.Errorf("config diff is empty")
	}

	log.Printf("[ConfigManager] 📥 Đang nhận config diff từ server...")

	// Đảm bảo có configData (nếu chưa có → initialize default)
	if cm.configData == nil || len(cm.configData) == 0 {
		if err := cm.InitializeDefaultConfig(); err != nil {
			return fmt.Errorf("failed to initialize default config: %w", err)
		}
	}

	// Deep merge config diff vào config hiện tại
	// Agent-level config diff
	if agentDiff, ok := configDiff["agent"].(map[string]interface{}); ok {
		log.Printf("[ConfigManager] 📝 Đang merge agent config diff...")
		if agentConfig, ok := cm.configData["agent"].(map[string]interface{}); ok {
			cm.mergeMap(agentConfig, agentDiff)
		} else {
			cm.configData["agent"] = agentDiff
		}
	}

	// Job-level config diff
	updatedJobs := []string{}
	if jobsDiff, ok := configDiff["jobs"].(map[string]interface{}); ok {
		log.Printf("[ConfigManager] 📝 Đang merge jobs config diff...")
		if jobsConfig, ok := cm.configData["jobs"].(map[string]interface{}); ok {
			// Merge từng job config
			for jobName, jobDiffRaw := range jobsDiff {
				if jobDiff, ok := jobDiffRaw.(map[string]interface{}); ok {
					if jobConfig, ok := jobsConfig[jobName].(map[string]interface{}); ok {
						cm.mergeMap(jobConfig, jobDiff)
						updatedJobs = append(updatedJobs, jobName)
					} else {
						// Job mới → tạo config mới
						jobsConfig[jobName] = jobDiff
						updatedJobs = append(updatedJobs, jobName)
					}
				}
			}
		} else {
			cm.configData["jobs"] = jobsDiff
		}
	}

	// Xóa jobs bị disable
	if deletedJobs, ok := configDiff["deletedJobs"].([]interface{}); ok {
		log.Printf("[ConfigManager] 🚫 Đang disable các jobs: %v", deletedJobs)
		if jobsConfig, ok := cm.configData["jobs"].(map[string]interface{}); ok {
			for _, jobNameRaw := range deletedJobs {
				if jobName, ok := jobNameRaw.(string); ok {
					delete(jobsConfig, jobName)
					// Disable job trong scheduler
					if cm.scheduler != nil {
						cm.scheduler.RemoveJob(jobName)
					}
				}
			}
		}
	}

	// Tính lại hash sau khi merge
	cm.currentHash = cm.calculateHash(cm.configData)

	// Apply config vào runtime
	log.Printf("[ConfigManager] 🔄 Đang apply config vào runtime...")
	cm.applyConfig()

	// Lưu local để lần sau dùng
	if err := cm.SaveLocalConfig(); err != nil {
		log.Printf("[ConfigManager] Warning: Failed to save local config after apply diff: %v", err)
	}

	if len(updatedJobs) > 0 {
		log.Printf("[ConfigManager] ✅ Đã apply config diff thành công cho %d jobs: %v", len(updatedJobs), updatedJobs)
		log.Printf("[ConfigManager] 💡 Các jobs sẽ đọc config mới khi chạy lần tiếp theo")
	} else {
		log.Printf("[ConfigManager] ✅ Đã apply config diff thành công")
	}
	return nil
}

// ApplyFullConfig áp dụng full config từ server (replace toàn bộ config hiện tại)
// Tham số:
// - configData: Full config data từ server
// - version: Version của config (Unix timestamp)
// - configHash: Hash của config
func (cm *ConfigManager) ApplyFullConfig(configData map[string]interface{}, version int64, configHash string) error {
	if configData == nil || len(configData) == 0 {
		return fmt.Errorf("config data không được để trống")
	}

	log.Printf("[ConfigManager] 📥 Đang nhận full config từ server: version %d, hash %s", version, configHash)

	// Đếm số jobs trong config mới
	jobCount := 0
	if jobsConfig, ok := configData["jobs"].(map[string]interface{}); ok {
		jobCount = len(jobsConfig)
	}

	log.Printf("[ConfigManager] 📊 Config mới có %d jobs", jobCount)

	// Replace toàn bộ config
	cm.configData = configData
	cm.currentVersion = version
	cm.currentHash = configHash

	// Apply config vào runtime
	log.Printf("[ConfigManager] 🔄 Đang apply config vào runtime...")
	cm.applyConfig()

	// Lưu local để lần sau dùng
	if err := cm.SaveLocalConfig(); err != nil {
		log.Printf("[ConfigManager] Warning: Failed to save local config: %v", err)
	}

	log.Printf("[ConfigManager] ✅ Đã apply full config thành công (version: %d, hash: %s)", version, configHash)
	log.Printf("[ConfigManager] 💡 Các jobs sẽ đọc config mới khi chạy lần tiếp theo")
	return nil
}

// mergeMap merge map2 vào map1 (deep merge)
func (cm *ConfigManager) mergeMap(map1, map2 map[string]interface{}) {
	for key, value2 := range map2 {
		if value1, exists := map1[key]; exists {
			// Nếu cả 2 đều là map → merge recursive
			if map1Value, ok1 := value1.(map[string]interface{}); ok1 {
				if map2Value, ok2 := value2.(map[string]interface{}); ok2 {
					cm.mergeMap(map1Value, map2Value)
					continue
				}
			}
		}
		// Override hoặc thêm mới
		map1[key] = value2
	}
}

// SubmitConfig gửi config hiện tại lên server
// Tối ưu: Chỉ gửi full config khi cần thiết
// QUAN TRỌNG: Có mutex để tránh submit config nhiều lần đồng thời
func (cm *ConfigManager) SubmitConfig(forceFullConfig bool) error {
	// Lock mutex để đảm bảo chỉ 1 goroutine submit config tại 1 thời điểm
	cm.submitMutex.Lock()
	defer cm.submitMutex.Unlock()

	// Kiểm tra xem đang submit không (tránh submit trùng lặp)
	if cm.isSubmitting {
		log.Printf("[ConfigManager] ⚠️  Đang submit config, bỏ qua request trùng lặp")
		return nil
	}

	// Đảm bảo có config (nếu chưa có → initialize default)
	if cm.configData == nil || len(cm.configData) == 0 {
		if err := cm.InitializeDefaultConfig(); err != nil {
			return fmt.Errorf("failed to initialize default config: %w", err)
		}
	}

	// Thu thập config hiện tại (merge với runtime values)
	configData := cm.collectCurrentConfig()

	// Tính hash từ configData thuần (không có metadata)
	hash := cm.calculateHash(configData)

	// Nếu đã có version trên server và hash không đổi → không cần submit
	if !forceFullConfig {
		if cm.currentVersion != 0 {
			if cm.currentHash == hash {
				log.Printf("[ConfigManager] Config không thay đổi (version: %d, hash: %s), bỏ qua submit", cm.currentVersion, hash)
				return nil
			}
		}

		// Reset flag sau khi submit
		cm.needSubmitFullConfig = false
	}

	// QUAN TRỌNG: Kiểm tra xem đã submit config với hash này chưa (tránh submit trùng lặp)
	// Nếu hash giống với hash hiện tại và đã có version → không submit lại
	if cm.currentHash == hash && cm.currentVersion != 0 {
		log.Printf("[ConfigManager] ⚠️  Config với hash %s đã được submit (version: %d), bỏ qua submit trùng lặp", hash, cm.currentVersion)
		return nil
	}

	// Đánh dấu đang submit
	cm.isSubmitting = true
	defer func() {
		cm.isSubmitting = false
	}()

	log.Printf("[ConfigManager] 📤 Đang submit config lên server (hash: %s, version hiện tại: %d)...", hash, cm.currentVersion)
	log.Printf("[ConfigManager] 🔍 AgentId được sử dụng để submit: %s (length: %d)", global.GlobalConfig.AgentId, len(global.GlobalConfig.AgentId))

	// QUAN TRỌNG: Kiểm tra agentId trước khi submit
	if global.GlobalConfig.AgentId == "" {
		log.Printf("[ConfigManager] ❌ LỖI: AgentId rỗng! Không thể submit config")
		return fmt.Errorf("AgentId không được để trống")
	}

	// Thu thập config với metadata inline (để server hiểu mục đích)
	fullConfig := cm.CollectCurrentConfig()

	// Log để kiểm tra xem có agentId trong configData không (không nên có)
	if agentConfig, ok := fullConfig["agent"].(map[string]interface{}); ok {
		if agentIdInConfig, exists := agentConfig["agentId"]; exists {
			log.Printf("[ConfigManager] ⚠️  CẢNH BÁO: Tìm thấy agentId trong configData: %v (không nên có)", agentIdInConfig)
		}
	}

	// Gửi full config lên server (bao gồm cả metadata inline)
	result, err := integrations.FolkForm_SubmitConfig(global.GlobalConfig.AgentId, fullConfig, hash)
	if err != nil {
		// Nếu server offline → Log warning nhưng không fail
		log.Printf("[ConfigManager] Warning: Failed to submit config to server: %v", err)
		log.Printf("[ConfigManager] Bot sẽ tiếp tục chạy với config hiện tại")
		return nil // Không return error để bot vẫn chạy được
	}

	// Lưu version và hash từ server (version là int64 từ backend v3.12+)
	// FolkForm_SubmitConfig đã parse và trả về version dạng int64
	if version, ok := result["version"].(int64); ok {
		cm.currentVersion = version
	} else if versionFloat, ok := result["version"].(float64); ok {
		// JSON unmarshal có thể trả về float64
		cm.currentVersion = int64(versionFloat)
	}
	if hash, ok := result["configHash"].(string); ok {
		cm.currentHash = hash
	}

	// Lưu configData
	cm.configData = configData

	// Lưu local để lần sau dùng
	if err := cm.SaveLocalConfig(); err != nil {
		log.Printf("[ConfigManager] Warning: Failed to save local config: %v", err)
	}

	log.Printf("[ConfigManager] ✅ Đã submit config lên server thành công, version: %d, hash: %s", cm.currentVersion, cm.currentHash)
	return nil
}

// PullConfig kéo config mới từ server (optional - thường không cần vì đã có trong check-in response)
func (cm *ConfigManager) PullConfig() error {
	config, err := integrations.FolkForm_GetCurrentConfig(global.GlobalConfig.AgentId)
	if err != nil {
		return err
	}

	// Verify hash
	expectedHash := cm.calculateHash(config.ConfigData)
	if expectedHash != config.ConfigHash {
		return fmt.Errorf("config hash mismatch")
	}

	// Apply config
	cm.currentVersion = config.Version
	cm.currentHash = config.ConfigHash
	cm.configData = config.ConfigData

	// Lưu local
	cm.SaveLocalConfig()

	// Apply config vào runtime
	cm.applyConfig()

	return nil
}

// CollectCurrentConfig thu thập config hiện tại từ runtime (public method)
// Theo API v3.14: Loại bỏ metadata chung của job (displayName, description, icon, color, category, tags)
// Config chỉ chứa job definition (name, enabled, schedule, timeout, retries, params)
func (cm *ConfigManager) CollectCurrentConfig() map[string]interface{} {
	config := cm.collectCurrentConfig()
	// Cleanup metadata chung của job trước khi submit (theo API v3.14)
	cm.cleanupJobMetadata(config)
	return config
}

// collectCurrentConfig thu thập config hiện tại từ runtime (internal)
func (cm *ConfigManager) collectCurrentConfig() map[string]interface{} {
	// Nếu đã có configData (từ local hoặc server) → merge với runtime values
	if cm.configData != nil && len(cm.configData) > 0 {
		config := cm.mergeWithRuntime(cm.configData)
		return config
	}

	// Nếu chưa có config → Tạo từ runtime (default) - với metadata đầy đủ
	// Sử dụng lại logic từ InitializeDefaultConfig để đảm bảo consistency
	config := make(map[string]interface{})

	// Agent-level config - với metadata đầy đủ
	// Lưu ý: Chỉ giữ lại các config thực sự được sử dụng và hợp logic cho agent-level
	agentConfig := make(map[string]interface{})

	// Mô tả tổng quan về agent
	agentConfig["description"] = "Cấu hình chung cho FolkForm Agent. Agent này quản lý việc đồng bộ dữ liệu giữa Pancake và FolkForm, bao gồm conversations, posts, customers, và Pancake POS data. Tất cả các jobs được quản lý và lập lịch tự động."

	// Check-In Config (HOẠT ĐỘNG - được dùng trong main.go và checkin_service.go)
	checkInConfig := make(map[string]interface{})
	checkInConfig["interval"] = cm.createConfigField(
		60,
		"interval",
		"Khoảng thời gian giữa các lần check-in với server (giây). Giảm giá trị để monitoring realtime hơn nhưng tốn tài nguyên hơn.",
	)
	checkInConfig["enabled"] = cm.createConfigField(
		true,
		"enabled",
		"Bật/tắt check-in service. Nếu tắt, server sẽ không nhận được thông tin trạng thái của bot.",
	)
	checkInConfig["systemMetricsCacheInterval"] = cm.createConfigField(
		300,
		"systemMetricsCacheInterval",
		"Khoảng thời gian cache system metrics (CPU, Memory, Disk) - giây. Giảm tải hệ thống bằng cách không thu thập metrics mỗi check-in.",
	)
	agentConfig["checkIn"] = checkInConfig

	// Health Check Config (Đề xuất: Config cho health status calculation)
	healthCheckConfig := make(map[string]interface{})
	healthCheckConfig["cpuThreshold"] = cm.createConfigField(
		90.0,
		"cpuThreshold",
		"Ngưỡng CPU usage (%) để đánh giá health. Nếu CPU > threshold → 'degraded' hoặc 'unhealthy'.",
	)
	healthCheckConfig["memoryThreshold"] = cm.createConfigField(
		90.0,
		"memoryThreshold",
		"Ngưỡng Memory usage (%) để đánh giá health. Nếu Memory > threshold → 'degraded' hoặc 'unhealthy'.",
	)
	healthCheckConfig["diskThreshold"] = cm.createConfigField(
		90.0,
		"diskThreshold",
		"Ngưỡng Disk usage (%) để đánh giá health. Nếu Disk > threshold → 'degraded' hoặc 'unhealthy'.",
	)
	agentConfig["healthCheck"] = healthCheckConfig

	// Error Reporting Config (Đề xuất: Config cho error reporting trong check-in)
	errorReportingConfig := make(map[string]interface{})
	errorReportingConfig["maxErrorsPerCheckIn"] = cm.createConfigField(
		10,
		"maxErrorsPerCheckIn",
		"Số lượng errors tối đa được gửi trong mỗi check-in. Giảm để tránh payload quá lớn.",
	)
	errorReportingConfig["errorRetentionHours"] = cm.createConfigField(
		24,
		"errorRetentionHours",
		"Thời gian giữ lại errors để báo cáo (giờ). Chỉ báo cáo errors xảy ra trong khoảng thời gian này.",
	)
	agentConfig["errorReporting"] = errorReportingConfig

	config["agent"] = agentConfig

	// Job-level config (từ scheduler) - với metadata đầy đủ
	// QUAN TRỌNG: Theo API v3.14, jobs phải là array, không phải object
	// Mỗi field trong job config giữ nguyên metadata (name, displayName, description, type, value)
	jobsArray := make([]interface{}, 0)
	if cm.scheduler != nil {
		// Lấy jobs từ scheduler - cần implement GetJobByName hoặc iterate
		// Tạm thời dùng GetJobs() để lấy danh sách
		for jobName := range cm.scheduler.GetJobs() {
			jobConfig := cm.createJobConfigWithMetadata(jobName)
			// Thêm field "name" vào job config (theo API v3.14)
			jobConfig["name"] = jobName
			jobsArray = append(jobsArray, jobConfig)
		}
	}
	config["jobs"] = jobsArray

	return config
}

// mergeWithRuntime merge config hiện tại với runtime values
// QUAN TRỌNG: Giữ nguyên metadata (value, name, description) khi override
func (cm *ConfigManager) mergeWithRuntime(configData map[string]interface{}) map[string]interface{} {
	// Deep copy để không modify original
	merged := make(map[string]interface{})

	// Copy toàn bộ configData
	data, _ := json.Marshal(configData)
	json.Unmarshal(data, &merged)

	// Đảm bảo có giá trị mới nhất từ ENV nhưng vẫn giữ metadata
	if agentConfig, ok := merged["agent"].(map[string]interface{}); ok {
		// Đảm bảo có mô tả agent nếu chưa có
		if _, hasDesc := agentConfig["description"]; !hasDesc {
			agentConfig["description"] = "Cấu hình chung cho FolkForm Agent. Agent này quản lý việc đồng bộ dữ liệu giữa Pancake và FolkForm, bao gồm conversations, posts, customers, và Pancake POS data. Tất cả các jobs được quản lý và lập lịch tự động."
		}

		if apiConfig, ok := agentConfig["api"].(map[string]interface{}); ok {
			// Override với ENV values (nếu có) nhưng giữ metadata structure
			if global.GlobalConfig.ApiBaseUrl != "" {
				baseUrlField := apiConfig["baseUrl"]
				if baseUrlMap, ok := baseUrlField.(map[string]interface{}); ok {
					// Có metadata structure → chỉ update value
					baseUrlMap["value"] = global.GlobalConfig.ApiBaseUrl
				} else {
					// Không có metadata → tạo mới với metadata
					apiConfig["baseUrl"] = cm.createConfigField(
						global.GlobalConfig.ApiBaseUrl,
						"baseUrl",
						"URL base của FolkForm API backend. Bắt buộc phải có, lấy từ ENV variable API_BASE_URL.",
					)
				}
			}
			if global.GlobalConfig.PancakeBaseUrl != "" {
				pancakeBaseUrlField := apiConfig["pancakeBaseUrl"]
				if pancakeBaseUrlMap, ok := pancakeBaseUrlField.(map[string]interface{}); ok {
					// Có metadata structure → chỉ update value
					pancakeBaseUrlMap["value"] = global.GlobalConfig.PancakeBaseUrl
				} else {
					// Không có metadata → tạo mới với metadata
					apiConfig["pancakeBaseUrl"] = cm.createConfigField(
						global.GlobalConfig.PancakeBaseUrl,
						"pancakeBaseUrl",
						"URL base của Pancake API. Bắt buộc phải có, lấy từ ENV variable PANCAKE_BASE_URL.",
					)
				}
			}
		}
	}

	// QUAN TRỌNG: Đảm bảo jobs config được thêm vào từ runtime (scheduler)
	// Merge jobs từ scheduler với jobs từ configData
	jobsConfig := make(map[string]interface{})

	// Copy jobs từ configData nếu có
	if existingJobs, ok := merged["jobs"].(map[string]interface{}); ok {
		// Deep copy existing jobs
		existingJobsData, _ := json.Marshal(existingJobs)
		json.Unmarshal(existingJobsData, &jobsConfig)
	}

	// Thêm/update jobs từ scheduler (runtime) - đảm bảo có đầy đủ config cho tất cả jobs
	if cm.scheduler != nil {
		for jobName := range cm.scheduler.GetJobs() {
			// Tạo config mới với metadata đầy đủ từ runtime
			fullJobConfig := cm.createJobConfigWithMetadata(jobName)

			if existingJobConfig, exists := jobsConfig[jobName]; exists {
				// Job đã có trong config → merge: giữ giá trị từ config cũ, nhưng thêm metadata nếu thiếu
				if existingJobConfigMap, ok := existingJobConfig.(map[string]interface{}); ok {
					// Merge từng field: giữ value từ existing, nhưng dùng metadata từ fullJobConfig nếu thiếu
					for fieldName, fullFieldConfig := range fullJobConfig {
						if fieldName == "description" {
							// Đảm bảo có description
							if _, hasDesc := existingJobConfigMap["description"]; !hasDesc {
								existingJobConfigMap["description"] = fullFieldConfig
							}
							continue
						}

						// Kiểm tra field trong existing config
						if existingFieldValue, exists := existingJobConfigMap[fieldName]; exists {
							// Field đã có → kiểm tra xem có metadata structure không
							if _, ok := existingFieldValue.(map[string]interface{}); ok {
								// Đã có metadata structure → giữ nguyên (có thể đã được server update)
								continue
							} else {
								// Không có metadata structure → dùng metadata từ fullJobConfig nhưng giữ value
								if fullFieldMap, ok := fullFieldConfig.(map[string]interface{}); ok {
									// Copy metadata structure và update value
									newFieldMap := make(map[string]interface{})
									for k, v := range fullFieldMap {
										newFieldMap[k] = v
									}
									newFieldMap["value"] = existingFieldValue
									existingJobConfigMap[fieldName] = newFieldMap
								}
							}
						} else {
							// Field chưa có → thêm từ fullJobConfig
							existingJobConfigMap[fieldName] = fullFieldConfig
						}
					}
				}
			} else {
				// Job chưa có trong config → dùng config mới với metadata đầy đủ
				jobsConfig[jobName] = fullJobConfig
			}
		}
	}

	// Gán jobs config vào merged
	// QUAN TRỌNG: Theo API v3.14, jobs phải là array, không phải object
	// Mỗi field trong job config giữ nguyên metadata (name, displayName, description, type, value)
	jobsArray := make([]interface{}, 0)
	for jobName, jobConfigRaw := range jobsConfig {
		jobConfig, ok := jobConfigRaw.(map[string]interface{})
		if !ok {
			continue
		}
		// Thêm field "name" vào job config
		jobConfig["name"] = jobName
		jobsArray = append(jobsArray, jobConfig)
	}
	merged["jobs"] = jobsArray

	return merged
}

// calculateHash tính SHA256 hash của config
func (cm *ConfigManager) calculateHash(configData map[string]interface{}) string {
	data, _ := json.Marshal(configData)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// applyConfig áp dụng config vào runtime
// Priority: ENV > Config từ server > Local config > Default
// Lưu ý: configData có thể có metadata inline (field.value) hoặc giá trị trực tiếp
func (cm *ConfigManager) applyConfig() {
	if cm.configData == nil {
		return
	}

	// Extract giá trị từ config (có thể có metadata inline)
	agentConfig := cm.extractValue(cm.configData["agent"])
	if agentConfigMap, ok := agentConfig.(map[string]interface{}); ok {
		// Apply job execution config (dùng chung)
		jobExecConfig := cm.extractValue(agentConfigMap["jobExecution"])
		if jobExecConfigMap, ok := jobExecConfig.(map[string]interface{}); ok {
			cm.applyJobExecutionConfig(jobExecConfigMap)
		}
	}

	// Apply job-level config
	jobsConfig := cm.extractValue(cm.configData["jobs"])
	if jobsConfigMap, ok := jobsConfig.(map[string]interface{}); ok {
		appliedCount := 0
		for jobName, jobConfigRaw := range jobsConfigMap {
			jobConfig := cm.extractValue(jobConfigRaw)
			if jobConfigMap, ok := jobConfig.(map[string]interface{}); ok {
				// Check enabled (có thể là field.value hoặc giá trị trực tiếp)
				enabled := cm.extractValue(jobConfigMap["enabled"])
				if enabledBool, ok := enabled.(bool); ok && !enabledBool {
					// Disable job - chỉ remove nếu job đã tồn tại trong scheduler
					if cm.scheduler != nil {
						// Kiểm tra xem job có tồn tại không trước khi remove
						jobs := cm.scheduler.GetJobs()
						if _, exists := jobs[jobName]; exists {
							log.Printf("[ConfigManager] 🚫 Disable job: %s (theo config)", jobName)
							cm.scheduler.RemoveJob(jobName)
						} else {
							log.Printf("[ConfigManager] ⚠️  Job %s không tồn tại trong scheduler, bỏ qua disable", jobName)
						}
					}
					continue
				}

				// Update schedule nếu có (override schedule từ code)
				if scheduleRaw, ok := jobConfigMap["schedule"]; ok {
					schedule := cm.extractValue(scheduleRaw)
					if scheduleStr, ok := schedule.(string); ok && scheduleStr != "" {
						// Kiểm tra xem job có tồn tại trong scheduler không
						if cm.scheduler != nil {
							jobs := cm.scheduler.GetJobs()
							if _, exists := jobs[jobName]; exists {
								// Lấy schedule hiện tại để so sánh
								if job := cm.scheduler.GetJobObject(jobName); job != nil {
									currentSchedule := job.GetSchedule()
									if currentSchedule != scheduleStr {
										log.Printf("[ConfigManager] 📅 Cập nhật schedule cho job: %s (từ '%s' sang '%s')", jobName, currentSchedule, scheduleStr)
										if err := cm.scheduler.UpdateJobSchedule(jobName, scheduleStr); err != nil {
											log.Printf("[ConfigManager] ❌ Lỗi khi cập nhật schedule cho job %s: %v", jobName, err)
										} else {
											log.Printf("[ConfigManager] ✅ Đã cập nhật schedule cho job: %s", jobName)
										}
									}
								}
							} else {
								log.Printf("[ConfigManager] ⚠️  Job %s chưa được đăng ký trong scheduler, không thể cập nhật schedule", jobName)
							}
						}
					}
				}

				// Apply job-specific config (timeout, retry, batchSize, workHours, logging, etc.)
				// Config được lưu trong configData, jobs sẽ đọc khi chạy thông qua GetJobConfig* helpers
				cm.applyJobConfig(jobName, jobConfigMap)
				appliedCount++
			}
		}
		if appliedCount > 0 {
			log.Printf("[ConfigManager] 📋 Đã apply config cho %d jobs. Các jobs sẽ đọc config mới khi chạy lần tiếp theo", appliedCount)
		}
	}
}

// extractValue trích xuất giá trị từ field (có thể là {value, name, description} hoặc giá trị trực tiếp)
func (cm *ConfigManager) extractValue(field interface{}) interface{} {
	if fieldMap, ok := field.(map[string]interface{}); ok {
		// Có thể là metadata inline {value, name, description}
		if value, ok := fieldMap["value"]; ok {
			return value
		}
		// Không có "value" → có thể là nested object, giữ nguyên
		// Recursively extract values từ nested objects
		result := make(map[string]interface{})
		for k, v := range fieldMap {
			result[k] = cm.extractValue(v)
		}
		return result
	}
	// Không phải map → giá trị trực tiếp
	return field
}

// applyJobExecutionConfig áp dụng config chung cho job execution
func (cm *ConfigManager) applyJobExecutionConfig(jobExecConfig map[string]interface{}) {
	// Lưu config để BaseJob có thể sử dụng
	// Tạm thời chỉ lưu, sẽ implement logic apply sau
}

// applyJobConfig áp dụng config cho job cụ thể
// Lưu ý: Config được lưu trong configData, jobs sẽ đọc khi chạy thông qua GetJobConfig* helpers
// Các giá trị như pageSize, timeout, maxRetries sẽ được đọc động mỗi lần job chạy
func (cm *ConfigManager) applyJobConfig(jobName string, jobConfig map[string]interface{}) {
	// Config được lưu trong configData, không cần apply trực tiếp vào job object
	// Jobs sẽ đọc config động mỗi lần chạy thông qua:
	// - GetJobConfigInt(jobName, "pageSize", defaultValue)
	// - GetJobConfigBool(jobName, "enabled", defaultValue)
	// - GetJobConfigString(jobName, "schedule", defaultValue)
	// Điều này đảm bảo config từ server luôn được sử dụng ngay khi có update
}

// GetJobConfig lấy toàn bộ config cho một job cụ thể
// Hàm này trả về map chứa tất cả các fields config của job (enabled, timeout, pageSize, etc.)
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//
// Trả về:
//   - map[string]interface{}: Map chứa config của job (đã extract value từ metadata)
//     Ví dụ: {"enabled": true, "timeout": 600, "pageSize": 50, "maxRetries": 3}
//   - nil: Nếu không tìm thấy config cho job này
//
// Lưu ý:
//   - Config có thể chứa metadata inline (value, name, description, type)
//   - Hàm này tự động extract value từ metadata nếu có
//   - Nếu config không có metadata, trả về giá trị trực tiếp
func (cm *ConfigManager) GetJobConfig(jobName string) map[string]interface{} {
	if cm.configData == nil {
		return nil
	}

	jobsConfig := cm.extractValue(cm.configData["jobs"])
	if jobsConfigMap, ok := jobsConfig.(map[string]interface{}); ok {
		if jobConfigRaw, exists := jobsConfigMap[jobName]; exists {
			jobConfig := cm.extractValue(jobConfigRaw)
			if jobConfigMap, ok := jobConfig.(map[string]interface{}); ok {
				return jobConfigMap
			}
		}
	}

	return nil
}

// GetJobConfigValue lấy giá trị config cho một field cụ thể của job
// Hàm này tìm field trong config của job và trả về giá trị (đã extract từ metadata nếu có)
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//   - fieldName: Tên của field cần lấy (ví dụ: "pageSize", "timeout", "enabled")
//
// Trả về:
//   - interface{}: Giá trị của field (có thể là bất kỳ kiểu nào: int, string, bool, map, array)
//   - bool: true nếu tìm thấy field, false nếu không tìm thấy
//
// Lưu ý:
//   - Nếu field có metadata inline (value, name, description, type), hàm sẽ tự động extract value
//   - Nếu field không có metadata, trả về giá trị trực tiếp
//   - Nếu job không có config, trả về (nil, false)
func (cm *ConfigManager) GetJobConfigValue(jobName, fieldName string) (interface{}, bool) {
	jobConfig := cm.GetJobConfig(jobName)
	if jobConfig == nil {
		return nil, false
	}

	value := cm.extractValue(jobConfig[fieldName])
	return value, true
}

// GetJobConfigInt lấy giá trị int từ config với fallback về default value
// Hàm này tự động convert các kiểu số (int, int64, float64) sang int
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//   - fieldName: Tên của field cần lấy (ví dụ: "pageSize", "timeout")
//   - defaultValue: Giá trị mặc định nếu không tìm thấy hoặc không thể convert sang int
//
// Trả về:
//   - int: Giá trị int từ config, hoặc defaultValue nếu không tìm thấy/không hợp lệ
//
// Lưu ý:
//   - Hỗ trợ convert từ int, int64, float64 sang int
//   - Nếu giá trị không phải số, trả về defaultValue
//   - Nếu không tìm thấy field, trả về defaultValue
func (cm *ConfigManager) GetJobConfigInt(jobName, fieldName string, defaultValue int) int {
	value, ok := cm.GetJobConfigValue(jobName, fieldName)
	if !ok {
		return defaultValue
	}

	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return defaultValue
	}
}

// GetJobConfigBool lấy giá trị bool từ config với fallback về default value
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//   - fieldName: Tên của field cần lấy (ví dụ: "enabled")
//   - defaultValue: Giá trị mặc định nếu không tìm thấy hoặc không phải bool
//
// Trả về:
//   - bool: Giá trị bool từ config, hoặc defaultValue nếu không tìm thấy/không hợp lệ
//
// Lưu ý:
//   - Chỉ trả về true nếu giá trị là bool và bằng true
//   - Nếu giá trị không phải bool, trả về defaultValue
//   - Nếu không tìm thấy field, trả về defaultValue
func (cm *ConfigManager) GetJobConfigBool(jobName, fieldName string, defaultValue bool) bool {
	value, ok := cm.GetJobConfigValue(jobName, fieldName)
	if !ok {
		return defaultValue
	}

	if boolValue, ok := value.(bool); ok {
		return boolValue
	}

	return defaultValue
}

// GetJobConfigString lấy giá trị string từ config với fallback về default value
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//   - fieldName: Tên của field cần lấy (ví dụ: "schedule")
//   - defaultValue: Giá trị mặc định nếu không tìm thấy hoặc không phải string
//
// Trả về:
//   - string: Giá trị string từ config, hoặc defaultValue nếu không tìm thấy/không hợp lệ
//
// Lưu ý:
//   - Chỉ trả về string nếu giá trị thực sự là string
//   - Nếu giá trị không phải string, trả về defaultValue
//   - Nếu không tìm thấy field, trả về defaultValue
func (cm *ConfigManager) GetJobConfigString(jobName, fieldName string, defaultValue string) string {
	value, ok := cm.GetJobConfigValue(jobName, fieldName)
	if !ok {
		return defaultValue
	}

	if strValue, ok := value.(string); ok {
		return strValue
	}

	return defaultValue
}

// GetCheckInInterval trả về check-in interval từ config
func (cm *ConfigManager) GetCheckInInterval() int {
	if cm.configData == nil {
		return 60 // Default 60 giây
	}

	if agentConfig, ok := cm.configData["agent"].(map[string]interface{}); ok {
		if checkInConfig, ok := agentConfig["checkIn"].(map[string]interface{}); ok {
			if interval, ok := checkInConfig["interval"].(float64); ok {
				return int(interval)
			}
		}
	}

	return 60 // Default
}

// createConfigField tạo một config field với metadata (value, name, description, type)
// Giúp user hiểu được ý nghĩa và kiểu dữ liệu của từng config field
func (cm *ConfigManager) createConfigField(value interface{}, name, description string) map[string]interface{} {
	// Xác định type của value
	fieldType := cm.getFieldType(value)

	return map[string]interface{}{
		"value":       value,
		"name":        name,
		"description": description,
		"type":        fieldType, // Data type: "string", "number", "boolean", "object", "array"
	}
}

// getFieldType xác định type của value
func (cm *ConfigManager) getFieldType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		// Kiểm tra type bằng reflection nếu cần
		valueType := reflect.TypeOf(value)
		if valueType == nil {
			return "null"
		}
		kind := valueType.Kind()
		if kind == reflect.Slice || kind == reflect.Array {
			return "array"
		}
		if kind == reflect.Map {
			return "object"
		}
		return "unknown"
	}
}

// createJobConfigWithMetadata tạo config cho một job với metadata đầy đủ
// Mỗi job sẽ có các config cụ thể tùy theo loại job
func (cm *ConfigManager) createJobConfigWithMetadata(jobName string) map[string]interface{} {
	jobConfig := make(map[string]interface{})

	// Mô tả tổng quan về job (giúp user hiểu job này làm gì)
	jobDescription := cm.getJobDescription(jobName)
	if jobDescription != "" {
		jobConfig["description"] = jobDescription
	}

	// Config chung cho tất cả jobs
	jobConfig["enabled"] = cm.createConfigField(
		true,
		"enabled",
		"Bật/tắt job. Nếu false, job sẽ không được chạy.",
	)

	// Lấy schedule hiện tại từ scheduler (nếu job đã được đăng ký)
	if cm.scheduler != nil {
		if job := cm.scheduler.GetJobObject(jobName); job != nil {
			currentSchedule := job.GetSchedule()
			jobConfig["schedule"] = cm.createConfigField(
				currentSchedule,
				"schedule",
				"Lịch chạy của job theo định dạng cron (6 trường: giây phút giờ ngày tháng thứ). Ví dụ: '0 */1 8-23 * * *' = chạy mỗi 1 phút từ 8h-23h. Có thể thay đổi để điều chỉnh tần suất chạy job.",
			)
		}
	}

	// Config cụ thể cho từng loại job
	switch jobName {
	// ========================================
	// CONVERSATIONS JOBS
	// ========================================
	case "sync-incremental-conversations-job":
		jobConfig["timeout"] = cm.createConfigField(
			600,
			"timeout",
			"Thời gian timeout tối đa cho job (giây). Nếu job chạy quá thời gian này sẽ bị hủy.",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			3,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại. Sau số lần này, job sẽ được đánh dấu failed.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			50,
			"pageSize",
			"Số lượng conversations được lấy mỗi lần gọi API. Tăng giá trị này để sync nhanh hơn nhưng tốn nhiều bộ nhớ hơn.",
		)

	case "sync-backfill-conversations-job":
		jobConfig["timeout"] = cm.createConfigField(
			1800,
			"timeout",
			"Thời gian timeout tối đa cho job (giây). Job backfill thường chạy lâu hơn nên timeout lớn hơn.",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			2,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			10,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			30,
			"pageSize",
			"Số lượng conversations cũ được lấy mỗi lần. Giảm giá trị để tránh quá tải khi sync dữ liệu cũ.",
		)

	case "sync-verify-conversations-job":
		jobConfig["timeout"] = cm.createConfigField(
			900,
			"timeout",
			"Thời gian timeout tối đa cho job verify (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			2,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			50,
			"pageSize",
			"Số lượng conversations được verify mỗi lần.",
		)

	case "sync-full-recovery-conversations-job":
		jobConfig["timeout"] = cm.createConfigField(
			3600,
			"timeout",
			"Thời gian timeout tối đa cho job full recovery (giây). Job này sync toàn bộ dữ liệu nên cần timeout lớn.",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			1,
			"maxRetries",
			"Số lần retry tối đa. Job full recovery chỉ nên retry 1 lần để tránh tốn tài nguyên.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			60,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			20,
			"pageSize",
			"Số lượng conversations được sync mỗi lần. Giảm để tránh quá tải khi sync toàn bộ dữ liệu.",
		)

	// ========================================
	// POSTS JOBS
	// ========================================
	case "sync-incremental-posts-job":
		jobConfig["timeout"] = cm.createConfigField(
			600,
			"timeout",
			"Thời gian timeout tối đa cho job (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			3,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			50,
			"pageSize",
			"Số lượng posts được lấy mỗi lần gọi API.",
		)

	case "sync-backfill-posts-job":
		jobConfig["timeout"] = cm.createConfigField(
			1800,
			"timeout",
			"Thời gian timeout tối đa cho job backfill (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			2,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			10,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			30,
			"pageSize",
			"Số lượng posts cũ được lấy mỗi lần.",
		)

	// ========================================
	// CUSTOMERS JOBS
	// ========================================
	case "sync-incremental-customers-job":
		jobConfig["timeout"] = cm.createConfigField(
			600,
			"timeout",
			"Thời gian timeout tối đa cho job (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			3,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			50,
			"pageSize",
			"Số lượng customers được lấy mỗi lần gọi API.",
		)

	case "sync-backfill-customers-job":
		jobConfig["timeout"] = cm.createConfigField(
			1800,
			"timeout",
			"Thời gian timeout tối đa cho job backfill (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			2,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			10,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			30,
			"pageSize",
			"Số lượng customers cũ được lấy mỗi lần.",
		)

	// ========================================
	// PANCAKE POS JOBS
	// ========================================
	case "sync-pancake-pos-shops-warehouses-job":
		jobConfig["timeout"] = cm.createConfigField(
			600,
			"timeout",
			"Thời gian timeout tối đa cho job (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			3,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)

	case "sync-incremental-pancake-pos-customers-job":
		jobConfig["timeout"] = cm.createConfigField(
			600,
			"timeout",
			"Thời gian timeout tối đa cho job (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			3,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			50,
			"pageSize",
			"Số lượng POS customers được lấy mỗi lần gọi API.",
		)

	case "sync-backfill-pancake-pos-customers-job":
		jobConfig["timeout"] = cm.createConfigField(
			1800,
			"timeout",
			"Thời gian timeout tối đa cho job backfill (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			2,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			10,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			30,
			"pageSize",
			"Số lượng POS customers cũ được lấy mỗi lần.",
		)

	case "sync-pancake-pos-products-job":
		jobConfig["timeout"] = cm.createConfigField(
			1800,
			"timeout",
			"Thời gian timeout tối đa cho job (giây). Job này sync products, variations và categories nên cần timeout lớn.",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			2,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			10,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			100,
			"pageSize",
			"Số lượng products được lấy mỗi lần gọi API Pancake POS.",
		)

	case "sync-incremental-pancake-pos-orders-job":
		jobConfig["timeout"] = cm.createConfigField(
			600,
			"timeout",
			"Thời gian timeout tối đa cho job (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			3,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			50,
			"pageSize",
			"Số lượng orders được lấy mỗi lần gọi API.",
		)

	case "sync-backfill-pancake-pos-orders-job":
		jobConfig["timeout"] = cm.createConfigField(
			1800,
			"timeout",
			"Thời gian timeout tối đa cho job backfill (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			2,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			10,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			30,
			"pageSize",
			"Số lượng orders cũ được lấy mỗi lần.",
		)

	// ========================================
	// WARNING JOBS
	// ========================================
	case "sync-warn-unreplied-conversations-job":
		jobConfig["timeout"] = cm.createConfigField(
			300,
			"timeout",
			"Thời gian timeout tối đa cho job (giây). Job này chỉ kiểm tra và gửi cảnh báo nên timeout ngắn.",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			2,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
		jobConfig["workHours"] = cm.createConfigField(
			map[string]interface{}{
				"start": "08:30",
				"end":   "22:30",
			},
			"workHours",
			"Khung giờ làm việc để gửi cảnh báo. Format: HH:MM (24h). Ngoài giờ này, job sẽ tự động skip.",
		)
		jobConfig["minDelayMinutes"] = cm.createConfigField(
			5,
			"minDelayMinutes",
			"Thời gian trễ tối thiểu (phút) để gửi cảnh báo. Conversations chưa trả lời dưới thời gian này sẽ không được cảnh báo.",
		)
		jobConfig["maxDelayMinutes"] = cm.createConfigField(
			300,
			"maxDelayMinutes",
			"Thời gian trễ tối đa (phút) để gửi cảnh báo. Conversations chưa trả lời quá thời gian này sẽ không được cảnh báo.",
		)
		jobConfig["notificationRateLimitMinutes"] = cm.createConfigField(
			5,
			"notificationRateLimitMinutes",
			"Thời gian tối thiểu giữa các lần gửi notification cho cùng một conversation (phút). Tránh spam notification.",
		)
		jobConfig["pageSize"] = cm.createConfigField(
			50,
			"pageSize",
			"Số lượng conversations được kiểm tra mỗi lần.",
		)

	default:
		// Config mặc định cho các job khác
		jobConfig["timeout"] = cm.createConfigField(
			600,
			"timeout",
			"Thời gian timeout tối đa cho job (giây).",
		)
		jobConfig["maxRetries"] = cm.createConfigField(
			3,
			"maxRetries",
			"Số lần retry tối đa khi job thất bại.",
		)
		jobConfig["retryDelay"] = cm.createConfigField(
			5,
			"retryDelay",
			"Thời gian delay giữa các lần retry (giây).",
		)
	}

	return jobConfig
}

// getJobDescription trả về mô tả tổng quan về job (job này làm gì)
func (cm *ConfigManager) getJobDescription(jobName string) string {
	switch jobName {
	// ========================================
	// CONVERSATIONS JOBS
	// ========================================
	case "sync-incremental-conversations-job":
		return "Đồng bộ các conversations mới/cập nhật gần đây từ Pancake về FolkForm. Job này chạy thường xuyên (mỗi 1 phút trong giờ làm việc) để đảm bảo dữ liệu real-time. Chỉ sync các conversations có updated_at mới hơn lastConversationId đã sync."

	case "sync-backfill-conversations-job":
		return "Đồng bộ các conversations cũ từ Pancake về FolkForm. Job này chạy ngoài giờ làm việc (mỗi 15 phút từ 0h-7h và 23h) để không ảnh hưởng hiệu năng. Sync các conversations có updated_at cũ hơn oldestUpdatedAt để đảm bảo không bỏ sót dữ liệu."

	case "sync-verify-conversations-job":
		return "Verify và đảm bảo đồng bộ 2 chiều giữa FolkForm và Pancake. Job này kiểm tra các conversations đã sync để đảm bảo dữ liệu nhất quán. Chạy mỗi 2 phút trong giờ làm việc."

	case "sync-full-recovery-conversations-job":
		return "Sync lại TOÀN BỘ conversations từ Pancake về FolkForm. Job này chạy mỗi ngày lúc 2h sáng để đảm bảo không bỏ sót conversations khi có lỗi. Không dựa vào checkpoint, sync lại tất cả từ đầu."

	// ========================================
	// POSTS JOBS
	// ========================================
	case "sync-incremental-posts-job":
		return "Đồng bộ các posts mới từ Pancake về FolkForm. Job này chạy mỗi 10 phút để lấy các posts mới hơn lastInsertedAt. Posts không cần sync quá thường xuyên như conversations."

	case "sync-backfill-posts-job":
		return "Đồng bộ các posts cũ từ Pancake về FolkForm. Job này chạy ngoài giờ làm việc (mỗi 30 phút từ 0h-7h và 23h) để lấy các posts cũ hơn oldestInsertedAt. Không ảnh hưởng hiệu năng trong giờ làm việc."

	// ========================================
	// CUSTOMERS JOBS
	// ========================================
	case "sync-incremental-customers-job":
		return "Đồng bộ các customers đã cập nhật gần đây từ Pancake về FolkForm. Job này chạy mỗi 15 phút để lấy các customers có updated_at từ lastUpdatedAt đến now. Đảm bảo thông tin khách hàng luôn được cập nhật."

	case "sync-backfill-customers-job":
		return "Đồng bộ các customers cập nhật cũ từ Pancake về FolkForm. Job này chạy mỗi ngày lúc 2h sáng để lấy các customers có updated_at từ 0 đến oldestUpdatedAt. Đảm bảo không bỏ sót dữ liệu khách hàng cũ."

	// ========================================
	// PANCAKE POS JOBS
	// ========================================
	case "sync-pancake-pos-shops-warehouses-job":
		return "Đồng bộ shops và warehouses từ Pancake POS về FolkForm. Job này chạy mỗi 30 phút để sync toàn bộ shops và warehouses. Shops và warehouses ít thay đổi nên không cần sync quá thường xuyên."

	case "sync-incremental-pancake-pos-customers-job":
		return "Đồng bộ các customers mới từ Pancake POS về FolkForm. Job này chạy mỗi 15 phút để lấy các customers có updated_at từ lastUpdatedAt đến now. Đảm bảo thông tin khách hàng POS luôn được cập nhật."

	case "sync-backfill-pancake-pos-customers-job":
		return "Đồng bộ các customers cũ từ Pancake POS về FolkForm. Job này chạy ngoài giờ làm việc (mỗi giờ từ 0h-7h và 23h) để lấy các customers cũ. Không ảnh hưởng hiệu năng trong giờ làm việc."

	case "sync-pancake-pos-products-job":
		return "Đồng bộ products, variations và categories từ Pancake POS về FolkForm. Job này chạy mỗi 30 phút để sync toàn bộ products, variations và categories. Products ít thay đổi nên không cần sync quá thường xuyên."

	case "sync-incremental-pancake-pos-orders-job":
		return "Đồng bộ các orders mới từ Pancake POS về FolkForm. Job này chạy mỗi 5 phút trong giờ làm việc (8h-23h) để đảm bảo orders real-time. Orders quan trọng nên cần sync thường xuyên trong giờ làm việc."

	case "sync-backfill-pancake-pos-orders-job":
		return "Đồng bộ các orders cũ từ Pancake POS về FolkForm. Job này chạy ngoài giờ làm việc (mỗi giờ từ 0h-7h và 23h) để lấy các orders cũ. Không ảnh hưởng hiệu năng trong giờ làm việc."

	// ========================================
	// WARNING JOBS
	// ========================================
	case "sync-warn-unreplied-conversations-job":
		return "Cảnh báo các hội thoại chưa được trả lời trong vòng 5-300 phút. Job này chạy mỗi 1 phút và tự động kiểm tra khung giờ làm việc (8h30-22h30). Chỉ gửi cảnh báo trong giờ làm việc và có rate limit 5 phút cho mỗi conversation để tránh spam."

	default:
		return "" // Không có mô tả cho job không xác định
	}
}

// getFieldDescription trả về mô tả cho một field cụ thể của job
func (cm *ConfigManager) getFieldDescription(jobName, fieldName string) string {
	// Mô tả chung cho các fields
	descriptions := map[string]string{
		"enabled":                      "Bật/tắt job. Nếu false, job sẽ không được chạy.",
		"timeout":                      "Thời gian timeout tối đa cho job (giây). Nếu job chạy quá thời gian này sẽ bị hủy.",
		"maxRetries":                   "Số lần retry tối đa khi job thất bại. Sau số lần này, job sẽ được đánh dấu failed.",
		"retryDelay":                   "Thời gian delay giữa các lần retry (giây).",
		"pageSize":                     "Số lượng items được lấy mỗi lần gọi API. Tăng giá trị này để sync nhanh hơn nhưng tốn nhiều bộ nhớ hơn.",
		"workHours":                    "Khung giờ làm việc (ví dụ: '8:30-22:30'). Job chỉ hoạt động trong khung giờ này.",
		"minDelayMinutes":              "Thời gian delay tối thiểu giữa các lần gửi notification (phút).",
		"maxDelayMinutes":              "Thời gian delay tối đa giữa các lần gửi notification (phút).",
		"notificationRateLimitMinutes": "Thời gian tối thiểu giữa các lần gửi notification cho cùng một conversation (phút). Tránh spam notification.",
	}

	// Trả về mô tả nếu có, nếu không trả về mô tả mặc định
	if desc, ok := descriptions[fieldName]; ok {
		return desc
	}

	// Mô tả mặc định
	return fmt.Sprintf("Cấu hình cho field %s của job %s", fieldName, jobName)
}

// cleanupJobMetadata loại bỏ metadata chung của job khỏi config (theo API v3.14)
// Metadata chung của job (displayName, description, icon, color, category, tags) đã được chuyển sang AgentRegistry.JobMetadata
// Config chỉ chứa job definition (name, enabled, schedule, timeout, retries, params)
// QUAN TRỌNG: Theo API v3.14, jobs phải là array, không phải object
// Tham số:
//   - config: Config data cần cleanup (sẽ được modify trực tiếp)
func (cm *ConfigManager) cleanupJobMetadata(config map[string]interface{}) {
	if config == nil {
		return
	}

	// Lấy jobs config - phải là array theo API v3.14
	jobsArray, ok := config["jobs"].([]interface{})
	if !ok {
		// Nếu là object (map) → convert sang array
		if jobsMap, ok := config["jobs"].(map[string]interface{}); ok {
			log.Printf("[ConfigManager] ⚠️  Jobs đang là object, đang convert sang array...")
			jobsArray = make([]interface{}, 0)
			for jobName, jobConfigRaw := range jobsMap {
				jobConfig, ok := jobConfigRaw.(map[string]interface{})
				if !ok {
					continue
				}
				// Thêm field "name" vào job config
				jobConfig["name"] = jobName
				jobsArray = append(jobsArray, jobConfig)
			}
			config["jobs"] = jobsArray
		} else {
			return // Không có jobs config hoặc format không hợp lệ
		}
	}

	// Metadata fields cần loại bỏ (theo API v3.14)
	metadataFields := []string{
		"displayName", // Tên hiển thị của job
		"description", // Mô tả của job
		"icon",        // Icon của job
		"color",       // Màu sắc của job
		"category",    // Danh mục của job
		"tags",        // Tags của job
	}

	// Loại bỏ metadata chung của job từ mỗi job trong array và extract values
	for i, jobConfigRaw := range jobsArray {
		jobConfig, ok := jobConfigRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Lấy job name để log
		jobName := "unknown"
		if name, ok := jobConfig["name"].(string); ok {
			jobName = name
		}

		// Loại bỏ từng metadata field chung của job (displayName, description, icon, color, category, tags)
		// Lưu ý: Metadata của các field trong job config (enabled, schedule, timeout, etc.) vẫn giữ nguyên
		// Mỗi field trong job config phải có metadata đầy đủ (name, displayName, description, type, value)
		for _, field := range metadataFields {
			if _, exists := jobConfig[field]; exists {
				delete(jobConfig, field)
				log.Printf("[ConfigManager] 🧹 Đã loại bỏ metadata field '%s' khỏi job '%s' (theo API v3.14)", field, jobName)
			}
		}

		// Đảm bảo có field "name"
		if _, exists := jobConfig["name"]; !exists {
			jobConfig["name"] = jobName
		}

		// Cập nhật lại trong array (giữ nguyên metadata của các field trong job config)
		jobsArray[i] = jobConfig
	}

	// Đảm bảo config["jobs"] là array
	config["jobs"] = jobsArray

	log.Printf("[ConfigManager] ✅ Đã cleanup metadata chung của job khỏi config (theo API v3.14)")
	// Lưu ý: Metadata của các field trong job config (enabled, schedule, timeout, etc.) vẫn giữ nguyên
	// Mỗi field trong job config phải có metadata đầy đủ (name, displayName, description, type, value)
}
