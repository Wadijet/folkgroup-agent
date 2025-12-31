package main

import (
	"agent_pancake/app/jobs"
	"agent_pancake/app/scheduler"
	"agent_pancake/config"
	"agent_pancake/global"
	"agent_pancake/utility/logger"
	"fmt"

	"github.com/sirupsen/logrus"
)

// Các Scheduler
var Scheduler = scheduler.NewScheduler() // Scheduler chứa các jobs

// AppLogger là logger chính của ứng dụng
var AppLogger *logrus.Logger

// registerJob đăng ký job vào scheduler với logging
func registerJob(s *scheduler.Scheduler, job scheduler.Job) error {
	jobName := job.GetName()
	AppLogger.WithField("job_name", jobName).Info("📝 Đang đăng ký job vào scheduler")

	err := s.AddJobObject(job)
	if err != nil {
		AppLogger.WithFields(logrus.Fields{
			"job_name": jobName,
			"error":    err.Error(),
		}).Error("❌ Lỗi khi thêm job")
		return err
	}

	AppLogger.WithField("job_name", jobName).Info("✅ Đã đăng ký job thành công")
	return nil
}

func main() {
	// Đọc dữ liệu từ file .env trước
	global.GlobalConfig = config.NewConfig()

	// Khởi tạo logger với cấu hình từ environment variables
	logCfg := config.LogConfig()
	if err := logger.InitLogger(logCfg); err != nil {
		panic(fmt.Sprintf("Không thể khởi tạo logger: %v", err))
	}

	// Lấy logger cho application
	AppLogger = logger.GetAppLogger()
	AppLogger.Info("Đã đọc cấu hình từ file .env")
	AppLogger.Info("Hệ thống logger đã được khởi tạo thành công")

	// Khởi tạo scheduler
	s := scheduler.NewScheduler()

	// ========================================
	// JOB V2 - Logic mới với order_by=updated_at
	// ========================================

	// Job sync_incremental_conversations (V2) - Incremental sync
	// Chạy mỗi 30 giây: Chỉ sync conversations mới/cập nhật gần đây, dừng khi gặp lastConversationId
	// Cron format: giây phút giờ ngày tháng thứ
	// "*/30 * * * * *" = chạy mỗi 30 giây
	syncIncrementalJob := jobs.NewSyncIncrementalConversationsJob("sync-incremental-conversations-job", "*/30 * * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncIncrementalJob.GetName(),
		"schedule": syncIncrementalJob.GetSchedule(),
		"type":     "incremental",
		"version":  "V2",
	}).Info("📋 Đã tạo job (V2): Incremental sync conversations")

	// Job sync_backfill_conversations (V2) - Backfill sync
	// Chạy mỗi 3 phút: Sync conversations cũ hơn oldestConversationId
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */3 * * * *" = chạy mỗi 3 phút vào giây 0
	syncBackfillJob := jobs.NewSyncBackfillConversationsJob("sync-backfill-conversations-job", "0 */3 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncBackfillJob.GetName(),
		"schedule": syncBackfillJob.GetSchedule(),
		"type":     "backfill",
		"version":  "V2",
	}).Info("📋 Đã tạo job (V2): Backfill sync conversations")

	// Job sync_verify_conversations (V2) - Verify sync
	// Chạy mỗi 30 giây: Verify conversations từ FolkForm với Pancake để đảm bảo đồng bộ 2 chiều
	// Cron format: giây phút giờ ngày tháng thứ
	// "*/30 * * * * *" = chạy mỗi 30 giây
	syncVerifyJob := jobs.NewSyncVerifyConversationsJob("sync-verify-conversations-job", "*/30 * * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncVerifyJob.GetName(),
		"schedule": syncVerifyJob.GetSchedule(),
		"type":     "verify",
		"version":  "V2",
	}).Info("📋 Đã tạo job (V2): Verify conversations từ FolkForm với Pancake")

	// Job sync_full_recovery_conversations - Full recovery sync
	// Chạy mỗi ngày lúc 2h sáng: Sync lại TOÀN BỘ conversations từ Pancake về FolkForm
	// Không dựa vào checkpoint, đảm bảo không bỏ sót conversations khi có lỗi
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 0 2 * * *" = chạy mỗi ngày lúc 2h sáng vào giây 0
	syncFullRecoveryJob := jobs.NewSyncFullRecoveryConversationsJob("sync-full-recovery-conversations-job", "0 0 2 * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncFullRecoveryJob.GetName(),
		"schedule": syncFullRecoveryJob.GetSchedule(),
		"type":     "full_recovery",
	}).Info("📋 Đã tạo job: Sync lại TOÀN BỘ conversations để đảm bảo không bỏ sót")

	// ========================================
	// POSTS JOBS - Để test
	// ========================================

	// Job sync_incremental_posts - Incremental sync
	// Chạy mỗi 5 phút: Lấy posts mới hơn lastInsertedAt
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */5 * * * *" = chạy mỗi 5 phút vào giây thứ 0
	syncIncrementalPostsJob := jobs.NewSyncIncrementalPostsJob("sync-incremental-posts-job", "0 */5 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncIncrementalPostsJob.GetName(),
		"schedule": syncIncrementalPostsJob.GetSchedule(),
		"type":     "incremental",
	}).Info("📋 Đã tạo job: Incremental sync posts")

	// Job sync_backfill_posts - Backfill sync
	// Chạy mỗi 10 phút: Lấy posts cũ hơn oldestInsertedAt
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */10 * * * *" = chạy mỗi 10 phút vào giây 0
	syncBackfillPostsJob := jobs.NewSyncBackfillPostsJob("sync-backfill-posts-job", "0 */10 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncBackfillPostsJob.GetName(),
		"schedule": syncBackfillPostsJob.GetSchedule(),
		"type":     "backfill",
	}).Info("📋 Đã tạo job: Backfill sync posts")

	// ========================================
	// ĐĂNG KÝ JOB VÀO SCHEDULER
	// ========================================

	// Thêm job sync_incremental_conversations vào scheduler để chạy theo lịch (mỗi 30 giây)
	if err := registerJob(s, syncIncrementalJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Thêm job sync_backfill_conversations vào scheduler để chạy theo lịch (mỗi 3 phút)
	if err := registerJob(s, syncBackfillJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Thêm job sync_verify_conversations vào scheduler để chạy theo lịch (mỗi 30 giây)
	if err := registerJob(s, syncVerifyJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Thêm job sync_full_recovery_conversations vào scheduler để chạy theo lịch (mỗi ngày lúc 2h sáng)
	if err := registerJob(s, syncFullRecoveryJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Thêm job sync_incremental_posts vào scheduler để chạy theo lịch (mỗi 5 phút)
	if err := registerJob(s, syncIncrementalPostsJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Thêm job sync_backfill_posts vào scheduler để chạy theo lịch (mỗi 10 phút)
	if err := registerJob(s, syncBackfillPostsJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// ========================================
	// CUSTOMERS JOBS
	// ========================================

	// Job sync_incremental_customers - Incremental sync
	// Chạy mỗi 10 phút: Lấy customers đã cập nhật gần đây (từ lastUpdatedAt đến now)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */10 * * * *" = chạy mỗi 10 phút vào giây thứ 0
	syncIncrementalCustomersJob := jobs.NewSyncIncrementalCustomersJob("sync-incremental-customers-job", "0 */10 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncIncrementalCustomersJob.GetName(),
		"schedule": syncIncrementalCustomersJob.GetSchedule(),
		"type":     "incremental",
	}).Info("📋 Đã tạo job: Incremental sync customers")

	// Job sync_backfill_customers - Backfill sync
	// Chạy mỗi ngày lúc 2h sáng: Lấy customers cập nhật cũ (từ 0 đến oldestUpdatedAt)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 0 2 * * *" = chạy mỗi ngày lúc 2h sáng vào giây 0
	syncBackfillCustomersJob := jobs.NewSyncBackfillCustomersJob("sync-backfill-customers-job", "0 0 2 * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncBackfillCustomersJob.GetName(),
		"schedule": syncBackfillCustomersJob.GetSchedule(),
		"type":     "backfill",
	}).Info("📋 Đã tạo job: Backfill sync customers")

	// Thêm job sync_incremental_customers vào scheduler để chạy theo lịch (mỗi 10 phút)
	if err := registerJob(s, syncIncrementalCustomersJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Thêm job sync_backfill_customers vào scheduler để chạy theo lịch (mỗi ngày lúc 2h sáng)
	if err := registerJob(s, syncBackfillCustomersJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// ========================================
	// PANCAKE POS JOBS - Shop & Warehouse Sync
	// ========================================

	// Job sync_pancake_pos_shops_warehouses - Đồng bộ shop và warehouse từ Pancake POS
	// Chạy mỗi 15 phút: Sync toàn bộ shops và warehouses từ Pancake POS về FolkForm
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */15 * * * *" = chạy mỗi 15 phút vào giây thứ 0
	syncPancakePosShopsWarehousesJob := jobs.NewSyncPancakePosShopsWarehousesJob("sync-pancake-pos-shops-warehouses-job", "0 */15 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncPancakePosShopsWarehousesJob.GetName(),
		"schedule": syncPancakePosShopsWarehousesJob.GetSchedule(),
		"type":     "sync_shops_warehouses",
	}).Info("📋 Đã tạo job: Sync shops và warehouses từ Pancake POS")

	// Thêm job sync_pancake_pos_shops_warehouses vào scheduler để chạy theo lịch (mỗi 15 phút)
	if err := registerJob(s, syncPancakePosShopsWarehousesJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// ========================================
	// PANCAKE POS CUSTOMERS JOBS
	// ========================================

	// Job sync_incremental_pancake_pos_customers - Incremental sync
	// Chạy mỗi 10 phút: Lấy customers mới từ POS (từ lastUpdatedAt đến now)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */10 * * * *" = chạy mỗi 10 phút vào giây thứ 0
	syncIncrementalPancakePosCustomersJob := jobs.NewSyncIncrementalPancakePosCustomersJob("sync-incremental-pancake-pos-customers-job", "0 */10 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncIncrementalPancakePosCustomersJob.GetName(),
		"schedule": syncIncrementalPancakePosCustomersJob.GetSchedule(),
		"type":     "incremental",
		"source":   "pancake_pos",
	}).Info("📋 Đã tạo job: Incremental sync customers từ Pancake POS")

	// Job sync_backfill_pancake_pos_customers - Backfill sync
	// Chạy mỗi 30 phút: Lấy customers cũ từ POS (từ 0 đến oldestUpdatedAt)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */30 * * * *" = chạy mỗi 30 phút vào giây thứ 0
	syncBackfillPancakePosCustomersJob := jobs.NewSyncBackfillPancakePosCustomersJob("sync-backfill-pancake-pos-customers-job", "0 */30 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncBackfillPancakePosCustomersJob.GetName(),
		"schedule": syncBackfillPancakePosCustomersJob.GetSchedule(),
		"type":     "backfill",
		"source":   "pancake_pos",
	}).Info("📋 Đã tạo job: Backfill sync customers từ Pancake POS")

	// Thêm job sync_incremental_pancake_pos_customers vào scheduler để chạy theo lịch (mỗi 10 phút)
	if err := registerJob(s, syncIncrementalPancakePosCustomersJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Thêm job sync_backfill_pancake_pos_customers vào scheduler để chạy theo lịch (mỗi 30 phút)
	if err := registerJob(s, syncBackfillPancakePosCustomersJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// ========================================
	// PANCAKE POS PRODUCTS JOBS
	// ========================================

	// Job sync_pancake_pos_products - Đồng bộ products, variations và categories từ Pancake POS
	// Chạy mỗi 15 phút: Sync toàn bộ products, variations và categories từ Pancake POS về FolkForm
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */15 * * * *" = chạy mỗi 15 phút vào giây thứ 0
	syncPancakePosProductsJob := jobs.NewSyncPancakePosProductsJob("sync-pancake-pos-products-job", "0 */15 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncPancakePosProductsJob.GetName(),
		"schedule": syncPancakePosProductsJob.GetSchedule(),
		"type":     "sync_products",
		"source":   "pancake_pos",
	}).Info("📋 Đã tạo job: Sync products, variations và categories từ Pancake POS")

	// Thêm job sync_pancake_pos_products vào scheduler để chạy theo lịch (mỗi 15 phút)
	if err := registerJob(s, syncPancakePosProductsJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// ========================================
	// PANCAKE POS ORDERS JOBS
	// ========================================

	// Job sync_incremental_pancake_pos_orders - Incremental sync
	// Chạy mỗi 10 phút: Lấy orders mới từ POS (từ lastUpdatedAt đến now)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */10 * * * *" = chạy mỗi 10 phút vào giây thứ 0
	syncIncrementalPancakePosOrdersJob := jobs.NewSyncIncrementalPancakePosOrdersJob("sync-incremental-pancake-pos-orders-job", "0 */10 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncIncrementalPancakePosOrdersJob.GetName(),
		"schedule": syncIncrementalPancakePosOrdersJob.GetSchedule(),
		"type":     "incremental",
		"source":   "pancake_pos",
	}).Info("📋 Đã tạo job: Incremental sync orders từ Pancake POS")

	// Job sync_backfill_pancake_pos_orders - Backfill sync
	// Chạy mỗi 30 phút: Lấy orders cũ từ POS (từ 0 đến oldestUpdatedAt)
	// Cron format: giây phút giờ ngày tháng thứ
	// "0 */30 * * * *" = chạy mỗi 30 phút vào giây thứ 0
	syncBackfillPancakePosOrdersJob := jobs.NewSyncBackfillPancakePosOrdersJob("sync-backfill-pancake-pos-orders-job", "0 */30 * * * *")
	AppLogger.WithFields(logrus.Fields{
		"job_name": syncBackfillPancakePosOrdersJob.GetName(),
		"schedule": syncBackfillPancakePosOrdersJob.GetSchedule(),
		"type":     "backfill",
		"source":   "pancake_pos",
	}).Info("📋 Đã tạo job: Backfill sync orders từ Pancake POS")

	// Thêm job sync_incremental_pancake_pos_orders vào scheduler để chạy theo lịch (mỗi 10 phút)
	if err := registerJob(s, syncIncrementalPancakePosOrdersJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Thêm job sync_backfill_pancake_pos_orders vào scheduler để chạy theo lịch (mỗi 30 phút)
	if err := registerJob(s, syncBackfillPancakePosOrdersJob); err != nil {
		AppLogger.WithError(err).Fatal("❌ Lỗi khi thêm job")
	}

	// Khởi động scheduler
	AppLogger.Info("═══════════════════════════════════════════════════════════")
	AppLogger.Info("🚀 Đang khởi động Scheduler...")
	s.Start()
	AppLogger.WithField("total_jobs", len(s.GetJobs())).Info("✅ Scheduler đã được khởi động thành công!")
	AppLogger.Info("═══════════════════════════════════════════════════════════")

	// Giữ chương trình chạy
	// Trong thực tế, bạn có thể thêm các logic khác ở đây
	select {}

}

func main_() {
	// Đọc dữ liệu từ file .env
	global.GlobalConfig = config.NewConfig()

	// Khởi tạo logger
	logCfg := config.LogConfig()
	if err := logger.InitLogger(logCfg); err != nil {
		panic(fmt.Sprintf("Không thể khởi tạo logger: %v", err))
	}
	AppLogger = logger.GetAppLogger()
	AppLogger.Info("Đã đọc cấu hình từ file .env")

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
