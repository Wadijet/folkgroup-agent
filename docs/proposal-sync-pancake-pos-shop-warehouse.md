# Đề Xuất Phương Án: Đồng Bộ Shop và Warehouse từ Pancake POS

## 📋 Tổng Quan

Đề xuất tạo job đồng bộ Shop và Warehouse từ Pancake POS API về FolkForm, sử dụng token được lưu trong FolkForm với system: "Pancake POS".

## 🎯 Yêu Cầu

1. **Token Management**: Sử dụng token lưu ở FolkForm với `system: "Pancake POS"`
2. **Đồng bộ một chiều**: Chỉ sync từ Pancake POS → FolkForm (không cần 2 chiều vì dữ liệu ít)
3. **Thứ tự đồng bộ**: Trong cùng 1 job, sync Shop trước, Warehouse sau
4. **Chiến lược sync**: Sync toàn bộ từ mới đến cũ (full sync, không incremental)

## 🏗️ Kiến Trúc Giải Pháp

### 1. Cấu Trúc File

```
app/jobs/
├── sync_pancake_pos_shops_warehouses_job.go  # Job chính
└── helpers.go                                 # Helper functions (đã có)

app/integrations/
├── pancake_pos.go                             # Module mới: Pancake POS API integration
└── folkform.go                                # Thêm functions cho Shop & Warehouse (đã có sẵn endpoints)
```

### 2. Luồng Xử Lý

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Job Khởi Động                                            │
│    - SyncBaseAuth() (đăng nhập FolkForm nếu cần)            │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Lấy Token từ FolkForm                                    │
│    - FolkForm_GetAccessTokens(filter: {"system":"Pancake POS"}) │
│    - Lặp qua tất cả tokens (pagination)                     │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Đồng Bộ Shop (cho mỗi token)                            │
│    - PancakePos_GetShops(apiKey)                           │
│    - Lặp qua tất cả shops                                   │
│    - FolkForm_UpsertShop(shopData)                          │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. Đồng Bộ Warehouse (cho mỗi shop)                         │
│    - PancakePos_GetWarehouses(apiKey, shopId)               │
│    - Lặp qua tất cả warehouses                              │
│    - FolkForm_UpsertWarehouse(warehouseData)                │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 5. Hoàn Thành                                               │
│    - Log kết quả                                            │
│    - Return nil/error                                       │
└─────────────────────────────────────────────────────────────┘
```

## 📝 Chi Tiết Implementation

### 3.1. Module Pancake POS Integration (`app/integrations/pancake_pos.go`)

#### 3.1.1. Hàm Lấy Danh Sách Shop

```go
// PancakePos_GetShops lấy danh sách shop từ Pancake POS API
// apiKey: API key từ FolkForm (system: "Pancake POS")
// Trả về: map[string]interface{} chứa danh sách shops
func PancakePos_GetShops(apiKey string) (result map[string]interface{}, err error) {
    // Base URL: https://pos.pages.fm/api/v1
    // Endpoint: GET /shops?api_key={apiKey}
    // Response: { shops: [...] }
}
```

**Chi tiết:**
- Base URL: `https://pos.pages.fm/api/v1`
- Endpoint: `GET /shops?api_key={apiKey}`
- Response format: `{ "shops": [...] }` hoặc `[...]` (array trực tiếp)
- Xử lý pagination nếu có (theo tài liệu, API này có thể không có pagination vì dữ liệu ít)

#### 3.1.2. Hàm Lấy Danh Sách Warehouse

```go
// PancakePos_GetWarehouses lấy danh sách warehouse từ Pancake POS API
// apiKey: API key từ FolkForm
// shopId: ID của shop (integer)
// Trả về: map[string]interface{} chứa danh sách warehouses
func PancakePos_GetWarehouses(apiKey string, shopId int) (result map[string]interface{}, err error) {
    // Endpoint: GET /shops/{shopId}/warehouses?api_key={apiKey}
    // Response: { warehouses: [...] } hoặc [...]
}
```

**Chi tiết:**
- Endpoint: `GET /shops/{shopId}/warehouses?api_key={apiKey}`
- Response format: `{ "warehouses": [...] }` hoặc `[...]` (array trực tiếp)
- Xử lý pagination nếu có

### 3.2. Module FolkForm Integration (Thêm vào `app/integrations/folkform.go`)

#### 3.2.1. Hàm Upsert Shop

```go
// FolkForm_UpsertShop tạo/cập nhật shop trong FolkForm
// shopData: Dữ liệu shop từ Pancake POS API (map[string]interface{})
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertShop(shopData interface{}) (result map[string]interface{}, err error) {
    // Endpoint: POST /api/v1/pancake-pos/shop/upsert-one?filter={"shopId":123}
    // Body: { "panCakeData": shopData }
    // Filter: {"shopId": shopData.id}
}
```

**Chi tiết:**
- Endpoint: `POST /api/v1/pancake-pos/shop/upsert-one`
- Filter: `{"shopId": shopData.id}` (extract từ `panCakeData.id`)
- Body: `{ "panCakeData": shopData }`
- Backend tự động extract: `shopId`, `name`, `avatarUrl`, `pages`

