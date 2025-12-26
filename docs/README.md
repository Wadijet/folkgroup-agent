# Tài Liệu Hệ Thống Sync

## 📚 Danh Sách Tài Liệu

### 1. Implementation Guides

#### `sync-implementation-guide.md` ⭐ **QUAN TRỌNG NHẤT**
**Mục đích:** Hướng dẫn chi tiết cách implement sync conversations với `since`/`until`

**Nội dung:**
- Tổng quan vấn đề và giải pháp
- Code implementation chi tiết (4 bước)
- So sánh trước và sau
- Edge cases và xử lý
- Checklist implementation

**Khi nào cần:**
- Khi implement sync incremental conversations
- Khi cần hiểu logic `since`/`until`

---

### 2. Analysis Documents

#### `system-evaluation.md`
**Mục đích:** Đánh giá tổng quan hệ thống hiện tại

**Nội dung:**
- Điểm mạnh và điểm yếu của kiến trúc
- Đánh giá các thành phần (retry, logging, scheduler, etc.)
- Khuyến nghị cải thiện

**Khi nào cần:**
- Khi cần đánh giá tổng quan hệ thống
- Khi cần roadmap cải thiện

---

#### `sync-coverage-analysis.md`
**Mục đích:** Phân tích dữ liệu nào đã sync, dữ liệu nào còn thiếu

**Nội dung:**
- So sánh Pancake API vs FolkForm collections
- Danh sách dữ liệu đã sync
- Danh sách dữ liệu còn thiếu (Posts, Customers, Tags, etc.)
- Priority implementation

**Khi nào cần:**
- Khi cần biết dữ liệu nào còn thiếu
- Khi cần plan sync thêm dữ liệu mới

---

#### `sync-issues-analysis.md`
**Mục đích:** Phân tích chi tiết các vấn đề trong logic sync hiện tại

**Nội dung:**
- Vấn đề với Messages pagination (chỉ lấy 30 messages đầu)
- Vấn đề với Conversations sync (có thể bỏ sót)
- Vấn đề với Upsert filter
- Giải pháp đề xuất

**Khi nào cần:**
- Khi cần hiểu các vấn đề hiện tại
- Khi cần fix bugs trong sync logic

---

#### `conversation-params-analysis.md`
**Mục đích:** Phân tích chi tiết các params của GetConversations API

**Nội dung:**
- Danh sách tất cả params có sẵn
- Phân tích từng param (order_by, type, tags, post_ids, unread_first)
- Ứng dụng cho từng scenario (sync tất cả, sync incremental, sync real-time)
- Đề xuất cải thiện code

**Khi nào cần:**
- Khi cần tối ưu sync với các params khác
- Khi cần filter conversations theo type, tags, etc.

---

### 3. API Documentation

**📍 Tài liệu API được quản lý tập trung tại `docs/ai-context/` (workspace-level)**

#### Pancake API Context
**Vị trí:** `../../docs/ai-context/pancake-api-context.md`

**Mục đích:** Tài liệu đầy đủ về Pancake API

**Nội dung:**
- Tất cả endpoints của Pancake API
- Request/Response structures
- Authentication
- Query parameters

**Khi nào cần:**
- Khi cần tra cứu Pancake API
- Khi implement sync dữ liệu mới

---

#### FolkForm API Context
**Vị trí:** `../../docs/ai-context/folkform-api-context.md`

**Mục đích:** Tài liệu đầy đủ về FolkForm API

**Nội dung:**
- Tất cả collections và models
- CRUD endpoints
- Data extraction mechanism
- Special endpoints (sort-by-api-update, etc.)

**Khi nào cần:**
- Khi cần tra cứu FolkForm API
- Khi cần hiểu data structure

---

#### Pancake POS API Context
**Vị trí:** `../../docs/ai-context/pancake-pos-api-context.md`

**Mục đích:** Tài liệu đầy đủ về Pancake POS API

**Nội dung:**
- Quản lý Shop và Warehouses
- Quản lý Orders và Customers
- Quản lý Products và Inventory
- Purchases, Transfers, Stocktakings

**Khi nào cần:**
- Khi cần tra cứu Pancake POS API
- Khi implement sync POS data

---

## 🎯 Quick Start

### Để implement sync incremental conversations:

1. **Đọc:** `sync-implementation-guide.md` - Hướng dẫn chi tiết
2. **Tham khảo:** `conversation-params-analysis.md` - Nếu cần tối ưu thêm
3. **Tra cứu:** `../../docs/ai-context/pancake-api-context.md` và `../../docs/ai-context/folkform-api-context.md` - Nếu cần chi tiết API

### Để đánh giá hệ thống:

1. **Đọc:** `system-evaluation.md` - Đánh giá tổng quan
2. **Đọc:** `sync-coverage-analysis.md` - Xem dữ liệu nào còn thiếu
3. **Đọc:** `sync-issues-analysis.md` - Xem các vấn đề cần fix

---

## 📝 Tóm Tắt Các Vấn Đề Chính

### Priority 1 (Cần fix ngay)
1. **Messages pagination** - Chỉ lấy 30 messages đầu tiên → Cần thêm `current_count`
2. **Conversations incremental sync** - Dùng `conversation_id` để dừng → Cần dùng `since`/`until`

### Priority 2 (Nên làm sớm)
1. **Thêm `order_by` params** - Đảm bảo thứ tự sắp xếp
2. **Thêm `type[]=INBOX`** - Tối ưu sync (chỉ sync inbox nếu không cần comment)

### Priority 3 (Có thể làm sau)
1. **Sync Posts** - Dữ liệu quan trọng nhưng chưa sync
2. **Sync Customers và Tags** - Nếu cần quản lý riêng

---

## 🔗 Liên Kết

### Tài Liệu Sync (Riêng cho Agent)
- **Implementation Guide:** `sync-implementation-guide.md`
- **System Evaluation:** `system-evaluation.md`
- **Coverage Analysis:** `sync-coverage-analysis.md`
- **Issues Analysis:** `sync-issues-analysis.md`
- **Params Analysis:** `conversation-params-analysis.md`

### API Documentation (Nguồn chính - Workspace-level)
- **AI Context README:** `../../docs/ai-context/README.md` ⭐ **BẮT ĐẦU TỪ ĐÂY**
- **Pancake API:** `../../docs/ai-context/pancake-api-context.md`
- **FolkForm API:** `../../docs/ai-context/folkform-api-context.md`
- **Pancake POS API:** `../../docs/ai-context/pancake-pos-api-context.md`

### Tài Liệu Khác
- **Workspace Docs:** `../../docs/README.md`
- **Backend Docs:** `../../ff_be_auth/docs/README.md`
