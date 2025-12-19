# Phân Tích Vấn Đề Đồng Bộ Dữ Liệu

**Ngày phân tích:** 2025-01-XX  
**Mục đích:** Rà soát logic sync hiện tại để đảm bảo lấy đủ dữ liệu và không bị trùng lặp

---

## 🔍 Phân Tích API Params

### Pancake API - Conversations

**Params có sẵn:**
- ✅ `last_conversation_id` (string, optional) - Pagination cursor
- ❌ `since` (integer, optional) - Lọc từ timestamp (giây) - **CHƯA DÙNG**
- ❌ `until` (integer, optional) - Lọc đến timestamp (giây) - **CHƯA DÙNG**
- ❌ `order_by` (string, optional) - Sắp xếp: `inserted_at`, `updated_at` - **CHƯA DÙNG**
- ❌ `type` (array[string], optional) - Lọc theo loại: INBOX, COMMENT - **CHƯA DÙNG**

**Code hiện tại:**
```go
// Chỉ dùng last_conversation_id
func Pancake_GetConversations_v2(page_id string, last_conversation_id string)
```

**Vấn đề:**
1. ⚠️ Không dùng `since`/`until` → không thể sync theo khoảng thời gian cụ thể
2. ⚠️ Không dùng `order_by` → không kiểm soát được thứ tự
3. ⚠️ Logic sync dựa trên `last_conversation_id` có thể bỏ sót nếu có conversation mới được insert giữa chừng

---

### Pancake API - Messages

**Params có sẵn:**
- ❌ `current_count` (number, optional) - Pagination: vị trí index để lấy 30 tin nhắn trước đó - **CHƯA DÙNG**

**Code hiện tại:**
```go
// KHÔNG dùng current_count → chỉ lấy 30 messages đầu tiên
func Pancake_GetMessages(page_id string, conversation_id string, customer_id string)
```

**Vấn đề NGHIÊM TRỌNG:**
1. ❌ **CHỈ LẤY 30 MESSAGES ĐẦU TIÊN** - Nếu conversation có > 30 messages → **BỎ SÓT**
2. ❌ Không có pagination cho messages → không lấy hết lịch sử

---

### Pancake API - Posts

**Params có sẵn:**
- `page_number` (integer, required) - Số trang
- `page_size` (integer, required) - Kích thước trang (tối đa 30)
- `since` (integer, required) - Thời gian bắt đầu (Unix timestamp)
- `until` (integer, required) - Thời gian kết thúc (Unix timestamp)
- `type` (string, optional) - Lọc theo loại: video, photo, text, livestream

**Code hiện tại:**
- ❌ **CHƯA CÓ** - Chưa implement sync posts

---

### Pancake API - Customers

**Params có sẵn:**
- `page_number` (integer, required)
- `page_size` (integer, optional, max 100)
- `since` (integer<int64>, required) - Thời gian bắt đầu
- `until` (integer<int64>, required) - Thời gian kết thúc
- `order_by` (string, optional) - Sắp xếp: `inserted_at`, `updated_at`

**Code hiện tại:**
- ❌ **CHƯA CÓ** - Chưa implement sync customers

---

## ⚠️ Vấn Đề Nghiêm Trọng

### 1. Messages - CHỈ LẤY 30 MESSAGES ĐẦU TIÊN

**Mức độ:** 🔴 **RẤT NGHIÊM TRỌNG**

**Vấn đề:**
- API Pancake trả về tối đa 30 messages mỗi lần
- Cần dùng `current_count` để pagination
- Code hiện tại **KHÔNG DÙNG** `current_count` → chỉ lấy 30 messages đầu tiên

**Ví dụ:**
- Conversation có 100 messages
- Code hiện tại chỉ lấy 30 messages đầu (mới nhất)
- **BỎ SÓT 70 messages cũ**

**Giải pháp:**
```go
// Cần implement pagination cho messages
func Pancake_GetMessages(page_id string, conversation_id string, customer_id string, current_count int) (result map[string]interface{}, err error)

// Logic pagination
current_count := 0
for {
    result, err := Pancake_GetMessages(page_id, conversation_id, customer_id, current_count)
    messages := result["messages"].([]interface{})
    if len(messages) == 0 {
        break
    }
    // Process messages
    current_count += len(messages)
    if len(messages) < 30 {
        break // Đã lấy hết
    }
}
```

---

### 2. Conversations - Logic Sync Có Thể Bỏ Sót

