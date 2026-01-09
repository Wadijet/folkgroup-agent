package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel định nghĩa các mức log
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// LogFormat định nghĩa format của log
type LogFormat string

const (
	LogFormatJSON LogFormat = "json" // JSON format cho production
	LogFormatText LogFormat = "text" // Text format cho development
)

// Config chứa cấu hình cho logger
type Config struct {
	// LogLevel: debug, info, warn, error, fatal (mặc định: info)
	Level string

	// LogFormat: json hoặc text (mặc định: text)
	Format string

	// LogDir: Thư mục lưu log files (mặc định: ./logs)
	LogDir string

	// EnableConsole: Bật/tắt log ra console (mặc định: true)
	EnableConsole string

	// EnableFile: Bật/tắt log ra file (mặc định: true)
	EnableFile string

	// MaxSize: Kích thước tối đa của log file trước khi rotate (MB) (mặc định: 100)
	MaxSize string

	// MaxBackups: Số lượng log files cũ được giữ lại (mặc định: 10)
	MaxBackups string

	// MaxAge: Số ngày giữ log files cũ (mặc định: 30)
	MaxAge string

	// Compress: Nén log files cũ (mặc định: true)
	Compress string

	// EnableCaller: Hiển thị thông tin caller (file:line) (mặc định: true)
	EnableCaller string
}

// NewConfig tạo config mới từ environment variables với default values
func NewConfig() *Config {
	return &Config{
		Level:         getEnv("LOG_LEVEL", "info"),
		Format:        getEnv("LOG_FORMAT", "text"),
		LogDir:        getEnv("LOG_DIR", "./logs"),
		EnableConsole: getEnv("LOG_ENABLE_CONSOLE", "true"),
		EnableFile:    getEnv("LOG_ENABLE_FILE", "true"),
		MaxSize:       getEnv("LOG_MAX_SIZE", "100"),
		MaxBackups:    getEnv("LOG_MAX_BACKUPS", "10"),
		MaxAge:        getEnv("LOG_MAX_AGE", "30"),
		Compress:      getEnv("LOG_COMPRESS", "true"),
		EnableCaller:  getEnv("LOG_ENABLE_CALLER", "true"),
	}
}

// getEnv lấy giá trị từ environment variable, nếu không có thì dùng default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var (
	loggers   = make(map[string]*logrus.Logger)
	loggersMu sync.Mutex
	rootDir   string
	globalCfg *Config
)

// InitLogger khởi tạo logger với cấu hình
func InitLogger(cfg *Config) error {
	if cfg == nil {
		cfg = &Config{}
	}
	globalCfg = cfg
	return nil
}

// getRootDir lấy root directory của project
func getRootDir() string {
	if rootDir != "" {
		return rootDir
	}
	executable, err := os.Executable()
	if err != nil {
		// Fallback về thư mục hiện tại
		wd, _ := os.Getwd()
		rootDir = wd
		return rootDir
	}
	rootDir = filepath.Dir(executable)
	return rootDir
}

// parseLogLevel chuyển đổi string sang logrus.Level
func parseLogLevel(level string) logrus.Level {
	switch strings.ToLower(level) {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	default:
		return logrus.InfoLevel
	}
}

// parseBool chuyển đổi string sang bool
func parseBool(s string, defaultValue bool) bool {
	if s == "" {
		return defaultValue
	}
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

// parseInt chuyển đổi string sang int
func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return result
}

// CustomTextFormatter là formatter tùy chỉnh để làm nổi bật log lỗi
type CustomTextFormatter struct {
	logrus.TextFormatter
}

