/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncBackfillPancakePosOrdersJob - job đồng bộ orders cũ từ Pancake POS (backfill sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncBackfillPancakePosOrdersJob là job đồng bộ orders cũ từ Pancake POS (backfill sync).
// Job này sẽ đồng bộ các orders cũ hơn oldestUpdatedAt từ POS.
// Sử dụng order_by="updated_at" và bỏ qua orders với updated_at > until.
type SyncBackfillPancakePosOrdersJob struct {
	*scheduler.BaseJob
}

// NewSyncBackfillPancakePosOrdersJob tạo một instance mới của SyncBackfillPancakePosOrdersJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncBackfillPancakePosOrdersJob
func NewSyncBackfillPancakePosOrdersJob(name, schedule string) *SyncBackfillPancakePosOrdersJob {
	job := &SyncBackfillPancakePosOrdersJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ orders cũ từ Pancake POS (backfill sync).
// Phương thức này gọi DoSyncBackfillPancakePosOrders_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncBackfillPancakePosOrdersJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncBackfillPancakePosOrders_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncBackfillPancakePosOrders_v2 thực thi logic đồng bộ orders cũ từ Pancake POS (backfill sync).
// Hàm này đồng bộ các orders cũ hơn oldestUpdatedAt.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncBackfillPancakePosOrders_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-backfill-pancake-pos-orders-job.log
	jobLogger := GetJobLoggerByName("sync-backfill-pancake-pos-orders-job")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Lấy pageSize từ config động (có thể thay đổi từ server)
	// pageSize: Số lượng access tokens/pages lấy mỗi lần
	// orderPageSize: Số lượng orders lấy mỗi lần
	// Nếu không có config, sử dụng default values
	// Config này có thể được thay đổi từ server mà không cần restart bot
	pageSize := GetJobConfigInt("sync-backfill-pancake-pos-orders-job", "pageSize", 30)
	orderPageSize := GetJobConfigInt("sync-backfill-pancake-pos-orders-job", "pageSize", 30) // Cùng giá trị với pageSize
	jobLogger.WithFields(map[string]interface{}{
		"pageSize":      pageSize,
		"orderPageSize": orderPageSize,
	}).Info("📋 Sử dụng pageSize từ config")

	// Đồng bộ orders cũ từ POS (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	jobLogger.Info("Bắt đầu đồng bộ orders cũ từ Pancake POS (backfill sync)...")
	err := integrations.BridgeV2_SyncAllOrders(pageSize, orderPageSize)
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ orders cũ từ Pancake POS")
		return err
	}
	jobLogger.Info("✅ Đồng bộ orders cũ từ Pancake POS thành công")
	return nil
}
