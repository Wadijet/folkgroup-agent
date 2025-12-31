/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa các helper functions để sử dụng logger trong jobs.
*/
package jobs

import (
	"agent_pancake/utility/logger"
	"github.com/sirupsen/logrus"
)

// JobLogger là logger chuyên dụng cho jobs
var JobLogger *logrus.Logger

// InitJobLogger khởi tạo logger cho jobs
func InitJobLogger() {
	JobLogger = logger.GetJobLogger()
}

// LogJobStart log khi job bắt đầu
func LogJobStart(jobName, schedule string) *logrus.Entry {
	return JobLogger.WithFields(logrus.Fields{
		"job_name": jobName,
		"schedule":  schedule,
		"status":    "started",
	})
}

// LogJobEnd log khi job kết thúc thành công
func LogJobEnd(jobName string, duration string, durationMs int64) {
	JobLogger.WithFields(logrus.Fields{
		"job_name":   jobName,
		"status":     "completed",
		"duration":    duration,
		"duration_ms": durationMs,
	}).Info("✅ JOB HOÀN THÀNH")
}

// LogJobError log khi job gặp lỗi
func LogJobError(jobName string, err error, duration string, durationMs int64) {
	JobLogger.WithFields(logrus.Fields{
		"job_name":    jobName,
		"status":      "failed",
		"error":       err.Error(),
		"duration":    duration,
		"duration_ms": durationMs,
	}).Error("❌ JOB THẤT BẠI")
}

// LogJobInfo log thông tin chung của job
func LogJobInfo(jobName string, message string, fields map[string]interface{}) {
	entry := JobLogger.WithField("job_name", jobName)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Info(message)
}

// LogJobDebug log debug của job
func LogJobDebug(jobName string, message string, fields map[string]interface{}) {
	entry := JobLogger.WithField("job_name", jobName)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Debug(message)
}

// LogJobWarn log cảnh báo của job
func LogJobWarn(jobName string, message string, fields map[string]interface{}) {
	entry := JobLogger.WithField("job_name", jobName)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Warn(message)
}

// LogJobErrorWithFields log lỗi với các fields bổ sung
func LogJobErrorWithFields(jobName string, err error, message string, fields map[string]interface{}) {
	entry := JobLogger.WithError(err).WithField("job_name", jobName)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Error(message)
}

// LogOperationDuration log thời gian thực thi của một operation
func LogOperationDuration(jobName, operation string, duration string, durationMs int64) {
	JobLogger.WithFields(logrus.Fields{
		"job_name":    jobName,
		"operation":   operation,
		"duration":    duration,
		"duration_ms": durationMs,
	}).Debug("Operation completed")
}
