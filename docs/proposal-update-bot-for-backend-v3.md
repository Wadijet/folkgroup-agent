# Đề Xuất Cập Nhật Bot Theo Backend Version 3.x

## 📋 Tổng Quan Thay Đổi Backend

Dựa trên file `docs-shared/ai-context/folkform-api-context.md`, backend đã có các thay đổi quan trọng từ Version 2.x lên Version 3.x:

### 🔴 Thay Đổi BẮT BUỘC (Breaking Changes)

#### 1. Organization Context System (Version 3.2) - **QUAN TRỌNG NHẤT**

**Vấn đề:**
- Backend mới yêu cầu header `X-Active-Role-ID` để xác định context làm việc
- Nếu không có header, backend sẽ tự động lấy role đầu tiên của user
- Context làm việc là **ROLE**, không phải organization

**Ảnh hưởng:**
- Bot hiện tại có thể không gửi header `X-Active-Role-ID`
- Cần đảm bảo bot gửi header này trong mọi request

**Giải pháp:**
1. Thêm header `X-Active-Role-ID` vào tất cả requests
2. Lấy role ID từ user profile sau khi login
3. Lưu role ID vào config hoặc global variable

#### 2. Customer API Separation (Version 2.9)

**Tình trạng hiện tại:**
- ✅ Bot đã sử dụng đúng endpoints mới:
  - `/fb-customer/upsert-one` - Cho FB customers
  - `/pc-pos-customer/upsert-one` - Cho POS customers
- ⚠️ Cần kiểm tra xem có còn dùng endpoint cũ `/customer` không

**Hành động:**
- Kiểm tra và loại bỏ mọi tham chiếu đến endpoint `/customer` cũ
- Đảm bảo tất cả customer operations dùng đúng endpoint mới

### 🟡 Thay Đổi Nên Cập Nhật (Recommended)

#### 3. Organization-Level Sharing (Version 3.3)

**Mô tả:**
- Hệ thống mới hỗ trợ chia sẻ dữ liệu giữa các organizations
- Bot không cần thay đổi gì, nhưng cần hiểu rằng dữ liệu có thể được share

**Hành động:**
- Không cần thay đổi code
- Chỉ cần lưu ý khi debug/logging

#### 4. Notification System (Version 3.1)

**Mô tả:**
- Backend có hệ thống notification mới
- Bot không cần tích hợp, nhưng có thể nhận notifications

**Hành động:**
- Không cần thay đổi code

## 🔧 Đề Xuất Cập Nhật Code

### 1. Thêm Organization Context Header

**File cần sửa:** `app/integrations/folkform.go`

**Thay đổi:**

1. **Thêm global variable để lưu active role ID:**
```go
// Thêm vào global/globalVars.go hoặc folkform.go
var ActiveRoleId string
```

2. **Cập nhật hàm `createAuthorizedClient` để thêm header:**
```go
// Helper function: Tạo HTTP client với authorization header và organization context
func createAuthorizedClient(timeout time.Duration) *httpclient.HttpClient {
	client := httpclient.NewHttpClient(global.GlobalConfig.ApiBaseUrl, timeout)
	client.SetHeader("Authorization", "Bearer "+global.ApiToken)
	
	// Thêm header X-Active-Role-ID nếu có
	if global.ActiveRoleId != "" {
		client.SetHeader("X-Active-Role-ID", global.ActiveRoleId)
	}
	
	return client
}
```

3. **Cập nhật hàm `FolkForm_Login` để lấy và lưu role ID:**
```go
// Sau khi login thành công, lấy role đầu tiên
if result["status"] == "success" {
	log.Printf("[FolkForm] [Login] Đăng nhập thành công!")
	
	// Lưu token
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if token, ok := dataMap["token"].(string); ok {
			global.ApiToken = token
			log.Printf("[FolkForm] [Login] Đã lưu JWT token (length: %d)", len(token))
		}
		
		// Lấy role ID đầu tiên nếu có
		if roles, ok := dataMap["roles"].([]interface{}); ok && len(roles) > 0 {
			if firstRole, ok := roles[0].(map[string]interface{}); ok {
				if roleId, ok := firstRole["id"].(string); ok {
					global.ActiveRoleId = roleId
					log.Printf("[FolkForm] [Login] Đã lưu Active Role ID: %s", roleId)
				}
			}
		}
	}
	
	return result, nil
}
```

4. **Thêm hàm để lấy roles từ backend:**
```go
// FolkForm_GetRoles lấy danh sách roles của user hiện tại
func FolkForm_GetRoles() ([]interface{}, error) {
	log.Printf("[FolkForm] Lấy danh sách roles của user")
	
	if err := checkApiToken(); err != nil {
		return nil, err
	}
	
	client := createAuthorizedClient(defaultTimeout)
	result, err := executeGetRequest(client, "/auth/roles", nil, "Lấy danh sách roles thành công")
	if err != nil {
		return nil, err
	}
	
	var roles []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if rolesArray, ok := dataMap["roles"].([]interface{}); ok {
			roles = rolesArray
		} else if rolesArray, ok := dataMap["data"].([]interface{}); ok {
			roles = rolesArray
		}
	} else if rolesArray, ok := result["data"].([]interface{}); ok {
		roles = rolesArray
	}
	
	return roles, nil
}
```

