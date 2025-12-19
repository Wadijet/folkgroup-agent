# Đề Xuất Phương Án Đồng Bộ Order từ Pancake POS

**Mục đích:** Đồng bộ Order từ Pancake POS về FolkForm sử dụng 2 jobs (Incremental + Backfill), tương tự như Customer sync

**Ngày tạo:** 2025-01-XX

---

## 🎯 Nguyên Lý

### 2 Jobs Sync (Giống Customer)

1. **Incremental Sync** - Chạy mỗi 5 phút
   - Sync orders đã cập nhật gần đây (từ `lastUpdatedAt` đến `now`)
   - Order by: `updated_at` (giảm dần - mới nhất trước)

2. **Backfill Sync** - Chạy mỗi ngày lúc 2h sáng
   - Sync orders cập nhật cũ (từ `0` đến `oldestUpdatedAt`)
   - Order by: `updated_at` (giảm dần)

**Lý do dùng `updated_at`:** Đảm bảo sync cả orders mới và orders đã được cập nhật thông tin (status, shipping, etc.)

---

## 📋 Pancake POS API

### Endpoint
`GET /shops/{SHOP_ID}/orders`

### Parameters
- `api_key` (required): API key từ FolkForm (system: "Pancake POS")
- `page_size` (optional, default: 30, max: 100): Số lượng orders mỗi trang
- `page_number` (optional, default: 1): Số trang
- `filter_status[]` (optional): Lọc theo trạng thái đơn hàng (array of integers)
- `include_removed` (optional): Bao gồm đơn đã xóa (0 hoặc 1)
- `updateStatus` (optional): Sắp xếp theo thời gian (`inserted_at`, `updated_at`, `paid_at`, etc.)
- `search` (optional): Tìm kiếm theo số điện thoại, tên khách hàng, ghi chú

**Lưu ý:** Pancake POS API không có tham số `since`/`until` như Pancake API, nên cần:
- Sử dụng `updateStatus=updated_at` để sắp xếp
- Lọc theo `updated_at` ở phía client sau khi nhận data
- Hoặc sử dụng pagination và dừng khi gặp order cũ hơn mốc thời gian

### Response
```json
{
  "data": [
    {
      "id": 123,
      "system_id": 1,
      "shop_id": 456,
      "inserted_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T12:00:00Z",
      "paid_at": "2024-01-01T10:00:00Z",
      "status": 1,
      "status_name": "Đã xác nhận",
      "bill_full_name": "Nguyễn Văn A",
      "bill_phone_number": "0999999999",
      "bill_email": "email@example.com",
      "page_id": "104438181227821",
      "post_id": "185187094667903_477083092110915",
      "shipping_fee": 10000,
      "total_discount": 50000,
      "note": "Ghi chú đơn hàng",
      "warehouse_id": "uuid-warehouse",
      "warehouse_info": {...},
      "customer": {...},
      "order_items": [...],
      "shipping_address": {...}
    }
  ],
  "pagination": {
    "page_number": 1,
    "page_size": 30,
    "total": 500
  }
}
```

---

## 🏗️ Implementation

### 1. Pancake POS API Integration

**File:** `app/integrations/pancake_pos.go`

```go
// PancakePos_GetOrders lấy danh sách orders từ Pancake POS API
// apiKey: API key từ FolkForm (system: "Pancake POS")
// shopId: ID của shop
// pageNumber: Số trang (bắt đầu từ 1)
// pageSize: Kích thước trang (tối đa 100)
// updateStatus: Sắp xếp theo thời gian ("inserted_at", "updated_at", "paid_at", etc.)
// Trả về: map[string]interface{} chứa orders và pagination
func PancakePos_GetOrders(apiKey string, shopId int, pageNumber int, pageSize int, updateStatus string) (result map[string]interface{}, err error)
```

**Đặc điểm:**
- Retry logic (5 lần)
- Adaptive rate limiter
- Parse response với pagination
- Xử lý lỗi và log chi tiết

### 2. FolkForm API Integration

**File:** `app/integrations/folkform.go`

#### `FolkForm_CreatePcPosOrder`
- Upsert với filter: `{"orderId": orderId, "shopId": shopId}`
- Endpoint: `/api/v1/pancake-pos/order/upsert-one`
- Permission: `PcPosOrder.Update`

#### `FolkForm_GetLastOrderUpdatedAt`
- Query: `filter={"shopId": shopId}`, `options={"sort":{"posUpdatedAt":-1},"limit":1}`
- Trả về: `posUpdatedAt` (Unix timestamp giây)
- Field: `posUpdatedAt` được extract từ `posData.updated_at`

#### `FolkForm_GetOldestOrderUpdatedAt`
- Query: `filter={"shopId": shopId}`, `options={"sort":{"posUpdatedAt":1},"limit":1}`
- Trả về: `posUpdatedAt` (Unix timestamp giây)

**Lưu ý:** Cần lấy theo `shopId` vì mỗi shop có danh sách orders riêng.

### 3. Bridge Logic

**File:** `app/integrations/bridge_v2.go`

