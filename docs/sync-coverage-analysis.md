# Phân Tích Độ Bao Phủ Đồng Bộ Dữ Liệu

**Ngày phân tích:** 2025-01-XX  
**Mục đích:** So sánh API Pancake và FolkForm để xác định những gì còn thiếu cần đồng bộ

---

## 📊 Bảng So Sánh API

### ✅ Đã Đồng Bộ

| Loại Dữ Liệu | Pancake API | FolkForm API | Trạng Thái | Hàm Đồng Bộ |
|-------------|-------------|--------------|------------|-------------|
| **Pages** | `GET /v1/pages` | `FbPage` collection | ✅ Đã sync | `Bridge_SyncPages()` |
| **Conversations** | `GET /pages/{page_id}/conversations` | `FbConversation` collection | ✅ Đã sync | `Bridge_SyncConversationsFromCloud()`, `Sync_NewMessagesOfPage()` |
| **Messages** | `GET /pages/{page_id}/conversations/{conversation_id}/messages` | `FbMessage` collection | ✅ Đã sync | `Bridge_SyncMessages()`, `bridge_SyncMessageOfConversation()` |

---

## ❌ Chưa Đồng Bộ

### 1. Posts (Bài Đăng) - ⚠️ QUAN TRỌNG

**Pancake API:**
- `GET /pages/{page_id}/posts`
- Có pagination, filter theo type (video, photo, text, livestream)
- Có thông tin: id, message, type, reactions, comment_count, inserted_at

**FolkForm API:**
- ✅ Có collection `FbPost`
- Model: `{ id, pageId, postId, panCakeData, createdAt, updatedAt }`
- Endpoint: `/api/v1/facebook/post/*`
- Có endpoint đặc biệt: `find-by-post-id/:id`

**Đánh giá:** 
- ⭐⭐⭐ **RẤT QUAN TRỌNG** - Cần đồng bộ
- FolkForm đã có sẵn collection và endpoints
- Có thể lưu full data trong `panCakeData`

**Đề xuất:**
- Tạo hàm `Bridge_SyncPosts()` tương tự `Bridge_SyncPages()`
- Sync theo page, có pagination
- Sử dụng upsert dựa trên `postId`

---

### 2. Customers (Khách Hàng) - ⚠️ CẦN XEM XÉT

**Pancake API:**
- `GET /pages/{page_id}/page_customers`
- Có pagination, filter theo since/until
- Có thông tin: psid, name, phone_numbers, birthday, gender, lives_in, notes

**FolkForm API:**
- ❌ **KHÔNG có collection riêng cho Customers**
- Nhưng `FbConversation` có field `customerId` (optional)
- Customer data có thể được lưu trong `panCakeData` của conversation/message

**Đánh giá:**
- ⭐⭐ **TÙY CHỌN** - Có thể cần nếu muốn quản lý customers riêng
- Hiện tại customer info đã có trong conversations/messages
- Nếu cần query customers độc lập → cần tạo collection mới trong FolkForm

**Đề xuất:**
- **Option 1:** Không sync riêng, dùng data từ conversations
- **Option 2:** Tạo collection `FbCustomer` trong FolkForm (cần backend support)
- **Option 3:** Sync và lưu vào `panCakeData` của conversation

---

### 3. Tags (Thẻ) - ⚠️ CẦN XEM XÉT

**Pancake API:**
- `GET /pages/{page_id}/tags`
- Có thông tin: id, text, color, lighten_color

**FolkForm API:**
- ❌ **KHÔNG có collection riêng cho Tags**
- Nhưng `FbConversation` có field `tags` trong `panCakeData`
- Tags được lưu dưới dạng array trong conversation

**Đánh giá:**
- ⭐⭐ **TÙY CHỌN** - Có thể cần nếu muốn quản lý tags riêng
- Hiện tại tags đã có trong conversations
- Nếu cần query/management tags → cần tạo collection mới

