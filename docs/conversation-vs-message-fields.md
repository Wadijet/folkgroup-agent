# Phân Tích Fields: Conversation vs Messages

**Ngày:** 2025-01-XX  
**Mục đích:** Phân tích các field ở conversation level không có trong từng message để xác định dữ liệu có thể bị mất

---

## 📊 So Sánh Fields

### Fields CHỈ có ở Conversation Level (KHÔNG có trong từng message)

Từ data mẫu Pancake API response, các field sau **CHỈ có ở conversation level**:

#### 1. Customer Information (Thông tin khách hàng)
- `from` - Thông tin người gửi conversation (name, email, id)
- `conv_customers` - Danh sách customers trong conversation
- `customers` - Thông tin chi tiết customers (với personal_info, ad_clicks, etc.)
- `customer_id` - ID khách hàng (có thể extract riêng)
- `page_customer` - Thông tin customer của page (birthday, global_id, id, gender, name, customer_id, notes, psid, recent_orders)

#### 2. Conversation Metadata (Metadata conversation)
- `id` / `conversation_id` - ID conversation (có trong message nhưng là metadata conversation)
- `type` - Loại conversation (INBOX, COMMENT, etc.)
- `inserted_at` - Thời gian tạo conversation
- `updated_at` - Thời gian cập nhật conversation
- `page_id` - ID của page
- `seen` - Đã xem conversation chưa
- `has_phone` - Có số điện thoại không
- `snippet` - Snippet/preview của conversation
- `message_count` - Số lượng messages trong conversation
- `last_sent_by` - Người gửi tin nhắn cuối cùng (name, email, id)

#### 3. Phone Numbers (Số điện thoại)
- `conv_phone_numbers` - Số điện thoại trong conversation
- `conv_recent_phone_numbers` - Số điện thoại gần đây
- `recent_phone_numbers` - Số điện thoại gần đây của customer
- `available_for_report_phone_numbers` - Số điện thoại có thể báo cáo
- `reports_by_phone` - Báo cáo theo số điện thoại

#### 4. Profile Information (Thông tin profile)
- `gender` - Giới tính
- `birthday` - Sinh nhật
- `profile_updated_at` - Thời gian cập nhật profile
- `read_watermarks` - Watermarks đã đọc (message_id, watermark, psid)

#### 5. Activities & Engagement (Hoạt động & tương tác)
- `activities` - Hoạt động (ADS, OPEN_THREAD, etc.)
- `ad_clicks` - Clicks quảng cáo (theo customer_id)
- `comment_count` - Số lượng comment
- `last_commented_at` - Thời gian comment cuối

#### 6. Posts & Orders (Posts & đơn hàng)
- `post` - Post liên quan đến conversation
- `suggested_posts` - Posts gợi ý
- `recent_orders` - Đơn hàng gần đây

#### 7. Tags & Assignment (Tags & phân công)
- `tags` - Tags hiện tại của conversation
- `tag_histories` - Lịch sử thay đổi tags (ai add/remove, khi nào)
- `assignee_ids` - Danh sách người được assign
- `assignee_histories` - Lịch sử assign
- `current_assign_users` - Người được assign hiện tại

#### 8. Conversation Summary (Tóm tắt conversation)
- `snippet` - Snippet/preview của conversation
- `message_count` - Số lượng messages trong conversation
- `last_sent_by` - Người gửi tin nhắn cuối cùng (name, email, id)
- `seen` - Đã xem chưa

#### 9. Ads & Posts (Quảng cáo & posts)
- `ads` - Danh sách quảng cáo liên quan (post_id, ad_id, inserted_at)
- `ad_ids` - Danh sách ad IDs
- `post_id` - Post ID liên quan

#### 10. Page Customer (Thông tin customer của page)
- `page_customer` - Thông tin customer của page (birthday, global_id, id, gender, name, customer_id, notes, psid, recent_orders)

