# 📋 Đề Xuất Phương Án Cập Nhật Job Sync Customer Theo Cấu Trúc Mới

## 📌 Tổng Quan

Backend đã tách collection `customers` thành 2 collections riêng biệt theo nguồn:
- **`fb_customers`** (FbCustomer) - Cho customers từ Pancake (Facebook)
- **`pc_pos_customers`** (PcPosCustomer) - Cho customers từ Pancake POS

Tài liệu này đề xuất phương án cập nhật các job sync customer để sử dụng các endpoint mới.

---

## 🎯 Mục Tiêu

1. **Cập nhật các hàm integration** để sử dụng endpoint mới thay vì endpoint deprecated
2. **Tách biệt logic sync** cho 2 nguồn khác nhau (FB và POS)
3. **Đảm bảo tương thích** với cấu trúc backend mới
4. **Giữ nguyên logic sync** (incremental và backfill) cho cả 2 nguồn

---

## 📚 Thông Tin Backend Mới

### Endpoints Mới

#### 1. Facebook Customer (`fb_customers`)
- **Upsert**: `POST /api/v1/fb-customer/upsert-one?filter={"customerId":"xxx"}`
- **Find**: `GET /api/v1/fb-customer/find`
- **Find One**: `GET /api/v1/fb-customer/find-one?filter={"customerId":"xxx"}`
- **Permission**: `FbCustomer.*`

**Request Body:**
```json
{
  "panCakeData": {
    "id": "600208cc-136b-4000-8fde-9572e45787a0",
    "psid": "25149177694676594",
    "page_id": "page_123",
    "name": "Mai Thao Nguyen",
    "phone_numbers": ["0903154539"],
    "email": "user@example.com",
    "updated_at": "2025-12-07T10:23:23.000000"
  }
}
```

#### 2. POS Customer (`pc_pos_customers`)
- **Upsert**: `POST /api/v1/pc-pos-customer/upsert-one?filter={"customerId":"xxx"}`
- **Find**: `GET /api/v1/pc-pos-customer/find`
- **Find One**: `GET /api/v1/pc-pos-customer/find-one?filter={"customerId":"xxx"}`
- **Permission**: `PcPosCustomer.*`

**Request Body:**
```json
{
  "posData": {
    "id": "b0110315-b102-436b-8b3b-ed8d16740327",
    "shop_id": 860225178,
    "name": "Trần Văn Hoàng",
    "phone_numbers": ["0999999999"],
    "emails": ["thudo@gmail.com"],
    "updated_at": "2025-01-15T10:18:41Z"
  }
}
```

### Endpoints Deprecated (Cần Thay Thế)
- ❌ `POST /api/v1/customer/upsert-one` → Dùng `/fb-customer/upsert-one` hoặc `/pc-pos-customer/upsert-one`
- ❌ `GET /api/v1/customer/find` → Dùng `/fb-customer/find` hoặc `/pc-pos-customer/find`

---

## 🔧 Phương Án Cập Nhật

### Bước 1: Cập Nhật Các Hàm Integration

#### 1.1. Cập Nhật `FolkForm_CreateCustomer()` → `FolkForm_UpsertFbCustomer()`

**File**: `app/integrations/folkform.go`

**Thay đổi:**
- Đổi tên hàm: `FolkForm_CreateCustomer` → `FolkForm_UpsertFbCustomer`
- Đổi endpoint: `/customer/upsert-one` → `/fb-customer/upsert-one`
- Giữ nguyên logic và format request body (`panCakeData`)

**Code mới:**
```go
// FolkForm_UpsertFbCustomer tạo/cập nhật FB customer vào FolkForm
// customerData: Dữ liệu customer từ Pancake API (map[string]interface{})
// Chỉ cần gửi đúng DTO: {panCakeData: customerData}
// Backend sẽ tự động extract dữ liệu từ panCakeData
// Filter: customerId (từ id) - ID để identify customer
func FolkForm_UpsertFbCustomer(customerData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu upsert FB customer")
	
	if err := checkApiToken(); err != nil {
		return nil, err
	}
	
	client := createAuthorizedClient(defaultTimeout)
	
	// Tạo params với filter cho upsert
	params := map[string]string{}
	
	// Tạo filter từ customer data để upsert
	if customerMap, ok := customerData.(map[string]interface{}); ok {
		if customerId, ok := customerMap["id"].(string); ok && customerId != "" {
			filter := fmt.Sprintf(`{"customerId":"%s"}`, customerId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert FB customer: %s", filter)
		} else {
			log.Printf("[FolkForm] CẢNH BÁO: Không tìm thấy id trong customer data")
		}
	}
	
	// Tạo data đúng DTO: {panCakeData: customerData}
	data := map[string]interface{}{
		"panCakeData": customerData,
	}
	
	log.Printf("[FolkForm] Đang gửi request upsert FB customer đến FolkForm backend...")
	result, err = executePostRequest(client, "/fb-customer/upsert-one", data, params, "Gửi FB customer thành công", "Gửi FB customer thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi upsert FB customer: %v", err)
	} else {
		log.Printf("[FolkForm] Upsert FB customer thành công")
	}
	return result, err
}
```

