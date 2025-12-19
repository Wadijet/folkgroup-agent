# Phân Tích Chi Tiết Params GetConversations API

**Ngày phân tích:** 2025-01-XX  
**Mục đích:** Xem kỹ các params của GetConversations để tìm cách ứng dụng cải thiện logic sync

---

## 📋 Danh Sách Params Có Sẵn

### Pancake API: `GET /pages/{page_id}/conversations`

**Tất cả params:**
1. ✅ `page_access_token` (required) - **Đang dùng**
2. ✅ `last_conversation_id` (optional) - **Đang dùng** - Pagination cursor
3. ❌ `order_by` (optional) - **CHƯA DÙNG** - Sắp xếp: `inserted_at`, `updated_at`
4. ❌ `post_ids` (optional) - **CHƯA DÙNG** - Lọc theo post IDs (array)
5. ❌ `since` (optional) - **CHƯA DÙNG** - Lọc từ timestamp (giây)
6. ❌ `tags` (optional) - **CHƯA DÙNG** - Lọc theo tag IDs (phân cách bằng dấu phẩy)
7. ❌ `type` (optional) - **CHƯA DÙNG** - Lọc theo loại: INBOX, COMMENT (array)
8. ❌ `unread_first` (optional) - **CHƯA DÙNG** - Ưu tiên conversations chưa đọc
9. ❌ `until` (optional) - **CHƯA DÙNG** - Lọc đến timestamp (giây)

---

## 🔍 Phân Tích Từng Param

### 1. `order_by` - Sắp Xếp

**Giá trị:** `inserted_at` hoặc `updated_at`

**Ứng dụng:**

#### A. Sync Tất Cả - Dùng `order_by=inserted_at`
```go
// Sync từ mới đến cũ theo thời gian tạo
params["order_by"] = "inserted_at"
```

**Lợi ích:**
- ✅ Đảm bảo thứ tự nhất quán - sắp xếp theo thời gian tạo
- ✅ Tránh bỏ sót - không bị ảnh hưởng bởi conversation được update
- ✅ Dễ track - biết chính xác đã sync đến conversation nào

**So sánh:**
- **Không có `order_by`:** Pancake có thể sắp xếp theo `updated_at` mặc định → conversation cũ được update sẽ nhảy lên đầu
- **Có `order_by=inserted_at`:** Luôn sắp xếp theo thời gian tạo → ổn định hơn

#### B. Sync Incremental - Dùng `order_by=updated_at`
```go
// Sync conversations được update gần đây
params["order_by"] = "updated_at"
```

**Lợi ích:**
- ✅ Lấy conversations có thay đổi gần đây
- ✅ Phù hợp cho sync incremental - chỉ sync conversations có update

**Kết luận:** 
- **Sync tất cả:** Nên dùng `order_by=inserted_at` để đảm bảo thứ tự
- **Sync incremental:** Nên dùng `order_by=updated_at` + `since`/`until`

---

### 2. `type` - Lọc Theo Loại

**Giá trị:** Array[string] - `["INBOX"]`, `["COMMENT"]`, `["LIVESTREAM"]`

**Ứng dụng:**

#### A. Chỉ Sync INBOX Conversations
```go
// Chỉ lấy conversations từ inbox (không lấy comment, livestream)
params["type[]"] = "INBOX"
```

**Lợi ích:**
- ✅ Giảm dữ liệu không cần thiết - nếu chỉ cần inbox
- ✅ Tăng tốc độ sync - ít dữ liệu hơn
- ✅ Tập trung vào dữ liệu quan trọng

**Khi nào cần:**
- Nếu chỉ quan tâm đến inbox messages
- Không cần sync conversations từ comments trên posts

#### B. Sync Tất Cả Loại
```go
// Không set type → lấy tất cả (INBOX, COMMENT, LIVESTREAM)
// Hoặc set nhiều loại
params["type[]"] = "INBOX,COMMENT"
```

**Kết luận:**
- **Nếu chỉ cần inbox:** Dùng `type[]=INBOX` để tối ưu
- **Nếu cần tất cả:** Không set hoặc set nhiều loại

---

### 3. `unread_first` - Ưu Tiên Chưa Đọc

**Giá trị:** `true` hoặc `false`

**Ứng dụng:**

#### A. Sync Ưu Tiên Conversations Chưa Đọc
```go
// Ưu tiên lấy conversations chưa đọc trước
params["unread_first"] = "true"
```

**Lợi ích:**
- ✅ Sync conversations quan trọng trước (chưa đọc)
- ✅ Phù hợp cho real-time sync - ưu tiên xử lý conversations mới