**Đề xuất:**
- **Option 1:** Không sync riêng, dùng data từ conversations
- **Option 2:** Tạo collection `FbTag` trong FolkForm (cần backend support)
- **Option 3:** Sync và lưu vào metadata của page

---

### 4. Users (Người Dùng Pancake) - ❌ KHÔNG CẦN

**Pancake API:**
- `GET /pages/{page_id}/users`
- Có thông tin: id, name, status, fb_id, page_permissions, status_in_page, is_online

**FolkForm API:**
- ❌ **KHÔNG có collection cho Pancake Users**
- FolkForm có `User` collection nhưng là cho hệ thống authentication (Firebase)

**Đánh giá:**
- ⭐ **KHÔNG CẦN** - Users của Pancake là internal, không liên quan đến FolkForm
- FolkForm có hệ thống user riêng (Firebase-based)

**Đề xuất:**
- Không cần sync

---

### 5. Statistics (Thống Kê) - ❌ KHÔNG CẦN

**Pancake API:**
- Nhiều loại statistics:
  - `GET /pages/{page_id}/statistics/pages_campaign` - Ads Campaign Statistics
  - `GET /pages/{page_id}/statistics/ads` - Ads Statistics
  - `GET /pages/{page_id}/statistics/customer_engagements` - Customer Engagement Statistics
  - `GET /pages/{page_id}/statistics/pages` - Page Statistics
  - `GET /pages/{page_id}/statistics/tags` - Tag Statistics
  - `GET /pages/{page_id}/statistics/users` - User Statistics

**FolkForm API:**
- ❌ **KHÔNG có collection cho Statistics**

**Đánh giá:**
- ⭐ **KHÔNG CẦN** - Statistics là dữ liệu analytics, không cần sync thường xuyên
- Có thể lấy real-time từ Pancake khi cần
- Nếu cần lưu lịch sử → cần tạo collection mới

**Đề xuất:**
- Không cần sync (hoặc sync on-demand khi cần)

---

### 6. Call Logs (Nhật Ký Cuộc Gọi) - ❌ TÙY CHỌN

**Pancake API:**
- `GET /pages/{page_id}/sip_call_logs`
- Có pagination, filter theo since/until
- Có thông tin: call_id, caller, callee, start_time, duration, status

**FolkForm API:**
- ❌ **KHÔNG có collection cho Call Logs**

**Đánh giá:**
- ⭐ **TÙY CHỌN** - Chỉ cần nếu muốn quản lý call logs
- Có thể liên quan đến customer service

**Đề xuất:**
- **Option 1:** Không sync (nếu không cần)
- **Option 2:** Tạo collection `FbCallLog` trong FolkForm (cần backend support)

---

### 7. Export Data (Xuất Dữ Liệu) - ❌ KHÔNG CẦN

**Pancake API:**
- `GET /pages/{page_id}/export_data?action=conversations_from_ads`
- Export conversations từ ads với since/until

**FolkForm API:**
- ❌ **KHÔNG có collection riêng**

**Đánh giá:**
- ⭐ **KHÔNG CẦN** - Export data là tính năng export, không phải sync
- Conversations từ ads đã được sync qua `Bridge_SyncConversationsFromCloud()`

**Đề xuất:**
- Không cần sync (đã có trong conversations sync)

---

### 8. PcOrder (Đơn Hàng) - ❓ CẦN LÀM RÕ

**Pancake API:**
- ❌ **KHÔNG có API cho Orders** (theo tài liệu hiện tại)

**FolkForm API:**
- ✅ Có collection `PcOrder`
- Model: `{ id, pancakeOrderId, status, panCakeData, createdAt, updatedAt }`

**Đánh giá:**
- ⭐⭐ **CẦN LÀM RÕ** - FolkForm có collection nhưng Pancake không có API
- Có thể orders đến từ nguồn khác (không phải Pancake API)
- Hoặc Pancake có API nhưng chưa được document