// Format định dạng log entry với prefix đặc biệt cho ERROR và FATAL
// Giữ nguyên màu sắc của logrus bằng cách thêm prefix vào đầu (sẽ có màu của level)
func (f *CustomTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Gọi formatter gốc để lấy format chuẩn (đã có color codes)
	data, err := f.TextFormatter.Format(entry)
	if err != nil {
		return nil, err
	}

	// Nếu là ERROR hoặc FATAL, thêm prefix nổi bật vào đầu dòng
	if entry.Level == logrus.ErrorLevel || entry.Level == logrus.FatalLevel {
		var prefix string
		if entry.Level == logrus.ErrorLevel {
			prefix = "🚨 [ERROR] "
		} else {
			prefix = "💀 [FATAL] "
		}

		// Thêm prefix vào đầu dòng (sẽ có màu đỏ từ logrus)
		result := append([]byte(prefix), data...)

		// Thêm dòng separator ở cuối (loại bỏ newline cuối cùng trước)
		if len(result) > 0 && result[len(result)-1] == '\n' {
			result = result[:len(result)-1]
		}
		// Thêm separator (sẽ có màu từ logrus nếu đang dùng màu)
		separator := "\n═══════════════════════════════════════════════════════════\n"
		result = append(result, []byte(separator)...)

		return result, nil
	}

	// Với WARN, thêm prefix nhẹ hơn vào đầu dòng
	if entry.Level == logrus.WarnLevel {
		prefix := "⚠️  [WARN] "
		result := append([]byte(prefix), data...)
		return result, nil
	}

	return data, nil
}

// createFormatter tạo formatter dựa trên config
func createFormatter(format string) logrus.Formatter {
	if strings.ToLower(format) == "json" {
		return &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "caller",
			},
		}
	}

	// Custom text formatter với màu sắc cho console và prefix đặc biệt cho lỗi
	return &CustomTextFormatter{
		TextFormatter: logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			ForceColors:     true,
			DisableColors:   false,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				file := fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
				return funcName, file
			},
		},
	}
}

// GetLogger trả về logger theo tên (app, jobs, ...)
// Mỗi logger sẽ có file log riêng
func GetLogger(name string) *logrus.Logger {
	loggersMu.Lock()
	defer loggersMu.Unlock()

	if logger, ok := loggers[name]; ok {
		return logger
	}

	// Tạo logger mới
	logger := logrus.New()

	// Cấu hình level
	cfg := globalCfg
	if cfg == nil {
		cfg = &Config{}
	}
	logger.SetLevel(parseLogLevel(cfg.Level))

	// Cấu hình formatter
	logger.SetFormatter(createFormatter(cfg.Format))

	// Cấu hình caller
	if parseBool(cfg.EnableCaller, true) {
		logger.SetReportCaller(true)
	}

	// Tạo writers
	var writers []io.Writer

	// Console writer
	if parseBool(cfg.EnableConsole, true) {
		writers = append(writers, os.Stdout)
	}

	// File writer với rotation
	if parseBool(cfg.EnableFile, true) {
		logDir := cfg.LogDir
		if logDir == "" || logDir == "./logs" {
			logDir = filepath.Join(getRootDir(), "logs")
		}

		// Đảm bảo thư mục logs tồn tại
		if err := os.MkdirAll(logDir, 0755); err != nil {
			panic(fmt.Sprintf("Không thể tạo thư mục logs tại %s: %v", logDir, err))
		}

		logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", name))

		// Cấu hình log rotation
		fileWriter := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    parseInt(cfg.MaxSize, 100), // MB
			MaxBackups: parseInt(cfg.MaxBackups, 10),
			MaxAge:     parseInt(cfg.MaxAge, 30), // days
			Compress:   parseBool(cfg.Compress, true),
			LocalTime:  true,
		}

		writers = append(writers, fileWriter)
	}

	// Set output
	if len(writers) > 0 {
		if len(writers) == 1 {
			logger.SetOutput(writers[0])
		} else {
			logger.SetOutput(io.MultiWriter(writers...))
		}
	}

	// Log thông tin khởi tạo
	logger.WithFields(logrus.Fields{
		"logger_name": name,
		"level":       logger.GetLevel().String(),
		"format":      cfg.Format,
		"console":     parseBool(cfg.EnableConsole, true),
		"file":        parseBool(cfg.EnableFile, true),
	}).Info("Logger đã được khởi tạo thành công")

	loggers[name] = logger
	return logger
}

// GetJobLogger trả về logger cho jobs
func GetJobLogger() *logrus.Logger {
	return GetLogger("job")
}

