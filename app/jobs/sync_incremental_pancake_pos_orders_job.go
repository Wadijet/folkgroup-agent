/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncIncrementalPancakePosOrdersJob - job đồng bộ orders mới từ Pancake POS (incremental sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncIncrementalPancakePosOrdersJob là job đồng bộ orders mới từ Pancake POS (incremental sync).
// Job này sẽ đồng bộ các orders có updated_at từ lastUpdatedAt đến now từ POS.
// Sử dụng order_by="updated_at" và dừng khi gặp order với updated_at < since.
type SyncIncrementalPancakePosOrdersJob struct {
	*scheduler.BaseJob
}

// NewSyncIncrementalPancakePosOrdersJob tạo một instance mới của SyncIncrementalPancakePosOrdersJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncIncrementalPancakePosOrdersJob
func NewSyncIncrementalPancakePosOrdersJob(name, schedule string) *SyncIncrementalPancakePosOrdersJob {
	job := &SyncIncrementalPancakePosOrdersJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ orders mới từ Pancake POS (incremental sync).
// Phương thức này gọi DoSyncIncrementalPancakePosOrders_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncIncrementalPancakePosOrdersJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncIncrementalPancakePosOrders_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncIncrementalPancakePosOrders_v2 thực thi logic đồng bộ orders mới từ Pancake POS (incremental sync).
// Hàm này đồng bộ các orders có updated_at từ lastUpdatedAt đến now.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncIncrementalPancakePosOrders_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-incremental-pancake-pos-orders-job.log
	jobLogger := GetJobLoggerByName("sync-incremental-pancake-pos-orders-job")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Đồng bộ orders mới từ POS (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	jobLogger.Info("Bắt đầu đồng bộ orders mới từ Pancake POS (incremental sync)...")
	err := integrations.BridgeV2_SyncNewOrders()
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ orders mới từ Pancake POS")
		return err
	}
	jobLogger.Info("✅ Đồng bộ orders mới từ Pancake POS thành công")
	return nil
}
