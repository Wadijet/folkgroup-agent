/*
Package logger chứa hệ thống logging với khả năng filter linh hoạt.
File này chứa cấu trúc config và quản lý filter log theo agent, job, log level, và phương thức log.
*/
package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// LogFilterConfig chứa cấu hình filter cho logging
type LogFilterConfig struct {
	// Enabled: Bật/tắt toàn bộ hệ thống filter (mặc định: false - tất cả log đều được ghi)
	Enabled bool `json:"enabled"`

	// DefaultAction: Hành động mặc định khi không match rule nào
	// "allow" = cho phép log (mặc định), "deny" = chặn log
	DefaultAction string `json:"default_action"` // "allow" hoặc "deny"

	// Agents: Filter theo agentId
	// Key: agentId (hoặc "*" cho tất cả agents)
	// Value: true = cho phép log, false = chặn log
	Agents map[string]bool `json:"agents"`

	// Jobs: Filter theo job name
	// Key: job name (hoặc "*" cho tất cả jobs)
	// Value: true = cho phép log, false = chặn log
	Jobs map[string]bool `json:"jobs"`

	// LogLevels: Filter theo log level
	// Key: log level (debug, info, warn, error, fatal) hoặc "*" cho tất cả
	// Value: true = cho phép log, false = chặn log
	LogLevels map[string]bool `json:"log_levels"`

	// LogMethods: Filter theo phương thức log
	// Key: "console", "file", hoặc "*" cho tất cả
	// Value: true = cho phép log, false = chặn log
	LogMethods map[string]bool `json:"log_methods"`

	// Rules: Các rule phức tạp hơn (kết hợp nhiều điều kiện)
	// Mỗi rule có thể kết hợp agent, job, log level, log method
	Rules []LogFilterRule `json:"rules"`
}

// LogFilterRule là một rule filter phức tạp
type LogFilterRule struct {
	// Name: Tên rule (để dễ quản lý)
	Name string `json:"name"`

	// Enabled: Bật/tắt rule này
	Enabled bool `json:"enabled"`

	// Agent: Filter theo agentId (rỗng hoặc "*" = tất cả agents)
	Agent string `json:"agent"`

	// Job: Filter theo job name (rỗng hoặc "*" = tất cả jobs)
	Job string `json:"job"`

	// LogLevel: Filter theo log level (rỗng hoặc "*" = tất cả levels)
	LogLevel string `json:"log_level"`

	// LogMethod: Filter theo phương thức log (rỗng hoặc "*" = tất cả methods)
	// "console", "file", hoặc "*"
	LogMethod string `json:"log_method"`

	// Action: "allow" = cho phép log, "deny" = chặn log
	Action string `json:"action"` // "allow" hoặc "deny"

	// Priority: Độ ưu tiên (số càng cao càng ưu tiên, mặc định: 0)
	// Rules có priority cao hơn sẽ được kiểm tra trước
	Priority int `json:"priority"`
}

// LogFilterContext chứa context của log entry để filter
type LogFilterContext struct {
	AgentID   string      // Agent ID (từ global.GlobalConfig.AgentId)
	JobName   string      // Job name (từ log fields hoặc logger name)
	LogLevel  logrus.Level // Log level
	LogMethod string      // "console" hoặc "file"
	Fields    logrus.Fields // Các fields trong log entry
}

var (
	logFilterConfig     *LogFilterConfig
	logFilterConfigMu   sync.RWMutex
	logFilterConfigPath string
)

// LoadLogFilterConfig đọc config từ file JSON
// Nếu file không tồn tại, sẽ tạo file mặc định
func LoadLogFilterConfig(configPath string) (*LogFilterConfig, error) {
	logFilterConfigMu.Lock()
	defer logFilterConfigMu.Unlock()

	logFilterConfigPath = configPath

	// Nếu file không tồn tại, tạo config mặc định
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := getDefaultLogFilterConfig()
		if err := SaveLogFilterConfig(defaultConfig, configPath); err != nil {
			return nil, fmt.Errorf("không thể tạo config mặc định: %v", err)
		}
		logFilterConfig = defaultConfig
		return defaultConfig, nil
	}

	// Đọc file config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("không thể đọc file config: %v", err)
	}

	var config LogFilterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("không thể parse JSON config: %v", err)
	}

	// Validate config
	if err := validateLogFilterConfig(&config); err != nil {
		return nil, fmt.Errorf("config không hợp lệ: %v", err)
	}

	logFilterConfig = &config
	return &config, nil
}

