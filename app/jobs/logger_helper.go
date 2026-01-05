/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa các helper functions để sử dụng logger trong jobs.
*/
package jobs

import (
	"agent_pancake/utility/logger"
	"log"
	"sync"

	"github.com/sirupsen/logrus"
)

// JobLogger là logger chuyên dụng cho jobs (dùng chung cho tất cả jobs)
// DEPRECATED: Nên sử dụng GetJobLoggerByName() để có file log riêng cho từng job
var JobLogger *logrus.Logger

// jobLoggers lưu trữ logger riêng cho từng job
var jobLoggers = make(map[string]*logrus.Logger)

// jobLoggersMu bảo vệ jobLoggers khỏi race condition
var jobLoggersMu sync.RWMutex

// InitJobLogger khởi tạo logger cho jobs (logger chung)
func InitJobLogger() {
	JobLogger = logger.GetJobLogger()
}

// GetJobLoggerByName trả về logger riêng cho một job cụ thể.
// Mỗi job sẽ có file log riêng với tên: {jobName}.log
// Ví dụ: "sync-incremental-conversations-job" -> "sync-incremental-conversations-job.log"
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//
// Trả về logger riêng cho job đó (hoặc fallback logger nếu có lỗi)
func GetJobLoggerByName(jobName string) *logrus.Logger {
	// Bắt panic để tránh crash
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[LoggerHelper] 🚨 PANIC trong GetJobLoggerByName: %v, jobName: %s", r, jobName)
			// Fallback: trả về logger chung nếu có, nếu không thì tạo logger mặc định
			if JobLogger != nil {
				JobLogger.WithField("job_name", jobName).Errorf("Lỗi khi tạo logger riêng, dùng logger chung: %v", r)
			}
		}
	}()

	// Kiểm tra jobName hợp lệ
	if jobName == "" {
		log.Printf("[LoggerHelper] ⚠️ jobName rỗng, dùng logger chung")
		if JobLogger != nil {
			return JobLogger
		}
		// Fallback: tạo logger mặc định
		return logrus.New()
	}

	// Kiểm tra xem logger đã được tạo chưa (với mutex để tránh race condition)
	jobLoggersMu.RLock()
	if logger, exists := jobLoggers[jobName]; exists {
		jobLoggersMu.RUnlock()
		return logger
	}
	jobLoggersMu.RUnlock()

	// Tạo logger mới với tên job (với mutex để tránh race condition)
	jobLoggersMu.Lock()
	defer jobLoggersMu.Unlock()

	// Double-check: có thể logger đã được tạo bởi goroutine khác
	if logger, exists := jobLoggers[jobName]; exists {
		return logger
	}

	// Tạo logger mới với tên job
	// Logger sẽ tự động tạo file log với tên: {jobName}.log
	// Bắt lỗi nếu logger.GetLogger panic
	var loggerInstance *logrus.Logger
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[LoggerHelper] 🚨 PANIC khi gọi logger.GetLogger(%s): %v", jobName, r)
				// Fallback: tạo logger mặc định
				loggerInstance = logrus.New()
				if JobLogger != nil {
					JobLogger.WithField("job_name", jobName).Errorf("Lỗi khi tạo logger riêng, dùng logger mặc định: %v", r)
				}
			}
		}()
		loggerInstance = logger.GetLogger(jobName)
	}()

	// Lưu logger vào map
	if loggerInstance != nil {
		jobLoggers[jobName] = loggerInstance
	} else {
		// Fallback: dùng logger chung nếu có
		if JobLogger != nil {
			jobLoggers[jobName] = JobLogger
			return JobLogger
		}
		// Fallback cuối cùng: tạo logger mặc định
		loggerInstance = logrus.New()
		jobLoggers[jobName] = loggerInstance
	}

	return loggerInstance
}

// LogJobStart log khi job bắt đầu
// Sử dụng logger riêng cho job nếu có, ngược lại dùng logger chung
func LogJobStart(jobName, schedule string) *logrus.Entry {
	logger := getLoggerForJob(jobName)
	return logger.WithFields(logrus.Fields{
		"job_name": jobName,
		"schedule": schedule,
		"status":   "started",
	})
}

// LogJobEnd log khi job kết thúc thành công
// Sử dụng logger riêng cho job nếu có, ngược lại dùng logger chung
func LogJobEnd(jobName string, duration string, durationMs int64) {
	logger := getLoggerForJob(jobName)
	logger.WithFields(logrus.Fields{
		"job_name":    jobName,
		"status":      "completed",
		"duration":    duration,
		"duration_ms": durationMs,
	}).Info("✅ JOB HOÀN THÀNH")
}

