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
			MaxSize:    parseInt(cfg.MaxSize, 100),    // MB
			MaxBackups: parseInt(cfg.MaxBackups, 10),
			MaxAge:     parseInt(cfg.MaxAge, 30),      // days
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
		"level":        logger.GetLevel().String(),
		"format":       cfg.Format,
		"console":      parseBool(cfg.EnableConsole, true),
		"file":         parseBool(cfg.EnableFile, true),
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
		"operation": operation,
		"duration":  duration.String(),
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
