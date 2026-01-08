/*
Package scheduler định nghĩa các interface và model cần thiết cho việc quản lý jobs.
File này cung cấp các thành phần cơ bản để xây dựng một job:
- Interface Job định nghĩa các phương thức cần thiết
- Struct JobMetadata lưu trữ thông tin về một lần chạy job
- Struct BaseJob cung cấp triển khai cơ bản của interface Job
*/
package scheduler

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// ================== INTERFACE ĐỊNH NGHĨA JOB ==================

// Job là interface chuẩn cho mọi job trong hệ thống.
type Job interface {
	// Execute thực thi logic chính của job
	// ctx: context để kiểm soát thời gian thực thi và hủy job
	// Trả về error nếu có lỗi xảy ra trong quá trình thực thi
	Execute(ctx context.Context) error

	// GetName trả về tên định danh của job
	// Tên này được sử dụng để đăng ký và quản lý job trong scheduler
	GetName() string

	// GetSchedule trả về biểu thức cron định nghĩa lịch chạy của job
	// Ví dụ: "0 0 * * *" - chạy lúc 00:00 mỗi ngày
	GetSchedule() string
}

// MetricsProvider interface để lấy metrics từ job
// BaseJob implement interface này
type MetricsProvider interface {
	// GetMetrics trả về metrics của job
	GetMetrics() JobMetrics

	// GetAvgDuration tính trung bình duration từ các lần chạy gần nhất
	GetAvgDuration() float64

	// GetMaxDuration trả về duration lớn nhất từ các lần chạy gần nhất
	GetMaxDuration() float64
}

// RunningProvider interface để kiểm tra job có đang chạy không
// BaseJob implement interface này
type RunningProvider interface {
	// IsRunning kiểm tra xem job có đang chạy không
	IsRunning() bool
}

// ================== BASE JOB ==================

// BaseJob cung cấp sẵn name, schedule và các hàm mặc định.
// Các job cụ thể chỉ cần nhúng *BaseJob và implement ExecuteInternal.
// Lưu ý: Các job con phải override ExecuteInternal() để có logic thực sự.
type BaseJob struct {
	name      string
	schedule  string
	mu        sync.Mutex
	isRunning bool
	// executeInternalFunc là callback function để gọi ExecuteInternal của job con
	// Nếu được set, sẽ gọi function này thay vì method ExecuteInternal của BaseJob
	executeInternalFunc func(ctx context.Context) error

	// Metrics tracking
	metricsMu sync.RWMutex
	metrics   JobMetrics
}

// JobMetrics lưu trữ metrics của job
type JobMetrics struct {
	RunCount        int64     `json:"runCount"`            // Tổng số lần chạy
	SuccessCount    int64     `json:"successCount"`        // Số lần thành công
	ErrorCount      int64     `json:"errorCount"`          // Số lần thất bại
	LastRunAt       time.Time `json:"lastRunAt"`           // Thời điểm chạy lần cuối
	LastRunDuration float64   `json:"lastRunDuration"`     // Thời gian chạy lần cuối (giây)
	LastRunStatus   string    `json:"lastRunStatus"`       // "success" hoặc "failed"
	LastError       string    `json:"lastError,omitempty"` // Lỗi lần cuối (nếu có)

	// Thống kê duration (giữ 100 lần chạy gần nhất để tính avg/max)
	durations    []float64
	maxDurations int // Giới hạn số lượng durations lưu trữ
}

// NewBaseJob khởi tạo BaseJob với tên và lịch chạy.
func NewBaseJob(name, schedule string) *BaseJob {
	return &BaseJob{
		name:      name,
		schedule:  schedule,
		mu:        sync.Mutex{},
		isRunning: false,
		metrics: JobMetrics{
			durations:    make([]float64, 0, 100),
			maxDurations: 100, // Giữ 100 lần chạy gần nhất
		},
	}
}

func (j *BaseJob) GetName() string     { return j.name }
func (j *BaseJob) GetSchedule() string { return j.schedule }

// Execute thực thi logic chính của job.
// Phương thức này kiểm soát trạng thái đang chạy của job và tracking metrics.
// Nếu job đang chạy thì bỏ qua, nếu chưa chạy thì thực thi.
func (j *BaseJob) Execute(ctx context.Context) error {
	// Kiểm tra và khóa mutex
	j.mu.Lock()
	if j.isRunning {
		j.mu.Unlock()
		return nil
	}
	j.isRunning = true
	j.mu.Unlock()

	// Bắt đầu tracking metrics
	startTime := time.Now()

	// Bắt panic để tránh crash toàn bộ ứng dụng
	// Sử dụng named return để có thể set error từ defer
	var err error
	defer func() {
		// Tính duration
		duration := time.Since(startTime).Seconds()

		// Cập nhật trạng thái khi kết thúc
		j.mu.Lock()
		j.isRunning = false
		j.mu.Unlock()

		// Bắt panic và chuyển thành error
		if r := recover(); r != nil {
			// Lấy stack trace để debug
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			stackTrace := string(buf[:n])

			// Log lỗi panic với đầy đủ thông tin
			log.Printf("[BaseJob] 🚨 PANIC trong job %s: %v", j.name, r)
			log.Printf("[BaseJob] 📋 Stack trace:\n%s", stackTrace)

			// Chuyển panic thành error
			err = fmt.Errorf("panic trong job %s: %v", j.name, r)
		}

		// Cập nhật metrics (sau khi xử lý panic để đảm bảo có error nếu panic)
		j.updateMetrics(err, duration)
	}()

	// Gọi phương thức ExecuteInternal của job con
	// Nếu có callback function được set, gọi callback function (method của job con)
	// Nếu không, gọi method mặc định của BaseJob
	if j.executeInternalFunc != nil {
		err = j.executeInternalFunc(ctx)
	} else {
		// Nếu không có callback, gọi method mặc định của BaseJob
		err = j.ExecuteInternal(ctx)
	}

	return err
}