**Mức độ:** 🟡 **TRUNG BÌNH**

**Vấn đề 1: Sync tất cả (`Bridge_SyncConversationsFromCloud`)**
- Dùng `last_conversation_id` để pagination
- Logic: lấy 60 conversations → lấy `last_conversation_id` → lấy tiếp 60 conversations cũ hơn
- **Vấn đề:** Nếu có conversation mới được insert vào giữa (ví dụ: conversation cũ được update) → có thể bỏ sót

**Vấn đề 2: Sync mới (`Sync_NewMessagesOfPage`)**
- Dùng `conversation_id_updated` để dừng khi gặp conversation đã có
- **Vấn đề:** Nếu có conversation mới hơn được insert vào giữa → có thể bỏ sót

**Giải pháp:**
- Dùng `since`/`until` với timestamp thay vì `last_conversation_id`
- Hoặc dùng `order_by=updated_at` và track `updated_at` cuối cùng

---

### 3. Không Dùng Since/Until - Không Sync Theo Khoảng Thời Gian

**Mức độ:** 🟡 **TRUNG BÌNH**

**Vấn đề:**
- Pancake API hỗ trợ `since` và `until` để lọc theo timestamp
- Code hiện tại không dùng → không thể sync theo khoảng thời gian cụ thể
- Không thể resume sync từ một thời điểm cụ thể

**Giải pháp:**
- Thêm params `since` và `until` vào các hàm Pancake API
- Track `panCakeUpdatedAt` cuối cùng để sync incremental

---

### 4. Upsert Logic - Có Thể Bị Trùng

**Mức độ:** 🟢 **THẤP** (đã có upsert nhưng cần kiểm tra)

**Vấn đề:**
- Đã dùng upsert → không bị trùng về mặt insert
- Nhưng filter có thể không đúng:

**Conversations:**
- ✅ Filter: `conversationId` (unique) → OK

**Messages:**
- ⚠️ Filter: `conversationId + pageId` → Có thể không đủ unique
- **Vấn đề:** Nếu có nhiều messages trong cùng conversation → có thể update nhầm
- **Cần:** Filter theo `messageId` (từ `panCakeData.id` hoặc `panCakeData.message_id`)

**Pages:**
- ✅ Filter: `pageId` (unique) → OK

---

## 📊 Bảng Tổng Hợp Vấn Đề

| Loại Dữ Liệu | Vấn Đề | Mức Độ | Giải Pháp |
|-------------|--------|--------|-----------|
| **Messages** | Chỉ lấy 30 messages đầu tiên | 🔴 Rất nghiêm trọng | Thêm pagination với `current_count` |
| **Conversations** | Logic sync có thể bỏ sót | 🟡 Trung bình | Dùng `since`/`until` với timestamp |
| **Conversations** | Không dùng `since`/`until` | 🟡 Trung bình | Thêm params `since`/`until` |
| **Messages** | Filter upsert có thể không đúng | 🟡 Trung bình | Kiểm tra và sửa filter |
| **Posts** | Chưa implement | 🟡 Trung bình | Implement sync posts |

---

## 🔧 Đề Xuất Sửa Lỗi

### Priority 1 (Cao - Cần sửa ngay)

#### 1. Sửa Messages Pagination

**File:** `app/integrations/pancake.go`

```go
// Thêm param current_count
func Pancake_GetMessages(page_id string, conversation_id string, customer_id string, current_count int) (result map[string]interface{}, err error) {
    // ...
    params := map[string]string{
        "page_access_token": page_access_token,
        "customer_id":       customer_id,
    }
    if current_count > 0 {
        params["current_count"] = strconv.Itoa(current_count)
    }
    // ...
}
```

**File:** `app/integrations/bridge.go`

```go
// Sửa hàm bridge_SyncMessageOfConversation để lấy hết messages
func bridge_SyncMessageOfConversation(page_id string, page_username string, conversation_id string, customer_id string) (resultErr error) {
    current_count := 0
    for {
        resultGetMessages, err := Pancake_GetMessages(page_id, conversation_id, customer_id, current_count)
        if err != nil {
            logError("Lỗi khi lấy danh sách tin nhắn từ server Pancake: %v", err)
            break
        }
        
        messages := resultGetMessages["messages"].([]interface{})
        if len(messages) == 0 {
            break
        }
        
        // Process messages
        _, err = FolkForm_CreateMessage(page_id, page_username, conversation_id, customer_id, resultGetMessages)
        if err != nil {
            logError("Lỗi khi tạo tin nhắn trên server FolkForm: %v", err)
            break
        }
        
        current_count += len(messages)
        if len(messages) < 30 {
            break // Đã lấy hết
        }
    }
    
    return nil
}
```

