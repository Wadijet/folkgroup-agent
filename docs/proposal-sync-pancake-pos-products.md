# Đề Xuất Phương Án Sync Products từ Pancake POS

## Tổng Quan

Tài liệu này đề xuất phương án tạo job sync toàn bộ các thông tin liên quan đến **Product** từ Pancake POS API về FolkForm backend, **theo pattern của sync Shop và Warehouse**.

Bao gồm:
- **Products** (Sản phẩm)
- **Variations** (Biến thể sản phẩm) - sync sau khi sync products
- **Categories** (Danh mục sản phẩm)

**Pattern:** Một job duy nhất sync toàn bộ (không có backfill/incremental riêng), tương tự `SyncPancakePosShopsWarehousesJob`.

**Lưu ý quan trọng:**
- ✅ Backend sử dụng `posData` (không phải `panCakeData`) cho tất cả Pancake POS API
- ✅ Schedule: `*/5 * * * *` (Mỗi 5 phút) - Chạy thường xuyên để đảm bảo dữ liệu luôn được cập nhật
- ✅ Endpoints: `/api/v1/pancake-pos/product/*`, `/api/v1/pancake-pos/variation/*`, `/api/v1/pancake-pos/category/*`

## Phân Tích API

### 1. Pancake POS API - Products

**Endpoint lấy danh sách products:**
```
GET /shops/{SHOP_ID}/products
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm theo tên, SKU
- `category_ids[]`: Lọc theo danh mục
- `tag_ids[]`: Lọc theo tags
- `is_hide`: Lọc sản phẩm ẩn/hiện (0 hoặc 1)

**Response format:**
- Có thể là array trực tiếp hoặc object có field `products` hoặc `data`
- Mỗi product có các field:
  - `id`: Product ID (number)
  - `shop_id`: Shop ID (number)
  - `name`: Tên sản phẩm
  - `category_ids`: Danh sách ID danh mục
  - `tags`: Danh sách ID tags
  - `is_hide`: Trạng thái ẩn/hiện
  - `note_product`: Ghi chú sản phẩm
  - `product_attributes`: Thuộc tính sản phẩm
  - `variations`: Danh sách biến thể (nếu có)

**Endpoint lấy variations:**
```
GET /shops/{SHOP_ID}/products/variations
```

**Query parameters:**
- `product_id`: Mã sản phẩm
- `warehouse_id`: Mã kho hàng
- `page_size`, `page_number`: Phân trang

**Endpoint lấy categories:**
```
GET /shops/{SHOP_ID}/categories
```

### 2. FolkForm Backend API - Products

**Endpoint upsert product:**
```
POST /api/v1/pancake-pos/product/upsert-one?filter={"productId":123,"shopId":456}
```

**Request body:**
```json
{
  "posData": {
    "id": 123,
    "shop_id": 456,
    "name": "Áo thun nam",
    "category_ids": [1, 2],
    "tags": [10, 20],
    "is_hide": false,
    "note_product": "Sản phẩm bán chạy",
    "product_attributes": [...]
  }
}
```

**Lưu ý:** Backend sử dụng `posData` (không phải `panCakeData`) cho Pancake POS API.

**Endpoint upsert variation:**
```
POST /api/v1/pancake-pos/variation/upsert-one?filter={"variationId":"uuid-here"}
```

**Request body:**
```json
{
  "posData": {
    "id": "uuid-here",
    "product_id": 123,
    "shop_id": 456,
    "sku": "SKU-001",
    "retail_price": 100000,
    "price_at_counter": 90000,
    "quantity": 100,
    "weight": 0.5,
    "fields": [...],
    "images": [...]
  }
}
```

**Endpoint upsert category:**
```
POST /api/v1/pancake-pos/category/upsert-one?filter={"categoryId":123,"shopId":456}
```

**Request body:**
```json
{
  "posData": {
    "id": 123,
    "shop_id": 456,
    "name": "Áo thun"
  }
}
```

## Kiến Trúc Sync

### 1. Pattern Sync (Tương Tự Shop và Warehouse)

Dựa trên pattern sync shop và warehouse, đề xuất tạo **1 job duy nhất** sync toàn bộ:

#### Sync Products Job (Tương Tự SyncPancakePosShopsWarehousesJob)
- **Mục đích**: Sync toàn bộ products, variations và categories từ Pancake POS
- **Logic**: 
  1. Lấy danh sách tokens từ FolkForm (system: "Pancake POS")
  2. Với mỗi token:
     a. Lấy danh sách shops
     b. Với mỗi shop:
        - Lấy danh sách products (pagination)
        - Upsert từng product vào FolkForm
        - Với mỗi product, lấy danh sách variations (nếu cần)
        - Upsert từng variation vào FolkForm
     c. Lấy danh sách categories cho shop
     d. Upsert từng category vào FolkForm
- **Schedule**: Chạy định kỳ (ví dụ: `0 2 * * *` - 2:00 AM mỗi ngày)

**Đặc điểm:**
- ✅ Không filter theo thời gian, sync toàn bộ mỗi lần
- ✅ Logic sync nằm trực tiếp trong job (không tách ra bridge_v2)
- ✅ Xử lý lỗi: tiếp tục với shop/token tiếp theo nếu lỗi
- ✅ Rate limiting và retry logic trong các hàm Pancake POS API

### 2. Cấu Trúc Files

```
app/integrations/
  ├── pancake_pos.go          # Thêm hàm:
  │   ├── PancakePos_GetProducts()
  │   ├── PancakePos_GetVariations()
  │   └── PancakePos_GetCategories()
  └── folkform.go             # Thêm hàm:
      ├── FolkForm_UpsertProductFromPos()
      ├── FolkForm_UpsertVariationFromPos()
      └── FolkForm_UpsertCategoryFromPos()