#### 3.2.2. Hàm Upsert Warehouse

```go
// FolkForm_UpsertWarehouse tạo/cập nhật warehouse trong FolkForm
// warehouseData: Dữ liệu warehouse từ Pancake POS API (map[string]interface{})
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertWarehouse(warehouseData interface{}) (result map[string]interface{}, err error) {
    // Endpoint: POST /api/v1/pancake-pos/warehouse/upsert-one?filter={"warehouseId":"uuid"}
    // Body: { "panCakeData": warehouseData }
    // Filter: {"warehouseId": warehouseData.id}
}
```

**Chi tiết:**
- Endpoint: `POST /api/v1/pancake-pos/warehouse/upsert-one`
- Filter: `{"warehouseId": warehouseData.id}` (extract từ `panCakeData.id`, UUID string)
- Body: `{ "panCakeData": warehouseData }`
- Backend tự động extract: `warehouseId`, `shopId`, `name`, `phoneNumber`, `fullAddress`, `provinceId`, `districtId`, `communeId`

### 3.3. Job Implementation (`app/jobs/sync_pancake_pos_shops_warehouses_job.go`)

#### 3.3.1. Cấu Trúc Job

```go
// SyncPancakePosShopsWarehousesJob là job đồng bộ shop và warehouse từ Pancake POS
type SyncPancakePosShopsWarehousesJob struct {
    *scheduler.BaseJob
}

// NewSyncPancakePosShopsWarehousesJob tạo instance mới
func NewSyncPancakePosShopsWarehousesJob(name, schedule string) *SyncPancakePosShopsWarehousesJob {
    job := &SyncPancakePosShopsWarehousesJob{
        BaseJob: scheduler.NewBaseJob(name, schedule),
    }
    job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
    return job
}
```

#### 3.3.2. Logic Đồng Bộ

```go
// DoSyncPancakePosShopsWarehouses_v2 thực thi logic đồng bộ
func DoSyncPancakePosShopsWarehouses_v2() error {
    // 1. Xác thực
    SyncBaseAuth()
    
    // 2. Lấy danh sách tokens từ FolkForm
    filter := `{"system":"Pancake POS"}`
    page := 1
    limit := 50
    
    for {
        // Lấy tokens với pagination
        accessTokens, err := FolkForm_GetAccessTokens(page, limit, filter)
        // Parse response, lấy items
        
        if len(items) == 0 {
            break // Hết tokens
        }
        
        // 3. Với mỗi token
        for _, item := range items {
            apiKey := item["value"].(string)
            
            // 4. Đồng bộ Shops
            shops, err := PancakePos_GetShops(apiKey)
            for _, shop := range shops {
                FolkForm_UpsertShop(shop)
            }
            
            // 5. Đồng bộ Warehouses (cho mỗi shop)
            for _, shop := range shops {
                shopId := shop["id"].(int)
                warehouses, err := PancakePos_GetWarehouses(apiKey, shopId)
                for _, warehouse := range warehouses {
                    FolkForm_UpsertWarehouse(warehouse)
                }
            }
        }
        
        page++
    }
    
    return nil
}
```

## 🔧 Các Điểm Kỹ Thuật Quan Trọng

### 4.1. Xử Lý Token

- **Lấy token**: Sử dụng `FolkForm_GetAccessTokens` với filter `{"system":"Pancake POS"}`
- **Pagination**: Lặp qua tất cả pages để lấy hết tokens
- **Format token**: Token được lưu trong field `value` của mỗi item

### 4.2. Xử Lý Response từ Pancake POS API

- **Shop API**: Response có thể là `{ "shops": [...] }` hoặc `[...]` (array trực tiếp)
- **Warehouse API**: Response có thể là `{ "warehouses": [...] }` hoặc `[...]` (array trực tiếp)
- **Cần kiểm tra**: Cả 2 format để đảm bảo tương thích

### 4.3. Xử Lý Filter cho Upsert

- **Shop**: Filter dùng `shopId` (integer) từ `panCakeData.id`
- **Warehouse**: Filter dùng `warehouseId` (UUID string) từ `panCakeData.id`
- **Format filter**: JSON string trong query parameter: `?filter={"shopId":123}`

### 4.4. Error Handling

- **Retry logic**: Sử dụng pattern retry giống các hàm khác (max 5 lần)
- **Rate limiting**: Sử dụng rate limiter nếu cần (có thể không cần vì dữ liệu ít)
- **Logging**: Log đầy đủ các bước và lỗi

### 4.5. Data Mapping

**Shop:**
- `panCakeData.id` → `shopId` (int64)
- `panCakeData.name` → `name` (string)
- `panCakeData.avatar_url` → `avatarUrl` (string)
- `panCakeData.pages` → `pages` (array)

**Warehouse:**
- `panCakeData.id` → `warehouseId` (UUID string)
- `panCakeData.shop_id` → `shopId` (int64)
- `panCakeData.name` → `name` (string)
- `panCakeData.phone_number` → `phoneNumber` (string)
- `panCakeData.full_address` → `fullAddress` (string)
- `panCakeData.province_id` → `provinceId` (string)
- `panCakeData.district_id` → `districtId` (string)
- `panCakeData.commune_id` → `communeId` (string)

