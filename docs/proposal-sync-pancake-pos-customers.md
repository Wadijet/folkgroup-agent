# 📋 Đề Xuất Phương Án Đồng Bộ Customer Từ POS Về FolkForm

## 📌 Tổng Quan

Tài liệu này đề xuất phương án đồng bộ dữ liệu Customer từ **Pancake POS API** về **FolkForm Backend**, tương tự như cách đã đồng bộ Shop và Warehouse.

## 🎯 Mục Tiêu

1. **Đồng bộ customers từ POS về FolkForm** để quản lý thống nhất khách hàng từ nhiều nguồn
2. **Hỗ trợ incremental sync và backfill sync** giống như đồng bộ customers từ Pancake
3. **Tự động identify và merge** customers từ nhiều nguồn (POS, Pancake) thông qua endpoint `upsert-from-pos`

## 📚 Thông Tin API

### Pancake POS API - Customers

**Endpoint:**
```
GET /shops/{SHOP_ID}/customers
```

**Query Parameters:**
- `page_size`: Số lượng items mỗi trang (mặc định: 30) - **DÙNG CHO PAGINATION**
- `page_number`: Số trang (mặc định: 1) - **DÙNG CHO PAGINATION**
- `start_time_updated_at`: Thời gian bắt đầu (Unix timestamp, giây) - **DÙNG CHO INCREMENTAL/BACKFILL SYNC**
- `end_time_updated_at`: Thời gian kết thúc (Unix timestamp, giây) - **DÙNG CHO INCREMENTAL/BACKFILL SYNC**
- `search`: Tìm kiếm theo tên, số điện thoại, email - **KHÔNG CẦN cho sync**
- `customer_level_id`: Lọc theo cấp độ khách hàng - **KHÔNG CẦN cho sync**
- `tag_ids[]`: Lọc theo tags - **KHÔNG CẦN cho sync**

**Lưu ý quan trọng:**
- **Có thể làm 2 jobs** giống như conversation sync:
  - **Incremental sync**: Dùng `start_time_updated_at` = lastUpdatedAt, `end_time_updated_at` = now
  - **Backfill sync**: Dùng `start_time_updated_at` = 0, `end_time_updated_at` = oldestUpdatedAt
- **Pagination**: Dùng `page_size` và `page_number` để lấy tất cả customers trong khoảng thời gian

**Response Schema:**
```json
{
  "id": 1,
  "name": "Tên khách hàng",
  "phone_number": "0999999999",
  "email": "email@example.com",
  "customer_level_id": 1,
  "point": 1000,
  "total_order": 10,
  "total_spent": 1000000,
  "tags": [1, 2]
}
```

**Lưu ý:** Pancake POS API không hỗ trợ filter theo `updated_at` hoặc `created_at`, nên cần lấy tất cả customers và filter ở phía client.

### FolkForm API - Customer Upsert from POS

**Endpoint:**
```
POST /api/v1/customer/upsert-from-pos
```

**Request Body:**
```json
{
  "posData": {
    "id": "b0110315-b102-436b-8b3b-ed8d16740327",
    "name": "Tên khách hàng",
    "phone_number": "0999999999",
    "email": "email@example.com",
    "customer_level_id": "uuid",
    "point": 1000,
    "total_order": 10,
    "total_spent": 1000000,
    "tags": [1, 2]
  }
}
```

**Đặc điểm:**
- Tự động identify customer theo thứ tự ưu tiên: `posCustomerId` → `panCakeCustomerId` → `psid` → `fb_id` → `phoneNumbers` → `email`
- Tự động merge dữ liệu từ nhiều nguồn nếu customer đã tồn tại
- Extract các field: `posCustomerId`, `name`, `phoneNumbers`, `email`, `point`, `totalOrder`, `totalSpent`, `customerLevelId`, etc.

## 🏗️ Kiến Trúc Đồng Bộ

