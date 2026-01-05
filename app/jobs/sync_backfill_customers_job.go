/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncBackfillCustomersJob - job đồng bộ customers cập nhật cũ (backfill sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncBackfillCustomersJob là job đồng bộ customers cập nhật cũ (backfill sync).
// Job này sẽ đồng bộ các customers cũ hơn oldestUpdatedAt từ FolkForm.
// Sử dụng order_by="updated_at" và bỏ qua customers với updated_at > until.
type SyncBackfillCustomersJob struct {
	*scheduler.BaseJob
}

// NewSyncBackfillCustomersJob tạo một instance mới của SyncBackfillCustomersJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncBackfillCustomersJob
func NewSyncBackfillCustomersJob(name, schedule string) *SyncBackfillCustomersJob {
	job := &SyncBackfillCustomersJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ customers cập nhật cũ (backfill sync).
// Phương thức này gọi DoSyncBackfillCustomers_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncBackfillCustomersJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncBackfillCustomers_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncBackfillCustomers_v2 thực thi logic đồng bộ customers cập nhật cũ (backfill sync).
// Hàm này đồng bộ các customers cũ hơn oldestUpdatedAt.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncBackfillCustomers_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-backfill-customers-job.log
	jobLogger := GetJobLoggerByName("sync-backfill-customers-job")

	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Đồng bộ customers cập nhật cũ (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	jobLogger.Info("Bắt đầu đồng bộ customers cập nhật cũ (backfill sync)...")
	err := integrations.BridgeV2_SyncAllCustomers()
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ customers cập nhật cũ")
		return err
	}
	jobLogger.Info("Đồng bộ customers cập nhật cũ thành công")
	return nil
}
