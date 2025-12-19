/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncIncrementalConversationsJob - job đồng bộ conversations mới (incremental sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"log"
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
	startTime := time.Now()
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
	log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
	log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Gọi hàm logic thực sự
	err := DoSyncIncrementalConversations_v2()
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

// DoSyncIncrementalConversations_v2 thực thi logic đồng bộ conversations mới (incremental sync).
// Hàm này đồng bộ các conversations mới/cập nhật gần đây và messages của chúng.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncIncrementalConversations_v2() error {
	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Đồng bộ conversations mới nhất (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	log.Println("Bắt đầu đồng bộ conversations mới (incremental sync)...")
	err := integrations.BridgeV2_SyncNewData()
	if err != nil {
		log.Printf("❌ Lỗi khi đồng bộ conversations mới: %v", err)
		return err
	}
	log.Println("Đồng bộ conversations mới thành công")
	return nil
}
