/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncBackfillPancakePosOrdersJob - job đồng bộ orders cũ từ Pancake POS (backfill sync).
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"log"
	"time"
)

// SyncBackfillPancakePosOrdersJob là job đồng bộ orders cũ từ Pancake POS (backfill sync).
// Job này sẽ đồng bộ các orders cũ hơn oldestUpdatedAt từ POS.
// Sử dụng order_by="updated_at" và bỏ qua orders với updated_at > until.
type SyncBackfillPancakePosOrdersJob struct {
	*scheduler.BaseJob
}

// NewSyncBackfillPancakePosOrdersJob tạo một instance mới của SyncBackfillPancakePosOrdersJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncBackfillPancakePosOrdersJob
func NewSyncBackfillPancakePosOrdersJob(name, schedule string) *SyncBackfillPancakePosOrdersJob {
	job := &SyncBackfillPancakePosOrdersJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ orders cũ từ Pancake POS (backfill sync).
// Phương thức này gọi DoSyncBackfillPancakePosOrders_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncBackfillPancakePosOrdersJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
	log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
	log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Gọi hàm logic thực sự
	err := DoSyncBackfillPancakePosOrders_v2()
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

// DoSyncBackfillPancakePosOrders_v2 thực thi logic đồng bộ orders cũ từ Pancake POS (backfill sync).
// Hàm này đồng bộ các orders cũ hơn oldestUpdatedAt.
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncBackfillPancakePosOrders_v2() error {
	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Đồng bộ orders cũ từ POS (chỉ chạy 1 lần, không có vòng lặp)
	// Scheduler sẽ tự động gọi lại job theo lịch
	log.Println("Bắt đầu đồng bộ orders cũ từ Pancake POS (backfill sync)...")
	err := integrations.BridgeV2_SyncAllOrders()
	if err != nil {
		log.Printf("❌ Lỗi khi đồng bộ orders cũ từ Pancake POS: %v", err)
		return err
	}
	log.Println("Đồng bộ orders cũ từ Pancake POS thành công")
	return nil
}
