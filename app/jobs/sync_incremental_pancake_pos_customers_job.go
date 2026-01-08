/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncIncrementalPancakePosCustomersJob - job đồng bộ customers mới từ Pancake POS (incremental sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncIncrementalPancakePosCustomersJob là job đồng bộ customers mới từ Pancake POS (incremental sync).
// Job này sẽ đồng bộ các customers có updated_at từ lastUpdatedAt đến now từ POS.
// Sử dụng start_time_updated_at và end_time_updated_at để filter theo thời gian.
type SyncIncrementalPancakePosCustomersJob struct {
	*scheduler.BaseJob
}

// NewSyncIncrementalPancakePosCustomersJob tạo một instance mới của SyncIncrementalPancakePosCustomersJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncIncrementalPancakePosCustomersJob
func NewSyncIncrementalPancakePosCustomersJob(name, schedule string) *SyncIncrementalPancakePosCustomersJob {
	job := &SyncIncrementalPancakePosCustomersJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ customers mới từ Pancake POS (incremental sync).
// Phương thức này gọi DoSyncIncrementalPancakePosCustomers_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncIncrementalPancakePosCustomersJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncIncrementalPancakePosCustomers_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncIncrementalPancakePosCustomers_v2 thực thi logic đồng bộ customers mới từ Pancake POS (incremental sync).
// Hàm này đồng bộ các customers có updated_at từ lastUpdatedAt đến now.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncIncrementalPancakePosCustomers_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-incremental-pancake-pos-customers-job.log
	jobLogger := GetJobLoggerByName("sync-incremental-pancake-pos-customers-job")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Lấy pageSize từ config động (có thể thay đổi từ server)
	// pageSize: Số lượng access tokens/pages lấy mỗi lần
	// customerPageSize: Số lượng customers lấy mỗi lần
	// Nếu không có config, sử dụng default values
	// Config này có thể được thay đổi từ server mà không cần restart bot
	pageSize := GetJobConfigInt("sync-incremental-pancake-pos-customers-job", "pageSize", 50)
	customerPageSize := GetJobConfigInt("sync-incremental-pancake-pos-customers-job", "pageSize", 50) // Cùng giá trị với pageSize
	jobLogger.WithFields(map[string]interface{}{
		"pageSize":        pageSize,
		"customerPageSize": customerPageSize,
	}).Info("📋 Sử dụng pageSize từ config")

	// Đồng bộ customers mới từ POS (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	jobLogger.Info("Bắt đầu đồng bộ customers mới từ Pancake POS (incremental sync)...")
	err := integrations.BridgeV2_SyncNewCustomersFromPos(pageSize, customerPageSize)
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ customers mới từ Pancake POS")
		return err
	}
	jobLogger.Info("✅ Đồng bộ customers mới từ Pancake POS thành công")
	return nil
}