// GetAppLogger trả về logger cho application
func GetAppLogger() *logrus.Logger {
	return GetLogger("app")
}

// GetDefaultLogger trả về logger mặc định
func GetDefaultLogger() *logrus.Logger {
	return GetLogger("default")
}

// WithContext tạo logger entry với context fields
// Sử dụng để thêm trace ID, request ID, job ID, etc.
func WithContext(logger *logrus.Logger, fields map[string]interface{}) *logrus.Entry {
	return logger.WithFields(logrus.Fields(fields))
}

// WithTraceID tạo logger entry với trace ID
func WithTraceID(logger *logrus.Logger, traceID string) *logrus.Entry {
	return logger.WithField("trace_id", traceID)
}

// WithJobID tạo logger entry với job ID
func WithJobID(logger *logrus.Logger, jobID string) *logrus.Entry {
	return logger.WithField("job_id", jobID)
}

// WithRequestID tạo logger entry với request ID
func WithRequestID(logger *logrus.Logger, requestID string) *logrus.Entry {
	return logger.WithField("request_id", requestID)
}

// LogDuration log thời gian thực thi của một function
func LogDuration(logger *logrus.Entry, operation string, startTime time.Time) {
	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"operation":   operation,
		"duration":    duration.String(),
		"duration_ms": duration.Milliseconds(),
	}).Debug("Operation completed")
}

// LogError log lỗi với stack trace
func LogError(logger *logrus.Entry, err error, message string, fields ...map[string]interface{}) {
	entry := logger.WithError(err)

	// Thêm các fields bổ sung
	for _, f := range fields {
		for k, v := range f {
			entry = entry.WithField(k, v)
		}
	}

	entry.Error(message)
}

// LogPanic log panic và recover
func LogPanic(logger *logrus.Logger) {
	if r := recover(); r != nil {
		logger.WithFields(logrus.Fields{
			"panic": r,
		}).Error("Panic recovered")
		panic(r) // Re-panic để stack trace được hiển thị
	}
}

// CleanupOldLogs xóa các log files cũ dựa trên MaxAge và MaxBackups
// Hàm này nên được gọi định kỳ (ví dụ: mỗi ngày) để đảm bảo log cũ được xóa
// ngay cả khi chưa đạt MaxSize (lumberjack chỉ cleanup khi rotate)
func CleanupOldLogs() error {
	cfg := globalCfg
	if cfg == nil {
		cfg = &Config{}
	}

	// Chỉ cleanup nếu file logging được bật
	if !parseBool(cfg.EnableFile, true) {
		return nil
	}

	logDir := cfg.LogDir
	if logDir == "" || logDir == "./logs" {
		logDir = filepath.Join(getRootDir(), "logs")
	}

	// Kiểm tra thư mục logs có tồn tại không
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return nil // Thư mục không tồn tại, không cần cleanup
	}

	maxAge := parseInt(cfg.MaxAge, 30)
	maxBackups := parseInt(cfg.MaxBackups, 10)
	cutoffTime := time.Now().AddDate(0, 0, -maxAge)

	// Đọc tất cả files trong thư mục logs
	files, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("không thể đọc thư mục logs: %v", err)
	}

	// Nhóm các log files theo logger name (ví dụ: app.log, app.log.2024-01-01.gz, job.log, ...)
	logFilesByLogger := make(map[string][]logFileInfo)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		filePath := filepath.Join(logDir, fileName)

		// Lấy thông tin file
		info, err := file.Info()
		if err != nil {
			continue
		}

		// Xác định logger name từ tên file
		// Format: {logger}.log hoặc {logger}.log.{timestamp}.gz
		loggerName := extractLoggerName(fileName)
		if loggerName == "" {
			continue // Không phải log file
		}

		// Thêm vào danh sách
		if _, ok := logFilesByLogger[loggerName]; !ok {
			logFilesByLogger[loggerName] = make([]logFileInfo, 0)
		}

		logFilesByLogger[loggerName] = append(logFilesByLogger[loggerName], logFileInfo{
			path:    filePath,
			name:    fileName,
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}

	// Cleanup cho từng logger
	totalDeleted := 0
	totalSizeFreed := int64(0)
	totalErrors := 0

	// Lấy logger để log lỗi
	appLogger := GetAppLogger()

	for loggerName, files := range logFilesByLogger {
		deleted, sizeFreed, errors := cleanupLoggerLogs(appLogger, loggerName, files, cutoffTime, maxBackups)
		totalDeleted += deleted
		totalSizeFreed += sizeFreed
		totalErrors += errors
	}

	// Log kết quả (luôn log để biết cleanup đã chạy)

	// Đếm tổng số log files trước khi cleanup
	totalLogFiles := 0
	for _, files := range logFilesByLogger {
		totalLogFiles += len(files)
	}

	if totalDeleted > 0 {
		appLogger.WithFields(logrus.Fields{
			"deleted_files":   totalDeleted,
			"size_freed_mb":   float64(totalSizeFreed) / 1024 / 1024,
			"max_age_days":    maxAge,
			"max_backups":     maxBackups,
			"total_files":     totalLogFiles,
			"remaining_files": totalLogFiles - totalDeleted,
			"delete_errors":   totalErrors,
		}).Info("🧹 Đã cleanup log files cũ")
	} else {
		fields := logrus.Fields{
			"max_age_days":    maxAge,
			"max_backups":     maxBackups,
			"log_dir":         logDir,
			"total_log_files": totalLogFiles,
		}
		if totalErrors > 0 {
			fields["delete_errors"] = totalErrors
			appLogger.WithFields(fields).Warn("🧹 Cleanup log: Không có file nào được xóa, nhưng có lỗi khi xóa file")
		} else {
			appLogger.WithFields(fields).Info("🧹 Cleanup log: Không có file nào cần xóa (tất cả files đều còn trong thời hạn)")
		}
	}

	return nil
}

