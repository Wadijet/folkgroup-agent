# Hướng Dẫn Đồng Bộ Customer

**Mục đích:** Đồng bộ Customer từ Pancake về FolkForm sử dụng 2 jobs (Incremental + Backfill)

---

## 🎯 Nguyên Lý

### 2 Jobs Sync

1. **Incremental Sync** - Chạy mỗi 5 phút
   - Sync customers đã cập nhật gần đây (từ `lastUpdatedAt` đến `now`)
   - Order by: `updated_at` (giảm dần)

2. **Backfill Sync** - Chạy mỗi ngày lúc 2h sáng
   - Sync customers cập nhật cũ (từ `0` đến `oldestUpdatedAt`)
   - Order by: `updated_at` (giảm dần)

**Lý do dùng `updated_at`:** Đảm bảo sync cả customers mới và customers đã được cập nhật thông tin.

---

## 📋 Pancake API

### Endpoint
`GET /pages/{page_id}/page_customers`

### Parameters
- `page_access_token` (required)
- `page_number` (required, min: 1)
- `page_size` (optional, max: 100)
- `since` (required, Unix timestamp giây)
- `until` (required, Unix timestamp giây)
- `order_by` (optional): `"updated_at"` (giảm dần - mới nhất trước)

### Response
```json
{
  "success": true,
  "total": 500,
  "customers": [
    {
      "psid": "string",
      "name": "string",
      "phone_numbers": ["string"],
      "updated_at": "2019-08-24T14:15:22Z",
      ...
    }
  ]
}
```

---

## 🏗️ Implementation

### 1. Pancake API Integration

**File:** `app/integrations/pancake.go`

```go
func Pancake_GetCustomers(page_id string, page_number int, page_size int, since int64, until int64, order_by string) (result map[string]interface{}, err error)
```

- Retry logic (5 lần)
- Adaptive rate limiter
- Auto refresh page_access_token

### 2. FolkForm API Integration

**File:** `app/integrations/folkform.go`

#### `FolkForm_CreateCustomer`
- Upsert với filter: `{"psid": psid, "pageId": pageId}`
- Endpoint: `/customer/upsert-one`

#### `FolkForm_GetLastCustomerUpdatedAt`
- Query: `filter={"pageId": pageId}`, `options={"sort":{"updatedAt":-1},"limit":1}`
- Trả về: `updatedAt` (Unix timestamp giây)

#### `FolkForm_GetOldestCustomerUpdatedAt`
- Query: `filter={"pageId": pageId}`, `options={"sort":{"updatedAt":1},"limit":1}`
- Trả về: `updatedAt` (Unix timestamp giây)

### 3. Bridge Logic

**File:** `app/integrations/bridge_v2.go`

#### Incremental Sync

```go
func BridgeV2_SyncNewCustomers() error
func bridgeV2_SyncNewCustomersOfPage(pageId string) error
```

**Logic:**
1. Lấy `lastUpdatedAt` từ FolkForm
2. Tính `since = lastUpdatedAt`, `until = now` (nếu chưa có → sync 30 ngày)
3. Pagination loop:
   - Gọi `Pancake_GetCustomers()` với `order_by="updated_at"`
   - Parse `updated_at` (ISO 8601 → Unix timestamp)
   - Nếu `updated_at < since` → **DỪNG**
   - Upsert vào FolkForm
   - `page_number++` nếu còn data

#### Backfill Sync

```go
func BridgeV2_SyncAllCustomers() error
func bridgeV2_SyncAllCustomersOfPage(pageId string) error
```

**Logic:**
1. Lấy `oldestUpdatedAt` từ FolkForm
2. Tính `since = 0`, `until = oldestUpdatedAt` (nếu chưa có → sync toàn bộ)
3. Pagination loop:
   - Refresh `oldestUpdatedAt` sau mỗi 10 batches
   - Gọi `Pancake_GetCustomers()` với `order_by="updated_at"`
   - Parse `updated_at`
   - Nếu `updated_at > until` → **BỎ QUA** (tiếp tục pagination)
   - Nếu `updated_at <= until` → Upsert vào FolkForm
   - `page_number++` nếu còn data

### 4. Jobs

**Files:**
- `app/jobs/sync_incremental_customers_job.go`
- `app/jobs/sync_backfill_customers_job.go`