**Đề xuất:**
- Kiểm tra lại Pancake API xem có endpoint cho orders không
- Nếu không có → không cần sync từ Pancake

---

## 📋 Tóm Tắt

### Đã Đồng Bộ (3/11)
1. ✅ Pages
2. ✅ Conversations  
3. ✅ Messages

### Cần Đồng Bộ Ngay (1/11)
1. ⚠️ **Posts** - Rất quan trọng, FolkForm đã có sẵn collection

### Cần Xem Xét (2/11)
2. ⚠️ **Customers** - Tùy chọn, có thể dùng data từ conversations
3. ⚠️ **Tags** - Tùy chọn, có thể dùng data từ conversations

### Không Cần Đồng Bộ (5/11)
4. ❌ **Users** - Internal Pancake users, không liên quan
5. ❌ **Statistics** - Analytics data, không cần sync thường xuyên
6. ❌ **Call Logs** - Tùy chọn, chỉ cần nếu quản lý calls
7. ❌ **Export Data** - Đã có trong conversations sync
8. ❌ **PcOrder** - Cần làm rõ nguồn dữ liệu

---

## 🎯 Đề Xuất Ưu Tiên

### Priority 1 (Cao - Cần làm ngay)
1. **Đồng bộ Posts**
   - Tạo hàm `Bridge_SyncPosts()` trong `bridge.go`
   - Tạo hàm `Pancake_GetPosts()` trong `pancake.go`
   - Tạo hàm `FolkForm_CreateFbPost()` trong `folkform.go`
   - Thêm job `SyncPostsJob` nếu cần sync định kỳ

### Priority 2 (Trung bình - Nên làm sớm)
2. **Đánh giá nhu cầu Customers và Tags**
   - Xác định xem có cần query customers/tags độc lập không
   - Nếu cần → đề xuất tạo collection mới trong FolkForm backend
   - Nếu không → giữ nguyên (dùng data từ conversations)

### Priority 3 (Thấp - Có thể làm sau)
3. **Call Logs** (nếu cần)
   - Tạo collection `FbCallLog` trong FolkForm backend
   - Implement sync logic

---

## 💡 Gợi Ý Implementation cho Posts

### 1. Tạo hàm Pancake API
```go
// Trong pancake.go
func Pancake_GetPosts(page_id string, page_access_token string, page_number int, page_size int, since int64, until int64, post_type string) (result map[string]interface{}, err error)
```

### 2. Tạo hàm FolkForm API
```go
// Trong folkform.go
func FolkForm_CreateFbPost(pageId string, postData interface{}) (result map[string]interface{}, err error)
func FolkForm_GetFbPosts(page int, limit int) (result map[string]interface{}, err error)
```

### 3. Tạo hàm Bridge
```go
// Trong bridge.go
func Bridge_SyncPosts() (resultErr error)
func bridge_SyncPostsOfPage(page_id string, page_username string) (resultErr error)
```

### 4. Thêm vào Job (nếu cần)
```go
// Trong sync_all_data_job.go hoặc sync_new_job.go
// Thêm sync posts vào DoSyncAllData() hoặc DoSyncNew()
```

---

## 📝 Lưu Ý

1. **Posts sync** nên tương tự như conversations sync:
   - Lấy posts từ Pancake theo page
   - Upsert vào FolkForm dựa trên `postId`
   - Có pagination support

2. **Customers và Tags**:
   - Hiện tại data đã có trong conversations
   - Chỉ cần tạo collection riêng nếu cần query/management độc lập
   - Cần backend support để tạo collection mới

3. **Performance**:
   - Posts có thể nhiều → cần pagination tốt
   - Có thể sync incremental dựa trên `inserted_at`

---

**Kết luận:** Cần ưu tiên đồng bộ **Posts** ngay vì FolkForm đã có sẵn collection và endpoints. Customers và Tags có thể xem xét sau tùy nhu cầu.