**Khi nào cần:**
- Sync real-time - cần xử lý conversations chưa đọc ngay
- Priority sync - ưu tiên conversations quan trọng

**Kết luận:**
- **Sync real-time:** Nên dùng `unread_first=true`
- **Sync tất cả:** Không cần (hoặc `false`)

---

### 4. `tags` - Lọc Theo Tags

**Giá trị:** String - tag IDs phân cách bằng dấu phẩy (ví dụ: `"1,2,3"`)

**Ứng dụng:**

#### A. Sync Conversations Có Tag Cụ Thể
```go
// Chỉ sync conversations có tag "urgent" hoặc "important"
params["tags"] = "tag_id_1,tag_id_2"
```

**Lợi ích:**
- ✅ Sync có chọn lọc - chỉ sync conversations quan trọng
- ✅ Tối ưu hiệu suất - ít dữ liệu hơn

**Khi nào cần:**
- Chỉ cần sync conversations có tag cụ thể
- Filter theo business logic (ví dụ: chỉ sync conversations có tag "cần xử lý")

**Kết luận:**
- **Nếu cần filter theo tag:** Dùng `tags` param
- **Nếu sync tất cả:** Không cần

---

### 5. `post_ids` - Lọc Theo Post IDs

**Giá trị:** Array[string] - Danh sách post IDs

**Ứng dụng:**

#### A. Sync Conversations Từ Post Cụ Thể
```go
// Chỉ sync conversations từ comments trên post cụ thể
params["post_ids[]"] = "post_id_1,post_id_2"
```

**Lợi ích:**
- ✅ Sync có chọn lọc - chỉ sync conversations từ posts cụ thể
- ✅ Tối ưu - ít dữ liệu hơn

**Khi nào cần:**
- Chỉ cần sync conversations từ một số posts cụ thể
- Filter theo campaign hoặc post quan trọng

**Kết luận:**
- **Nếu cần filter theo post:** Dùng `post_ids`
- **Nếu sync tất cả:** Không cần

---

### 6. `since` và `until` - Lọc Theo Thời Gian

**Giá trị:** Integer (Unix timestamp, giây)

**Ứng dụng:**

#### A. Sync Incremental - Chỉ Sync Mới
```go
// Lấy panCakeUpdatedAt cuối cùng từ FolkForm
lastUpdatedAt := getLastPanCakeUpdatedAt(page_id)

// Sync từ lastUpdatedAt đến hiện tại
params["since"] = strconv.FormatInt(lastUpdatedAt, 10)
params["until"] = strconv.FormatInt(time.Now().Unix(), 10)
```

**Lợi ích:**
- ✅ Sync incremental hiệu quả - chỉ sync mới từ lần cuối
- ✅ Không bỏ sót - lấy tất cả conversations trong khoảng thời gian
- ✅ Có thể resume - từ bất kỳ thời điểm nào

#### B. Sync Theo Khoảng Thời Gian
```go
// Sync conversations trong 1 tuần qua
since := time.Now().AddDate(0, 0, -7).Unix()
until := time.Now().Unix()
params["since"] = strconv.FormatInt(since, 10)
params["until"] = strconv.FormatInt(until, 10)
```

**Lợi ích:**
- ✅ Sync có giới hạn - không sync quá nhiều dữ liệu cũ
- ✅ Tối ưu - chỉ sync dữ liệu cần thiết

**Kết luận:**
- **Sync incremental:** **CẦN** `since`/`until` để track chính xác
- **Sync tất cả:** Không cần (nhưng có thể dùng để giới hạn)

---

## 💡 Ứng Dụng Cụ Thể Cho Hệ Thống

### Scenario 1: Sync Tất Cả (`Bridge_SyncConversationsFromCloud`)

**Mục đích:** Sync tất cả conversations từ đầu đến giờ

**Params nên dùng:**
```go
params := map[string]string{
    "page_access_token": page_access_token,
    "last_conversation_id": last_conversation_id,
    "order_by": "inserted_at",  // ✅ Đảm bảo thứ tự
    "type[]": "INBOX",          // ✅ Chỉ sync inbox (nếu chỉ cần inbox)
}
```

**Lợi ích:**
- ✅ `order_by=inserted_at` → Đảm bảo thứ tự nhất quán, không bị ảnh hưởng bởi update
- ✅ `type[]=INBOX` → Chỉ sync inbox (nếu không cần comment/livestream)

---

### Scenario 2: Sync Incremental (`Sync_NewMessagesOfPage`)

**Mục đích:** Chỉ sync conversations mới từ lần sync cuối

