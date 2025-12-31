# Hướng Dẫn Sử Dụng Hệ Thống Logger

## Tổng Quan

Hệ thống logger mới được thiết kế chuyên nghiệp với các tính năng:
- **Structured Logging**: Hỗ trợ JSON và Text format
- **Log Rotation**: Tự động rotate log files theo kích thước và thời gian
- **Context Support**: Hỗ trợ trace ID, job ID, request ID
- **Multiple Loggers**: Hỗ trợ nhiều logger cho các module khác nhau
- **Configurable**: Cấu hình hoàn toàn qua environment variables

## Cấu Hình

### Environment Variables

Thêm các biến sau vào file `.env`:

```env
# Log Level: debug, info, warn, error, fatal (mặc định: info)
LOG_LEVEL=info

# Log Format: json hoặc text (mặc định: text)
# - text: Dễ đọc cho development
# - json: Phù hợp cho production và log aggregation tools
LOG_FORMAT=text

# Thư mục lưu log files (mặc định: ./logs)
LOG_DIR=./logs

# Bật/tắt log ra console (mặc định: true)
LOG_ENABLE_CONSOLE=true

# Bật/tắt log ra file (mặc định: true)
LOG_ENABLE_FILE=true

# Kích thước tối đa của log file trước khi rotate (MB) (mặc định: 100)
LOG_MAX_SIZE=100

# Số lượng log files cũ được giữ lại (mặc định: 10)
LOG_MAX_BACKUPS=10

# Số ngày giữ log files cũ (mặc định: 30)
LOG_MAX_AGE=30

# Nén log files cũ (mặc định: true)
LOG_COMPRESS=true

# Hiển thị thông tin caller (file:line) (mặc định: true)
LOG_ENABLE_CALLER=true
```

### Ví Dụ Cấu Hình

**Development:**
```env
LOG_LEVEL=debug
LOG_FORMAT=text
LOG_ENABLE_CONSOLE=true
LOG_ENABLE_FILE=true
```

**Production:**
```env
LOG_LEVEL=info
LOG_FORMAT=json
LOG_ENABLE_CONSOLE=false
LOG_ENABLE_FILE=true
LOG_MAX_SIZE=500
LOG_MAX_BACKUPS=30
LOG_MAX_AGE=90
```

## Sử Dụng

### 1. Trong Main Application

```go
import (
    "agent_pancake/config"
    "agent_pancake/utility/logger"
    "github.com/sirupsen/logrus"
)

// Khởi tạo logger
logCfg := config.LogConfig()
if err := logger.InitLogger(logCfg); err != nil {
    panic(fmt.Sprintf("Không thể khởi tạo logger: %v", err))
}

// Lấy logger cho application
appLogger := logger.GetAppLogger()
appLogger.Info("Ứng dụng đã khởi động")
```

### 2. Trong Jobs

```go
import (
    "agent_pancake/app/jobs"
    "time"
)

// Sử dụng helper functions
func (j *MyJob) ExecuteInternal(ctx context.Context) error {
    // Đảm bảo logger đã được khởi tạo
    if jobs.JobLogger == nil {
        jobs.InitJobLogger()
    }
    
    startTime := time.Now()
    
    // Log khi job bắt đầu
    jobs.LogJobStart(j.GetName(), j.GetSchedule()).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")
    
    // Thực thi logic
    err := doWork()
    duration := time.Since(startTime)
    
    if err != nil {
        // Log lỗi
        jobs.LogJobError(j.GetName(), err, duration.String(), duration.Milliseconds())
        return err
    }
    
    // Log thành công
    jobs.LogJobEnd(j.GetName(), duration.String(), duration.Milliseconds())
    return nil
}
```

### 3. Sử Dụng Trực Tiếp Logger

```go
import (
    "agent_pancake/utility/logger"
    "github.com/sirupsen/logrus"
)

// Lấy logger
logger := logger.GetJobLogger()

// Log với fields
logger.WithFields(logrus.Fields{
    "user_id": 123,
    "action": "create_order",
}).Info("Đã tạo đơn hàng")

// Log với error
logger.WithError(err).Error("Lỗi khi xử lý")

// Log với context
entry := logger.WithField("trace_id", "abc-123")
entry.Info("Request started")
entry.WithField("request_id", "req-456").Info("Processing request")
```

