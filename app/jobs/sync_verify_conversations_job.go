/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncVerifyConversationsJob - job verify conversations từ FolkForm với Pancake để đảm bảo đồng bộ 2 chiều.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"log"
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
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
	log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
	log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Gọi hàm logic thực sự
	err := DoVerifyConversations_v2()
	if err != nil {
		duration := time.Since(startTime)
		log.Printf("═══════════════════════════════════════════════════════════")
		log.Printf("❌ JOB THẤT BẠI: %s", j.GetName())
		log.Printf("⏱️  Thời gian thực thi: %v", duration)
		log.Printf("❌ Lỗi: %v", err)
		log.Printf("═══════════════════════════════════════════════════════════")
		return err
	}

	duration := time.Since(startTime)
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("✅ JOB HOÀN THÀNH: %s", j.GetName())
	log.Printf("⏱️  Thời gian thực thi: %v", duration)
	log.Printf("⏰ Thời gian kết thúc: %s", time.Now().Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")
	return nil
}

// DoVerifyConversations_v2 thực thi logic verify conversations từ FolkForm với Pancake.
// Hàm này verify conversations unseen và đã đọc từ FolkForm với Pancake để đảm bảo đồng bộ 2 chiều.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoVerifyConversations_v2() error {
	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Verify conversations từ FolkForm với Pancake (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	log.Println("Bắt đầu verify conversations từ FolkForm với Pancake...")
	err := integrations.BridgeV2_VerifyConversations()
	if err != nil {
		log.Printf("❌ Lỗi khi verify conversations: %v", err)
		return err
	}
	log.Println("Verify conversations thành công")
	return nil
}

