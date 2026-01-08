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
	scheduler *scheduler.Scheduler
}

// NewMetricsCollector tạo một instance mới của MetricsCollector
func NewMetricsCollector(s *scheduler.Scheduler) *MetricsCollector {
	return &MetricsCollector{
		scheduler: s,
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
}

// ErrorReport chứa thông tin lỗi
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