### Luồng Đồng Bộ Tổng Quan

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Xác Thực và Lấy Danh Sách                                │
│    - FolkForm_Login()                                        │
│    - FolkForm_GetAccessTokens(filter: {"system":"Pancake POS"}) │
│    - Lặp qua tất cả tokens (pagination)                     │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Lấy Danh Sách Shops (cho mỗi token)                     │
│    - PancakePos_GetShops(apiKey)                            │
│    - Lặp qua tất cả shops                                    │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Đồng Bộ Customers (cho mỗi shop)                        │
│    - PancakePos_GetCustomers(apiKey, shopId, page, pageSize) │
│    - Lặp qua tất cả customers (pagination)                  │
│    - FolkForm_UpsertCustomerFromPos(customerData)           │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. Hoàn Thành                                               │
```

## 🔧 Implementation

### 1. Hàm Lấy Customers Từ POS

**File:** `app/integrations/pancake_pos.go`

```go
// PancakePos_GetCustomers lấy danh sách customers từ Pancake POS API
// apiKey: API key từ FolkForm (system: "Pancake POS")
// shopId: ID của shop (integer)
// pageNumber: Số trang (mặc định: 1)
// pageSize: Số lượng items mỗi trang (mặc định: 30)
// startTimeUpdatedAt: Thời gian bắt đầu (Unix timestamp, giây) - 0 nếu không filter
// endTimeUpdatedAt: Thời gian kết thúc (Unix timestamp, giây) - 0 nếu không filter
// Trả về: []interface{} chứa danh sách customers
func PancakePos_GetCustomers(apiKey string, shopId int, pageNumber int, pageSize int, startTimeUpdatedAt int64, endTimeUpdatedAt int64) (customers []interface{}, err error) {
    log.Printf("[PancakePOS] Bắt đầu lấy danh sách customers từ Pancake POS - shopId: %d, page: %d, size: %d", shopId, pageNumber, pageSize)
    
    // Khởi tạo client
    client := httpclient.NewHttpClient("https://pos.pages.fm/api/v1", 60*time.Second)
    
    // Thiết lập params
    // Dùng page_size và page_number để pagination
    // Dùng start_time_updated_at và end_time_updated_at để filter theo thời gian
    params := map[string]string{
        "api_key":                apiKey,
        "page_number":            strconv.Itoa(pageNumber),
        "page_size":              strconv.Itoa(pageSize),
        "start_time_updated_at":  strconv.FormatInt(startTimeUpdatedAt, 10), // Unix timestamp (seconds)
        "end_time_updated_at":    strconv.FormatInt(endTimeUpdatedAt, 10),   // Unix timestamp (seconds)
    }
    
    // Retry logic (tương tự PancakePos_GetShops)
    // ... (implementation tương tự PancakePos_GetShops)
    
    endpoint := fmt.Sprintf("/shops/%d/customers", shopId)
    // Gọi API và parse response (tương tự PancakePos_GetShops)
    // Response có thể là array trực tiếp hoặc object có field "customers"
    
    return customersArray, nil
}
```

**Lưu ý:** 
- Response từ POS API có thể là array trực tiếp `[...]` hoặc object `{"customers": [...]}`
- Cần xử lý cả 2 format giống như `PancakePos_GetShops` và `PancakePos_GetWarehouses`

### 2. Hàm Lấy Mốc Thời Gian Từ FolkForm

**File:** `app/integrations/folkform.go`

```go
// FolkForm_GetLastPosCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của customer từ POS cập nhật gần nhất
// shopId: ID của shop (integer)
// Trả về: updatedAt (seconds), error
func FolkForm_GetLastPosCustomerUpdatedAt(shopId int) (updatedAt int64, err error) {
    log.Printf("[FolkForm] Lấy customer từ POS cập nhật gần nhất - shopId: %d", shopId)
    
    if err := checkApiToken(); err != nil {
        return 0, err
    }
    
    client := createAuthorizedClient(defaultTimeout)
    
    // Query: filter theo posCustomerId có giá trị và shopId, sort theo updatedAt DESC, limit 1
    // Filter: customers có posCustomerId (từ POS) và thuộc shop này
    // Có thể filter theo: {"posCustomerId":{"$exists":true},"posData.shop_id":shopId}
    // Hoặc nếu có field shopId riêng: {"posCustomerId":{"$exists":true},"shopId":shopId}
    params := map[string]string{
        "filter":  fmt.Sprintf(`{"posCustomerId":{"$exists":true},"posData.shop_id":%d}`, shopId),
        "options": `{"sort":{"updatedAt":-1},"limit":1}`, // Sort desc (mới nhất trước)
    }
    
    result, err := executeGetRequest(
        client,
        "/customer/find",
        params,
        "Lấy customer từ POS cập nhật gần nhất thành công",
    )
    
    if err != nil {
        return 0, err
    }
    
    // Parse response
    var items []interface{}
    if dataMap, ok := result["data"].(map[string]interface{}); ok {
        if itemsArray, ok := dataMap["items"].([]interface{}); ok {
            items = itemsArray
        }
    }
    
    if len(items) == 0 {
        log.Printf("[FolkForm] Không tìm thấy customer từ POS nào - shopId: %d", shopId)
        return 0, nil // Không có customer → trả về 0
    }
    
    // items[0] = customer cập nhật gần nhất (updatedAt lớn nhất)
    firstItem := items[0]
    if customer, ok := firstItem.(map[string]interface{}); ok {
        var updatedAtFloat float64 = 0
        if ua, ok := customer["updatedAt"].(float64); ok {
            updatedAtFloat = ua
        }
        // Convert từ milliseconds sang seconds
        updatedAtSeconds := int64(updatedAtFloat) / 1000
        log.Printf("[FolkForm] Tìm thấy customer từ POS cập nhật gần nhất - shopId: %d, updatedAt: %d (seconds)", shopId, updatedAtSeconds)
        return updatedAtSeconds, nil
    }
    
    return 0, nil
}

