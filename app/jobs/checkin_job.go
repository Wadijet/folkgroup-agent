/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa CheckInJob - job thực hiện check-in với server FolkForm (chỉ đảm bảo đăng nhập, không đồng bộ page/token).
*/
package jobs

import (
	"agent_pancake/app/scheduler"
	"agent_pancake/app/services"
	"context"
	"time"
)

// CheckInJob là job thực hiện check-in với server FolkForm.
// Job này chỉ làm việc với server FolkForm, không cần đồng bộ token/page:
// 1. EnsureFolkFormLoggedIn: kiểm tra đã login chưa, chưa thì login (token + role ID)
// 2. Gửi enhanced check-in với metrics, system info, job status, config
// 3. Nhận và xử lý commands/config updates từ server
type CheckInJob struct {
	*scheduler.BaseJob
	checkInService *services.CheckInService
}

// NewCheckInJob tạo một instance mới của CheckInJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy (ví dụ: "*/60 * * * * *" = mỗi 60 giây)
// - checkInService: CheckInService để gửi check-in data
// Trả về một instance của CheckInJob
func NewCheckInJob(name, schedule string, checkInService *services.CheckInService) *CheckInJob {
	job := &CheckInJob{
		BaseJob:        scheduler.NewBaseJob(name, schedule),
		checkInService: checkInService,
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic check-in với FolkForm.
// Phương thức này:
// 1. EnsureFolkFormLoggedIn: kiểm tra đã login chưa, chưa thì login (không đồng bộ page/token)
// 2. Gửi enhanced check-in với đầy đủ thông tin
// 3. Xử lý response (commands, config updates) - được xử lý tự động trong SendCheckIn
func (j *CheckInJob) ExecuteInternal(ctx context.Context) error {
	// Logger riêng cho job này
	jobLogger := GetJobLoggerByName("check-in-job")

	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 Check-in job bắt đầu")

	// Bước 1: Đảm bảo đã đăng nhập FolkForm (chỉ login nếu cần, không sync page/token)
	jobLogger.Info("Bước 1/2: Kiểm tra đăng nhập FolkForm...")
	EnsureFolkFormLoggedIn()

	// Bước 2: Gửi enhanced check-in với đầy đủ thông tin
	jobLogger.Info("Bước 2/2: Gửi enhanced check-in...")
	_, err := j.checkInService.SendCheckIn()

	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		jobLogger.WithError(err).Error("❌ Lỗi khi gửi check-in")
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	jobLogger.WithField("duration_ms", durationMs).Info("✅ Check-in job hoàn thành thành công")
	return nil
}