#### 1.2. Cập Nhật `FolkForm_UpsertCustomerFromPos()`

**File**: `app/integrations/folkform.go`

**Thay đổi:**
- Đổi endpoint: `/customer/upsert-one` → `/pc-pos-customer/upsert-one`
- Giữ nguyên logic và format request body (`posData`)

**Code mới:**
```go
// FolkForm_UpsertCustomerFromPos tạo/cập nhật POS customer vào FolkForm
// customerData: Dữ liệu customer từ Pancake POS API (map[string]interface{})
// Chỉ cần gửi đúng format: {posData: customerData}
// Server sẽ tự động extract dữ liệu từ posData
// Filter: customerId (từ id) - ID để identify customer
func FolkForm_UpsertCustomerFromPos(customerData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu upsert POS customer")
	
	if err := checkApiToken(); err != nil {
		return nil, err
	}
	
	client := createAuthorizedClient(defaultTimeout)
	
	// Tạo params với filter cho upsert
	params := map[string]string{}
	
	// Tạo filter từ customer data để upsert
	if customerMap, ok := customerData.(map[string]interface{}); ok {
		if customerId, ok := customerMap["id"].(string); ok && customerId != "" {
			filter := fmt.Sprintf(`{"customerId":"%s"}`, customerId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert POS customer: %s", filter)
		} else {
			log.Printf("[FolkForm] CẢNH BÁO: Không tìm thấy id trong customer data từ POS")
		}
	}
	
	// Tạo data đúng DTO: {posData: customerData}
	data := map[string]interface{}{
		"posData": customerData,
	}
	
	log.Printf("[FolkForm] Đang gửi request upsert POS customer đến FolkForm backend...")
	result, err = executePostRequest(client, "/pc-pos-customer/upsert-one", data, params, "Gửi POS customer thành công", "Gửi POS customer thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi upsert POS customer: %v", err)
	} else {
		log.Printf("[FolkForm] Upsert POS customer thành công")
	}
	return result, err
}
```

#### 1.3. Cập Nhật `FolkForm_GetLastCustomerUpdatedAt()` → `FolkForm_GetLastFbCustomerUpdatedAt()`

**File**: `app/integrations/folkform.go`

**Thay đổi:**
- Đổi tên hàm: `FolkForm_GetLastCustomerUpdatedAt` → `FolkForm_GetLastFbCustomerUpdatedAt`
- Đổi endpoint: `/customer/find` → `/fb-customer/find`
- Giữ nguyên logic query (filter theo `pageId`, sort theo `updatedAt DESC`)

**Code mới:**
```go
// FolkForm_GetLastFbCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của FB customer cập nhật gần nhất
// Trả về: updatedAt (seconds), error
func FolkForm_GetLastFbCustomerUpdatedAt(pageId string) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy FB customer cập nhật gần nhất - pageId: %s", pageId)
	
	if err := checkApiToken(); err != nil {
		return 0, err
	}
	
	client := createAuthorizedClient(defaultTimeout)
	
	// Query: filter theo pageId, sort theo updatedAt DESC, limit 1
	params := map[string]string{
		"filter":  `{"pageId":"` + pageId + `"}`,
		"options": `{"sort":{"updatedAt":-1},"limit":1}`, // Sort desc (mới nhất trước)
	}
	
	result, err := executeGetRequest(
		client,
		"/fb-customer/find",
		params,
		"Lấy FB customer cập nhật gần nhất thành công",
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
		log.Printf("[FolkForm] Không tìm thấy FB customer nào - pageId: %s", pageId)
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
		log.Printf("[FolkForm] Tìm thấy FB customer cập nhật gần nhất - updatedAt: %d (seconds)", updatedAtSeconds)
		return updatedAtSeconds, nil
	}
	
	return 0, nil
}
```

