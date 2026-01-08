/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncIncrementalPostsJob - job đồng bộ posts mới (incremental sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncIncrementalPostsJob là job đồng bộ posts mới (incremental sync).
// Job này sẽ đồng bộ các posts mới hơn lastInsertedAt từ FolkForm.
// Sử dụng since/until và dừng khi gặp post với inserted_at < since.
type SyncIncrementalPostsJob struct {
	*scheduler.BaseJob
}

// NewSyncIncrementalPostsJob tạo một instance mới của SyncIncrementalPostsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncIncrementalPostsJob
func NewSyncIncrementalPostsJob(name, schedule string) *SyncIncrementalPostsJob {
	job := &SyncIncrementalPostsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ posts mới (incremental sync).
// Phương thức này gọi DoSyncIncrementalPosts_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncIncrementalPostsJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncIncrementalPosts_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncIncrementalPosts_v2 thực thi logic đồng bộ posts mới (incremental sync).
// Hàm này đồng bộ các posts mới hơn lastInsertedAt.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncIncrementalPosts_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-incremental-posts-job.log
	jobLogger := GetJobLoggerByName("sync-incremental-posts-job")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Đồng bộ posts mới nhất (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	jobLogger.Info("Bắt đầu đồng bộ posts mới (incremental sync)...")
	err := integrations.BridgeV2_SyncNewPosts()
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ posts mới")
		return err
	}
	jobLogger.Info("Đồng bộ posts mới thành công")
	return nil
}
