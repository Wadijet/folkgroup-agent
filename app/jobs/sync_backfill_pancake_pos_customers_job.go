/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncBackfillPancakePosCustomersJob - job đồng bộ customers cũ từ Pancake POS (backfill sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"log"
	"time"
)

// SyncBackfillPancakePosCustomersJob là job đồng bộ customers cũ từ Pancake POS (backfill sync).
// Job này sẽ đồng bộ các customers có updated_at từ 0 đến oldestUpdatedAt từ POS.
// Sử dụng start_time_updated_at và end_time_updated_at để filter theo thời gian.
type SyncBackfillPancakePosCustomersJob struct {
	*scheduler.BaseJob
}

// NewSyncBackfillPancakePosCustomersJob tạo một instance mới của SyncBackfillPancakePosCustomersJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncBackfillPancakePosCustomersJob
func NewSyncBackfillPancakePosCustomersJob(name, schedule string) *SyncBackfillPancakePosCustomersJob {
	job := &SyncBackfillPancakePosCustomersJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ customers cũ từ Pancake POS (backfill sync).
// Phương thức này gọi DoSyncBackfillPancakePosCustomers_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncBackfillPancakePosCustomersJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
	log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
	log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Gọi hàm logic thực sự
	err := DoSyncBackfillPancakePosCustomers_v2()
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

// DoSyncBackfillPancakePosCustomers_v2 thực thi logic đồng bộ customers cũ từ Pancake POS (backfill sync).
// Hàm này đồng bộ các customers có updated_at từ 0 đến oldestUpdatedAt.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncBackfillPancakePosCustomers_v2() error {
	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Đồng bộ customers cũ từ POS (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	log.Println("Bắt đầu đồng bộ customers cũ từ Pancake POS (backfill sync)...")
	err := integrations.BridgeV2_SyncAllCustomersFromPos()
	if err != nil {
		log.Printf("❌ Lỗi khi đồng bộ customers cũ từ Pancake POS: %v", err)
		return err
	}
	log.Println("Đồng bộ customers cũ từ Pancake POS thành công")
	return nil
}
