/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncIncrementalConversationsJob - job đồng bộ conversations mới (incremental sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncIncrementalConversationsJob là job đồng bộ conversations mới (incremental sync).
// Job này sẽ đồng bộ các conversations mới/cập nhật gần đây và messages của chúng.
// Sử dụng order_by=updated_at và dừng khi gặp lastConversationId từ FolkForm.
type SyncIncrementalConversationsJob struct {
	*scheduler.BaseJob
}

// NewSyncIncrementalConversationsJob tạo một instance mới của SyncIncrementalConversationsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncIncrementalConversationsJob
func NewSyncIncrementalConversationsJob(name, schedule string) *SyncIncrementalConversationsJob {
	job := &SyncIncrementalConversationsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ conversations mới (incremental sync).
// Phương thức này gọi DoSyncIncrementalConversations_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncIncrementalConversationsJob) ExecuteInternal(ctx context.Context) error {
	// Đảm bảo logger đã được khởi tạo
	if JobLogger == nil {
		InitJobLogger()
	}

	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncIncrementalConversations_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncIncrementalConversations_v2 thực thi logic đồng bộ conversations mới (incremental sync).
// Hàm này đồng bộ các conversations mới/cập nhật gần đây và messages của chúng.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncIncrementalConversations_v2() error {
	// Đảm bảo logger đã được khởi tạo
	if JobLogger == nil {
		InitJobLogger()
	}

	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Đồng bộ conversations mới nhất (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	JobLogger.Info("Bắt đầu đồng bộ conversations mới (incremental sync)...")
	err := integrations.BridgeV2_SyncNewData()
	if err != nil {
		JobLogger.WithError(err).Error("❌ Lỗi khi đồng bộ conversations mới")
		return err
	}
	JobLogger.Info("Đồng bộ conversations mới thành công")
	return nil
}