#### Incremental Sync

```go
func BridgeV2_SyncNewOrders() error
func bridgeV2_SyncNewOrdersOfShop(shopId int, apiKey string) error
```

**Logic:**
1. Lấy tất cả shops từ FolkForm (hoặc từ Pancake POS)
2. Với mỗi shop:
   a. Lấy `lastUpdatedAt` từ FolkForm (theo shopId)
   b. Tính `since = lastUpdatedAt`, `until = now` (nếu chưa có → sync 30 ngày)
   c. Pagination loop:
      - Gọi `PancakePos_GetOrders()` với `updateStatus="updated_at"`
      - Parse `updated_at` (ISO 8601 → Unix timestamp)
      - Nếu `updated_at < since` → **DỪNG**
      - Upsert vào FolkForm với filter `{"orderId": orderId, "shopId": shopId}`
      - `page_number++` nếu còn data

#### Backfill Sync

```go
func BridgeV2_SyncAllOrders() error
func bridgeV2_SyncAllOrdersOfShop(shopId int, apiKey string) error
```

**Logic:**
1. Lấy tất cả shops từ FolkForm
2. Với mỗi shop:
   a. Lấy `oldestUpdatedAt` từ FolkForm (theo shopId)
   b. Tính `since = 0`, `until = oldestUpdatedAt` (nếu chưa có → sync toàn bộ)
   c. Pagination loop:
      - Refresh `oldestUpdatedAt` sau mỗi 10 batches
      - Gọi `PancakePos_GetOrders()` với `updateStatus="updated_at"`
      - Parse `updated_at`
      - Nếu `updated_at > until` → **BỎ QUA** (tiếp tục pagination)
      - Nếu `updated_at <= until` → Upsert vào FolkForm
      - `page_number++` nếu còn data

### 4. Jobs

**Files:**
- `app/jobs/sync_incremental_pancake_pos_orders_job.go`
- `app/jobs/sync_backfill_pancake_pos_orders_job.go`

**Structure:**
```go
type SyncIncrementalPancakePosOrdersJob struct {
    *scheduler.BaseJob
}

func (j *SyncIncrementalPancakePosOrdersJob) ExecuteInternal(ctx context.Context) error {
    return DoSyncIncrementalPancakePosOrders_v2()
}

func DoSyncIncrementalPancakePosOrders_v2() error {
    return integrations.BridgeV2_SyncNewOrders()
}
```

### 5. Scheduler

**File:** `main.go`

```go
// Incremental sync - Mỗi 5 phút
syncIncrementalPancakePosOrdersJob := jobs.NewSyncIncrementalPancakePosOrdersJob(
    "SyncIncrementalPancakePosOrders",
    "0 */5 * * * *",
)
scheduler.AddJob(syncIncrementalPancakePosOrdersJob)

// Backfill sync - Mỗi ngày lúc 2h sáng
syncBackfillPancakePosOrdersJob := jobs.NewSyncBackfillPancakePosOrdersJob(
    "SyncBackfillPancakePosOrders",
    "0 0 2 * * *",
)
scheduler.AddJob(syncBackfillPancakePosOrdersJob)
```

---

## 🔑 Điểm Quan Trọng

### 1. Unique Constraint
- Order được xác định bởi `orderId + shopId` (unique per shop)
- Upsert filter: `{"orderId": orderId, "shopId": shopId}`

### 2. Time Format
- `updated_at` từ Pancake POS: ISO 8601 string (`"2024-01-01T12:00:00Z"`)
- Parse sang Unix timestamp (giây)
- Field trong FolkForm: `posUpdatedAt` (extract từ `posData.updated_at`)

### 3. Logic Dừng

**Incremental:**
- Dừng khi: `updated_at < since` (đã sync hết)

**Backfill:**
- Bỏ qua khi: `updated_at > until` (tiếp tục pagination)
- Dừng khi: `len(orders) < page_size` (hết data)

### 4. Helper Function

