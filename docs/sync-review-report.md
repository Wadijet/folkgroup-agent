# Báo Cáo Rà Soát Đồng Bộ Dữ Liệu Pancake ↔ FolkForm

**Ngày rà soát:** 2025-01-XX  
**Mục đích:** Xác định dữ liệu còn thiếu cần đồng bộ giữa Pancake và FolkForm

---

## 📊 Tổng Quan Tình Trạng Đồng Bộ

### ✅ Đã Đồng Bộ Hoàn Chỉnh (4/4 loại dữ liệu chính)

| Loại Dữ Liệu | Pancake API | FolkForm Collection | Hàm Đồng Bộ | Trạng Thái |
|-------------|-------------|---------------------|-------------|------------|
| **Pages** | `GET /v1/pages` | `FbPage` | `Bridge_SyncPages()` | ✅ Hoàn chỉnh |
| **Conversations** | `GET /pages/{page_id}/conversations` | `FbConversation` | `BridgeV2_SyncNewData()`, `BridgeV2_SyncAllData()` | ✅ Hoàn chỉnh |
| **Messages** | `GET /pages/{page_id}/conversations/{conversation_id}/messages` | `FbMessage`, `FbMessageItem` | `bridge_SyncMessageOfConversation()` | ✅ Hoàn chỉnh |
| **Posts** | `GET /pages/{page_id}/posts` | `FbPost` | `BridgeV2_SyncNewPosts()`, `BridgeV2_SyncAllPosts()` | ✅ Hoàn chỉnh |

**Ghi chú:** Tất cả 4 loại dữ liệu chính đã được đồng bộ với cả incremental sync (mới) và backfill sync (cũ).

---

## ⚠️ Dữ Liệu Chưa Đồng Bộ - Cần Xem Xét

### 1. Comments trên Posts - ⚠️ CẦN KIỂM TRA

**Pancake API:**
- ❓ **Cần kiểm tra:** Pancake có API để lấy comments của posts không?
- Nếu có: `GET /pages/{page_id}/posts/{post_id}/comments` (cần xác nhận)

**FolkForm API:**
- ❌ **KHÔNG có collection riêng cho Comments**
- Comments có thể được lưu trong `panCakeData` của post (nếu Pancake trả về trong post data)

**Đánh giá:**
- ⭐⭐ **TÙY CHỌN** - Phụ thuộc vào nhu cầu
- Nếu cần quản lý comments riêng → cần tạo collection `FbComment` trong FolkForm
- Nếu chỉ cần xem comments → có thể lấy từ `panCakeData` của post

**Đề xuất:**
1. **Kiểm tra Pancake API:** Xem có endpoint riêng cho comments không
2. **Option 1:** Nếu comments đã có trong post data → không cần sync riêng
3. **Option 2:** Nếu có API riêng và cần quản lý comments → tạo collection `FbComment` trong FolkForm

---

### 2. Customers (Khách Hàng) - ⚠️ CẦN XEM XÉT

**Pancake API:**
- ✅ `GET /pages/{page_id}/page_customers`
- Có pagination, filter theo since/until
- Có thông tin: `psid`, `name`, `phone_numbers`, `birthday`, `gender`, `lives_in`, `notes`

**FolkForm API:**
- ❌ **KHÔNG có collection riêng cho Customers**
- Nhưng `FbConversation` có field `customerId` (optional)
- Customer data có thể được lưu trong `panCakeData` của conversation/message

**Đánh giá:**
- ⭐⭐ **TÙY CHỌN** - Phụ thuộc vào nhu cầu quản lý customers
- **Hiện tại:** Customer info đã có trong conversations/messages
- **Nếu cần:** Query customers độc lập, quản lý customer database → cần tạo collection mới

**Đề xuất:**
- **Option 1 (Khuyến nghị):** Không sync riêng, dùng data từ conversations
  - ✅ Đơn giản, không cần thay đổi backend
  - ✅ Customer data đã có sẵn trong conversations
- **Option 2:** Tạo collection `FbCustomer` trong FolkForm (cần backend support)
  - ✅ Cho phép query customers độc lập
  - ✅ Quản lý customer database tập trung
  - ❌ Cần thay đổi backend FolkForm

**Quyết định:** Tùy vào yêu cầu nghiệp vụ. Nếu không cần query customers độc lập → Option 1.

---

### 3. Tags (Thẻ) - ⚠️ CẦN XEM XÉT

**Pancake API:**
- ✅ `GET /pages/{page_id}/tags`
- Có thông tin: `id`, `text`, `color`, `lighten_color`

**FolkForm API:**
- ❌ **KHÔNG có collection riêng cho Tags**
- Nhưng `FbConversation` có field `tags` trong `panCakeData` (array)
- Tags được lưu dưới dạng array trong conversation

**Đánh giá:**
- ⭐⭐ **TÙY CHỌN** - Phụ thuộc vào nhu cầu quản lý tags
- **Hiện tại:** Tags đã có trong conversations
- **Nếu cần:** Quản lý tags tập trung, tạo/sửa/xóa tags → cần tạo collection mới

**Đề xuất:**
- **Option 1 (Khuyến nghị):** Không sync riêng, dùng data từ conversations
  - ✅ Đơn giản, tags đã có trong conversation data
  - ✅ Không cần thay đổi backend