// SaveLogFilterConfig lưu config vào file JSON
func SaveLogFilterConfig(config *LogFilterConfig, configPath string) error {
	// Validate config trước khi lưu
	if err := validateLogFilterConfig(config); err != nil {
		return fmt.Errorf("config không hợp lệ: %v", err)
	}

	// Đảm bảo thư mục tồn tại
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("không thể tạo thư mục: %v", err)
	}

	// Format JSON đẹp
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("không thể serialize config: %v", err)
	}

	// Ghi file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("không thể ghi file: %v", err)
	}

	return nil
}

// GetLogFilterConfig trả về config hiện tại
func GetLogFilterConfig() *LogFilterConfig {
	logFilterConfigMu.RLock()
	defer logFilterConfigMu.RUnlock()
	return logFilterConfig
}

// ReloadLogFilterConfig reload config từ file
func ReloadLogFilterConfig() error {
	if logFilterConfigPath == "" {
		return fmt.Errorf("chưa có đường dẫn config")
	}
	_, err := LoadLogFilterConfig(logFilterConfigPath)
	return err
}

// getDefaultLogFilterConfig tạo config mặc định (tất cả log đều được ghi)
func getDefaultLogFilterConfig() *LogFilterConfig {
	return &LogFilterConfig{
		Enabled:      false, // Mặc định tắt filter (tất cả log đều được ghi)
		DefaultAction: "allow",
		Agents: map[string]bool{
			"*": true, // Tất cả agents đều được log
		},
		Jobs: map[string]bool{
			"*": true, // Tất cả jobs đều được log
		},
		LogLevels: map[string]bool{
			"*":     true, // Tất cả log levels
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
			"fatal": true,
		},
		LogMethods: map[string]bool{
			"*":      true, // Tất cả phương thức
			"console": true,
			"file":   true,
		},
		Rules: []LogFilterRule{},
	}
}

// validateLogFilterConfig kiểm tra tính hợp lệ của config
func validateLogFilterConfig(config *LogFilterConfig) error {
	if config == nil {
		return fmt.Errorf("config không được nil")
	}

	// Validate default action
	if config.DefaultAction != "allow" && config.DefaultAction != "deny" {
		return fmt.Errorf("default_action phải là 'allow' hoặc 'deny'")
	}

	// Validate log levels
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
		"*":     true,
	}
	for level := range config.LogLevels {
		if !validLevels[strings.ToLower(level)] {
			return fmt.Errorf("log level không hợp lệ: %s", level)
		}
	}

	// Validate log methods
	validMethods := map[string]bool{
		"console": true,
		"file":    true,
		"*":       true,
	}
	for method := range config.LogMethods {
		if !validMethods[strings.ToLower(method)] {
			return fmt.Errorf("log method không hợp lệ: %s", method)
		}
	}

	// Validate rules
	for i, rule := range config.Rules {
		if err := validateLogFilterRule(&rule); err != nil {
			return fmt.Errorf("rule[%d] không hợp lệ: %v", i, err)
		}
	}

	return nil
}

// validateLogFilterRule kiểm tra tính hợp lệ của một rule
func validateLogFilterRule(rule *LogFilterRule) error {
	if rule == nil {
		return fmt.Errorf("rule không được nil")
	}

	if rule.Action != "allow" && rule.Action != "deny" {
		return fmt.Errorf("action phải là 'allow' hoặc 'deny'")
	}

	return nil
}