app/jobs/
  └── sync_pancake_pos_products_job.go      # Job sync products, variations, categories
      # (Tương tự sync_pancake_pos_shops_warehouses_job.go)
```

## Chi Tiết Implementation

### 1. Hàm Lấy Products từ Pancake POS

**File:** `app/integrations/pancake_pos.go`

```go
// PancakePos_GetProducts lấy danh sách products từ Pancake POS API
// apiKey: API key từ FolkForm (system: "Pancake POS")
// shopId: ID của shop (integer)
// pageNumber: Số trang (mặc định: 1)
// pageSize: Số lượng items mỗi trang (mặc định: 30)
// Trả về: []interface{} chứa danh sách products
func PancakePos_GetProducts(apiKey string, shopId int, pageNumber int, pageSize int) (products []interface{}, err error)
```

**Đặc điểm:**
- Sử dụng rate limiter (tương tự `PancakePos_GetCustomers`)
- Retry logic (tối đa 5 lần)
- Parse response linh hoạt (array hoặc object có field `products`/`data`)
- Ghi nhận success/failure để điều chỉnh rate limiter

### 2. Hàm Upsert Product vào FolkForm

**File:** `app/integrations/folkform.go`

```go
// FolkForm_UpsertProductFromPos tạo/cập nhật product trong FolkForm
// Filter: {"productId": 123, "shopId": 456}
// Data: {posData: productData}
func FolkForm_UpsertProductFromPos(productData interface{}) (result map[string]interface{}, err error)
```

**Đặc điểm:**
- Tự động extract `productId` và `shopId` từ `productData.id` và `productData.shop_id`
- Tạo filter JSON: `{"productId": 123, "shopId": 456}`
- Gửi `posData` đầy đủ (không phải `panCakeData`), backend tự extract các field
- Endpoint: `/api/v1/pancake-pos/product/upsert-one`

### 3. Hàm Lấy Variations từ Pancake POS

**File:** `app/integrations/pancake_pos.go`

```go
// PancakePos_GetVariations lấy danh sách variations từ Pancake POS API
// apiKey: API key từ FolkForm
// shopId: ID của shop (integer)
// productId: ID của product (integer, 0 nếu lấy tất cả)
// pageNumber: Số trang
// pageSize: Số lượng items mỗi trang
// Trả về: []interface{} chứa danh sách variations
func PancakePos_GetVariations(apiKey string, shopId int, productId int, pageNumber int, pageSize int) (variations []interface{}, err error)
```

### 4. Hàm Upsert Variation vào FolkForm

**File:** `app/integrations/folkform.go`

```go
// FolkForm_UpsertVariationFromPos tạo/cập nhật variation trong FolkForm
// Filter: {"variationId": "uuid-here"}
// Data: {posData: variationData}
func FolkForm_UpsertVariationFromPos(variationData interface{}) (result map[string]interface{}, err error)
```

**Đặc điểm:**
- Tự động extract `variationId` từ `variationData.id` (UUID string)
- Tạo filter JSON: `{"variationId": "uuid-here"}`
- Gửi `posData` đầy đủ, backend tự extract các field
- Endpoint: `/api/v1/pancake-pos/variation/upsert-one`

### 5. Hàm Lấy Categories từ Pancake POS

**File:** `app/integrations/pancake_pos.go`

```go
// PancakePos_GetCategories lấy danh sách categories từ Pancake POS API
// apiKey: API key từ FolkForm
// shopId: ID của shop (integer)
// Trả về: []interface{} chứa danh sách categories
func PancakePos_GetCategories(apiKey string, shopId int) (categories []interface{}, err error)
```

### 6. Hàm Upsert Category vào FolkForm

**File:** `app/integrations/folkform.go`

```go
// FolkForm_UpsertCategoryFromPos tạo/cập nhật category trong FolkForm
// Filter: {"categoryId": 123, "shopId": 456}
// Data: {posData: categoryData}
func FolkForm_UpsertCategoryFromPos(categoryData interface{}) (result map[string]interface{}, err error)
```

**Đặc điểm:**
- Tự động extract `categoryId` và `shopId` từ `categoryData.id` và `categoryData.shop_id`
- Tạo filter JSON: `{"categoryId": 123, "shopId": 456}`
- Gửi `posData` đầy đủ, backend tự extract các field
- Endpoint: `/api/v1/pancake-pos/category/upsert-one`

### 7. Logic Sync Products trong Job

**File:** `app/jobs/sync_pancake_pos_products_job.go`

Logic sync nằm trực tiếp trong job (tương tự `DoSyncPancakePosShopsWarehouses_v2`):

```go
func DoSyncPancakePosProducts_v2() error {
    SyncBaseAuth()
    
    // 1. Lấy danh sách tokens từ FolkForm (system: "Pancake POS")
    filter := `{"system":"Pancake POS"}`
    page := 1
    limit := 50
    
    for {
        // Lấy access tokens
        accessTokens, err := integrations.FolkForm_GetAccessTokens(page, limit, filter)
        // Parse response...
        
        // 2. Với mỗi token
        for _, item := range items {
            apiKey := itemMap["value"].(string)
            
            // 3. Lấy danh sách shops
            shops, err := integrations.PancakePos_GetShops(apiKey)
            
            // 4. Với mỗi shop
            for _, shop := range shops {
                shopId := extractShopId(shop)
                
                // 5. Sync Products (pagination)
                pageNumber := 1
                pageSize := 100
                for {
                    products, err := integrations.PancakePos_GetProducts(apiKey, shopId, pageNumber, pageSize)
                    if len(products) == 0 {
                        break
                    }
                    
                    // Upsert từng product
                    for _, product := range products {
                        _, err := integrations.FolkForm_UpsertProductFromPos(product)
                    }
                    
                    if len(products) < pageSize {
                        break
                    }
                    pageNumber++
                }
                
                // 6. Sync Variations cho mỗi product (nếu cần)
                // Hoặc lấy từ API riêng /products/variations
                
                // 7. Sync Categories cho shop
                categories, err := integrations.PancakePos_GetCategories(apiKey, shopId)
                for _, category := range categories {
                    _, err := integrations.FolkForm_UpsertCategoryFromPos(category)
                }
            }
        }
    }
}
```

**Lưu ý:** 
- Không filter theo thời gian, sync toàn bộ mỗi lần
- Backend tự xử lý duplicate thông qua filter trong upsert
- Variations có thể được sync cùng lúc với products hoặc riêng

### 8. Helper Functions trong FolkForm

**File:** `app/integrations/folkform.go`

**Lưu ý:** Không cần helper functions lấy `lastUpdatedAt` vì sync toàn bộ mỗi lần (không có incremental sync).

## Jobs

### Sync Pancake POS Products Job

**File:** `app/jobs/sync_pancake_pos_products_job.go`

**Cấu trúc:** Tương tự `SyncPancakePosShopsWarehousesJob`

```go
type SyncPancakePosProductsJob struct {
    *scheduler.BaseJob
}

