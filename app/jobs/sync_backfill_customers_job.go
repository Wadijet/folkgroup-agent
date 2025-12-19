/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncBackfillCustomersJob - job đồng bộ customers cập nhật cũ (backfill sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"log"
	"time"
)

// SyncBackfillCustomersJob là job đồng bộ customers cập nhật cũ (backfill sync).
// Job này sẽ đồng bộ các customers cũ hơn oldestUpdatedAt từ FolkForm.
// Sử dụng order_by="updated_at" và bỏ qua customers với updated_at > until.
type SyncBackfillCustomersJob struct {
	*scheduler.BaseJob
}

// NewSyncBackfillCustomersJob tạo một instance mới của SyncBackfillCustomersJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncBackfillCustomersJob
func NewSyncBackfillCustomersJob(name, schedule string) *SyncBackfillCustomersJob {
	job := &SyncBackfillCustomersJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ customers cập nhật cũ (backfill sync).
// Phương thức này gọi DoSyncBackfillCustomers_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncBackfillCustomersJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
	log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
	log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Gọi hàm logic thực sự
	err := DoSyncBackfillCustomers_v2()
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

// DoSyncBackfillCustomers_v2 thực thi logic đồng bộ customers cập nhật cũ (backfill sync).
// Hàm này đồng bộ các customers cũ hơn oldestUpdatedAt.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncBackfillCustomers_v2() error {
	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Đồng bộ customers cập nhật cũ (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	log.Println("Bắt đầu đồng bộ customers cập nhật cũ (backfill sync)...")
	err := integrations.BridgeV2_SyncAllCustomers()
	if err != nil {
		log.Printf("❌ Lỗi khi đồng bộ customers cập nhật cũ: %v", err)
		return err
	}
	log.Println("Đồng bộ customers cập nhật cũ thành công")
	return nil
}
