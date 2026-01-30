# Tổng Hợp Nguồn Log Trong Project

Tài liệu liệt kê toàn bộ file có sử dụng logging và loại log. **Toàn bộ log** đều đi qua **logrus** và **log filter**, theo **format chung** (xem [log-filter-guide.md](log-filter-guide.md)).

---

## 1. Log từ standard `log` package — đi qua StdLogBridge → logrus

Sau khi gọi `log.SetOutput(logger.NewStdLogBridge())` trong main, mọi **`log.Printf` / `log.Println`** được chuyển sang logrus với **logger_name = `stdlog`**, nên **có đi qua filter** và **dùng format chung**.

### 1.1. Standard `log` package (`log.Printf`, `log.Println`) — ~1384 dòng trong 17 file

| File | Số dòng (ước) | Ghi chú |
|------|----------------|---------|
| `app/integrations/folkform.go` | ~589 | [FolkForm], [Login], [ClaimWorkflowCommands], API... |
| `app/integrations/bridge_v2.go` | ~172 | Bridge API |
| `app/integrations/pancake_pos.go` | ~107 | Pancake POS API |
| `app/services/step_executor.go` | ~107 | [StepExecutor], AI step |
| `app/integrations/pancake.go` | ~97 | Pancake API |
| `app/integrations/bridge.go` | ~81 | Bridge |
| `app/services/command_handler.go` | ~38 | Command handler |
| `app/scheduler/scheduler.go` | ~23 | [Scheduler] |
| `app/services/config_manager.go` | ~22 | [ConfigManager] |
| `config/config.go` | ~41 | [Config] |
| `app/services/ai_client.go` | ~42 | AI client |
| `app/services/workflow_executor.go` | ~53 | Workflow executor |
| `app/scheduler/basejob.go` | ~2 | Base job |
| `app/utility/rate_limiter.go` | ~2 | Rate limiter |
| `app/integrations/localData.go` | ~1 | Local data |
| `app/jobs/logger_helper.go` | ~6 | Fallback khi logger lỗi |
| `utility/logger/logger.go` | ~1 | Init logger |

**Hành vi:** Output của `log` đi qua **StdLogBridge** → logrus với `logger_name=stdlog`. Filter có thể allow/deny theo `job` = `"stdlog"` (vì ExtractLogContext gán `JobName = logger_name`).

### 1.2. `fmt.Printf` — ~10 dòng trong 5 file

| File | Ghi chú |
|------|---------|
| `main.go` | [MAIN_TEST_AI] AgentId — chỉ in khi `LOG_VERBOSE=1` |
| `utility/logger/logger.go` | [Logger] warning khi không load được log-filter |
| `config/config.go` | In lỗi parse config |
| `utility/common.go` | In lỗi chung |
| `app/integrations/bridge.go` | In lỗi bridge |

**Lưu ý:** `fmt` luôn ra stdout, không bị tắt bởi log filter. Chỉ có 2 dòng trong `main.go` được bảo vệ bởi `LOG_VERBOSE=1`.

---

## 2. Log **CÓ** đi qua Log Filter (logrus)

Các log này dùng **logrus** (GetLogger, GetAppLogger, GetJobLoggerByName, AppLogger, JobLogger, .WithFields, .Info, .Debug, .Warn, .Error, .Fatal) và **bị filter** bởi `config/log-filter.json` (theo job_name, logger_name, agent, level, method).

### 2.1. Main & Scheduler

| File | Logger dùng | Ghi chú |
|------|--------------|---------|
| `main.go` | AppLogger | Khởi động, đăng ký job, scheduler, check-in |

### 2.2. Jobs (mỗi job có logger riêng = tên job)