// LogJobError log khi job gặp lỗi
// Sử dụng logger riêng cho job nếu có, ngược lại dùng logger chung
func LogJobError(jobName string, err error, duration string, durationMs int64) {
	logger := getLoggerForJob(jobName)
	logger.WithFields(logrus.Fields{
		"job_name":    jobName,
		"status":      "failed",
		"error":       err.Error(),
		"duration":    duration,
		"duration_ms": durationMs,
	}).Error("❌ JOB THẤT BẠI")
}

// LogJobInfo log thông tin chung của job
// Sử dụng logger riêng cho job nếu có, ngược lại dùng logger chung
func LogJobInfo(jobName string, message string, fields map[string]interface{}) {
	logger := getLoggerForJob(jobName)
	entry := logger.WithField("job_name", jobName)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Info(message)
}

// LogJobDebug log debug của job
// Sử dụng logger riêng cho job nếu có, ngược lại dùng logger chung
func LogJobDebug(jobName string, message string, fields map[string]interface{}) {
	logger := getLoggerForJob(jobName)
	entry := logger.WithField("job_name", jobName)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Debug(message)
}

// LogJobWarn log cảnh báo của job
// Sử dụng logger riêng cho job nếu có, ngược lại dùng logger chung
func LogJobWarn(jobName string, message string, fields map[string]interface{}) {
	logger := getLoggerForJob(jobName)
	entry := logger.WithField("job_name", jobName)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Warn(message)
}

// LogJobErrorWithFields log lỗi với các fields bổ sung
// Sử dụng logger riêng cho job nếu có, ngược lại dùng logger chung
func LogJobErrorWithFields(jobName string, err error, message string, fields map[string]interface{}) {
	logger := getLoggerForJob(jobName)
	entry := logger.WithError(err).WithField("job_name", jobName)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Error(message)
}

// LogOperationDuration log thời gian thực thi của một operation
// Sử dụng logger riêng cho job nếu có, ngược lại dùng logger chung
func LogOperationDuration(jobName, operation string, duration string, durationMs int64) {
	logger := getLoggerForJob(jobName)
	logger.WithFields(logrus.Fields{
		"job_name":    jobName,
		"operation":   operation,
		"duration":    duration,
		"duration_ms": durationMs,
	}).Debug("Operation completed")
}

// getLoggerForJob trả về logger riêng cho job nếu đã được khởi tạo,
// ngược lại trả về logger chung (JobLogger)
// Hàm này tự động tạo logger riêng cho job khi được gọi lần đầu
// Có xử lý panic để tránh crash
func getLoggerForJob(jobName string) *logrus.Logger {
	// Bắt panic để tránh crash
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[LoggerHelper] 🚨 PANIC trong getLoggerForJob: %v, jobName: %s", r, jobName)
			// Fallback: trả về logger chung nếu có
			if JobLogger != nil {
				JobLogger.WithField("job_name", jobName).Errorf("Lỗi trong getLoggerForJob, dùng logger chung: %v", r)
			}
		}
	}()

	// Kiểm tra jobName hợp lệ
	if jobName == "" {
		log.Printf("[LoggerHelper] ⚠️ jobName rỗng trong getLoggerForJob, dùng logger chung")
		if JobLogger != nil {
			return JobLogger
		}
		// Fallback: tạo logger mặc định
		return logrus.New()
	}

	// Kiểm tra xem logger đã được tạo chưa (với mutex để tránh race condition)
	jobLoggersMu.RLock()
	if logger, exists := jobLoggers[jobName]; exists {
		jobLoggersMu.RUnlock()
		return logger
	}
	jobLoggersMu.RUnlock()

	// Tự động tạo logger riêng cho job
	// Điều này đảm bảo mỗi job sẽ có file log riêng
	// GetJobLoggerByName đã có xử lý panic và mutex
	loggerInstance := GetJobLoggerByName(jobName)

	// Fallback: nếu GetJobLoggerByName trả về nil (không nên xảy ra nhưng phòng ngừa)
	if loggerInstance == nil {
		log.Printf("[LoggerHelper] ⚠️ GetJobLoggerByName trả về nil cho jobName: %s, dùng logger chung", jobName)
		if JobLogger != nil {
			return JobLogger
		}
		// Fallback cuối cùng: tạo logger mặc định
		return logrus.New()
	}

	return loggerInstance
}