**Params nên dùng:**
```go
// Lấy panCakeUpdatedAt cuối cùng từ FolkForm
lastUpdatedAt := getLastPanCakeUpdatedAt(page_id)

params := map[string]string{
    "page_access_token": page_access_token,
    "order_by": "updated_at",   // ✅ Sắp xếp theo updated_at
    "since": strconv.FormatInt(lastUpdatedAt, 10),  // ✅ Từ lần sync cuối
    "until": strconv.FormatInt(time.Now().Unix(), 10), // ✅ Đến hiện tại
    "type[]": "INBOX",          // ✅ Chỉ inbox (nếu chỉ cần inbox)
}
```

**Lợi ích:**
- ✅ `order_by=updated_at` → Lấy conversations có update gần đây
- ✅ `since`/`until` → Chỉ lấy conversations trong khoảng thời gian
- ✅ Không bỏ sót - lấy tất cả conversations có `updated_at` trong khoảng

---

### Scenario 3: Sync Real-Time (Ưu Tiên Chưa Đọc)

**Mục đích:** Sync conversations chưa đọc ngay lập tức

**Params nên dùng:**
```go
params := map[string]string{
    "page_access_token": page_access_token,
    "unread_first": "true",     // ✅ Ưu tiên chưa đọc
    "order_by": "updated_at",   // ✅ Sắp xếp theo updated_at
    "type[]": "INBOX",          // ✅ Chỉ inbox
}
```

**Lợi ích:**
- ✅ `unread_first=true` → Ưu tiên conversations chưa đọc
- ✅ Xử lý conversations quan trọng trước

---

### Scenario 4: Sync Theo Tag (Business Logic)

**Mục đích:** Chỉ sync conversations có tag cụ thể

**Params nên dùng:**
```go
// Ví dụ: chỉ sync conversations có tag "urgent" hoặc "cần xử lý"
urgentTagId := "123"
importantTagId := "456"

params := map[string]string{
    "page_access_token": page_access_token,
    "tags": urgentTagId + "," + importantTagId,  // ✅ Lọc theo tags
    "order_by": "updated_at",
}
```

**Lợi ích:**
- ✅ Sync có chọn lọc - chỉ conversations quan trọng
- ✅ Tối ưu hiệu suất

---

## 📊 Bảng Tổng Hợp Ứng Dụng

| Param | Sync Tất Cả | Sync Incremental | Sync Real-Time | Khi Nào Cần |
|-------|-------------|------------------|----------------|-------------|
| `order_by=inserted_at` | ✅ **Nên dùng** | ❌ | ❌ | Đảm bảo thứ tự nhất quán |
| `order_by=updated_at` | ❌ | ✅ **Nên dùng** | ✅ **Nên dùng** | Sync conversations có update |
| `since`/`until` | ❌ | ✅ **CẦN** | ✅ **Có thể** | Track timestamp, sync incremental |
| `type[]=INBOX` | ✅ **Nên dùng** | ✅ **Nên dùng** | ✅ **Nên dùng** | Nếu chỉ cần inbox |
| `unread_first=true` | ❌ | ❌ | ✅ **Nên dùng** | Ưu tiên conversations chưa đọc |
| `tags` | ⚠️ Tùy chọn | ⚠️ Tùy chọn | ⚠️ Tùy chọn | Nếu cần filter theo tag |
| `post_ids` | ⚠️ Tùy chọn | ⚠️ Tùy chọn | ⚠️ Tùy chọn | Nếu cần filter theo post |

---

## 🎯 Đề Xuất Cải Thiện Code

### 1. Cải Thiện `Pancake_GetConversations_v2`

**Hiện tại:**
```go
func Pancake_GetConversations_v2(page_id string, last_conversation_id string)
```

**Đề xuất:**
```go
type ConversationQueryParams struct {
    LastConversationId string
    OrderBy            string  // "inserted_at" hoặc "updated_at"
    Since              int64   // Unix timestamp
    Until              int64   // Unix timestamp
    Type               []string // ["INBOX"], ["COMMENT"], etc.
    Tags               []string // Tag IDs
    PostIds            []string // Post IDs
    UnreadFirst        bool
}

func Pancake_GetConversations_v2(page_id string, params ConversationQueryParams) (result map[string]interface{}, err error) {
    // Build params map
    queryParams := map[string]string{
        "page_access_token": page_access_token,
    }
    
    if params.LastConversationId != "" {
        queryParams["last_conversation_id"] = params.LastConversationId
    }
    if params.OrderBy != "" {
        queryParams["order_by"] = params.OrderBy
    }
    if params.Since > 0 {
        queryParams["since"] = strconv.FormatInt(params.Since, 10)
    }
    if params.Until > 0 {
        queryParams["until"] = strconv.FormatInt(params.Until, 10)
    }
    if len(params.Type) > 0 {
        queryParams["type[]"] = strings.Join(params.Type, ",")
    }
    if len(params.Tags) > 0 {
        queryParams["tags"] = strings.Join(params.Tags, ",")
    }
    if len(params.PostIds) > 0 {
        queryParams["post_ids[]"] = strings.Join(params.PostIds, ",")
    }
    if params.UnreadFirst {
        queryParams["unread_first"] = "true"
    }
    
    // ... rest of code
}
```

