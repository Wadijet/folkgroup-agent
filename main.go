package main

import (
	"agent_pancake/app/jobs"
	"agent_pancake/app/scheduler"
	"agent_pancake/config"
	"agent_pancake/global"
	"log"
	"os"
)

// Các Scheduler
var Scheduler = scheduler.NewScheduler() // Scheduler chứa các jobs

func main() {
	// Cấu hình log để hiển thị đầy đủ thông tin và đảm bảo flush ngay lập tức
	// SetFlags: Ldate (ngày), Ltime (giờ), Lmicroseconds (micro giây), Lshortfile (file:line)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	// Đảm bảo log được ghi vào stdout để có thể xem được (log package mặc định dùng stderr)
	// Dùng stdout để log hiển thị tốt hơn trong console
	log.SetOutput(os.Stdout)

	// Đọc dữ liệu từ file .env
	global.GlobalConfig = config.NewConfig()
	log.Println("Đã đọc cấu hình từ file .env")

	// Khởi tạo scheduler
	s := scheduler.NewScheduler()

	// ========================================
	// JOB V2 - Logic mới với order_by=updated_at
	// ========================================

	// Job sync_incremental_conversations (V2) - Incremental sync
	// Chạy mỗi 1 phút: Chỉ sync conversations mới/cập nhật gần đây, dừng khi gặp lastConversationId
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */1 * * * *" = chạy mỗi 1 phút vào giây thứ 0
	syncIncrementalJob := jobs.NewSyncIncrementalConversationsJob("sync-incremental-conversations-job", "0 */1 * * * *")
	log.Printf("📋 Đã tạo job (V2): %s (Lịch: %s) - Incremental sync conversations", syncIncrementalJob.GetName(), syncIncrementalJob.GetSchedule())

	// Job sync_backfill_conversations (V2) - Backfill sync
	// Chạy mỗi 1 phút: Sync conversations cũ hơn oldestConversationId
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */1 * * * *" = chạy mỗi 1 phút vào giây 0
	syncBackfillJob := jobs.NewSyncBackfillConversationsJob("sync-backfill-conversations-job", "0 */1 * * * *")
	log.Printf("📋 Đã tạo job (V2): %s (Lịch: %s) - Backfill sync conversations", syncBackfillJob.GetName(), syncBackfillJob.GetSchedule())

	// ========================================
	// POSTS JOBS - Để test
	// ========================================

	// Job sync_incremental_posts - Incremental sync
	// Chạy mỗi 1 phút: Lấy posts mới hơn lastInsertedAt
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */1 * * * *" = chạy mỗi 1 phút vào giây thứ 0
	syncIncrementalPostsJob := jobs.NewSyncIncrementalPostsJob("sync-incremental-posts-job", "0 */1 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Incremental sync posts", syncIncrementalPostsJob.GetName(), syncIncrementalPostsJob.GetSchedule())

	// Job sync_backfill_posts - Backfill sync
	// Chạy mỗi 1 phút: Lấy posts cũ hơn oldestInsertedAt
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */1 * * * *" = chạy mỗi 1 phút vào giây 0
	syncBackfillPostsJob := jobs.NewSyncBackfillPostsJob("sync-backfill-posts-job", "0 */1 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Backfill sync posts", syncBackfillPostsJob.GetName(), syncBackfillPostsJob.GetSchedule())

	// ========================================
	// ĐĂNG KÝ JOB VÀO SCHEDULER
	// ========================================

	// Thêm job sync_incremental_conversations vào scheduler để chạy theo lịch (mỗi 1 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncIncrementalJob.GetName())
	err := s.AddJobObject(syncIncrementalJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncIncrementalJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncIncrementalJob.GetName())
	}

	// Thêm job sync_backfill_conversations vào scheduler để chạy theo lịch (mỗi 1 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncBackfillJob.GetName())
	err = s.AddJobObject(syncBackfillJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncBackfillJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncBackfillJob.GetName())
	}

	// Thêm job sync_incremental_posts vào scheduler để chạy theo lịch (mỗi 1 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncIncrementalPostsJob.GetName())
	err = s.AddJobObject(syncIncrementalPostsJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncIncrementalPostsJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncIncrementalPostsJob.GetName())
	}

	// Thêm job sync_backfill_posts vào scheduler để chạy theo lịch (mỗi 1 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncBackfillPostsJob.GetName())
	err = s.AddJobObject(syncBackfillPostsJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncBackfillPostsJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncBackfillPostsJob.GetName())
	}

	// ========================================
	// CUSTOMERS JOBS
	// ========================================

	// Job sync_incremental_customers - Incremental sync
	// Chạy mỗi 5 phút: Lấy customers đã cập nhật gần đây (từ lastUpdatedAt đến now)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */5 * * * *" = chạy mỗi 5 phút vào giây thứ 0
	syncIncrementalCustomersJob := jobs.NewSyncIncrementalCustomersJob("sync-incremental-customers-job", "0 */5 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Incremental sync customers", syncIncrementalCustomersJob.GetName(), syncIncrementalCustomersJob.GetSchedule())

	// Job sync_backfill_customers - Backfill sync
	// Chạy mỗi ngày lúc 2h sáng: Lấy customers cập nhật cũ (từ 0 đến oldestUpdatedAt)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 0 2 * * *" = chạy mỗi ngày lúc 2h sáng vào giây 0
	syncBackfillCustomersJob := jobs.NewSyncBackfillCustomersJob("sync-backfill-customers-job", "0 */5 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Backfill sync customers", syncBackfillCustomersJob.GetName(), syncBackfillCustomersJob.GetSchedule())

	// Thêm job sync_incremental_customers vào scheduler để chạy theo lịch (mỗi 5 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncIncrementalCustomersJob.GetName())
	err = s.AddJobObject(syncIncrementalCustomersJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncIncrementalCustomersJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncIncrementalCustomersJob.GetName())
	}

	// Thêm job sync_backfill_customers vào scheduler để chạy theo lịch (mỗi ngày lúc 2h sáng)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncBackfillCustomersJob.GetName())
	err = s.AddJobObject(syncBackfillCustomersJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncBackfillCustomersJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncBackfillCustomersJob.GetName())
	}

	// ========================================
	// PANCAKE POS JOBS - Shop & Warehouse Sync
	// ========================================

	// Job sync_pancake_pos_shops_warehouses - Đồng bộ shop và warehouse từ Pancake POS
	// Chạy mỗi 5 phút: Sync toàn bộ shops và warehouses từ Pancake POS về FolkForm
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */5 * * * *" = chạy mỗi 5 phút vào giây thứ 0
	syncPancakePosShopsWarehousesJob := jobs.NewSyncPancakePosShopsWarehousesJob("sync-pancake-pos-shops-warehouses-job", "0 */5 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Sync shops và warehouses từ Pancake POS", syncPancakePosShopsWarehousesJob.GetName(), syncPancakePosShopsWarehousesJob.GetSchedule())

	// Thêm job sync_pancake_pos_shops_warehouses vào scheduler để chạy theo lịch (mỗi 5 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncPancakePosShopsWarehousesJob.GetName())
	err = s.AddJobObject(syncPancakePosShopsWarehousesJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncPancakePosShopsWarehousesJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncPancakePosShopsWarehousesJob.GetName())
	}

	// ========================================
	// PANCAKE POS CUSTOMERS JOBS
	// ========================================

	// Job sync_incremental_pancake_pos_customers - Incremental sync
	// Chạy mỗi 5 phút: Lấy customers mới từ POS (từ lastUpdatedAt đến now)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */5 * * * *" = chạy mỗi 5 phút vào giây thứ 0
	syncIncrementalPancakePosCustomersJob := jobs.NewSyncIncrementalPancakePosCustomersJob("sync-incremental-pancake-pos-customers-job", "0 */5 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Incremental sync customers từ Pancake POS", syncIncrementalPancakePosCustomersJob.GetName(), syncIncrementalPancakePosCustomersJob.GetSchedule())

	// Job sync_backfill_pancake_pos_customers - Backfill sync
	// Chạy mỗi 5 phút: Lấy customers cũ từ POS (từ 0 đến oldestUpdatedAt)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */5 * * * *" = chạy mỗi 5 phút vào giây thứ 0
	syncBackfillPancakePosCustomersJob := jobs.NewSyncBackfillPancakePosCustomersJob("sync-backfill-pancake-pos-customers-job", "0 */5 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Backfill sync customers từ Pancake POS", syncBackfillPancakePosCustomersJob.GetName(), syncBackfillPancakePosCustomersJob.GetSchedule())

	// Thêm job sync_incremental_pancake_pos_customers vào scheduler để chạy theo lịch (mỗi 5 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncIncrementalPancakePosCustomersJob.GetName())
	err = s.AddJobObject(syncIncrementalPancakePosCustomersJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncIncrementalPancakePosCustomersJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncIncrementalPancakePosCustomersJob.GetName())
	}

	// Thêm job sync_backfill_pancake_pos_customers vào scheduler để chạy theo lịch (mỗi 5 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncBackfillPancakePosCustomersJob.GetName())
	err = s.AddJobObject(syncBackfillPancakePosCustomersJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncBackfillPancakePosCustomersJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncBackfillPancakePosCustomersJob.GetName())
	}

	// ========================================
	// PANCAKE POS PRODUCTS JOBS
	// ========================================

	// Job sync_pancake_pos_products - Đồng bộ products, variations và categories từ Pancake POS
	// Chạy mỗi 5 phút: Sync toàn bộ products, variations và categories từ Pancake POS về FolkForm
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */5 * * * *" = chạy mỗi 5 phút vào giây thứ 0
	syncPancakePosProductsJob := jobs.NewSyncPancakePosProductsJob("sync-pancake-pos-products-job", "0 */5 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Sync products, variations và categories từ Pancake POS", syncPancakePosProductsJob.GetName(), syncPancakePosProductsJob.GetSchedule())

	// Thêm job sync_pancake_pos_products vào scheduler để chạy theo lịch (mỗi 5 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncPancakePosProductsJob.GetName())
	err = s.AddJobObject(syncPancakePosProductsJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncPancakePosProductsJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncPancakePosProductsJob.GetName())
	}

	// ========================================
	// PANCAKE POS ORDERS JOBS
	// ========================================

	// Job sync_incremental_pancake_pos_orders - Incremental sync
	// Chạy mỗi 5 phút: Lấy orders mới từ POS (từ lastUpdatedAt đến now)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */5 * * * *" = chạy mỗi 5 phút vào giây thứ 0
	syncIncrementalPancakePosOrdersJob := jobs.NewSyncIncrementalPancakePosOrdersJob("sync-incremental-pancake-pos-orders-job", "0 */5 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Incremental sync orders từ Pancake POS", syncIncrementalPancakePosOrdersJob.GetName(), syncIncrementalPancakePosOrdersJob.GetSchedule())

	// Job sync_backfill_pancake_pos_orders - Backfill sync
	// Chạy mỗi 5 phút: Lấy orders cũ từ POS (từ 0 đến oldestUpdatedAt)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */5 * * * *" = chạy mỗi 5 phút vào giây thứ 0
	syncBackfillPancakePosOrdersJob := jobs.NewSyncBackfillPancakePosOrdersJob("sync-backfill-pancake-pos-orders-job", "0 */5 * * * *")
	log.Printf("📋 Đã tạo job: %s (Lịch: %s) - Backfill sync orders từ Pancake POS", syncBackfillPancakePosOrdersJob.GetName(), syncBackfillPancakePosOrdersJob.GetSchedule())

	// Thêm job sync_incremental_pancake_pos_orders vào scheduler để chạy theo lịch (mỗi 5 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncIncrementalPancakePosOrdersJob.GetName())
	err = s.AddJobObject(syncIncrementalPancakePosOrdersJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncIncrementalPancakePosOrdersJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncIncrementalPancakePosOrdersJob.GetName())
	}

	// Thêm job sync_backfill_pancake_pos_orders vào scheduler để chạy theo lịch (mỗi 5 phút)
	log.Printf("📝 Đang đăng ký job vào scheduler: %s", syncBackfillPancakePosOrdersJob.GetName())
	err = s.AddJobObject(syncBackfillPancakePosOrdersJob)
	if err != nil {
		log.Printf("❌ Lỗi khi thêm job %s: %v", syncBackfillPancakePosOrdersJob.GetName(), err)
		log.Fatalf("❌ Lỗi khi thêm job: %v", err)
	} else {
		log.Printf("✅ Đã đăng ký job thành công: %s", syncBackfillPancakePosOrdersJob.GetName())
	}

	// Khởi động scheduler
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("🚀 Đang khởi động Scheduler...")
	s.Start()
	log.Println("✅ Scheduler đã được khởi động thành công!")
	log.Printf("📊 Tổng số jobs đã đăng ký: %d", len(s.GetJobs()))
	log.Println("═══════════════════════════════════════════════════════════")

	// Giữ chương trình chạy
	// Trong thực tế, bạn có thể thêm các logic khác ở đây
	select {}

}