#### 2. Sửa Messages Upsert Filter

**File:** `app/integrations/folkform.go`

```go
// Sửa filter để dùng messageId thay vì chỉ conversationId
func FolkForm_CreateMessage(...) {
    // ...
    var messageId string
    if messageMap, ok := messageData.(map[string]interface{}); ok {
        // Lấy messageId từ panCakeData
        if id, ok := messageMap["id"].(string); ok && id != "" {
            messageId = id
        }
    }
    
    // Filter theo messageId (unique) thay vì conversationId
    if messageId != "" {
        filter := `{"messageId":"` + messageId + `"}`
        params["filter"] = filter
    }
    // ...
}
```

**Lưu ý:** Cần kiểm tra FolkForm backend có field `messageId` không, hoặc extract từ `panCakeData.id`

---

### Priority 2 (Trung bình - Nên sửa sớm)

#### 3. Thêm Since/Until cho Conversations

**File:** `app/integrations/pancake.go`

```go
// Thêm params since và until
func Pancake_GetConversations_v2(page_id string, last_conversation_id string, since int64, until int64) (result map[string]interface{}, err error) {
    // ...
    params := map[string]string{
        "page_access_token":    page_access_token,
        "last_conversation_id": last_conversation_id,
    }
    if since > 0 {
        params["since"] = strconv.FormatInt(since, 10)
    }
    if until > 0 {
        params["until"] = strconv.FormatInt(until, 10)
    }
    // ...
}
```

#### 4. Cải thiện Logic Sync Mới

**File:** `app/integrations/bridge.go`

```go
// Dùng timestamp thay vì conversation_id để track
func Sync_NewMessagesOfPage(page_id string, page_username string) (resultErr error) {
    // Lấy conversation mới nhất từ FolkForm
    lastUpdatedAt := int64(0) // Thay vì conversation_id_updated
    
    resultGetConversations, err := FolkForm_GetConversationsWithPageId(1, 1, page_id)
    // ... parse để lấy panCakeUpdatedAt cuối cùng
    
    // Sync từ Pancake với since = lastUpdatedAt
    since := lastUpdatedAt
    until := time.Now().Unix()
    
    for {
        resultGetConversations, err := Pancake_GetConversations_v2(page_id, "", since, until)
        // ... process conversations
    }
}
```

---

### Priority 3 (Thấp - Có thể làm sau)

#### 5. Thêm Order By

```go
// Thêm order_by param
params["order_by"] = "updated_at" // hoặc "inserted_at"
```

#### 6. Thêm Type Filter

```go
// Chỉ sync INBOX conversations
params["type[]"] = "INBOX"
```

---

## 📝 Checklist Sửa Lỗi

### Messages
- [ ] Thêm `current_count` param vào `Pancake_GetMessages()`
- [ ] Implement pagination loop trong `bridge_SyncMessageOfConversation()`
- [ ] Test với conversation có > 30 messages
- [ ] Sửa filter upsert để dùng `messageId` thay vì `conversationId`

### Conversations
- [ ] Thêm `since` và `until` params vào `Pancake_GetConversations_v2()`
- [ ] Cải thiện `Sync_NewMessagesOfPage()` để dùng timestamp
- [ ] Test sync không bỏ sót conversations

### Posts (nếu implement)
- [ ] Dùng đầy đủ params: `page_number`, `page_size`, `since`, `until`
- [ ] Implement pagination đầy đủ

---

## 🎯 Kết Luận

### Vấn Đề Nghiêm Trọng Nhất
1. **Messages chỉ lấy 30 messages đầu tiên** → Cần sửa ngay
2. **Messages filter upsert có thể không đúng** → Cần kiểm tra và sửa

### Vấn Đề Trung Bình
3. **Conversations không dùng since/until** → Nên sửa sớm
4. **Logic sync có thể bỏ sót** → Cải thiện logic

### Khuyến Nghị
- **Ưu tiên sửa Messages pagination** vì đây là vấn đề nghiêm trọng nhất
- Sau đó sửa filter upsert để đảm bảo không trùng
- Cuối cùng cải thiện logic sync conversations