### 2. Cập Nhật Login Flow

**File cần sửa:** `app/jobs/helpers.go`

**Thay đổi:**

```go
func SyncBaseAuth() {
	// Login vào hệ thống
	log.Println("Đang đăng nhập vào hệ thống...")
	_, err := integrations.FolkForm_Login()
	if err != nil {
		log.Printf("❌ Lỗi khi đăng nhập: %v", err)
		return
	}
	
	// Lấy roles nếu chưa có ActiveRoleId
	if global.ActiveRoleId == "" {
		log.Println("Lấy danh sách roles...")
		roles, err := integrations.FolkForm_GetRoles()
		if err != nil {
			log.Printf("❌ Lỗi khi lấy roles: %v", err)
			// Tiếp tục, backend sẽ tự động detect role đầu tiên
		} else if len(roles) > 0 {
			if firstRole, ok := roles[0].(map[string]interface{}); ok {
				if roleId, ok := firstRole["id"].(string); ok {
					global.ActiveRoleId = roleId
					log.Printf("✅ Đã lưu Active Role ID: %s", roleId)
				} else if roleId, ok := firstRole["roleId"].(string); ok {
					global.ActiveRoleId = roleId
					log.Printf("✅ Đã lưu Active Role ID: %s", roleId)
				}
			}
		}
	}
	
	// Check-in
	log.Println("Đang điểm danh...")
	_, err = integrations.FolkForm_CheckIn()
	if err != nil {
		log.Printf("❌ Lỗi khi điểm danh: %v", err)
	}
}
```

### 3. Kiểm Tra và Loại Bỏ Endpoint Cũ

**File cần kiểm tra:** Tất cả files trong `app/integrations/`

**Hành động:**
- Tìm kiếm tất cả tham chiếu đến `/customer/` (endpoint cũ)
- Thay thế bằng `/fb-customer/` hoặc `/pc-pos-customer/` tùy trường hợp

**Command để tìm:**
```bash
grep -r "/customer/" app/integrations/
```

### 4. Cập Nhật Config (Nếu Cần)

**File cần sửa:** `config/config.go`

**Thay đổi (nếu muốn hardcode role ID):**
```go
// Thêm vào Config struct
type Config struct {
	// ... existing fields ...
	ActiveRoleId string `env:"ACTIVE_ROLE_ID" envDefault:""` // Optional: Role ID để làm việc
}
```

**Lưu ý:** Không bắt buộc, vì backend sẽ tự động detect role đầu tiên nếu không có header.

## 📝 Checklist Cập Nhật

### Bước 1: Cập Nhật Global Variables
- [ ] Thêm `ActiveRoleId` vào `global/globalVars.go`
- [ ] Export variable để có thể truy cập từ các package khác

### Bước 2: Cập Nhật HTTP Client
- [ ] Cập nhật `createAuthorizedClient()` để thêm header `X-Active-Role-ID`
- [ ] Đảm bảo header được thêm vào tất cả requests

### Bước 3: Cập Nhật Login Flow
- [ ] Thêm hàm `FolkForm_GetRoles()` để lấy roles
- [ ] Cập nhật `FolkForm_Login()` để lưu role ID từ response (nếu có)
- [ ] Cập nhật `SyncBaseAuth()` để lấy và lưu role ID

### Bước 4: Kiểm Tra Endpoints
- [ ] Tìm và thay thế tất cả endpoint `/customer/` cũ
- [ ] Đảm bảo tất cả customer operations dùng đúng endpoint mới

### Bước 5: Testing
- [ ] Test login và verify header được gửi đúng
- [ ] Test các operations với header mới
- [ ] Verify không có lỗi từ backend về missing context

## 🚨 Lưu Ý Quan Trọng

1. **Backward Compatibility:**
   - Backend vẫn hỗ trợ không có header `X-Active-Role-ID`
   - Backend sẽ tự động lấy role đầu tiên nếu không có header
   - Nhưng nên thêm header để đảm bảo context đúng

2. **Error Handling:**
   - Nếu không lấy được role ID, bot vẫn có thể hoạt động
   - Backend sẽ fallback về role đầu tiên
   - Log warning nếu không lấy được role ID

3. **Testing:**
   - Test với user có nhiều roles
   - Test với user chỉ có 1 role
   - Test với user không có role (should fail)

## 📚 Tài Liệu Tham Khảo

- File context: `docs-shared/ai-context/folkform-api-context.md`
- Version 3.2: Organization Context System
- Version 2.9: Customer Separation

## 🔄 Migration Path

1. **Phase 1 (Không Breaking):**
   - Thêm header `X-Active-Role-ID` vào requests
   - Backend sẽ tự động detect nếu không có header
   - Bot vẫn hoạt động bình thường

2. **Phase 2 (Tối Ưu):**
   - Lấy và lưu role ID sau khi login
   - Gửi header trong mọi request
   - Đảm bảo context đúng

3. **Phase 3 (Cleanup):**
   - Loại bỏ endpoint cũ nếu còn
   - Cập nhật documentation
   - Final testing
