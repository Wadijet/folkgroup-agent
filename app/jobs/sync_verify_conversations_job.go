/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncVerifyConversationsJob - job verify conversations từ FolkForm với Pancake để đảm bảo đồng bộ 2 chiều.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"
)

// SyncVerifyConversationsJob là job verify conversations từ FolkForm với Pancake.
// Job này đảm bảo đồng bộ 2 chiều, sửa lỗi không đồng bộ giữa FolkForm và Pancake.
// Verify conversations unseen và đã đọc từ FolkForm với Pancake để đảm bảo trạng thái đồng bộ.
type SyncVerifyConversationsJob struct {
	*scheduler.BaseJob
}

// NewSyncVerifyConversationsJob tạo một instance mới của SyncVerifyConversationsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncVerifyConversationsJob
func NewSyncVerifyConversationsJob(name, schedule string) *SyncVerifyConversationsJob {
	job := &SyncVerifyConversationsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic verify conversations từ FolkForm với Pancake.
// Phương thức này gọi DoVerifyConversations_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncVerifyConversationsJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoVerifyConversations_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoVerifyConversations_v2 thực thi logic verify conversations từ FolkForm với Pancake.
// Hàm này verify conversations unseen và đã đọc từ FolkForm với Pancake để đảm bảo đồng bộ 2 chiều.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoVerifyConversations_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-verify-conversations-job.log
	jobLogger := GetJobLoggerByName("sync-verify-conversations-job")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Lấy pageSize từ config động (có thể thay đổi từ server)
	// Nếu không có config, sử dụng default value 50
	// Config này có thể được thay đổi từ server mà không cần restart bot
	pageSize := GetJobConfigInt("sync-verify-conversations-job", "pageSize", 50)
	jobLogger.WithField("pageSize", pageSize).Info("📋 Sử dụng pageSize từ config")

	// Verify conversations từ FolkForm với Pancake (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	jobLogger.Info("Bắt đầu verify conversations từ FolkForm với Pancake...")
	err := integrations.BridgeV2_VerifyConversations(pageSize)
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi verify conversations")
		return err
	}
	jobLogger.Info("Verify conversations thành công")
	return nil
}
