/*
Package services chứa các services hỗ trợ cho agent.
File này thu thập metrics từ scheduler và jobs.
*/
package services

import (
	"agent_pancake/app/scheduler"
	"time"
)

// MetricsCollector thu thập metrics từ scheduler và jobs
type MetricsCollector struct {
	scheduler     *scheduler.Scheduler
	configManager *ConfigManager // Thêm configManager để lấy metadata của job
}

// NewMetricsCollector tạo một instance mới của MetricsCollector
func NewMetricsCollector(s *scheduler.Scheduler) *MetricsCollector {
	return &MetricsCollector{
		scheduler:     s,
		configManager: GetGlobalConfigManager(), // Lấy config manager để lấy metadata
	}
}

// AgentMetrics chứa metrics tổng của bot
type AgentMetrics struct {
	TotalJobsRun   int64   `json:"totalJobsRun"`
	SuccessfulJobs int64   `json:"successfulJobs"`
	FailedJobs     int64   `json:"failedJobs"`
	AvgJobDuration float64 `json:"avgJobDuration"`
	TotalAPICalls  int64   `json:"totalAPICalls"`
	FailedAPICalls int64   `json:"failedAPICalls"`
}

// JobStatus chứa trạng thái và metrics của một job
// Theo API v3.14: Thêm metadata của job vào jobStatus để UI-friendly
type JobStatus struct {
	JobName         string  `json:"jobName"`
	Schedule        string  `json:"schedule"`
	Status          string  `json:"status"` // "idle", "running", "error", "disabled"
	IsEnabled       bool    `json:"isEnabled"`
	LastRunAt       int64   `json:"lastRunAt"`
	LastRunDuration float64 `json:"lastRunDuration"`
	LastRunStatus   string  `json:"lastRunStatus"` // "success", "failed"
	RunCount        int64   `json:"runCount"`
	SuccessCount    int64   `json:"successCount"`
	ErrorCount      int64   `json:"errorCount"`
	AvgDuration     float64 `json:"avgDuration"`
	MaxDuration     float64 `json:"maxDuration"`
	NextRunAt       int64   `json:"nextRunAt"`
	// Error information (gửi mảng errors của job trực tiếp trong jobStatus)
	Errors []JobError `json:"errors"` // Mảng các lỗi gần đây của job (số lượng giới hạn bởi config). Luôn gửi mảng, kể cả khi rỗng.
	// Metadata fields (theo API v3.14 - Agent UI-Friendly Metadata Updates)
	DisplayName string   `json:"displayName,omitempty"` // Tên hiển thị của job
	Description string   `json:"description,omitempty"` // Mô tả của job
	Icon        string   `json:"icon,omitempty"`        // Icon của job
	Color       string   `json:"color,omitempty"`       // Màu sắc của job
	Category    string   `json:"category,omitempty"`    // Danh mục của job
	Tags        []string `json:"tags,omitempty"`        // Tags của job
}

// JobError chứa thông tin lỗi của một job
type JobError struct {
	Message    string  `json:"message"`            // Nội dung lỗi
	OccurredAt int64   `json:"occurredAt"`         // Thời điểm lỗi xảy ra (Unix timestamp)
	RunCount   int64   `json:"runCount,omitempty"` // Số lần chạy tại thời điểm lỗi
	Duration   float64 `json:"duration,omitempty"` // Thời gian chạy tại thời điểm lỗi (giây)
}