func NewSyncPancakePosProductsJob(name, schedule string) *SyncPancakePosProductsJob {
    job := &SyncPancakePosProductsJob{
        BaseJob: scheduler.NewBaseJob(name, schedule),
    }
    job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
    return job
}

func (j *SyncPancakePosProductsJob) ExecuteInternal(ctx context.Context) error {
    startTime := time.Now()
    log.Printf("═══════════════════════════════════════════════════════════")
    log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
    log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
    log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
    log.Printf("═══════════════════════════════════════════════════════════")
    
    err := DoSyncPancakePosProducts_v2()
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

func DoSyncPancakePosProducts_v2() error {
    SyncBaseAuth()
    
    // Logic sync products, variations, categories
    // (Xem chi tiết ở phần 7)
}
```

**Schedule đề xuất:** `*/5 * * * *` (Mỗi 5 phút)

**Lưu ý:** 
- Một job duy nhất sync tất cả (products, variations, categories)
- Không có backfill/incremental riêng
- Sync toàn bộ mỗi lần chạy
- **Schedule: `*/5 * * * *` (Mỗi 5 phút)** - Chạy thường xuyên để đảm bảo dữ liệu luôn được cập nhật

## Đăng Ký Jobs trong Scheduler

**File:** `app/scheduler/scheduler.go` hoặc `main.go`

```go
// Đăng ký job sync products, variations, categories
scheduler.AddJob(NewSyncPancakePosProductsJob("sync_pancake_pos_products", "*/5 * * * *"))
```

**Lưu ý:** Chỉ cần 1 job duy nhất, sync tất cả products, variations và categories.

## Xử Lý Edge Cases

### 1. Products Không Có `updated_at`

- **Vấn đề:** Pancake POS API có thể không trả về `updated_at` cho products
- **Giải pháp:** 
  - ✅ Sync toàn bộ products mỗi lần (không filter thời gian)
  - ✅ Backend tự xử lý duplicate thông qua filter `{"productId": 123, "shopId": 456}`

### 2. Variations Nested trong Product

- **Vấn đề:** API có thể trả về variations trong product response
- **Giải pháp:**
  - Nếu có → extract và sync riêng variations
  - Nếu không → gọi API riêng để lấy variations

### 3. Rate Limiting

- Sử dụng `apputility.GetPancakeRateLimiter()` (chung với Pancake API)
- Hoặc tạo rate limiter riêng cho Pancake POS nếu cần

### 4. Error Handling

- Tiếp tục với shop/token tiếp theo nếu một shop/token lỗi
- Log đầy đủ để debug
- Không dừng toàn bộ job vì lỗi một phần

## Testing

### 1. Unit Tests

- Test các hàm parse response từ Pancake POS
- Test các hàm tạo filter cho upsert
- Test logic pagination

### 2. Integration Tests

- Test sync products với 1 shop
- Test sync variations với 1 product
- Test sync categories với 1 shop
- Verify data trong FolkForm sau khi sync

### 3. Manual Testing

- Chạy job sync backfill cho 1 shop
- Verify products được sync đúng
- Chạy job sync incremental
- Verify chỉ sync products mới/cập nhật

## Timeline Implementation

### Phase 1: Products (Ưu tiên cao)
1. ✅ Tạo hàm `PancakePos_GetProducts()` trong `pancake_pos.go`
2. ✅ Tạo hàm `FolkForm_UpsertProductFromPos()` trong `folkform.go`
3. ✅ Tạo job `sync_pancake_pos_products_job.go` với logic sync products
4. ✅ Đăng ký job trong scheduler
5. ✅ Test và fix bugs

### Phase 2: Variations (Ưu tiên trung bình)
1. ✅ Tạo hàm `PancakePos_GetVariations()` trong `pancake_pos.go`
2. ✅ Tạo hàm `FolkForm_UpsertVariationFromPos()` trong `folkform.go`
3. ✅ Thêm logic sync variations vào job `sync_pancake_pos_products_job.go`
4. ✅ Test

### Phase 3: Categories (Ưu tiên thấp)
1. ✅ Tạo hàm `PancakePos_GetCategories()` trong `pancake_pos.go`
2. ✅ Tạo hàm `FolkForm_UpsertCategoryFromPos()` trong `folkform.go`
3. ✅ Thêm logic sync categories vào job `sync_pancake_pos_products_job.go`
4. ✅ Test

## Kết Luận

Phương án này tuân theo **pattern của sync Shop và Warehouse**, đảm bảo:
- ✅ Tính nhất quán trong codebase (cùng pattern với shop/warehouse)
- ✅ Đơn giản hơn (1 job thay vì 2 job backfill/incremental)
- ✅ Dễ maintain và mở rộng
- ✅ Xử lý lỗi tốt (tiếp tục với shop/token tiếp theo)
- ✅ Rate limiting phù hợp (sử dụng `GetPancakeRateLimiter()`)
- ✅ Logging đầy đủ (tương tự shop/warehouse job)

**Điểm khác biệt so với sync customers:**
- ❌ Không có backfill/incremental riêng
- ✅ Sync toàn bộ mỗi lần chạy
- ✅ Logic sync nằm trực tiếp trong job (không tách ra bridge_v2)
- ✅ Backend tự xử lý duplicate thông qua filter

**Bước tiếp theo:** Review và implement Phase 1 (Products) trước.