#### 1.4. Cập Nhật `FolkForm_GetOldestCustomerUpdatedAt()` → `FolkForm_GetOldestFbCustomerUpdatedAt()`

**File**: `app/integrations/folkform.go`

**Thay đổi:**
- Đổi tên hàm: `FolkForm_GetOldestCustomerUpdatedAt` → `FolkForm_GetOldestFbCustomerUpdatedAt`
- Đổi endpoint: `/customer/find` → `/fb-customer/find`
- Giữ nguyên logic query (filter theo `pageId`, sort theo `updatedAt ASC`)

**Code mới:**
```go
// FolkForm_GetOldestFbCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của FB customer cập nhật cũ nhất
// Trả về: updatedAt (seconds), error
func FolkForm_GetOldestFbCustomerUpdatedAt(pageId string) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy FB customer cập nhật cũ nhất - pageId: %s", pageId)
	
	if err := checkApiToken(); err != nil {
		return 0, err
	}
	
	client := createAuthorizedClient(defaultTimeout)
	
	// Query: filter theo pageId, sort theo updatedAt ASC, limit 1
	params := map[string]string{
		"filter":  `{"pageId":"` + pageId + `"}`,
		"options": `{"sort":{"updatedAt":1},"limit":1}`, // Sort asc (cũ nhất trước)
	}
	
	result, err := executeGetRequest(
		client,
		"/fb-customer/find",
		params,
		"Lấy FB customer cập nhật cũ nhất thành công",
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
		log.Printf("[FolkForm] Không tìm thấy FB customer nào - pageId: %s", pageId)
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
		log.Printf("[FolkForm] Tìm thấy FB customer cập nhật cũ nhất - updatedAt: %d (seconds)", updatedAtSeconds)
		return updatedAtSeconds, nil
	}
	
	return 0, nil
}
```

#### 1.5. Tạo Hàm Mới Cho POS Customers

**File**: `app/integrations/folkform.go`

**Thêm 2 hàm mới:**

```go
// FolkForm_GetLastPosCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của POS customer cập nhật gần nhất
// Trả về: updatedAt (seconds), error
func FolkForm_GetLastPosCustomerUpdatedAt(shopId int64) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy POS customer cập nhật gần nhất - shopId: %d", shopId)
	
	if err := checkApiToken(); err != nil {
		return 0, err
	}
	
	client := createAuthorizedClient(defaultTimeout)
	
	// Query: filter theo shopId, sort theo updatedAt DESC, limit 1
	params := map[string]string{
		"filter":  fmt.Sprintf(`{"shopId":%d}`, shopId),
		"options": `{"sort":{"updatedAt":-1},"limit":1}`, // Sort desc (mới nhất trước)
	}
	
	result, err := executeGetRequest(
		client,
		"/pc-pos-customer/find",
		params,
		"Lấy POS customer cập nhật gần nhất thành công",
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
		log.Printf("[FolkForm] Không tìm thấy POS customer nào - shopId: %d", shopId)
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
		log.Printf("[FolkForm] Tìm thấy POS customer cập nhật gần nhất - updatedAt: %d (seconds)", updatedAtSeconds)
		return updatedAtSeconds, nil
	}
	
	return 0, nil
}

// FolkForm_GetOldestPosCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của POS customer cập nhật cũ nhất
// Trả về: updatedAt (seconds), error
func FolkForm_GetOldestPosCustomerUpdatedAt(shopId int64) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy POS customer cập nhật cũ nhất - shopId: %d", shopId)
	
	if err := checkApiToken(); err != nil {
		return 0, err
	}
	
	client := createAuthorizedClient(defaultTimeout)
	
	// Query: filter theo shopId, sort theo updatedAt ASC, limit 1
	params := map[string]string{
		"filter":  fmt.Sprintf(`{"shopId":%d}`, shopId),
		"options": `{"sort":{"updatedAt":1},"limit":1}`, // Sort asc (cũ nhất trước)
	}
	
	result, err := executeGetRequest(
		client,
		"/pc-pos-customer/find",
		params,
		"Lấy POS customer cập nhật cũ nhất thành công",
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
		log.Printf("[FolkForm] Không tìm thấy POS customer nào - shopId: %d", shopId)
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
		log.Printf("[FolkForm] Tìm thấy POS customer cập nhật cũ nhất - updatedAt: %d (seconds)", updatedAtSeconds)
		return updatedAtSeconds, nil
	}
	
	return 0, nil
}
```

