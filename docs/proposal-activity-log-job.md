# Đề xuất: Job Ghi Nhật Ký Làm Việc (Activity Log Job)

## 1. Mục đích

Tạo một job ghi nhật ký làm việc để:
- Ghi lại những thay đổi lớn (ví dụ: đổi config, job được enable/disable)
- Ghi lại kết quả chạy của các jobs quan trọng
- Đẩy log lên server để admin biết agent đã nhận config và trạng thái hoạt động

## 2. Kiến trúc

### 2.1. Activity Log Service
Service quản lý activity logs:
- Thu thập các sự kiện quan trọng từ hệ thống
- Lưu trữ tạm thời trong memory (ring buffer)
- Gửi batch logs lên server định kỳ

### 2.2. Activity Log Job
Job chạy định kỳ để:
- Thu thập logs từ service
- Gửi batch logs lên server
- Xóa logs đã gửi thành công

### 2.3. Tích hợp với hệ thống hiện có
- **ConfigManager**: Ghi log khi config thay đổi (apply, submit, pull)
- **Scheduler**: Ghi log khi job được enable/disable, schedule thay đổi
- **Jobs**: Ghi log kết quả chạy (thành công/thất bại) cho các jobs quan trọng

## 3. Cấu trúc Activity Log

```go
type ActivityLog struct {
    ID          string                 `json:"id"`          // UUID
    AgentID     string                 `json:"agentId"`     // Agent ID
    Timestamp   int64                  `json:"timestamp"`   // Unix timestamp
    Type        string                 `json:"type"`       // "config_change", "job_result", "job_status_change", "system_event"
    Category    string                 `json:"category"`   // "config", "job", "system"
    Level       string                 `json:"level"`      // "info", "warning", "error"
    Title       string                 `json:"title"`      // Tiêu đề ngắn gọn
    Description string                 `json:"description"` // Mô tả chi tiết
    Metadata    map[string]interface{} `json:"metadata"`   // Dữ liệu bổ sung
    Status      string                 `json:"status"`     // "pending", "sent", "failed"
}
```

### 3.1. Các loại Activity Log

#### Config Change
- **Type**: `config_change`
- **Category**: `config`
- **Level**: `info`
- **Metadata**: 
  - `action`: "apply", "submit", "pull"
  - `version`: Config version
  - `hash`: Config hash
  - `changeLog`: Mô tả thay đổi (nếu có)

#### Job Result
- **Type**: `job_result`
- **Category**: `job`
- **Level**: `info` hoặc `error` (tùy kết quả)
- **Metadata**:
  - `jobName`: Tên job
  - `status`: "success", "failed"
  - `duration`: Thời gian chạy (giây)
  - `error`: Lỗi (nếu có)
  - `metrics`: Metrics của job

#### Job Status Change
- **Type**: `job_status_change`
- **Category**: `job`
- **Level**: `info`
- **Metadata**:
  - `jobName`: Tên job
  - `action`: "enable", "disable", "schedule_change"
  - `oldValue`: Giá trị cũ
  - `newValue`: Giá trị mới

#### System Event
- **Type**: `system_event`
- **Category**: `system`
- **Level**: `info`, `warning`, `error`
- **Metadata**: Tùy theo sự kiện

## 4. Implementation

### 4.1. ActivityLogService

```go
// app/services/activity_log_service.go
type ActivityLogService struct {
    logs        []ActivityLog  // Priority queue (error > warning > info)
    maxLogs     int            // Tối đa số logs lưu (ví dụ: 500)
    mu          sync.RWMutex
    agentId     string
    
    // Tối ưu
    batchSize   int            // Số logs tối đa mỗi lần gửi (ví dụ: 20)
    lastSentAt  time.Time      // Thời điểm gửi lần cuối (rate limiting)
    minInterval time.Duration  // Khoảng thời gian tối thiểu giữa các lần gửi (ví dụ: 5 phút)
    
    // Aggregation
    aggregatedLogs map[string]*AggregatedLog  // Gộp logs tương tự
}

// Log ghi một activity log mới (với filter và sampling)
func (s *ActivityLogService) Log(logType, category, level, title, description string, metadata map[string]interface{})

// GetPendingLogs lấy các logs chưa gửi (đã filter, deduplicate, aggregate)
// Trả về tối đa batchSize logs, ưu tiên logs quan trọng
func (s *ActivityLogService) GetPendingLogs() []ActivityLog

// ShouldSend kiểm tra xem có nên gửi logs không (rate limiting)
func (s *ActivityLogService) ShouldSend() bool

// MarkLogsAsSent đánh dấu logs đã gửi thành công
func (s *ActivityLogService) MarkLogsAsSent(logIds []string)

// ClearSentLogs xóa logs đã gửi (giữ lại logs failed)
func (s *ActivityLogService) ClearSentLogs()

// Deduplicate loại bỏ logs trùng lặp
func (s *ActivityLogService) Deduplicate(logs []ActivityLog) []ActivityLog

// Aggregate gộp logs tương tự
func (s *ActivityLogService) Aggregate(logs []ActivityLog) []ActivityLog
```

### 4.2. ActivityLogJob

```go
// app/jobs/activity_log_job.go
type ActivityLogJob struct {
    *scheduler.BaseJob
    activityLogService *services.ActivityLogService
}

// ExecuteInternal:
// 1. Kiểm tra rate limiting (ShouldSend)
// 2. Lấy pending logs từ service (đã filter, deduplicate, aggregate)
// 3. Nếu có logs → Gửi batch logs lên server (tối đa batchSize)
// 4. Đánh dấu logs đã gửi thành công
// 5. Xóa logs đã gửi thành công
// 6. Cập nhật lastSentAt
```