// ErrorReport chứa thông tin lỗi (dùng cho system errors, không phải job errors)
type ErrorReport struct {
	Type       string                 `json:"type"` // "job_error", "system_error", "api_error"
	Message    string                 `json:"message"`
	StackTrace string                 `json:"stackTrace,omitempty"`
	OccurredAt int64                  `json:"occurredAt"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// CollectJobStatuses thu thập trạng thái của tất cả jobs
// Cải thiện: Kiểm tra trạng thái paused, disabled, và running để tương ứng với hệ thống command
func (m *MetricsCollector) CollectJobStatuses() []JobStatus {
	if m.scheduler == nil {
		return []JobStatus{}
	}

	// Lấy tất cả jobs từ scheduler
	allJobs := m.scheduler.GetAllJobObjects()
	jobStatuses := make([]JobStatus, 0, len(allJobs))

	// Lấy danh sách jobs đã đăng ký để kiểm tra enabled/disabled
	registeredJobs := m.scheduler.GetJobs()

	// Lấy danh sách paused và disabled jobs từ scheduler
	pausedJobs := m.scheduler.GetPausedJobs()
	disabledJobs := m.scheduler.GetDisabledJobs()

	for jobName, job := range allJobs {
		status := JobStatus{
			JobName:  jobName,
			Schedule: job.GetSchedule(),
			Errors:   make([]JobError, 0), // Khởi tạo mảng errors rỗng
		}

		// Kiểm tra job có enabled không (có trong registeredJobs = enabled)
		_, isEnabled := registeredJobs[jobName]
		status.IsEnabled = isEnabled

		// Kiểm tra job có bị paused không
		_, isPaused := pausedJobs[jobName]

		// Kiểm tra job có bị disabled không
		_, isDisabled := disabledJobs[jobName]

		// Kiểm tra job có đang chạy không (nếu implement RunningProvider)
		isRunning := false
		if runningProvider, ok := job.(scheduler.RunningProvider); ok {
			isRunning = runningProvider.IsRunning()
		}

		// Kiểm tra job có implement MetricsProvider không để lấy metrics
		// Vì jobs embed *scheduler.BaseJob, methods sẽ được promoted
		// Nên có thể type assert trực tiếp
		metricsProvider, ok := job.(scheduler.MetricsProvider)

		if ok {
			metrics := metricsProvider.GetMetrics()

			// Xác định status dựa trên trạng thái thực tế từ scheduler và job
			// Ưu tiên: running > paused > disabled > error > idle
			if isRunning {
				status.Status = "running"
			} else if isPaused {
				status.Status = "paused"
			} else if isDisabled || !isEnabled {
				status.Status = "disabled"
			} else if metrics.LastRunStatus == "failed" && !metrics.LastRunAt.IsZero() {
				status.Status = "error"
			} else {
				status.Status = "idle"
			}

			// Lấy metrics từ BaseJob
			status.LastRunAt = metrics.LastRunAt.Unix()
			status.LastRunDuration = metrics.LastRunDuration
			status.LastRunStatus = metrics.LastRunStatus
			status.RunCount = metrics.RunCount
			status.SuccessCount = metrics.SuccessCount
			status.ErrorCount = metrics.ErrorCount
			status.AvgDuration = metricsProvider.GetAvgDuration()
			status.MaxDuration = metricsProvider.GetMaxDuration()

			// Lấy errors của job (nếu có và gần đây)
			// Lưu ý: BaseJob chỉ lưu LastError, nên tạm thời chỉ gửi error gần nhất
			// Trong tương lai có thể cải thiện BaseJob để lưu error history
			status.Errors = m.collectJobErrors(jobName, metrics)

			// NextRunAt: Có thể tính từ cron schedule, nhưng tạm thời để 0
			// TODO: Tính next run time từ cron schedule
			status.NextRunAt = 0
		} else {
			// Job không phải BaseJob → chỉ có thông tin cơ bản
			if isRunning {
				status.Status = "running"
			} else if isPaused {
				status.Status = "paused"
			} else if isDisabled || !isEnabled {
				status.Status = "disabled"
			} else {
				status.Status = "idle"
			}
		}

		// Lấy metadata của job từ config (theo API v3.14 - ghép metadata vào jobStatus)
		// Metadata có thể từ AgentRegistry.JobMetadata (server) hoặc từ config (nếu có)
		if m.configManager != nil {
			jobMetadata := m.getJobMetadataFromConfig(jobName)
			status.DisplayName = jobMetadata.DisplayName
			status.Description = jobMetadata.Description
			status.Icon = jobMetadata.Icon
			status.Color = jobMetadata.Color
			status.Category = jobMetadata.Category
			status.Tags = jobMetadata.Tags
		}

		jobStatuses = append(jobStatuses, status)
	}

	return jobStatuses
}

// CollectBotMetrics thu thập metrics tổng của bot
func (m *MetricsCollector) CollectBotMetrics() AgentMetrics {
	if m.scheduler == nil {
		return AgentMetrics{}
	}

	metrics := AgentMetrics{}

	// Lấy tất cả jobs từ scheduler
	allJobs := m.scheduler.GetAllJobObjects()

	var totalDuration float64
	var jobCount int

	// Aggregate metrics từ tất cả jobs
	for _, job := range allJobs {
		if metricsProvider, ok := job.(scheduler.MetricsProvider); ok {
			jobMetrics := metricsProvider.GetMetrics()

			// Tổng số lần chạy
			metrics.TotalJobsRun += jobMetrics.RunCount
			metrics.SuccessfulJobs += jobMetrics.SuccessCount
			metrics.FailedJobs += jobMetrics.ErrorCount

			// Tính average duration
			avgDuration := metricsProvider.GetAvgDuration()
			if avgDuration > 0 {
				totalDuration += avgDuration
				jobCount++
			}
		}
	}

	// Tính average job duration
	if jobCount > 0 {
		metrics.AvgJobDuration = totalDuration / float64(jobCount)
	}

	// API calls metrics: Tạm thời để 0, có thể mở rộng sau
	// TODO: Track API calls từ HTTP client hoặc integration layer
	metrics.TotalAPICalls = 0
	metrics.FailedAPICalls = 0

	return metrics
}

// CollectErrors thu thập errors (nếu có)
func (m *MetricsCollector) CollectErrors() []ErrorReport {
	if m.scheduler == nil {
		return []ErrorReport{}
	}

	errors := make([]ErrorReport, 0)

	// Lấy tất cả jobs từ scheduler
	allJobs := m.scheduler.GetAllJobObjects()

	// Thu thập errors từ jobs (chỉ lấy lỗi gần nhất của mỗi job nếu có)
	for jobName, job := range allJobs {
		if metricsProvider, ok := job.(scheduler.MetricsProvider); ok {
			metrics := metricsProvider.GetMetrics()

			// Chỉ thêm error nếu có lỗi và lỗi xảy ra gần đây (trong vòng 1 giờ)
			if metrics.LastError != "" && metrics.LastRunStatus == "failed" {
				// Kiểm tra xem lỗi có gần đây không (trong vòng 1 giờ)
				if time.Since(metrics.LastRunAt) < time.Hour {
					errors = append(errors, ErrorReport{
						Type:       "job_error",
						Message:    metrics.LastError,
						OccurredAt: metrics.LastRunAt.Unix(),
						Context: map[string]interface{}{
							"jobName":         jobName,
							"runCount":        metrics.RunCount,
							"errorCount":      metrics.ErrorCount,
							"lastRunDuration": metrics.LastRunDuration,
						},
					})
				}
			}
		}
	}

	return errors
}

// JobMetadata chứa metadata của job (theo API v3.14)
type JobMetadata struct {
	DisplayName string
	Description string
	Icon        string
	Color       string
	Category    string
	Tags        []string
}

// getJobDisplayName trả về displayName tiếng Việt cho job
func getJobDisplayName(jobName string) string {
	displayNames := map[string]string{
		"sync-incremental-conversations-job":         "Đồng Bộ Cuộc Trò Chuyện Mới",
		"sync-backfill-conversations-job":            "Đồng Bộ Cuộc Trò Chuyện Cũ",
		"sync-verify-conversations-job":              "Xác Minh Cuộc Trò Chuyện",
		"sync-warn-unreplied-conversations-job":      "Cảnh Báo Cuộc Trò Chuyện Chưa Trả Lời",
		"sync-full-recovery-conversations-job":       "Khôi Phục Toàn Bộ Cuộc Trò Chuyện",
		"sync-incremental-posts-job":                 "Đồng Bộ Bài Viết Mới",
		"sync-backfill-posts-job":                    "Đồng Bộ Bài Viết Cũ",
		"sync-incremental-customers-job":             "Đồng Bộ Khách Hàng Mới",
		"sync-backfill-customers-job":                "Đồng Bộ Khách Hàng Cũ",
		"sync-incremental-pancake-pos-customers-job": "Đồng Bộ Khách Hàng POS Mới",
		"sync-backfill-pancake-pos-customers-job":    "Đồng Bộ Khách Hàng POS Cũ",
		"sync-incremental-pancake-pos-orders-job":    "Đồng Bộ Đơn Hàng POS Mới",
		"sync-backfill-pancake-pos-orders-job":       "Đồng Bộ Đơn Hàng POS Cũ",
		"sync-pancake-pos-products-job":              "Đồng Bộ Sản Phẩm POS",
		"sync-pancake-pos-shops-warehouses-job":      "Đồng Bộ Cửa Hàng & Kho POS",
		"check-in-job":                               "Check-In",
	}

	if displayName, ok := displayNames[jobName]; ok {
		return displayName
	}
	// Nếu không tìm thấy, trả về jobName
	return jobName
}

// getJobMetadataFromConfig lấy metadata của job từ config
// Theo API v3.14: Metadata chung của job đã được chuyển sang AgentRegistry.JobMetadata
// Nhưng có thể vẫn còn trong config trước khi cleanup, hoặc server sẽ thêm vào sau
func (m *MetricsCollector) getJobMetadataFromConfig(jobName string) JobMetadata {
	metadata := JobMetadata{
		DisplayName: getJobDisplayName(jobName), // Default: dùng displayName tiếng Việt
		Description: "",
		Icon:        "⚙️",
		Color:       "#6B7280",
		Category:    "sync",
		Tags:        []string{},
	}

	if m.configManager == nil {
		return metadata
	}

	// Lấy job config từ config manager
	// Lưu ý: Config có thể là array hoặc object (backward compatibility)
	configData := m.configManager.GetConfigData()
	if configData == nil {
		return metadata
	}

	// Tìm job trong config (có thể là array hoặc object)
	jobsRaw := configData["jobs"]
	if jobsRaw == nil {
		return metadata
	}

	// Xử lý jobs là array (theo API v3.14)
	if jobsArray, ok := jobsRaw.([]interface{}); ok {
		for _, jobRaw := range jobsArray {
			jobConfig, ok := jobRaw.(map[string]interface{})
			if !ok {
				continue
			}
			// Kiểm tra job name
			if name, ok := jobConfig["name"].(string); ok && name == jobName {
				// Lấy metadata từ job config (nếu có, trước khi cleanup)
				if displayName, ok := jobConfig["displayName"].(string); ok && displayName != "" {
					metadata.DisplayName = displayName
				}
				if description, ok := jobConfig["description"].(string); ok && description != "" {
					metadata.Description = description
				}
				if icon, ok := jobConfig["icon"].(string); ok && icon != "" {
					metadata.Icon = icon
				}
				if color, ok := jobConfig["color"].(string); ok && color != "" {
					metadata.Color = color
				}
				if category, ok := jobConfig["category"].(string); ok && category != "" {
					metadata.Category = category
				}
				if tagsRaw, ok := jobConfig["tags"]; ok {
					if tagsArray, ok := tagsRaw.([]interface{}); ok {
						tags := make([]string, 0, len(tagsArray))
						for _, tag := range tagsArray {
							if tagStr, ok := tag.(string); ok {
								tags = append(tags, tagStr)
							}
						}
						if len(tags) > 0 {
							metadata.Tags = tags
						}
					}
				}
				break
			}
		}
	} else if jobsMap, ok := jobsRaw.(map[string]interface{}); ok {
		// Xử lý jobs là object (backward compatibility)
		if jobConfig, ok := jobsMap[jobName].(map[string]interface{}); ok {
			// Lấy metadata từ job config (nếu có)
			if displayName, ok := jobConfig["displayName"].(string); ok && displayName != "" {
				metadata.DisplayName = displayName
			}
			if description, ok := jobConfig["description"].(string); ok && description != "" {
				metadata.Description = description
			}
			if icon, ok := jobConfig["icon"].(string); ok && icon != "" {
				metadata.Icon = icon
			}
			if color, ok := jobConfig["color"].(string); ok && color != "" {
				metadata.Color = color
			}
			if category, ok := jobConfig["category"].(string); ok && category != "" {
				metadata.Category = category
			}
			if tagsRaw, ok := jobConfig["tags"]; ok {
				if tagsArray, ok := tagsRaw.([]interface{}); ok {
					tags := make([]string, 0, len(tagsArray))
					for _, tag := range tagsArray {
						if tagStr, ok := tag.(string); ok {
							tags = append(tags, tagStr)
						}
					}
					if len(tags) > 0 {
						metadata.Tags = tags
					}
				}
			}
		}
	}

	return metadata
}

// collectJobErrors thu thập errors của một job
// Số lượng errors được giới hạn bởi config agent.errorReporting.maxErrorsPerCheckIn
func (m *MetricsCollector) collectJobErrors(jobName string, metrics scheduler.JobMetrics) []JobError {
	errors := make([]JobError, 0)

	// Lấy max errors từ config (default: 10)
	maxErrors := 10
	if m.configManager != nil {
		// Lấy từ agent.errorReporting.maxErrorsPerCheckIn
		if agentConfig, ok := m.configManager.GetConfigData()["agent"].(map[string]interface{}); ok {
			if errorReportingConfig, ok := agentConfig["errorReporting"].(map[string]interface{}); ok {
				if maxErrorsField, ok := errorReportingConfig["maxErrorsPerCheckIn"]; ok {
					// Extract value từ config field (có thể là map với "value" hoặc trực tiếp là số)
					if maxErrorsMap, ok := maxErrorsField.(map[string]interface{}); ok {
						if value, ok := maxErrorsMap["value"].(float64); ok {
							maxErrors = int(value)
						}
					} else if value, ok := maxErrorsField.(float64); ok {
						maxErrors = int(value)
					}
				}
			}
		}
	}

	// Lấy error retention hours từ config (default: 24)
	errorRetentionHours := 24
	if m.configManager != nil {
		if agentConfig, ok := m.configManager.GetConfigData()["agent"].(map[string]interface{}); ok {
			if errorReportingConfig, ok := agentConfig["errorReporting"].(map[string]interface{}); ok {
				if retentionField, ok := errorReportingConfig["errorRetentionHours"]; ok {
					if retentionMap, ok := retentionField.(map[string]interface{}); ok {
						if value, ok := retentionMap["value"].(float64); ok {
							errorRetentionHours = int(value)
						}
					} else if value, ok := retentionField.(float64); ok {
						errorRetentionHours = int(value)
					}
				}
			}
		}
	}

	// Chỉ thêm error nếu có lỗi và lỗi xảy ra gần đây (trong vòng errorRetentionHours)
	if metrics.LastError != "" && metrics.LastRunStatus == "failed" {
		// Kiểm tra xem lỗi có gần đây không
		if time.Since(metrics.LastRunAt) < time.Duration(errorRetentionHours)*time.Hour {
			errors = append(errors, JobError{
				Message:    metrics.LastError,
				OccurredAt: metrics.LastRunAt.Unix(),
				RunCount:   metrics.RunCount,
				Duration:   metrics.LastRunDuration,
			})
		}
	}

	// Giới hạn số lượng errors
	if len(errors) > maxErrors {
		errors = errors[:maxErrors]
	}

	return errors
}