**Lưu ý**: Backend tự động extract, client chỉ cần gửi `panCakeData` đầy đủ.

## 📊 Ví Dụ Request/Response

### 5.1. Lấy Shops từ Pancake POS

**Request:**
```http
GET https://pos.pages.fm/api/v1/shops?api_key=YOUR_API_KEY
```

**Response:**
```json
[
  {
    "id": 123,
    "name": "Cửa hàng ABC",
    "avatar_url": "https://example.com/avatar.jpg",
    "pages": [
      {
        "id": "page_123",
        "name": "Page Name"
      }
    ]
  }
]
```

### 5.2. Upsert Shop vào FolkForm

**Request:**
```http
POST /api/v1/pancake-pos/shop/upsert-one?filter={"shopId":123}
Authorization: Bearer <token>
Content-Type: application/json

{
  "panCakeData": {
    "id": 123,
    "name": "Cửa hàng ABC",
    "avatar_url": "https://example.com/avatar.jpg",
    "pages": [...]
  }
}
```

### 5.3. Lấy Warehouses từ Pancake POS

**Request:**
```http
GET https://pos.pages.fm/api/v1/shops/123/warehouses?api_key=YOUR_API_KEY
```

**Response:**
```json
[
  {
    "id": "uuid-warehouse-1",
    "shop_id": 123,
    "name": "Kho hàng chính",
    "phone_number": "0912345678",
    "full_address": "123 Đường ABC, Quận XYZ",
    "province_id": "717",
    "district_id": "71705",
    "commune_id": "7170510"
  }
]
```

### 5.4. Upsert Warehouse vào FolkForm

**Request:**
```http
POST /api/v1/pancake-pos/warehouse/upsert-one?filter={"warehouseId":"uuid-warehouse-1"}
Authorization: Bearer <token>
Content-Type: application/json

{
  "panCakeData": {
    "id": "uuid-warehouse-1",
    "shop_id": 123,
    "name": "Kho hàng chính",
    "phone_number": "0912345678",
    "full_address": "123 Đường ABC, Quận XYZ",
    "province_id": "717",
    "district_id": "71705",
    "commune_id": "7170510"
  }
}
```

## 🚀 Kế Hoạch Triển Khai

### Phase 1: Tạo Module Pancake POS Integration
1. Tạo file `app/integrations/pancake_pos.go`
2. Implement `PancakePos_GetShops()`
3. Implement `PancakePos_GetWarehouses()`
4. Thêm retry logic và error handling

### Phase 2: Thêm Functions vào FolkForm Integration
1. Thêm `FolkForm_UpsertShop()` vào `app/integrations/folkform.go`
2. Thêm `FolkForm_UpsertWarehouse()` vào `app/integrations/folkform.go`
3. Sử dụng pattern tương tự `FolkForm_CreateCustomer()`

### Phase 3: Tạo Job
1. Tạo file `app/jobs/sync_pancake_pos_shops_warehouses_job.go`
2. Implement `DoSyncPancakePosShopsWarehouses_v2()`
3. Implement job struct và ExecuteInternal

### Phase 4: Đăng Ký Job vào Scheduler
1. Thêm job vào `main.go` hoặc `app/scheduler/scheduler.go`
2. Cấu hình schedule (ví dụ: chạy mỗi ngày lúc 2:00 AM)

## ⚠️ Lưu Ý

1. **Token Security**: Không log token ra console, chỉ log length
2. **Rate Limiting**: Pancake POS API có thể có rate limit, cần xử lý cẩn thận
3. **Error Recovery**: Nếu một shop/warehouse lỗi, tiếp tục với các item khác
4. **Data Validation**: Validate dữ liệu trước khi upsert (kiểm tra required fields)
5. **Logging**: Log đầy đủ để debug và monitor

## 📝 Checklist Implementation

- [ ] Tạo `app/integrations/pancake_pos.go`
- [ ] Implement `PancakePos_GetShops()`
- [ ] Implement `PancakePos_GetWarehouses()`
- [ ] Thêm `FolkForm_UpsertShop()` vào `folkform.go`
- [ ] Thêm `FolkForm_UpsertWarehouse()` vào `folkform.go`
- [ ] Tạo `app/jobs/sync_pancake_pos_shops_warehouses_job.go`
- [ ] Implement `DoSyncPancakePosShopsWarehouses_v2()`
- [ ] Đăng ký job vào scheduler
- [ ] Test với dữ liệu thực
- [ ] Document và code review

## 🎯 Kết Luận

Phương án này đáp ứng đầy đủ các yêu cầu:
- ✅ Sử dụng token từ FolkForm với system: "Pancake POS"
- ✅ 1 job duy nhất sync toàn bộ
- ✅ Sync Shop trước, Warehouse sau
- ✅ Sử dụng pattern tương tự các job hiện có
- ✅ Dễ maintain và mở rộng