### 4.3. Tích hợp với ConfigManager

Thêm logging vào các methods:
- `ApplyConfigDiff()`: Log khi apply config diff
- `ApplyFullConfig()`: Log khi apply full config
- `SubmitConfig()`: Log khi submit config
- `PullConfig()`: Log khi pull config

### 4.4. Tích hợp với Scheduler

Thêm logging vào các methods:
- `RemoveJob()`: Log khi job bị disable
- `UpdateJobSchedule()`: Log khi schedule thay đổi

### 4.5. Tích hợp với Jobs

Thêm logging vào các jobs quan trọng:
- Ghi log khi job chạy thành công/thất bại
- Chỉ ghi log cho các jobs quan trọng (có thể config)

## 5. API Endpoint

Cần tạo API endpoint mới trên server:
```
POST /v1/agent/activity-logs
Body: {
    agentId: string,
    logs: ActivityLog[]
}
Response: {
    code: 200,
    message: "Success",
    data: {
        sentCount: number,
        failedLogIds: string[]
    }
}
```

Hoặc có thể gửi qua check-in (thêm field `activityLogs` vào `AgentCheckInRequest`).

## 6. Schedule

ActivityLogJob nên chạy:
- **Mỗi 10 phút**: Để giảm tải đường truyền (khuyến nghị)
- Hoặc **mỗi 5 phút**: Nếu cần real-time hơn
- **Không nên** chạy mỗi 1 phút (quá nhiều requests)

## 7. Tối ưu để Giảm Tải Đường Truyền

### 7.1. Filter Logs - Chỉ Gửi Logs Quan Trọng

**Chỉ gửi logs có level >= "info" và các sự kiện quan trọng:**
- ✅ **Config thay đổi** (apply, submit, pull) - **LUÔN GỬI**
- ✅ **Job được enable/disable** - **LUÔN GỬI**
- ✅ **Job schedule thay đổi** - **LUÔN GỬI**
- ✅ **Job chạy thất bại (error)** - **LUÔN GỬI**
- ⚠️ **Job chạy thành công** - **CHỈ GỬI KHI CẦN** (sampling hoặc config)
- ❌ **Không gửi** logs thông thường (tránh spam)

### 7.2. Batch và Giới Hạn Số Lượng

- **Batch size**: Tối đa **10-20 logs** mỗi lần gửi
- **Priority queue**: Ưu tiên gửi logs quan trọng trước (error > warning > info)
- **Deduplication**: Loại bỏ logs trùng lặp (cùng type, category, metadata trong 1 phút)

### 7.3. Aggregation - Gộp Logs Tương Tự

Gộp các logs tương tự trong khoảng thời gian (ví dụ: 5 phút):
- **Job results**: Gộp nhiều lần chạy thành công của cùng 1 job thành 1 log summary
  - Ví dụ: "Job X chạy thành công 10 lần trong 5 phút, avg duration: 2.5s"
- **System events**: Gộp các events tương tự

### 7.4. Sampling cho Job Results

Chỉ gửi log cho một phần nhỏ các lần chạy thành công:
- **Error logs**: Gửi 100% (luôn quan trọng)
- **Success logs**: Sampling 1-5% (có thể config)
- **Config/Status changes**: Gửi 100% (luôn quan trọng)

### 7.5. Rate Limiting

- **Tần suất gửi**: Tối đa **1 lần mỗi 5-10 phút** (thay vì mỗi 1 phút)
- **Giới hạn tổng**: Tối đa **50-100 logs/giờ** để tránh quá tải

### 7.6. Compression (Tùy chọn)

- Nén logs trước khi gửi (gzip) nếu batch > 10KB
- Giảm 60-80% kích thước

### 7.7. Local Storage Fallback

- Lưu logs vào file local nếu không gửi được
- Gửi lại khi có kết nối
- Tự động xóa logs cũ (> 7 ngày)

## 8. Ước Tính Tải Đường Truyền

### 8.1. Kích Thước Log Trung Bình
- Mỗi log: ~200-500 bytes (JSON)
- 20 logs/batch: ~4-10 KB
- Nén (gzip): ~1.5-3 KB

### 8.2. Tần Suất Gửi
- **Mỗi 10 phút**: 6 lần/giờ
- **Mỗi lần**: Tối đa 20 logs
- **Tổng**: ~120 logs/giờ (nếu có đủ logs)

### 8.3. Băng Thông Ước Tính
- **Không nén**: 6 requests/giờ × 10 KB = ~60 KB/giờ = ~1.4 MB/ngày
- **Có nén**: 6 requests/giờ × 3 KB = ~18 KB/giờ = ~430 KB/ngày

**Kết luận**: Rất nhẹ, không ảnh hưởng đáng kể đến đường truyền.

## 9. Lợi ích

1. **Theo dõi config**: Admin biết agent đã nhận config chưa
2. **Debug**: Dễ dàng debug khi có vấn đề
3. **Audit trail**: Có lịch sử thay đổi config và job status
4. **Monitoring**: Theo dõi health của agent và jobs
5. **Tối ưu băng thông**: Chỉ gửi logs quan trọng, batch và nén

## 10. Tùy chọn Implementation

### Option 1: Gửi qua Check-In (Đơn giản)
- Thêm field `activityLogs` vào `AgentCheckInRequest`
- Server xử lý logs trong check-in handler
- Không cần API riêng

### Option 2: API riêng (Linh hoạt)
- Tạo API endpoint riêng cho activity logs
- Có thể gửi logs độc lập với check-in
- Linh hoạt hơn nhưng cần thêm code

**Khuyến nghị**: Option 1 (gửi qua check-in) vì đơn giản và đủ dùng.
