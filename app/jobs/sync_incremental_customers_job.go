/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncIncrementalCustomersJob - job đồng bộ customers đã cập nhật gần đây (incremental sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncIncrementalCustomersJob là job đồng bộ customers đã cập nhật gần đây (incremental sync).
// Job này sẽ đồng bộ các customers mới hơn lastUpdatedAt từ FolkForm.
// Sử dụng order_by="updated_at" và dừng khi gặp customer với updated_at < since.
type SyncIncrementalCustomersJob struct {
	*scheduler.BaseJob
}

// NewSyncIncrementalCustomersJob tạo một instance mới của SyncIncrementalCustomersJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncIncrementalCustomersJob
func NewSyncIncrementalCustomersJob(name, schedule string) *SyncIncrementalCustomersJob {
	job := &SyncIncrementalCustomersJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ customers đã cập nhật gần đây (incremental sync).
// Phương thức này gọi DoSyncIncrementalCustomers_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncIncrementalCustomersJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncIncrementalCustomers_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncIncrementalCustomers_v2 thực thi logic đồng bộ customers đã cập nhật gần đây (incremental sync).
// Hàm này đồng bộ các customers mới hơn lastUpdatedAt.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncIncrementalCustomers_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-incremental-customers-job.log
	jobLogger := GetJobLoggerByName("sync-incremental-customers-job")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Lấy pageSize từ config động (có thể thay đổi từ server)
	// Nếu không có config, sử dụng default value 50
	// Config này có thể được thay đổi từ server mà không cần restart bot
	pageSize := GetJobConfigInt("sync-incremental-customers-job", "pageSize", 50)
	jobLogger.WithField("pageSize", pageSize).Info("📋 Sử dụng pageSize từ config")

	// Đồng bộ customers đã cập nhật gần đây (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	jobLogger.Info("Bắt đầu đồng bộ customers đã cập nhật gần đây (incremental sync)...")
	err := integrations.BridgeV2_SyncNewCustomers(pageSize)
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ customers đã cập nhật gần đây")
		return err
	}
	jobLogger.Info("Đồng bộ customers đã cập nhật gần đây thành công")
	return nil
}