// logFileInfo chứa thông tin về một log file
type logFileInfo struct {
	path    string
	name    string
	modTime time.Time
	size    int64
}

// extractLoggerName trích xuất tên logger từ tên file
// Ví dụ: "app.log" -> "app", "app.log.2024-01-01.gz" -> "app"
func extractLoggerName(fileName string) string {
	// Loại bỏ extension .gz nếu có
	fileName = strings.TrimSuffix(fileName, ".gz")

	// Tách theo dấu chấm
	parts := strings.Split(fileName, ".")
	if len(parts) < 2 {
		return ""
	}

	// Format của lumberjack: {logger}.log hoặc {logger}.log.{timestamp}
	// Tìm phần "log" trong tên file
	for i, part := range parts {
		if part == "log" {
			// Lấy tất cả phần trước "log" làm logger name
			if i > 0 {
				return strings.Join(parts[:i], ".")
			}
			return ""
		}
	}

	return ""
}

// cleanupLoggerLogs cleanup log files cho một logger cụ thể
// Trả về: số file đã xóa, tổng dung lượng đã giải phóng, số lỗi khi xóa
func cleanupLoggerLogs(logger *logrus.Logger, loggerName string, files []logFileInfo, cutoffTime time.Time, maxBackups int) (deleted int, sizeFreed int64, errors int) {
	// Tách các file backup (bỏ qua file hiện tại vì nó đang được sử dụng)
	var backupFiles []logFileInfo

	expectedCurrentFile := loggerName + ".log"

	for i := range files {
		// Bỏ qua file hiện tại (đang được sử dụng)
		if files[i].name == expectedCurrentFile {
			continue
		}
		// File backup có format: {logger}.log.{timestamp} hoặc {logger}.log.{timestamp}.gz
		if strings.HasPrefix(files[i].name, expectedCurrentFile+".") {
			backupFiles = append(backupFiles, files[i])
		}
	}

	// Sắp xếp backup files theo thời gian (mới nhất trước)
	sortLogFilesByTime(backupFiles)

	// Xóa các file cũ hơn cutoffTime
	for _, file := range backupFiles {
		if file.modTime.Before(cutoffTime) {
			if err := os.Remove(file.path); err == nil {
				deleted++
				sizeFreed += file.size
			} else {
				// Log lỗi chi tiết khi xóa file thất bại (quan trọng cho Linux)
				errors++
				logger.WithFields(logrus.Fields{
					"file_path": file.path,
					"file_name": file.name,
					"error":     err.Error(),
					"mod_time":  file.modTime.Format(time.RFC3339),
				}).Error("❌ Không thể xóa log file cũ (có thể do quyền truy cập trên Linux)")
			}
		}
	}

	// Giữ chỉ maxBackups files mới nhất (sau khi đã xóa theo MaxAge)
	// Xóa các file vượt quá maxBackups
	if len(backupFiles) > maxBackups {
		// Đã sắp xếp, lấy các file từ maxBackups trở đi
		for i := maxBackups; i < len(backupFiles); i++ {
			// Chỉ xóa nếu chưa bị xóa bởi MaxAge
			if !backupFiles[i].modTime.Before(cutoffTime) {
				if err := os.Remove(backupFiles[i].path); err == nil {
					deleted++
					sizeFreed += backupFiles[i].size
				} else {
					// Log lỗi chi tiết khi xóa file thất bại (quan trọng cho Linux)
					errors++
					logger.WithFields(logrus.Fields{
						"file_path": backupFiles[i].path,
						"file_name": backupFiles[i].name,
						"error":     err.Error(),
						"mod_time":  backupFiles[i].modTime.Format(time.RFC3339),
					}).Error("❌ Không thể xóa log file cũ (có thể do quyền truy cập trên Linux)")
				}
			}
		}
	}

	return deleted, sizeFreed, errors
}