#### 11. Other (Khác)
- `extra_info` - Thông tin thêm
- `matched_wa_fb_customers` - Khách hàng khớp WA-FB
- `app` - App ID
- `allow_use_data_for_training_ai` - Cho phép dùng data để train AI
- `success` - Trạng thái success

---

### Fields CÓ trong cả Conversation VÀ Messages

- `conversation_id` - Có trong cả 2 (nhưng là metadata của conversation)
- `page_id` - Có trong cả 2
- `type` - Có trong cả 2 (INBOX, COMMENT, etc.)

---

## ⚠️ Dữ Liệu Có Thể Bị Mất

### Nếu CHỈ lưu từng message (không lưu conversation với panCakeData):

**Sẽ MẤT các dữ liệu quan trọng:**

1. **Customer Information:**
   - `from`, `conv_customers`, `customers` - Thông tin khách hàng chi tiết
   - `page_customer` - Thông tin customer của page (birthday, global_id, gender, name, notes, psid, recent_orders)
   - `global_id` - Global ID
   - `personal_info` (gender, birthday, profile_updated_at, etc.)

2. **Conversation Metadata:**
   - `snippet` - Preview conversation
   - `message_count` - Số lượng messages
   - `last_sent_by` - Người gửi cuối cùng
   - `seen` - Trạng thái đã xem
   - `has_phone` - Có số điện thoại không
   - `is_banned`, `banned_count`, `banned_by` - Trạng thái ban (nếu có)
   - `notes` - Ghi chú (nếu có)
   - `can_inbox` - Quyền inbox (nếu có)

3. **Phone Numbers:**
   - Tất cả các số điện thoại liên quan đến conversation

4. **Activities & Engagement:**
   - `activities` - Hoạt động (ADS clicks, etc.)
   - `ad_clicks` - Clicks quảng cáo
   - `read_watermarks` - Watermarks đã đọc

5. **Tags & Assignment:**
   - `tags` - Tags hiện tại
   - `tag_histories` - Lịch sử thay đổi tags
   - `assignee_ids`, `assignee_histories`, `current_assign_users` - Phân công

6. **Conversation Summary:**
   - `snippet` - Preview conversation
   - `message_count` - Số lượng messages
   - `last_sent_by` - Người gửi cuối cùng
   - `seen` - Trạng thái đã xem

7. **Ads & Posts:**
   - `ads`, `ad_ids` - Quảng cáo liên quan
   - `post` - Post liên quan
   - `recent_orders` - Đơn hàng gần đây

8. **Page Customer:**
   - `page_customer` - Thông tin customer của page

---

## ✅ Giải Pháp Đề Xuất

### Option 1: Lưu Conversation KHÔNG có messages[] (Đang làm)
- ✅ Lưu conversation với tất cả metadata (không có `messages[]`)
- ✅ Lưu từng message riêng lẻ vào collection `FbMessage`
- ✅ Không mất dữ liệu conversation metadata
- ✅ Không đè mất messages cũ

**Kết luận:** ✅ **ĐÂY LÀ GIẢI PHÁP ĐÚNG**

### Option 2: Lưu Conversation CÓ messages[] (KHÔNG NÊN)
- ❌ Mỗi lần upsert sẽ đè mất messages cũ
- ❌ Cần merge messages thủ công → phức tạp

### Option 3: Chỉ lưu Messages (KHÔNG NÊN)
- ❌ Mất tất cả metadata conversation (customer info, activities, etc.)
- ❌ Không có thông tin tổng quan về conversation

---

## 📝 Kết Luận

**Giải pháp hiện tại (Option 1) là ĐÚNG:**
- ✅ Lưu conversation với tất cả metadata (trừ `messages[]`)
- ✅ Lưu từng message riêng lẻ
- ✅ Không mất dữ liệu
- ✅ Không đè mất messages cũ

**KHÔNG cần thêm endpoint merge messages** vì:
- Messages đã được lưu riêng trong collection `FbMessage`
- Conversation chỉ cần metadata, không cần lưu toàn bộ messages trong `panCakeData.messages[]`
