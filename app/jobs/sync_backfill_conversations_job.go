/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncBackfillConversationsJob - job đồng bộ conversations cũ (backfill sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncBackfillConversationsJob là job đồng bộ conversations cũ (backfill sync).
// Job này sẽ đồng bộ các conversations cũ hơn oldestConversationId và messages của chúng.
// Sử dụng order_by=updated_at và bắt đầu từ oldestConversationId từ FolkForm.
type SyncBackfillConversationsJob struct {
	*scheduler.BaseJob
}

// NewSyncBackfillConversationsJob tạo một instance mới của SyncBackfillConversationsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncBackfillConversationsJob
func NewSyncBackfillConversationsJob(name, schedule string) *SyncBackfillConversationsJob {
	job := &SyncBackfillConversationsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ conversations cũ (backfill sync).
// Phương thức này gọi DoSyncBackfillConversations_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncBackfillConversationsJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncBackfillConversations_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncBackfillConversations_v2 thực thi logic đồng bộ conversations cũ (backfill sync).
// Hàm này đồng bộ các conversations cũ hơn oldestConversationId và messages của chúng.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncBackfillConversations_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-backfill-conversations-job.log
	jobLogger := GetJobLoggerByName("sync-backfill-conversations-job")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Đồng bộ conversations cũ (backfill sync)
	jobLogger.Info("Bắt đầu đồng bộ conversations cũ (backfill sync)...")
	err := integrations.BridgeV2_SyncAllData()
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ conversations cũ")
		return err
	}
	jobLogger.Info("Đồng bộ conversations cũ thành công")

	return nil
}