// hasTimestamp kiểm tra xem tên file có chứa timestamp không
func hasTimestamp(fileName string) bool {
	// Timestamp thường có format: YYYY-MM-DD hoặc YYYYMMDD
	// Kiểm tra pattern: .log.YYYY-MM-DD hoặc .log.YYYYMMDD
	parts := strings.Split(fileName, ".")
	if len(parts) < 3 {
		return false
	}

	// Phần cuối cùng (trước .gz nếu có) có thể là timestamp
	lastPart := parts[len(parts)-1]
	if strings.HasSuffix(fileName, ".gz") {
		lastPart = parts[len(parts)-2]
	}

	// Kiểm tra format YYYY-MM-DD hoặc YYYYMMDD
	if len(lastPart) == 10 && strings.Count(lastPart, "-") == 2 {
		return true // Format: YYYY-MM-DD
	}
	if len(lastPart) == 8 {
		// Có thể là YYYYMMDD
		return true
	}

	return false
}

// sortLogFilesByTime sắp xếp log files theo thời gian (mới nhất trước)
func sortLogFilesByTime(files []logFileInfo) {
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.Before(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}

// StartLogCleanupScheduler khởi động scheduler để cleanup log định kỳ
// interval: khoảng thời gian giữa các lần cleanup (ví dụ: 24 * time.Hour)
func StartLogCleanupScheduler(interval time.Duration) {
	appLogger := GetAppLogger()
	appLogger.WithFields(logrus.Fields{
		"interval_hours": interval.Hours(),
		"interval":       interval.String(),
	}).Info("🔄 Khởi động log cleanup scheduler")

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Chạy cleanup ngay lập tức lần đầu
		appLogger.Info("🧹 Chạy cleanup log lần đầu...")
		if err := CleanupOldLogs(); err != nil {
			appLogger.WithError(err).Error("❌ Lỗi khi cleanup log files")
		} else {
			appLogger.Info("✅ Cleanup log lần đầu hoàn tất")
		}

		// Sau đó chạy định kỳ
		for range ticker.C {
			appLogger.Info("🧹 Chạy cleanup log định kỳ...")
			if err := CleanupOldLogs(); err != nil {
				appLogger.WithError(err).Error("❌ Lỗi khi cleanup log files")
			} else {
				appLogger.Info("✅ Cleanup log định kỳ hoàn tất")
			}
		}
	}()
}