// updateMetrics cập nhật metrics sau mỗi lần chạy job
func (j *BaseJob) updateMetrics(err error, duration float64) {
	j.metricsMu.Lock()
	defer j.metricsMu.Unlock()

	// Tăng run count
	j.metrics.RunCount++

	// Cập nhật last run info
	j.metrics.LastRunAt = time.Now()
	j.metrics.LastRunDuration = duration

	// Thêm duration vào danh sách (giữ tối đa maxDurations)
	j.metrics.durations = append(j.metrics.durations, duration)
	if len(j.metrics.durations) > j.metrics.maxDurations {
		// Xóa phần tử đầu tiên (FIFO)
		j.metrics.durations = j.metrics.durations[1:]
	}

	// Cập nhật success/error count và status
	if err != nil {
		j.metrics.ErrorCount++
		j.metrics.LastRunStatus = "failed"
		j.metrics.LastError = err.Error()
	} else {
		j.metrics.SuccessCount++
		j.metrics.LastRunStatus = "success"
		j.metrics.LastError = "" // Clear error nếu thành công
	}
}

// SetExecuteInternalCallback thiết lập callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách.
// Các job con nên gọi method này trong constructor để đảm bảo ExecuteInternal của job con được gọi.
// Tham số:
// - fn: Function callback có signature func(ctx context.Context) error
func (j *BaseJob) SetExecuteInternalCallback(fn func(ctx context.Context) error) {
	j.executeInternalFunc = fn
}

// ExecuteInternal thực thi logic riêng của job con.
// Các job con phải override phương thức này.
// Lưu ý: Do cách Go xử lý embedded struct, các job con nên gọi SetExecuteInternalCallback
// trong constructor để đảm bảo method của job con được gọi đúng cách.
func (j *BaseJob) ExecuteInternal(ctx context.Context) error {
	// Mặc định không làm gì, job con phải override
	return nil
}

// ================== METRICS ==================

// GetMetrics trả về metrics của job (thread-safe)
func (j *BaseJob) GetMetrics() JobMetrics {
	j.metricsMu.RLock()
	defer j.metricsMu.RUnlock()

	// Copy metrics để tránh data race
	metrics := JobMetrics{
		RunCount:        j.metrics.RunCount,
		SuccessCount:    j.metrics.SuccessCount,
		ErrorCount:      j.metrics.ErrorCount,
		LastRunAt:       j.metrics.LastRunAt,
		LastRunDuration: j.metrics.LastRunDuration,
		LastRunStatus:   j.metrics.LastRunStatus,
		LastError:       j.metrics.LastError,
	}

	// Copy durations
	metrics.durations = make([]float64, len(j.metrics.durations))
	copy(metrics.durations, j.metrics.durations)
	metrics.maxDurations = j.metrics.maxDurations

	return metrics
}

// GetAvgDuration tính trung bình duration từ các lần chạy gần nhất
func (j *BaseJob) GetAvgDuration() float64 {
	j.metricsMu.RLock()
	defer j.metricsMu.RUnlock()

	if len(j.metrics.durations) == 0 {
		return 0
	}

	var sum float64
	for _, d := range j.metrics.durations {
		sum += d
	}
	return sum / float64(len(j.metrics.durations))
}

// GetMaxDuration trả về duration lớn nhất từ các lần chạy gần nhất
func (j *BaseJob) GetMaxDuration() float64 {
	j.metricsMu.RLock()
	defer j.metricsMu.RUnlock()

	if len(j.metrics.durations) == 0 {
		return 0
	}

	max := j.metrics.durations[0]
	for _, d := range j.metrics.durations {
		if d > max {
			max = d
		}
	}
	return max
}

// IsRunning kiểm tra xem job có đang chạy không (thread-safe)
func (j *BaseJob) IsRunning() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.isRunning
}

// ================== TRẠNG THÁI & METADATA ==================

// JobStatus là enum trạng thái job.
type JobStatus string

const (
	// JobStatusPending: job đã được lập lịch nhưng chưa bắt đầu chạy
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning: job đang trong quá trình thực thi
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted: job đã hoàn thành thành công
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed: job thực thi thất bại, có thể cần retry
	JobStatusFailed JobStatus = "failed"
)

// JobMetadata lưu thông tin về từng lần chạy job.
type JobMetadata struct {
	// Name: tên định danh của job
	Name string `json:"name" bson:"name"`
	// Schedule: biểu thức cron định nghĩa lịch chạy
	Schedule string `json:"schedule" bson:"schedule"`
	// Status: trạng thái hiện tại của job
	Status JobStatus `json:"status" bson:"status"`
	// LastRun: thời điểm job chạy lần cuối
	LastRun time.Time `json:"last_run" bson:"last_run"`
	// NextRun: thời điểm dự kiến job sẽ chạy lần tiếp theo
	NextRun time.Time `json:"next_run" bson:"next_run"`
	// Duration: thời gian thực thi của lần chạy cuối (tính bằng giây)
	Duration float64 `json:"duration" bson:"duration"`
	// Error: thông tin lỗi nếu job thất bại
	Error string `json:"error,omitempty" bson:"error,omitempty"`
	// RetryCount: số lần đã retry
	RetryCount int `json:"retry_count" bson:"retry_count"`
	// MaxRetries: số lần retry tối đa cho phép
	MaxRetries int `json:"max_retries" bson:"max_retries"`
	// CreatedAt: thời điểm job được tạo
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	// UpdatedAt: thời điểm cập nhật thông tin gần nhất
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}
