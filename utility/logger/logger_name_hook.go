/*
Package logger chứa hook để tự động thêm logger_name vào log entries.
Hook này giúp filter log có thể nhận diện được job name từ logger name.
*/
package logger

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// LoggerNameHook là hook để tự động thêm logger_name vào log entries
type LoggerNameHook struct {
	loggerName string
}

// NewLoggerNameHook tạo hook mới với logger name
func NewLoggerNameHook(loggerName string) *LoggerNameHook {
	return &LoggerNameHook{
		loggerName: loggerName,
	}
}

// Levels trả về các log levels mà hook này sẽ xử lý
func (h *LoggerNameHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire được gọi mỗi khi có log entry
// Hook này sẽ tự động thêm logger_name vào entry.Data
func (h *LoggerNameHook) Fire(entry *logrus.Entry) error {
	// Chỉ thêm logger_name nếu chưa có
	if _, ok := entry.Data["logger_name"]; !ok {
		entry.Data["logger_name"] = h.loggerName
	}

	// Nếu logger name là job name (có pattern *-job), tự động thêm job_name
	// để đảm bảo filter có thể nhận diện
	if h.loggerName != "" && (len(h.loggerName) > 4 && h.loggerName[len(h.loggerName)-4:] == "-job") {
		if _, ok := entry.Data["job_name"]; !ok {
			entry.Data["job_name"] = h.loggerName
		}
	}

	return nil
}

// LoggerNameMap lưu trữ mapping giữa logger instance và logger name
var loggerNameMap = make(map[*logrus.Logger]string)
var loggerNameMapMu sync.RWMutex

// RegisterLoggerName đăng ký logger name cho một logger instance
func RegisterLoggerName(logger *logrus.Logger, name string) {
	loggerNameMapMu.Lock()
	defer loggerNameMapMu.Unlock()
	loggerNameMap[logger] = name
}

// GetLoggerName lấy logger name từ logger instance
func GetLoggerName(logger *logrus.Logger) string {
	loggerNameMapMu.RLock()
	defer loggerNameMapMu.RUnlock()
	if name, ok := loggerNameMap[logger]; ok {
		return name
	}
	return ""
}