---

### 2. Cải Thiện `bridge_SyncConversationsOfPage`

**Hiện tại:**
```go
func bridge_SyncConversationsOfPage(page_id string, page_username string) {
    last_conversation_id := ""
    for {
        result := Pancake_GetConversations_v2(page_id, last_conversation_id)
        // ...
    }
}
```

**Đề xuất:**
```go
func bridge_SyncConversationsOfPage(page_id string, page_username string) {
    params := ConversationQueryParams{
        OrderBy: "inserted_at",  // ✅ Đảm bảo thứ tự
        Type:    []string{"INBOX"}, // ✅ Chỉ inbox (nếu chỉ cần inbox)
    }
    
    for {
        result := Pancake_GetConversations_v2(page_id, params)
        // ...
        params.LastConversationId = last_conversation_id
    }
}
```

---

### 3. Cải Thiện `Sync_NewMessagesOfPage`

**Hiện tại:**
```go
func Sync_NewMessagesOfPage(page_id string, page_username string) {
    conversation_id_updated := getLastConversationId(page_id)
    // Dùng conversation_id để dừng → CÓ VẤN ĐỀ
}
```

**Đề xuất:**
```go
func Sync_NewMessagesOfPage(page_id string, page_username string) {
    // Lấy panCakeUpdatedAt cuối cùng từ FolkForm
    lastUpdatedAt := getLastPanCakeUpdatedAt(page_id)
    
    params := ConversationQueryParams{
        OrderBy: "updated_at",  // ✅ Sắp xếp theo updated_at
        Since:   lastUpdatedAt, // ✅ Từ lần sync cuối
        Until:   time.Now().Unix(), // ✅ Đến hiện tại
        Type:    []string{"INBOX"}, // ✅ Chỉ inbox
    }
    
    for {
        result := Pancake_GetConversations_v2(page_id, params)
        // ... process conversations
        params.LastConversationId = last_conversation_id
    }
}
```

---

## 📝 Checklist Ứng Dụng Params

### Priority 1 (Cao - Nên làm ngay)
- [ ] Thêm `order_by=inserted_at` cho sync tất cả
- [ ] Thêm `order_by=updated_at` + `since`/`until` cho sync incremental
- [ ] Thêm `type[]=INBOX` nếu chỉ cần inbox

### Priority 2 (Trung bình - Nên làm sớm)
- [ ] Thêm `unread_first=true` cho sync real-time
- [ ] Refactor hàm `Pancake_GetConversations_v2` để nhận struct params

### Priority 3 (Thấp - Tùy chọn)
- [ ] Thêm `tags` filter nếu cần filter theo tag
- [ ] Thêm `post_ids` filter nếu cần filter theo post

---

## 🎯 Kết Luận

### Params Quan Trọng Nhất

1. **`order_by`** - ⭐⭐⭐ **RẤT QUAN TRỌNG**
   - `inserted_at` cho sync tất cả → Đảm bảo thứ tự
   - `updated_at` cho sync incremental → Lấy conversations có update

2. **`since`/`until`** - ⭐⭐⭐ **RẤT QUAN TRỌNG**
   - Cần cho sync incremental → Track timestamp chính xác
   - Tránh bỏ sót conversations

3. **`type[]`** - ⭐⭐ **QUAN TRỌNG**
   - Nếu chỉ cần inbox → Dùng `type[]=INBOX` để tối ưu

4. **`unread_first`** - ⭐ **TÙY CHỌN**
   - Chỉ cần nếu sync real-time và ưu tiên chưa đọc

5. **`tags`/`post_ids`** - ⭐ **TÙY CHỌN**
   - Chỉ cần nếu có business logic filter cụ thể

---

**Khuyến nghị:** 
- **Ưu tiên thêm `order_by` và `since`/`until`** vì đây là params quan trọng nhất
- Sau đó thêm `type[]=INBOX` nếu chỉ cần inbox
- Các params khác (`tags`, `post_ids`, `unread_first`) tùy vào nhu cầu cụ thể