// ShouldLog kiểm tra xem log có nên được ghi hay không
func ShouldLog(ctx *LogFilterContext) bool {
	logFilterConfigMu.RLock()
	config := logFilterConfig
	logFilterConfigMu.RUnlock()

	// Nếu filter chưa được khởi tạo hoặc bị tắt, cho phép tất cả log
	if config == nil || !config.Enabled {
		return true
	}

	// Kiểm tra rules trước (có priority cao hơn)
	// Sắp xếp rules theo priority (cao -> thấp)
	rules := make([]LogFilterRule, len(config.Rules))
	copy(rules, config.Rules)
	
	// Sort rules by priority (descending)
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority < rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	// Kiểm tra từng rule (theo thứ tự priority)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Kiểm tra xem rule có match không
		if matchesRule(ctx, &rule) {
			// Rule match, trả về action của rule
			return rule.Action == "allow"
		}
	}

	// Không có rule nào match, kiểm tra các filter đơn giản

	// Kiểm tra agent
	if ctx.AgentID != "" {
		// Kiểm tra agent cụ thể
		if allowed, ok := config.Agents[ctx.AgentID]; ok {
			if !allowed {
				return false
			}
		} else {
			// Kiểm tra wildcard
			if allowed, ok := config.Agents["*"]; ok && !allowed {
				return false
			}
		}
	}

	// Kiểm tra job
	if ctx.JobName != "" {
		// Kiểm tra job cụ thể
		if allowed, ok := config.Jobs[ctx.JobName]; ok {
			if !allowed {
				return false
			}
		} else {
			// Kiểm tra wildcard
			if allowed, ok := config.Jobs["*"]; ok && !allowed {
				return false
			}
		}
	}

	// Kiểm tra log level
	levelStr := strings.ToLower(ctx.LogLevel.String())
	if allowed, ok := config.LogLevels[levelStr]; ok {
		if !allowed {
			return false
		}
	} else {
		// Kiểm tra wildcard
		if allowed, ok := config.LogLevels["*"]; ok && !allowed {
			return false
		}
	}

	// Kiểm tra log method
	if ctx.LogMethod != "" {
		methodStr := strings.ToLower(ctx.LogMethod)
		if allowed, ok := config.LogMethods[methodStr]; ok {
			if !allowed {
				return false
			}
		} else {
			// Kiểm tra wildcard
			if allowed, ok := config.LogMethods["*"]; ok && !allowed {
				return false
			}
		}
	}

	// Không match rule nào và không bị chặn bởi filter đơn giản
	// Trả về default action
	return config.DefaultAction == "allow"
}

// matchesRule kiểm tra xem context có match với rule không
func matchesRule(ctx *LogFilterContext, rule *LogFilterRule) bool {
	// Kiểm tra agent
	if rule.Agent != "" && rule.Agent != "*" {
		if ctx.AgentID != rule.Agent {
			return false
		}
	}

	// Kiểm tra job
	if rule.Job != "" && rule.Job != "*" {
		if ctx.JobName != rule.Job {
			return false
		}
	}

	// Kiểm tra log level
	if rule.LogLevel != "" && rule.LogLevel != "*" {
		levelStr := strings.ToLower(ctx.LogLevel.String())
		if levelStr != strings.ToLower(rule.LogLevel) {
			return false
		}
	}

	// Kiểm tra log method
	if rule.LogMethod != "" && rule.LogMethod != "*" {
		methodStr := strings.ToLower(ctx.LogMethod)
		if methodStr != strings.ToLower(rule.LogMethod) {
			return false
		}
	}

	// Tất cả điều kiện đều match
	return true
}

// ExtractLogContext trích xuất context từ log entry
func ExtractLogContext(entry *logrus.Entry, agentID string) *LogFilterContext {
	ctx := &LogFilterContext{
		AgentID:   agentID,
		LogLevel:  entry.Level,
		LogMethod: "", // Sẽ được set bởi hook
		Fields:    entry.Data,
	}

	// Tìm job name từ fields hoặc logger name
	if jobName, ok := entry.Data["job_name"].(string); ok && jobName != "" {
		ctx.JobName = jobName
	} else {
		// Thử tìm từ logger_name (được thêm tự động bởi LoggerNameHook)
		// Logger name thường là job name trong jobs (ví dụ: "workflow-commands-job")
		if loggerName, ok := entry.Data["logger_name"].(string); ok && loggerName != "" {
			// Nếu logger name có pattern *-job, coi như là job name
			if len(loggerName) > 4 && loggerName[len(loggerName)-4:] == "-job" {
				ctx.JobName = loggerName
			}
		}
	}

	// Nếu agentID rỗng, thử lấy từ fields
	if ctx.AgentID == "" {
		if agentIDVal, ok := entry.Data["agentId"].(string); ok && agentIDVal != "" {
			ctx.AgentID = agentIDVal
		} else if agentIDVal, ok := entry.Data["agent_id"].(string); ok && agentIDVal != "" {
			ctx.AgentID = agentIDVal
		}
	}

	return ctx
}