// FolkForm_GetOldestPosCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của customer từ POS cập nhật cũ nhất
// shopId: ID của shop (integer)
// Trả về: updatedAt (seconds), error
func FolkForm_GetOldestPosCustomerUpdatedAt(shopId int) (updatedAt int64, err error) {
    log.Printf("[FolkForm] Lấy customer từ POS cập nhật cũ nhất - shopId: %d", shopId)
    
    if err := checkApiToken(); err != nil {
        return 0, err
    }
    
    client := createAuthorizedClient(defaultTimeout)
    
    // Query: filter theo posCustomerId có giá trị và shopId, sort theo updatedAt ASC, limit 1
    params := map[string]string{
        "filter":  fmt.Sprintf(`{"posCustomerId":{"$exists":true},"posData.shop_id":%d}`, shopId),
        "options": `{"sort":{"updatedAt":1},"limit":1}`, // Sort asc (cũ nhất trước)
    }
    
    result, err := executeGetRequest(
        client,
        "/customer/find",
        params,
        "Lấy customer từ POS cập nhật cũ nhất thành công",
    )
    
    if err != nil {
        return 0, err
    }
    
    // Parse response
    var items []interface{}
    if dataMap, ok := result["data"].(map[string]interface{}); ok {
        if itemsArray, ok := dataMap["items"].([]interface{}); ok {
            items = itemsArray
        }
    }
    
    if len(items) == 0 {
        log.Printf("[FolkForm] Không tìm thấy customer từ POS nào - shopId: %d", shopId)
        return 0, nil // Không có customer → trả về 0
    }
    
    // items[0] = customer cập nhật cũ nhất (updatedAt nhỏ nhất)
    firstItem := items[0]
    if customer, ok := firstItem.(map[string]interface{}); ok {
        var updatedAtFloat float64 = 0
        if ua, ok := customer["updatedAt"].(float64); ok {
            updatedAtFloat = ua
        }
        // Convert từ milliseconds sang seconds
        updatedAtSeconds := int64(updatedAtFloat) / 1000
        log.Printf("[FolkForm] Tìm thấy customer từ POS cập nhật cũ nhất - shopId: %d, updatedAt: %d (seconds)", shopId, updatedAtSeconds)
        return updatedAtSeconds, nil
    }
    
    return 0, nil
}
```

**Lưu ý:**
- **Filter**: `{"posCustomerId":{"$exists":true},"posData.shop_id":shopId}` - Lấy customers có `posCustomerId` (từ POS) và thuộc shop này
- **Sort**: `updatedAt` desc/asc - Giống như customer từ Pancake
- **Field**: Lấy `updatedAt` (milliseconds) và convert sang seconds
- **Tương tự**: Giống `FolkForm_GetLastCustomerUpdatedAt` và `FolkForm_GetOldestCustomerUpdatedAt` nhưng filter theo `posCustomerId` và `shopId` thay vì `pageId`

### 3. Hàm Upsert Customer Từ POS Vào FolkForm

**File:** `app/integrations/folkform.go`

```go
// FolkForm_UpsertCustomerFromPos tạo/cập nhật customer từ POS vào FolkForm
// customerData: Dữ liệu customer từ Pancake POS API (map[string]interface{})
// Chỉ cần gửi đúng format: {posData: customerData}
// Server sẽ tự động extract, identify và merge customer
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertCustomerFromPos(customerData interface{}) (result map[string]interface{}, err error) {
    log.Printf("[FolkForm] Bắt đầu upsert customer từ POS")
    
    if err := checkApiToken(); err != nil {
        return nil, err
    }
    
    client := createAuthorizedClient(defaultTimeout)
    
    // Tạo data đúng DTO: {posData: customerData}
    // Server sẽ tự động:
    // - Extract các field: posCustomerId, name, phoneNumbers, email, point, etc.
    // - Identify customer theo thứ tự ưu tiên: posCustomerId → panCakeCustomerId → psid → phone → email
    // - Merge dữ liệu nếu customer đã tồn tại
    data := map[string]interface{}{
        "posData": customerData, // Gửi nguyên dữ liệu từ POS API
    }
    
    log.Printf("[FolkForm] Đang gửi request upsert customer từ POS đến FolkForm backend...")
    result, err = executePostRequest(client, "/customer/upsert-from-pos", data, nil, "Gửi customer từ POS thành công", "Gửi customer từ POS thất bại. Thử lại lần thứ", false)
    if err != nil {
        log.Printf("[FolkForm] LỖI khi upsert customer từ POS: %v", err)
    } else {
        log.Printf("[FolkForm] Upsert customer từ POS thành công")
    }
    return result, err
}
```

**Lưu ý quan trọng:**
- **Chỉ cần gửi đúng format**: `{posData: customerData}` - không cần transform hay extract gì
- **Server tự động xử lý**: Server sẽ tự động extract, identify và merge customer
- **Gửi nguyên dữ liệu**: Gửi nguyên dữ liệu từ POS API, không cần map field names

### 3. Hàm Đồng Bộ Customers Từ POS

**File:** `app/integrations/bridge_v2.go`

```go
// BridgeV2_SyncCustomersFromPos đồng bộ customers từ POS về FolkForm
// Lấy tất cả tokens, shops, và customers từ POS
func BridgeV2_SyncCustomersFromPos() error {
    log.Println("[BridgeV2] Bắt đầu đồng bộ customers từ POS về FolkForm")
    
    // 1. Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
    filter := `{"system":"Pancake POS"}`
    page := 1
    limit := 50
    
    for {
        // Lấy danh sách access token
        accessTokens, err := FolkForm_GetAccessTokens(page, limit, filter)
        if err != nil {
            logError("[BridgeV2] Lỗi khi lấy danh sách access token: %v", err)
            return errors.New("Lỗi khi lấy danh sách access token")
        }
        
        // Parse response
        items, itemCount, err := parseResponseData(accessTokens)
        if err != nil {
            logError("[BridgeV2] LỖI khi parse response: %v", err)
            return err
        }
        
        if itemCount == 0 || len(items) == 0 {
            log.Printf("[BridgeV2] Không còn tokens nào, dừng sync")
            break
        }
        
        log.Printf("[BridgeV2] Nhận được %d tokens (page=%d, limit=%d)", len(items), page, limit)
        
        // 2. Với mỗi token
        for _, item := range items {
            itemMap, ok := item.(map[string]interface{})
            if !ok {
                continue
            }
            
            apiKey, ok := itemMap["value"].(string)
            if !ok || apiKey == "" {
                logError("[BridgeV2] Token không có value, bỏ qua")
                continue
            }
            
            // 3. Lấy danh sách shops
            shops, err := PancakePos_GetShops(apiKey)
            if err != nil {
                logError("[BridgeV2] Lỗi khi lấy danh sách shops: %v", err)
                continue
            }
            
            // 4. Với mỗi shop
            for _, shop := range shops {
                shopMap, ok := shop.(map[string]interface{})
                if !ok {
                    continue
                }
                
                shopIdRaw, ok := shopMap["id"]
                if !ok {
                    continue
                }
                
                // Convert shopId sang int
                var shopId int
                switch v := shopIdRaw.(type) {
                case float64:
                    shopId = int(v)
                case int:
                    shopId = v
                case int64:
                    shopId = int(v)
                default:
                    logError("[BridgeV2] shopId không phải là số: %T", shopIdRaw)
                    continue
                }
                
                // 5. Đồng bộ customers cho shop này
                err = bridgeV2_SyncCustomersFromPosForShop(apiKey, shopId)
                if err != nil {
                    logError("[BridgeV2] Lỗi khi đồng bộ customers cho shop %d: %v", shopId, err)
                    // Tiếp tục với shop tiếp theo
                    continue
                }
            }
        }
        
        // Kiểm tra xem còn tokens không
        if len(items) < limit {
            break
        }
        
        page++
    }
    
    log.Println("[BridgeV2] ✅ Hoàn thành đồng bộ customers từ POS")
    return nil
}