### Bước 2: Cập Nhật Các Hàm Bridge V2

#### 2.1. Cập Nhật `BridgeV2_SyncNewCustomers()` (Incremental Sync - FB)

**File**: `app/integrations/bridge_v2.go`

**Thay đổi:**
- `FolkForm_GetLastCustomerUpdatedAt(pageId)` → `FolkForm_GetLastFbCustomerUpdatedAt(pageId)`
- `FolkForm_CreateCustomer(customer)` → `FolkForm_UpsertFbCustomer(customer)`

#### 2.2. Cập Nhật `BridgeV2_SyncAllCustomers()` (Backfill Sync - FB)

**File**: `app/integrations/bridge_v2.go`

**Thay đổi:**
- `FolkForm_GetOldestCustomerUpdatedAt(pageId)` → `FolkForm_GetOldestFbCustomerUpdatedAt(pageId)`
- `FolkForm_CreateCustomer(customer)` → `FolkForm_UpsertFbCustomer(customer)`

#### 2.3. Cập Nhật `BridgeV2_SyncNewCustomersFromPos()` (Incremental Sync - POS)

**File**: `app/integrations/bridge_v2.go`

**Thay đổi:**
- Thêm logic lấy `lastUpdatedAt` từ POS customer collection
- Sử dụng `FolkForm_GetLastPosCustomerUpdatedAt(shopId)` thay vì hardcode hoặc logic cũ
- `FolkForm_UpsertCustomerFromPos()` đã được cập nhật ở Bước 1.2

**Code mới (phần lấy lastUpdatedAt):**
```go
// 1. Lấy lastUpdatedAt từ POS customer collection
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
} else {
	// Có customers → sync từ lastUpdatedAt đến now
	startTime = lastUpdatedAt
	endTime = time.Now().Unix()
}
```

#### 2.4. Cập Nhật `BridgeV2_SyncAllCustomersFromPos()` (Backfill Sync - POS)

**File**: `app/integrations/bridge_v2.go`

**Thay đổi:**
- Thêm logic lấy `oldestUpdatedAt` từ POS customer collection
- Sử dụng `FolkForm_GetOldestPosCustomerUpdatedAt(shopId)` thay vì hardcode hoặc logic cũ
- `FolkForm_UpsertCustomerFromPos()` đã được cập nhật ở Bước 1.2

**Code mới (phần lấy oldestUpdatedAt):**
```go
// 1. Lấy oldestUpdatedAt từ POS customer collection
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
} else {
	// Có customers → sync từ 0 đến oldestUpdatedAt
	startTime = 0
	endTime = oldestUpdatedAt
}
```

### Bước 3: Các Job Không Cần Thay Đổi

Các job sau **KHÔNG CẦN THAY ĐỔI** vì chúng chỉ gọi các hàm bridge, không gọi trực tiếp các hàm integration:

- ✅ `app/jobs/sync_backfill_customers_job.go` - Gọi `BridgeV2_SyncAllCustomers()`
- ✅ `app/jobs/sync_incremental_customers_job.go` - Gọi `BridgeV2_SyncNewCustomers()`
- ✅ `app/jobs/sync_backfill_pancake_pos_customers_job.go` - Gọi `BridgeV2_SyncAllCustomersFromPos()`
- ✅ `app/jobs/sync_incremental_pancake_pos_customers_job.go` - Gọi `BridgeV2_SyncNewCustomersFromPos()`

---

## 📝 Tóm Tắt Thay Đổi

### Files Cần Sửa