- **Option 2:** Tạo collection `FbTag` trong FolkForm (cần backend support)
  - ✅ Quản lý tags tập trung (tạo/sửa/xóa)
  - ✅ Có thể gán tags cho conversations/posts
  - ❌ Cần thay đổi backend FolkForm

**Quyết định:** Tùy vào yêu cầu nghiệp vụ. Nếu chỉ cần xem tags → Option 1.

---

### 4. Reactions trên Posts - ✅ ĐÃ CÓ TRONG POST DATA

**Pancake API:**
- ✅ Reactions được trả về trong post data
- Field: `reactions` với `like_count`, `love_count`, etc.

**FolkForm API:**
- ✅ Reactions được lưu trong `panCakeData.reactions` của `FbPost`
- Không cần collection riêng

**Đánh giá:**
- ✅ **KHÔNG CẦN SYNC RIÊNG** - Đã có trong post data
- Reactions được cập nhật tự động khi sync posts

---

## ❌ Dữ Liệu Không Cần Đồng Bộ

### 1. Users (Người Dùng Pancake)
- **Lý do:** Users của Pancake là internal, không liên quan đến FolkForm
- **FolkForm:** Có hệ thống user riêng (Firebase-based)

### 2. Statistics (Thống Kê)
- **Lý do:** Statistics là dữ liệu analytics, không cần sync thường xuyên
- **Có thể:** Lấy real-time từ Pancake khi cần

### 3. Call Logs (Nhật Ký Cuộc Gọi)
- **Lý do:** Chỉ cần nếu muốn quản lý call logs riêng
- **Đánh giá:** ⭐ Tùy chọn, cần tạo collection `FbCallLog` nếu cần

### 4. Export Data
- **Lý do:** Export data là tính năng export, không phải sync
- **Ghi chú:** Conversations từ ads đã được sync qua conversations sync

### 5. PcOrder (Đơn Hàng)
- **Lý do:** Pancake API không có endpoint cho orders (theo tài liệu hiện tại)
- **FolkForm:** Có collection `PcOrder` nhưng có thể đến từ nguồn khác

---

## 🎯 Kết Luận và Đề Xuất

### Tình Trạng Hiện Tại
✅ **4/4 loại dữ liệu chính đã được đồng bộ hoàn chỉnh:**
- Pages ✅
- Conversations ✅
- Messages ✅
- Posts ✅

### Cần Quyết Định (3 loại)

1. **Comments trên Posts**
   - ⚠️ Cần kiểm tra Pancake API có endpoint riêng không
   - Nếu có và cần quản lý riêng → tạo collection `FbComment`

2. **Customers**
   - ⚠️ Tùy chọn - Phụ thuộc nhu cầu
   - Khuyến nghị: Không sync riêng (đã có trong conversations)

3. **Tags**
   - ⚠️ Tùy chọn - Phụ thuộc nhu cầu
   - Khuyến nghị: Không sync riêng (đã có trong conversations)

### Đề Xuất Hành Động

#### Priority 1: Kiểm Tra Comments API
1. Kiểm tra Pancake API documentation xem có endpoint cho comments không
2. Nếu có → đánh giá nhu cầu quản lý comments riêng
3. Nếu cần → đề xuất tạo collection `FbComment` trong FolkForm backend

#### Priority 2: Quyết Định Customers và Tags
1. Xác định nhu cầu nghiệp vụ:
   - Có cần query customers độc lập không?
   - Có cần quản lý tags tập trung không?
2. Nếu cần → đề xuất tạo collections mới trong FolkForm backend
3. Nếu không → giữ nguyên (dùng data từ conversations)

#### Priority 3: Tối Ưu Hóa (Nếu cần)
1. Xem xét sync comments nếu có API và cần quản lý riêng
2. Xem xét sync customers nếu cần customer database tập trung
3. Xem xét sync tags nếu cần quản lý tags tập trung

---

## 📝 Ghi Chú Kỹ Thuật

### Kiến Trúc Đồng Bộ Hiện Tại

**Incremental Sync (Mới):**
- `BridgeV2_SyncNewData()` - Conversations mới
- `BridgeV2_SyncNewPosts()` - Posts mới
- Chạy định kỳ (mỗi 5 phút) để sync dữ liệu mới nhất

**Backfill Sync (Cũ):**
- `BridgeV2_SyncAllData()` - Tất cả conversations cũ
- `BridgeV2_SyncAllPosts()` - Tất cả posts cũ
- Chạy định kỳ (mỗi ngày) để sync dữ liệu cũ

**Jobs:**
- `SyncIncrementalConversationsJob` - Sync conversations mới
- `SyncIncrementalPostsJob` - Sync posts mới
- `SyncBackfillConversationsJob` - Sync conversations cũ
- `SyncBackfillPostsJob` - Sync posts cũ

### Lưu Ý Khi Thêm Dữ Liệu Mới

1. **Tạo hàm Pancake API** trong `pancake.go`
2. **Tạo hàm FolkForm API** trong `folkform.go`
3. **Tạo hàm Bridge** trong `bridge.go` hoặc `bridge_v2.go`
4. **Thêm Jobs** nếu cần sync định kỳ
5. **Cập nhật scheduler** để chạy jobs mới

---

**Kết luận:** Hệ thống đã đồng bộ đầy đủ 4 loại dữ liệu chính. Các loại dữ liệu còn lại (Comments, Customers, Tags) là tùy chọn và phụ thuộc vào nhu cầu nghiệp vụ cụ thể.
