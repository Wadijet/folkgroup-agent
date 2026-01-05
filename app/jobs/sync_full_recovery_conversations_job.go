/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncFullRecoveryConversationsJob - job sync lại TOÀN BỘ conversations để đảm bảo không bỏ sót.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncFullRecoveryConversationsJob là job sync lại TOÀN BỘ conversations từ Pancake về FolkForm.
// Job này không dựa vào lastConversationId hay oldestConversationId - sync từ đầu đến cuối.
// Mục đích: Đảm bảo không bỏ sót conversations khi có lỗi ở giữa quá trình sync.
// Chạy chậm cũng được, quan trọng là đảm bảo đầy đủ dữ liệu.
type SyncFullRecoveryConversationsJob struct {
	*scheduler.BaseJob
}

// NewSyncFullRecoveryConversationsJob tạo một instance mới của SyncFullRecoveryConversationsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncFullRecoveryConversationsJob
func NewSyncFullRecoveryConversationsJob(name, schedule string) *SyncFullRecoveryConversationsJob {
	job := &SyncFullRecoveryConversationsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic sync lại TOÀN BỘ conversations.
// Phương thức này gọi DoSyncFullRecoveryConversations() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncFullRecoveryConversationsJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncFullRecoveryConversations()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncFullRecoveryConversations thực thi logic sync lại TOÀN BỘ conversations.
// Hàm này sync lại tất cả conversations từ Pancake về FolkForm, không dựa vào checkpoint.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncFullRecoveryConversations() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-full-recovery-conversations-job.log
	jobLogger := GetJobLoggerByName("sync-full-recovery-conversations-job")

	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Sync lại TOÀN BỘ conversations (full recovery sync)
	jobLogger.Info("Bắt đầu sync lại TOÀN BỘ conversations (full recovery sync)...")
	err := integrations.BridgeV2_SyncFullRecovery()
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi sync lại TOÀN BỘ conversations")
		return err
	}
	jobLogger.Info("Sync lại TOÀN BỘ conversations thành công")
	return nil
}