1. **`app/integrations/folkform.go`**
   - ✅ Đổi tên: `FolkForm_CreateCustomer` → `FolkForm_UpsertFbCustomer`
   - ✅ Đổi endpoint: `/customer/upsert-one` → `/fb-customer/upsert-one`
   - ✅ Đổi tên: `FolkForm_GetLastCustomerUpdatedAt` → `FolkForm_GetLastFbCustomerUpdatedAt`
   - ✅ Đổi endpoint: `/customer/find` → `/fb-customer/find`
   - ✅ Đổi tên: `FolkForm_GetOldestCustomerUpdatedAt` → `FolkForm_GetOldestFbCustomerUpdatedAt`
   - ✅ Đổi endpoint: `/customer/find` → `/fb-customer/find`
   - ✅ Cập nhật: `FolkForm_UpsertCustomerFromPos` - đổi endpoint `/customer/upsert-one` → `/pc-pos-customer/upsert-one`
   - ✅ Thêm mới: `FolkForm_GetLastPosCustomerUpdatedAt`
   - ✅ Thêm mới: `FolkForm_GetOldestPosCustomerUpdatedAt`

2. **`app/integrations/bridge_v2.go`**
   - ✅ Cập nhật: `BridgeV2_SyncNewCustomers()` - dùng hàm mới cho FB
   - ✅ Cập nhật: `BridgeV2_SyncAllCustomers()` - dùng hàm mới cho FB
   - ✅ Cập nhật: `BridgeV2_SyncNewCustomersFromPos()` - thêm logic lấy lastUpdatedAt từ POS
   - ✅ Cập nhật: `BridgeV2_SyncAllCustomersFromPos()` - thêm logic lấy oldestUpdatedAt từ POS

### Files Không Cần Sửa

- ✅ `app/jobs/sync_backfill_customers_job.go`
- ✅ `app/jobs/sync_incremental_customers_job.go`
- ✅ `app/jobs/sync_backfill_pancake_pos_customers_job.go`
- ✅ `app/jobs/sync_incremental_pancake_pos_customers_job.go`

---

## ✅ Checklist Triển Khai

- [ ] **Bước 1**: Cập nhật các hàm integration trong `folkform.go`
  - [ ] Đổi `FolkForm_CreateCustomer` → `FolkForm_UpsertFbCustomer`
  - [ ] Đổi `FolkForm_GetLastCustomerUpdatedAt` → `FolkForm_GetLastFbCustomerUpdatedAt`
  - [ ] Đổi `FolkForm_GetOldestCustomerUpdatedAt` → `FolkForm_GetOldestFbCustomerUpdatedAt`
  - [ ] Cập nhật `FolkForm_UpsertCustomerFromPos` (đổi endpoint)
  - [ ] Thêm `FolkForm_GetLastPosCustomerUpdatedAt`
  - [ ] Thêm `FolkForm_GetOldestPosCustomerUpdatedAt`

- [ ] **Bước 2**: Cập nhật các hàm bridge trong `bridge_v2.go`
  - [ ] Cập nhật `BridgeV2_SyncNewCustomers()` (FB incremental)
  - [ ] Cập nhật `BridgeV2_SyncAllCustomers()` (FB backfill)
  - [ ] Cập nhật `BridgeV2_SyncNewCustomersFromPos()` (POS incremental)
  - [ ] Cập nhật `BridgeV2_SyncAllCustomersFromPos()` (POS backfill)

- [ ] **Bước 3**: Test
  - [ ] Test sync FB customers (incremental)
  - [ ] Test sync FB customers (backfill)
  - [ ] Test sync POS customers (incremental)
  - [ ] Test sync POS customers (backfill)
  - [ ] Verify data được lưu vào đúng collections (`fb_customers` và `pc_pos_customers`)

---

## 🔍 Lưu Ý

1. **Tương thích ngược**: Endpoint `/customer/*` vẫn hoạt động nhưng deprecated. Nên chuyển sang endpoint mới càng sớm càng tốt.

2. **Permissions**: Đảm bảo token có quyền:
   - `FbCustomer.Update` cho sync FB customers
   - `PcPosCustomer.Update` cho sync POS customers

3. **Filter**: Cả 2 collections đều dùng `customerId` làm unique identifier, nhưng:
   - FB customers: `customerId` từ `panCakeData.id`
   - POS customers: `customerId` từ `posData.id` (UUID string)

4. **Query fields**: 
   - FB customers: filter theo `pageId`
   - POS customers: filter theo `shopId`

---

## 📚 Tài Liệu Tham Khảo

- `docs/backend/folkform-api-context.md` - Tài liệu API backend mới
- `docs/customer-sync-guide.md` - Hướng dẫn sync customer (có thể cần cập nhật)
- `docs/proposal-sync-pancake-pos-customers.md` - Đề xuất sync POS customers
