/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncBackfillPostsJob - job đồng bộ posts cũ (backfill sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncBackfillPostsJob là job đồng bộ posts cũ (backfill sync).
// Job này sẽ đồng bộ các posts cũ hơn oldestInsertedAt từ FolkForm.
// Sử dụng since/until và dừng khi gặp post với inserted_at > until.
type SyncBackfillPostsJob struct {
	*scheduler.BaseJob
}

// NewSyncBackfillPostsJob tạo một instance mới của SyncBackfillPostsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncBackfillPostsJob
func NewSyncBackfillPostsJob(name, schedule string) *SyncBackfillPostsJob {
	job := &SyncBackfillPostsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ posts cũ (backfill sync).
// Phương thức này gọi DoSyncBackfillPosts_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncBackfillPostsJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncBackfillPosts_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncBackfillPosts_v2 thực thi logic đồng bộ posts cũ (backfill sync).
// Hàm này đồng bộ các posts cũ hơn oldestInsertedAt.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncBackfillPosts_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-backfill-posts-job.log
	jobLogger := GetJobLoggerByName("sync-backfill-posts-job")

	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Đồng bộ posts cũ (backfill sync)
	jobLogger.Info("Bắt đầu đồng bộ posts cũ (backfill sync)...")
	err := integrations.BridgeV2_SyncAllPosts()
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ posts cũ")
		return err
	}
	jobLogger.Info("Đồng bộ posts cũ thành công")

	return nil
}