```go
func parseOrderUpdatedAt(updatedAtStr string) (int64, error) {
    layouts := []string{
        "2006-01-02T15:04:05.000000Z",
        "2006-01-02T15:04:05Z",
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

### 5. Lấy Shops

Có 2 cách:
1. **Từ Pancake POS API:** `PancakePos_GetShops()` - lấy tất cả shops
2. **Từ FolkForm:** Query shops đã sync (nếu có collection `PcPosShop`)

**Khuyến nghị:** Lấy từ Pancake POS API để đảm bảo sync tất cả shops.

### 6. API Key

- Lấy API key từ FolkForm: System "Pancake POS"
- Mỗi shop có thể dùng chung API key hoặc có API key riêng (tùy cấu hình)

---

## 📝 Checklist Implementation

### Backend FolkForm
- [x] Model `PcPosOrder` với struct tags extract (đã có sẵn)
- [x] Collection `pc_pos_orders` với indexes:
  - `{orderId: 1, shopId: 1}` (unique)
  - `{shopId: 1, posUpdatedAt: -1}`
  - `{shopId: 1, posUpdatedAt: 1}`
- [x] CRUD endpoints `/api/v1/pancake-pos/order/*` (đã có sẵn)
- [x] Endpoint `/api/v1/pancake-pos/order/upsert-one` (đã có sẵn)
- [x] Permissions: `PcPosOrder.Insert`, `PcPosOrder.Read`, `PcPosOrder.Update`, `PcPosOrder.Delete` (đã có sẵn)

### Agent
- [ ] `PancakePos_GetOrders()` trong `pancake_pos.go`
- [ ] `FolkForm_CreatePcPosOrder()` trong `folkform.go`
- [ ] `FolkForm_GetLastOrderUpdatedAt()` trong `folkform.go`
- [ ] `FolkForm_GetOldestOrderUpdatedAt()` trong `folkform.go`
- [ ] `BridgeV2_SyncNewOrders()` và `bridgeV2_SyncNewOrdersOfShop()` trong `bridge_v2.go`
- [ ] `BridgeV2_SyncAllOrders()` và `bridgeV2_SyncAllOrdersOfShop()` trong `bridge_v2.go`
- [ ] `parseOrderUpdatedAt()` helper trong `bridge_v2.go`
- [ ] `sync_incremental_pancake_pos_orders_job.go`
- [ ] `sync_backfill_pancake_pos_orders_job.go`
- [ ] Thêm jobs vào scheduler trong `main.go`

---

## ⚠️ Lưu Ý

1. **Shop ID trong Order Data:**
   - Pancake POS trả về `shop_id` (snake_case) trong order data
   - Cần đảm bảo `shopId` có trong `posData` khi upsert

2. **Rate Limiting:**
   - Sử dụng adaptive rate limiter trước mỗi API call
   - Gọi `rateLimiter.Wait()` trước `PancakePos_GetOrders()`
   - Có thể tạo rate limiter riêng cho Pancake POS hoặc dùng chung với Pancake

3. **Error Handling:**
   - Retry logic (5 lần) cho Pancake POS API
   - Tiếp tục với order/shop tiếp theo nếu có lỗi (không dừng toàn bộ job)

4. **Refresh Mốc (Backfill):**
   - Refresh `oldestUpdatedAt` sau mỗi 10 batches
   - Cập nhật `until` nếu có order cũ hơn

5. **Pagination:**
   - Pancake POS API trả về `pagination` object với `total`, `page_number`, `page_size`
   - Kiểm tra `len(orders) < page_size` để biết đã hết data

6. **Filter theo Updated At:**
   - Pancake POS API không hỗ trợ `since`/`until` trong query params
   - Cần filter ở phía client sau khi nhận data
   - Hoặc sử dụng logic dừng khi gặp order cũ hơn mốc thời gian

---

## 🔄 So Sánh Với Customer Sync

| Đặc điểm | Customer Sync | Order Sync |
|---------|---------------|------------|
| **Nguồn API** | Pancake API | Pancake POS API |
| **Endpoint** | `/pages/{page_id}/page_customers` | `/shops/{shop_id}/orders` |
| **Query Params** | `since`, `until`, `order_by` | `updateStatus`, `page_size`, `page_number` |
| **Unique Key** | `psid + pageId` | `orderId + shopId` |
| **Time Field** | `updated_at` | `updated_at` (trong posData) |
| **FolkForm Field** | `updatedAt` | `posUpdatedAt` |
| **Filter Scope** | Theo `pageId` | Theo `shopId` |
| **Pagination** | `page_number`, `page_size` | `page_number`, `page_size` |

**Khác biệt chính:**
- Order sync cần lấy danh sách shops trước
- Order sync không có `since`/`until` trong API, cần filter ở client
- Order sync dùng `posUpdatedAt` thay vì `updatedAt`

---

## 📊 Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Incremental Sync                          │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
              Lấy danh sách Shops từ Pancake POS
                          │
                          ▼
              Với mỗi Shop:
                          │
        ┌─────────────────┴─────────────────┐
        │                                   │
        ▼                                   ▼
Lấy lastUpdatedAt từ FolkForm      Lấy Orders từ Pancake POS
(theo shopId)                       (updateStatus=updated_at)
        │                                   │
        └─────────────────┬─────────────────┘
                          ▼
              Parse updated_at và so sánh
                          │
        ┌─────────────────┴─────────────────┐
        │                                   │
   updated_at >= since              updated_at < since
        │                                   │
        ▼                                   ▼
   Upsert vào FolkForm                  DỪNG
        │
        ▼
   Tiếp tục pagination
```

---

## 🎯 Kết Luận

Phương án sync Order từ Pancake POS tương tự như Customer sync:
- ✅ 2 jobs: Incremental (5 phút) + Backfill (hàng ngày)
- ✅ Sử dụng `updated_at` để đảm bảo sync cả orders mới và đã cập nhật
- ✅ Update 2 chiều: Pancake POS → FolkForm
- ✅ Unique constraint: `orderId + shopId`
- ✅ Filter theo `shopId` thay vì `pageId`

**Pattern nhất quán với Customer sync, đảm bảo tính nhất quán trong codebase.**