| File | Logger name / job_name | Ghi chú |
|------|------------------------|---------|
| `app/jobs/workflow_commands_job.go` | workflow-commands-job | Job AI workflow |
| `app/jobs/checkin_job.go` | check-in-job | Check-in |
| `app/jobs/logger_helper.go` | (helper) | LogJobStart, LogJobEnd, LogJobError, GetJobLoggerByName |
| `app/jobs/helpers.go` | (theo job gọi) | Helper dùng logger của job |
| `app/jobs/sync_priority_conversations_job.go` | sync-priority-conversations-job | |
| `app/jobs/sync_warn_unreplied_conversations_job.go` | sync-warn-unreplied-conversations-job | |
| `app/jobs/sync_verify_conversations_job.go` | sync-verify-conversations-job | |
| `app/jobs/sync_full_recovery_conversations_job.go` | sync-full-recovery-conversations-job | |
| `app/jobs/sync_backfill_posts_job.go` | sync-backfill-posts-job | |
| `app/jobs/sync_incremental_posts_job.go` | sync-incremental-posts-job | |
| `app/jobs/sync_backfill_pancake_pos_orders_job.go` | sync-backfill-pancake-pos-orders-job | |
| `app/jobs/sync_incremental_pancake_pos_orders_job.go` | sync-incremental-pancake-pos-orders-job | |
| `app/jobs/sync_backfill_pancake_pos_customers_job.go` | sync-backfill-pancake-pos-customers-job | |
| `app/jobs/sync_incremental_pancake_pos_customers_job.go` | sync-incremental-pancake-pos-customers-job | |
| `app/jobs/sync_backfill_customers_job.go` | sync-backfill-customers-job | |
| `app/jobs/sync_incremental_customers_job.go` | sync-incremental-customers-job | |
| `app/jobs/sync_backfill_conversations_job.go` | sync-backfill-conversations-job | |
| `app/jobs/sync_incremental_conversations_job.go` | sync-incremental-conversations-job | |
| `app/jobs/sync_pancake_pos_shops_warehouses_job.go` | sync-pancake-pos-shops-warehouses-job | |
| `app/jobs/sync_pancake_pos_products_job.go` | sync-pancake-pos-products-job | |

### 2.3. Services & Integrations (logrus)

| File | Logger / context | Ghi chú |
|------|------------------|---------|
| `app/services/checkin_service.go` | (dùng logger được truyền / app) | Check-in service |
| `app/scheduler/scheduler.go` | (có thể dùng logger) | Scheduler |
| `app/scheduler/basejob.go` | (logger từ job) | Base job |
| `app/integrations/folkform.go` | (có chỗ dùng logrus) | FolkForm API |
| `app/integrations/pancake_pos.go` | (có chỗ dùng logrus) | Pancake POS |
| `app/integrations/pancake.go` | (có chỗ dùng logrus) | Pancake |
| `utility/logger/*.go` | Nội bộ logger | Init, filter, hook |

---

## 3. Tóm tắt

| Loại | Số file | Đi qua logrus? | Bị filter bởi log-filter.json? | Format chung? |
|------|---------|-----------------|----------------------------------|---------------|
| **log.Printf / log.Println** | 17 | **Có** (StdLogBridge → logger `stdlog`) | **Có** (theo job `stdlog`, agent, level, method) | **Có** |
| **fmt.Printf** | 5 | Một số (main dùng AppLogger khi LOG_VERBOSE=1) | Theo logger dùng | Theo logger |
| **logrus (AppLogger, GetLogger, JobLogger, .Info, .Debug...)** | 29 | Có | **Có** (theo job, agent, level, method) | **Có** |

---

## 4. Gợi ý khi chỉnh log-filter

- **Chỉ xem log job AI:**  
  `config/log-filter.json`: `enabled: true`, `default_action: "deny"`, rule allow `workflow-commands-job`.  
  → Chỉ còn logrus của job AI; standard `log` đã bị tắt.

- **Bật lại log đầy đủ:**  
  `enabled: false` hoặc `default_action: "allow"` (và không rule deny toàn bộ).

- **Xem thêm log Config/FolkForm/Firebase:**  
  Tắt filter hoặc đổi default_action; không dùng chế độ “chỉ workflow-commands-job”.

- **Xem log startup trong main:**  
  Set env `LOG_VERBOSE=1` để in 2 dòng [MAIN_TEST_AI] AgentId.