func main_() {

	// Cấu hình log để hiển thị đầy đủ thông tin và đảm bảo flush ngay lập tức
	// SetFlags: Ldate (ngày), Ltime (giờ), Lmicroseconds (micro giây), Lshortfile (file:line)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	// Đảm bảo log được ghi vào stdout để có thể xem được (log package mặc định dùng stderr)
	// Dùng stdout để log hiển thị tốt hơn trong console
	log.SetOutput(os.Stdout)

	// Đọc dữ liệu từ file .env
	global.GlobalConfig = config.NewConfig()
	log.Println("Đã đọc cấu hình từ file .env")

	//jobs.DoSyncBackfillConversations_v2()
	//jobs.DoSyncIncrementalConversations_v2()
	//jobs.DoSyncIncrementalPosts_v2()
	//jobs.DoSyncBackfillPosts_v2()
	//jobs.DoSyncBackfillCustomers_v2()
	//jobs.DoSyncIncrementalCustomers_v2()
	//jobs.DoSyncPancakePosShopsWarehouses_v2()
	//jobs.DoSyncIncrementalPancakePosCustomers_v2()
	//jobs.DoSyncBackfillPancakePosCustomers_v2()
	//jobs.DoSyncPancakePosProducts_v2()
	//jobs.DoSyncIncrementalPancakePosOrders_v2()
	//jobs.DoSyncBackfillPancakePosOrders_v2()
}