### 4. Helper Functions Cho Jobs

```go
// Log job start
jobs.LogJobStart(jobName, schedule)

// Log job end
jobs.LogJobEnd(jobName, duration, durationMs)

// Log job error
jobs.LogJobError(jobName, err, duration, durationMs)

// Log với fields
jobs.LogJobInfo(jobName, message, fields)
jobs.LogJobDebug(jobName, message, fields)
jobs.LogJobWarn(jobName, message, fields)
jobs.LogJobErrorWithFields(jobName, err, message, fields)

// Log operation duration
jobs.LogOperationDuration(jobName, operation, duration, durationMs)
```

## Log Levels

- **DEBUG**: Thông tin chi tiết cho debugging
- **INFO**: Thông tin chung về hoạt động của ứng dụng
- **WARN**: Cảnh báo về các vấn đề tiềm ẩn
- **ERROR**: Lỗi xảy ra nhưng ứng dụng vẫn tiếp tục chạy
- **FATAL**: Lỗi nghiêm trọng, ứng dụng không thể tiếp tục

## Log Rotation

Log files sẽ tự động được rotate khi:
- Đạt kích thước tối đa (`LOG_MAX_SIZE`)
- Đạt số ngày tối đa (`LOG_MAX_AGE`)

Các file cũ sẽ được:
- Giữ lại tối đa `LOG_MAX_BACKUPS` files
- Nén nếu `LOG_COMPRESS=true`
- Tự động xóa khi vượt quá `LOG_MAX_AGE`

## Structured Logging

### Text Format (Development)
```
INFO[2024-01-15 10:30:45.123] Đã tạo đơn hàng    caller=main.go:45 job_id=123 user_id=456
```

### JSON Format (Production)
```json
{
  "timestamp": "2024-01-15T10:30:45.123Z",
  "level": "info",
  "message": "Đã tạo đơn hàng",
  "caller": "main.go:45",
  "job_id": 123,
  "user_id": 456
}
```

## Best Practices

1. **Sử dụng đúng log level**: 
   - DEBUG cho thông tin chi tiết
   - INFO cho các hoạt động bình thường
   - WARN cho cảnh báo
   - ERROR cho lỗi
   - FATAL chỉ khi ứng dụng không thể tiếp tục

2. **Thêm context fields**:
   ```go
   logger.WithFields(logrus.Fields{
       "job_id": jobID,
       "user_id": userID,
       "operation": "sync_data",
   }).Info("Bắt đầu sync")
   ```

3. **Log errors với stack trace**:
   ```go
   logger.WithError(err).Error("Lỗi khi xử lý")
   ```

4. **Sử dụng structured logging**:
   - Thêm fields thay vì format string
   - Dễ dàng query và filter logs
   - Tương thích với log aggregation tools

5. **Performance logging**:
   ```go
   startTime := time.Now()
   // ... do work ...
   logger.WithField("duration_ms", time.Since(startTime).Milliseconds()).Info("Operation completed")
   ```

## Migration từ Log Cũ

Thay thế:
```go
log.Printf("Message: %s", value)
```

Bằng:
```go
logger.WithField("key", value).Info("Message")
```

Hoặc:
```go
logger.Infof("Message: %s", value)
```

## Troubleshooting

### Log không xuất hiện
- Kiểm tra `LOG_LEVEL` có đúng không
- Kiểm tra `LOG_ENABLE_CONSOLE` và `LOG_ENABLE_FILE`
- Kiểm tra quyền ghi file trong thư mục logs

### Log files quá lớn
- Giảm `LOG_MAX_SIZE`
- Giảm `LOG_LEVEL` (ví dụ: từ debug xuống info)
- Tăng `LOG_MAX_BACKUPS` để giữ nhiều files hơn

### Log không rotate
- Kiểm tra `LOG_MAX_SIZE` và `LOG_MAX_AGE`
- Kiểm tra quyền ghi file
- Kiểm tra disk space