**Structure:**
```go
type SyncIncrementalCustomersJob struct {
    *scheduler.BaseJob
}

func (j *SyncIncrementalCustomersJob) ExecuteInternal(ctx context.Context) error {
    return DoSyncIncrementalCustomers_v2()
}

func DoSyncIncrementalCustomers_v2() error {
    return integrations.BridgeV2_SyncNewCustomers()
}
```

### 5. Scheduler

**File:** `main.go`

```go
// Incremental sync - Mỗi 5 phút
syncIncrementalCustomersJob := jobs.NewSyncIncrementalCustomersJob(
    "SyncIncrementalCustomers",
    "0 */5 * * * *",
)
scheduler.AddJob(syncIncrementalCustomersJob)

// Backfill sync - Mỗi ngày lúc 2h sáng
syncBackfillCustomersJob := jobs.NewSyncBackfillCustomersJob(
    "SyncBackfillCustomers",
    "0 0 2 * * *",
)
scheduler.AddJob(syncBackfillCustomersJob)
```

---

## 🔑 Điểm Quan Trọng

### 1. Unique Constraint
- Customer được xác định bởi `psid + pageId` (unique per page)
- Upsert filter: `{"psid": psid, "pageId": pageId}`

### 2. Time Format
- `updated_at` từ Pancake: ISO 8601 string (`"2019-08-24T14:15:22.000000"`)
- Parse sang Unix timestamp (giây)

### 3. Logic Dừng

**Incremental:**
- Dừng khi: `updated_at < since` (đã sync hết)

**Backfill:**
- Bỏ qua khi: `updated_at > until` (tiếp tục pagination)
- Dừng khi: `len(customers) < page_size` (hết data)

### 4. Helper Function

```go
func parseCustomerUpdatedAt(updatedAtStr string) (int64, error) {
    layouts := []string{
        "2006-01-02T15:04:05.000000",
        "2006-01-02T15:04:05",
        time.RFC3339,
        time.RFC3339Nano,
    }
    
    for _, layout := range layouts {
        t, err := time.Parse(layout, updatedAtStr)
        if err == nil {
            return t.Unix(), nil
        }
    }
    
    return 0, errors.New("Không thể parse updated_at: " + updatedAtStr)
}
```

---

## 📝 Checklist Implementation

### Backend FolkForm
- [ ] Model `FbCustomer` với struct tags extract
- [ ] Collection `customers` với indexes:
  - `{psid: 1, pageId: 1}` (unique)
  - `{pageId: 1, updatedAt: -1}`
  - `{pageId: 1, updatedAt: 1}`
- [ ] CRUD endpoints `/api/v1/customer/*`
- [ ] Endpoint `/api/v1/customer/upsert-one`
- [ ] Permissions: `Customer.Insert`, `Customer.Read`, `Customer.Update`, `Customer.Delete`

### Agent
- [ ] `Pancake_GetCustomers()` trong `pancake.go`
- [ ] `FolkForm_CreateCustomer()` trong `folkform.go`
- [ ] `FolkForm_GetLastCustomerUpdatedAt()` trong `folkform.go`
- [ ] `FolkForm_GetOldestCustomerUpdatedAt()` trong `folkform.go`
- [ ] `BridgeV2_SyncNewCustomers()` và `bridgeV2_SyncNewCustomersOfPage()` trong `bridge_v2.go`
- [ ] `BridgeV2_SyncAllCustomers()` và `bridgeV2_SyncAllCustomersOfPage()` trong `bridge_v2.go`
- [ ] `parseCustomerUpdatedAt()` helper trong `bridge_v2.go`
- [ ] `sync_incremental_customers_job.go`
- [ ] `sync_backfill_customers_job.go`
- [ ] Thêm jobs vào scheduler trong `main.go`

---

## ⚠️ Lưu Ý

1. **Page ID trong Customer Data:**
   - Pancake trả về `page_id` (snake_case) trong customer data
   - Cần thêm vào `panCakeData` khi upsert

2. **Rate Limiting:**
   - Sử dụng adaptive rate limiter trước mỗi API call
   - Gọi `rateLimiter.Wait()` trước `Pancake_GetCustomers()`

3. **Error Handling:**
   - Retry logic (5 lần) cho Pancake API
   - Tiếp tục với customer/page tiếp theo nếu có lỗi (không dừng toàn bộ job)

4. **Refresh Mốc (Backfill):**
   - Refresh `oldestUpdatedAt` sau mỗi 10 batches
   - Cập nhật `until` nếu có customer cũ hơn

---

**Kết luận:** Pattern tương tự Posts sync, đảm bảo tính nhất quán trong codebase.