// bridgeV2_SyncNewCustomersFromPosForShop đồng bộ customers mới từ POS cho một shop (incremental sync)
func bridgeV2_SyncNewCustomersFromPosForShop(apiKey string, shopId int) error {
    log.Printf("[BridgeV2] Bắt đầu đồng bộ customers mới từ POS cho shop %d (incremental sync)", shopId)
    
    // 1. Lấy mốc từ FolkForm
    // Filter: customers có posCustomerId (từ POS) và thuộc shop này
    // Sort theo updatedAt desc, limit 1 → lấy customer mới nhất
    lastUpdatedAt, err := FolkForm_GetLastPosCustomerUpdatedAt(shopId)
    if err != nil {
        logError("[BridgeV2] Lỗi khi lấy lastUpdatedAt cho shop %d: %v", shopId, err)
        return err
    }
    
    // 2. Tính khoảng thời gian sync
    var startTime, endTime int64
    if lastUpdatedAt == 0 {
        // Chưa có customers → sync 30 ngày gần nhất
        endTime = time.Now().Unix()
        startTime = endTime - (30 * 24 * 60 * 60) // 30 ngày trước
        log.Printf("[BridgeV2] Shop %d - Chưa có customers, sync 30 ngày gần nhất", shopId)
    } else {
        startTime = lastUpdatedAt
        endTime = time.Now().Unix()
        log.Printf("[BridgeV2] Shop %d - Sync customers từ %d đến %d", shopId, startTime, endTime)
    }
    
    // 3. Pagination loop
    pageNumber := 1
    pageSize := 100
    rateLimiter := apputility.GetPancakeRateLimiter()
    
    for {
        // Rate limiter
        rateLimiter.Wait()
        
        // Lấy customers từ POS với filter theo thời gian
        customers, err := PancakePos_GetCustomers(apiKey, shopId, pageNumber, pageSize, startTime, endTime)
        if err != nil {
            logError("[BridgeV2] Lỗi khi lấy customers cho shop %d: %v", shopId, err)
            break
        }
        
        if len(customers) == 0 {
            log.Printf("[BridgeV2] Shop %d - Không còn customers nào, dừng sync", shopId)
            break
        }
        
        log.Printf("[BridgeV2] Shop %d - Lấy được %d customers (page=%d)", shopId, len(customers), pageNumber)
        
        // Upsert từng customer vào FolkForm
        for _, customer := range customers {
            customerMap, ok := customer.(map[string]interface{})
            if !ok {
                continue
            }
            
            // Upsert customer từ POS
            _, err = FolkForm_UpsertCustomerFromPos(customerMap)
            if err != nil {
                logError("[BridgeV2] Lỗi khi upsert customer từ POS: %v", err)
                // Tiếp tục với customer tiếp theo
                continue
            }
        }
        
        // Kiểm tra điều kiện dừng
        if len(customers) < pageSize {
            log.Printf("[BridgeV2] Shop %d - Đã lấy hết customers (len=%d < page_size=%d)", shopId, len(customers), pageSize)
            break
        }
        
        // Tiếp tục với page tiếp theo
        pageNumber++
    }
    
    log.Printf("[BridgeV2] ✅ Hoàn thành đồng bộ customers mới từ POS cho shop %d", shopId)
    return nil
}

// bridgeV2_SyncAllCustomersFromPosForShop đồng bộ customers cũ từ POS cho một shop (backfill sync)
func bridgeV2_SyncAllCustomersFromPosForShop(apiKey string, shopId int) error {
    log.Printf("[BridgeV2] Bắt đầu đồng bộ customers cũ từ POS cho shop %d (backfill sync)", shopId)
    
    // 1. Lấy mốc từ FolkForm
    // Filter: customers có posCustomerId (từ POS) và thuộc shop này
    // Sort theo updatedAt asc, limit 1 → lấy customer cũ nhất
    oldestUpdatedAt, err := FolkForm_GetOldestPosCustomerUpdatedAt(shopId)
    if err != nil {
        logError("[BridgeV2] Lỗi khi lấy oldestUpdatedAt cho shop %d: %v", shopId, err)
        return err
    }
    
    // 2. Tính khoảng thời gian sync
    var startTime, endTime int64
    if oldestUpdatedAt == 0 {
        // Chưa có customers → sync toàn bộ
        startTime = 0
        endTime = time.Now().Unix()
        log.Printf("[BridgeV2] Shop %d - Chưa có customers, sync toàn bộ", shopId)
    } else {
        startTime = 0
        endTime = oldestUpdatedAt
        log.Printf("[BridgeV2] Shop %d - Sync customers cũ từ 0 đến %d", shopId, endTime)
    }
    
    // 3. Pagination loop với refresh mốc
    pageNumber := 1
    pageSize := 100
    batchCount := 0
    const REFRESH_OLDEST_AFTER_BATCHES = 10
    rateLimiter := apputility.GetPancakeRateLimiter()
    
    for {
        // Refresh oldestUpdatedAt sau mỗi N batches
        if batchCount > 0 && batchCount%REFRESH_OLDEST_AFTER_BATCHES == 0 {
            newOldest, _ := FolkForm_GetOldestPosCustomerUpdatedAt(shopId)
            if newOldest > 0 && newOldest < endTime {
                log.Printf("[BridgeV2] Shop %d - Cập nhật endTime: %d -> %d (có customer cũ hơn)", shopId, endTime, newOldest)
                endTime = newOldest
                oldestUpdatedAt = newOldest
            }
        }
        
        batchCount++
        
        // Rate limiter
        rateLimiter.Wait()
        
        // Lấy customers từ POS với filter theo thời gian
        customers, err := PancakePos_GetCustomers(apiKey, shopId, pageNumber, pageSize, startTime, endTime)
```

### 4. Jobs Đồng Bộ Customers Từ POS

**File:** `app/jobs/sync_incremental_pancake_pos_customers_job.go`

```go
package jobs

import (
    "agent_pancake/app/integrations"
    "agent_pancake/app/scheduler"
    "context"
    "log"
    "time"
)

// SyncIncrementalPancakePosCustomersJob là job đồng bộ customers mới từ POS (incremental sync)
type SyncIncrementalPancakePosCustomersJob struct {
    *scheduler.BaseJob
}

// NewSyncIncrementalPancakePosCustomersJob tạo một instance mới
func NewSyncIncrementalPancakePosCustomersJob(name, schedule string) *SyncIncrementalPancakePosCustomersJob {
    job := &SyncIncrementalPancakePosCustomersJob{
        BaseJob: scheduler.NewBaseJob(name, schedule),
    }
    job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
    return job
}

// ExecuteInternal thực thi logic đồng bộ customers mới từ POS
func (j *SyncIncrementalPancakePosCustomersJob) ExecuteInternal(ctx context.Context) error {
    startTime := time.Now()
    log.Printf("═══════════════════════════════════════════════════════════")
    log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
    log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
    log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
    log.Printf("═══════════════════════════════════════════════════════════")
    
    // Thực hiện xác thực và đồng bộ dữ liệu cơ bản
    SyncBaseAuth()
    
    // Đồng bộ customers mới từ POS
    err := integrations.BridgeV2_SyncNewCustomersFromPos()
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
```

**File:** `app/jobs/sync_backfill_pancake_pos_customers_job.go`

```go
package jobs

import (
    "agent_pancake/app/integrations"
    "agent_pancake/app/scheduler"
    "context"
    "log"
    "time"
)

// SyncBackfillPancakePosCustomersJob là job đồng bộ customers cũ từ POS (backfill sync)
type SyncBackfillPancakePosCustomersJob struct {
    *scheduler.BaseJob
}

// NewSyncBackfillPancakePosCustomersJob tạo một instance mới
func NewSyncBackfillPancakePosCustomersJob(name, schedule string) *SyncBackfillPancakePosCustomersJob {
    job := &SyncBackfillPancakePosCustomersJob{
        BaseJob: scheduler.NewBaseJob(name, schedule),
    }
    job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
    return job
}

// ExecuteInternal thực thi logic đồng bộ customers cũ từ POS
func (j *SyncBackfillPancakePosCustomersJob) ExecuteInternal(ctx context.Context) error {
    startTime := time.Now()
    log.Printf("═══════════════════════════════════════════════════════════")
    log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
    log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
    log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
    log.Printf("═══════════════════════════════════════════════════════════")
    
    // Thực hiện xác thực và đồng bộ dữ liệu cơ bản
    SyncBaseAuth()
    
    // Đồng bộ customers cũ từ POS
    err := integrations.BridgeV2_SyncAllCustomersFromPos()
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
```

### 5. Đăng Ký Jobs Trong Scheduler

**File:** `app/scheduler/scheduler.go` (cần thêm vào)

```go
// Thêm job đồng bộ customers mới từ POS (incremental sync)
syncIncrementalPancakePosCustomersJob := jobs.NewSyncIncrementalPancakePosCustomersJob(
    "sync_incremental_pancake_pos_customers",
    "0 */5 * * * *", // Chạy mỗi 5 phút
)
scheduler.AddJobObject(syncIncrementalPancakePosCustomersJob)

// Thêm job đồng bộ customers cũ từ POS (backfill sync)
syncBackfillPancakePosCustomersJob := jobs.NewSyncBackfillPancakePosCustomersJob(
    "sync_backfill_pancake_pos_customers",
    "0 */5 * * * *", // Chạy mỗi 5 phút
)
scheduler.AddJobObject(syncBackfillPancakePosCustomersJob)
```

## 🔍 Các Điểm Kỹ Thuật Quan Trọng

### 1. Xử Lý Token

- **Lấy token**: Sử dụng `FolkForm_GetAccessTokens` với filter `{"system":"Pancake POS"}`
- **Pagination**: Lặp qua tất cả pages để lấy hết tokens
- **Format token**: Token được lưu trong field `value` của mỗi item

### 2. Xử Lý Response từ Pancake POS API

- **Customer API**: Response có thể là `{ "customers": [...] }` hoặc `[...]` (array trực tiếp)
- **Cần kiểm tra**: Cả 2 format để đảm bảo tương thích
- **Pagination**: Sử dụng `page_number` và `page_size` để lấy tất cả customers

### 3. Xử Lý Dữ Liệu Customer

- **posData**: Gửi toàn bộ customer data từ POS trong field `posData` - **gửi nguyên dữ liệu, không cần transform**
- **Server tự động xử lý**: Server sẽ tự động:
  - Extract các field: `posCustomerId` (từ `id`), `name`, `phoneNumbers` (từ `phone_number`), `emails` (từ `email`), `point`, `totalOrder`, `totalSpent`, etc.
  - Identify customer theo thứ tự ưu tiên: `posCustomerId` → `panCakeCustomerId` → `psid` → `phoneNumbers` → `email`
  - Merge dữ liệu nếu customer đã tồn tại (ưu tiên POS hơn Pancake cho thông tin cá nhân)
- **Không cần map field names**: Bot chỉ cần đọc từ POS và gửi về server, server sẽ tự động map

### 4. Error Handling

- **Retry logic**: Sử dụng pattern retry giống các hàm khác (max 5 lần)
- **Rate limiting**: Sử dụng adaptive rate limiter cho cả Pancake POS và FolkForm
- **Continue on error**: Nếu lỗi ở một customer/shop, tiếp tục với customer/shop tiếp theo

### 5. Performance

- **Pagination**: Sử dụng `pageSize=100` để giảm số lượng API calls
- **Rate limiting**: Sử dụng adaptive rate limiter để tránh rate limit
- **Batch processing**: Xử lý từng batch customers để tránh timeout

## 📊 So Sánh Với Đồng Bộ Customers Từ Pancake

| Đặc điểm | Pancake API | POS API |
|----------|-------------|---------|
| **Filter theo updated_at** | ✅ Có (`since`, `until`, `order_by`) | ✅ Có (`start_time_updated_at`, `end_time_updated_at`) |
| **Incremental sync** | ✅ Có thể (2 jobs) | ✅ Có thể (2 jobs) |
| **Backfill sync** | ✅ Có thể (2 jobs) | ✅ Có thể (2 jobs) |
| **Pagination** | ✅ Có (`page`, `limit`) | ✅ Có (`page_size`, `page_number`) |
| **Query params** | `since`, `until`, `order_by`, `page`, `limit` | `start_time_updated_at`, `end_time_updated_at`, `page_size`, `page_number` |
| **Identify customer** | Theo `panCakeCustomerId` hoặc `psid + pageId` | Theo `posCustomerId` hoặc `phone/email` |

**Kết luận:** 
- **Cả 2 API đều hỗ trợ filter theo thời gian** → có thể làm 2 jobs (incremental + backfill)
- **Pancake API**: Dùng `since`, `until`, `order_by`
- **POS API**: Dùng `start_time_updated_at`, `end_time_updated_at`
- **Không cần đồng bộ từ 2 đầu**: Server tự động identify và merge customers từ cả 2 nguồn
- **Tần suất chạy**: Incremental thường xuyên hơn (1-2 giờ), backfill ít hơn (6-12 giờ)

## 🚀 Lịch Chạy Đề Xuất

### Số Lượng Jobs

**Cần 2 jobs cho POS customers (giống conversation sync):**
- `SyncIncrementalPancakePosCustomersJob` - Sync customers mới (dùng `start_time_updated_at`, `end_time_updated_at`)
- `SyncBackfillPancakePosCustomersJob` - Sync customers cũ (dùng `start_time_updated_at`, `end_time_updated_at`)

**So sánh với Pancake customers (có 2 jobs):**
- `SyncIncrementalCustomersJob` - Sync customers mới (dùng `since`, `until`, `order_by`)
- `SyncBackfillCustomersJob` - Sync customers cũ (dùng `since`, `until`, `order_by`)

**Logic tương tự conversation sync:**
- **Incremental**: Lấy `lastUpdatedAt` từ FolkForm → sync từ `lastUpdatedAt` đến `now`
- **Backfill**: Lấy `oldestUpdatedAt` từ FolkForm → sync từ `0` đến `oldestUpdatedAt`

### Tần Suất Chạy

**Incremental sync (customers mới):**
- **Tần suất**: Mỗi 5 phút (để cập nhật nhanh customers mới)
- **Cron expression**: `0 */5 * * * *` (mỗi 5 phút)

**Backfill sync (customers cũ):**
- **Tần suất**: Mỗi 5 phút (để sync dữ liệu cũ)
- **Cron expression**: `0 */5 * * * *` (mỗi 5 phút)

### Có Cần Đồng Bộ Từ 2 Đầu Không?

**KHÔNG CẦN!** Server tự động xử lý:

1. **Pancake sync** → Gửi `{panCakeData: customerData}` → Server identify theo `panCakeCustomerId` hoặc `psid + pageId`
2. **POS sync** → Gửi `{posData: customerData}` → Server identify theo `posCustomerId` hoặc `phone/email`
3. **Server tự động merge**: Nếu cùng customer (qua phone/email/posCustomerId) → merge dữ liệu từ cả 2 nguồn

**Kết luận**: Chạy 2 jobs độc lập:
- Pancake customers job (incremental + backfill)
- POS customers job (full sync)
- Server tự động identify và merge → không cần logic phức tạp ở bot

## ⚠️ Lưu Ý

1. **Cần 2 jobs**: POS API hỗ trợ `start_time_updated_at` và `end_time_updated_at` → có thể làm incremental và backfill sync
2. **Không cần đồng bộ từ 2 đầu**: Server tự động identify và merge customers từ Pancake và POS → chạy 2 jobs độc lập
3. **Dùng time filter params**: Dùng `start_time_updated_at` và `end_time_updated_at` để filter theo thời gian, kết hợp với `page_size` và `page_number` để pagination
4. **Cần thêm hàm lấy mốc**: Cần thêm `FolkForm_GetLastPosCustomerUpdatedAt(shopId)` và `FolkForm_GetOldestPosCustomerUpdatedAt(shopId)` để lấy mốc thời gian
5. **Performance**: Incremental sync nhanh hơn (chỉ sync mới), backfill sync chậm hơn (sync cũ) → cần chạy với tần suất khác nhau
6. **Rate limiting**: Cần sử dụng rate limiter để tránh bị rate limit từ POS API
7. **Duplicate handling**: Backend tự động identify và merge customers, không cần lo về duplicate
8. **Shop ID**: Cần đảm bảo shopId được convert đúng kiểu (int) từ response của POS API

## 📝 Checklist Implementation

- [ ] Tạo hàm `PancakePos_GetCustomers()` trong `app/integrations/pancake_pos.go` (với params `start_time_updated_at`, `end_time_updated_at`)
- [ ] Tạo hàm `FolkForm_UpsertCustomerFromPos()` trong `app/integrations/folkform.go`
- [ ] Tạo hàm `FolkForm_GetLastPosCustomerUpdatedAt(shopId)` trong `app/integrations/folkform.go`
  - Filter: `{"posCustomerId":{"$exists":true},"posData.shop_id":shopId}` hoặc `{"posCustomerId":{"$ne":null},"posData.shop_id":shopId}`
  - Sort: `{"updatedAt":-1}` (desc - mới nhất trước)
  - Limit: 1
  - Lấy field `updatedAt` (milliseconds, convert sang seconds)
- [ ] Tạo hàm `FolkForm_GetOldestPosCustomerUpdatedAt(shopId)` trong `app/integrations/folkform.go`
  - Filter: `{"posCustomerId":{"$exists":true},"posData.shop_id":shopId}` hoặc `{"posCustomerId":{"$ne":null},"posData.shop_id":shopId}`
  - Sort: `{"updatedAt":1}` (asc - cũ nhất trước)
  - Limit: 1
  - Lấy field `updatedAt` (milliseconds, convert sang seconds)
- [ ] Tạo hàm `BridgeV2_SyncNewCustomersFromPos()` trong `app/integrations/bridge_v2.go` (incremental sync)
- [ ] Tạo hàm `BridgeV2_SyncAllCustomersFromPos()` trong `app/integrations/bridge_v2.go` (backfill sync)
- [ ] Tạo hàm helper `bridgeV2_SyncNewCustomersFromPosForShop()` trong `app/integrations/bridge_v2.go`
- [ ] Tạo hàm helper `bridgeV2_SyncAllCustomersFromPosForShop()` trong `app/integrations/bridge_v2.go`
- [ ] Tạo job `SyncIncrementalPancakePosCustomersJob` trong `app/jobs/sync_incremental_pancake_pos_customers_job.go`
- [ ] Tạo job `SyncBackfillPancakePosCustomersJob` trong `app/jobs/sync_backfill_pancake_pos_customers_job.go`
- [ ] Đăng ký 2 jobs trong scheduler (`app/scheduler/scheduler.go`)
- [ ] Test với dữ liệu thực tế
- [ ] Monitor performance và điều chỉnh tần suất chạy nếu cần

## 🎯 Kết Luận

Phương án này đề xuất đồng bộ customers từ POS về FolkForm bằng cách:
1. Lấy tất cả tokens từ FolkForm (system: "Pancake POS")
2. Với mỗi token, lấy danh sách shops
3. Với mỗi shop, lấy tất cả customers (pagination)
4. Upsert từng customer vào FolkForm thông qua endpoint `upsert-from-pos` với format: `{posData: customerData}`

**Điểm quan trọng:**
- **Bot chỉ cần đọc và gửi**: Đọc dữ liệu từ POS API và gửi về server đúng format `{posData: customerData}`
- **Server tự động xử lý**: Server sẽ tự động extract, identify và merge customers từ nhiều nguồn
- **Không cần transform**: Gửi nguyên dữ liệu từ POS API, không cần map field names hay transform gì
