# Hướng Dẫn Implementation Sync Conversations với Since/Until

**Ngày:** 2025-01-XX  
**Mục đích:** Tài liệu tổng hợp về chiến lược và implementation sync conversations với `since`/`until` params

---

## 📋 Tổng Quan

### Vấn Đề Hiện Tại

**Code hiện tại (`Sync_NewMessagesOfPage`):**
- ❌ Dùng `conversation_id` để dừng sync → Có thể bỏ sót conversations mới
- ❌ Không track timestamp → Không biết đã sync đến thời điểm nào
- ❌ Nếu conversation cũ được update → có thể bỏ sót conversations mới thực sự

### Giải Pháp

**Dùng `since`/`until` với Timestamp:**
1. Lấy `panCakeUpdatedAt` cuối cùng từ FolkForm
2. Sync từ `lastUpdatedAt` → `now` bằng `since`/`until`
3. Lấy tất cả conversations có `updated_at` trong khoảng thời gian này

---

## 🔧 Implementation

### Bước 1: Sửa `Pancake_GetConversations_v2` - Thêm Params `since`/`until`

**File:** `app/integrations/pancake.go`

```go
func Pancake_GetConversations_v2(page_id string, last_conversation_id string, since int64, until int64) (result map[string]interface{}, err error) {
    // ... existing code ...
    
    // Thiết lập params
    params := map[string]string{
        "page_access_token":    page_access_token,
        "last_conversation_id": last_conversation_id,
    }
    
    // Thêm since/until nếu có
    if since > 0 {
        params["since"] = strconv.FormatInt(since, 10)
        log.Printf("[Pancake] [Lần thử %d/5] Thêm param since: %d", requestCount, since)
    }
    if until > 0 {
        params["until"] = strconv.FormatInt(until, 10)
        log.Printf("[Pancake] [Lần thử %d/5] Thêm param until: %d", requestCount, until)
    }
    
    // ... rest of code ...
}
```

**Lưu ý:**
- `since` và `until` là Unix timestamp (giây)
- Nếu `since <= 0` hoặc `until <= 0` → không thêm param (optional)
- Giữ backward compatibility - code cũ vẫn hoạt động nếu truyền `0, 0`

---

### Bước 2: Tạo Helper Function - Lấy `panCakeUpdatedAt` Từ FolkForm

**File:** `app/integrations/bridge.go`

```go
// getLastPanCakeUpdatedAt lấy panCakeUpdatedAt cuối cùng từ FolkForm cho một page
// Trả về Unix timestamp (giây), hoặc 0 nếu không tìm thấy
func getLastPanCakeUpdatedAt(page_id string) int64 {
    log.Printf("[Bridge] Lấy panCakeUpdatedAt cuối cùng từ FolkForm cho page_id: %s", page_id)
    
    // Lấy conversations từ FolkForm (sắp xếp theo panCakeUpdatedAt giảm dần với -1)
    // Có thể dùng limit=1 vì items[0] đã là conversation mới nhất
    resultGetConversations, err := FolkForm_GetConversationsWithPageId(1, 1, page_id)
    if err != nil {
        logError("[Bridge] Lỗi khi lấy conversations từ FolkForm: %v", err)
        return 0
    }
    
    // Parse response
    var items []interface{}
    if dataMap, ok := resultGetConversations["data"].(map[string]interface{}); ok {
        if itemCount, ok := dataMap["itemCount"].(float64); ok && itemCount > 0 {
            if itemsArray, ok := dataMap["items"].([]interface{}); ok {
                items = itemsArray
            }
        }
    } else if dataArray, ok := resultGetConversations["data"].([]interface{}); ok {
        items = dataArray
    }
    
    if len(items) == 0 {
        log.Printf("[Bridge] Không tìm thấy conversation nào trong FolkForm cho page_id: %s", page_id)
        return 0
    }
    
    // Lấy item đầu tiên (mới nhất) vì API sắp xếp giảm dần (panCakeUpdatedAt: -1)
    // items[0] = conversation mới nhất (panCakeUpdatedAt lớn nhất)
    firstItem := items[0]
    if conversation, ok := firstItem.(map[string]interface{}); ok {
        // panCakeUpdatedAt có thể là number (float64) hoặc int64
        if panCakeUpdatedAt, ok := conversation["panCakeUpdatedAt"].(float64); ok {
            result := int64(panCakeUpdatedAt)
            log.Printf("[Bridge] Tìm thấy panCakeUpdatedAt mới nhất: %d (Unix timestamp)", result)
            // Convert sang time để log dễ đọc
            lastUpdatedTime := time.Unix(result, 0)
            log.Printf("[Bridge] Thời gian tương ứng: %s", lastUpdatedTime.Format("2006-01-02 15:04:05"))
            return result
        } else if panCakeUpdatedAt, ok := conversation["panCakeUpdatedAt"].(int64); ok {
            log.Printf("[Bridge] Tìm thấy panCakeUpdatedAt mới nhất: %d (Unix timestamp)", panCakeUpdatedAt)
            lastUpdatedTime := time.Unix(panCakeUpdatedAt, 0)
            log.Printf("[Bridge] Thời gian tương ứng: %s", lastUpdatedTime.Format("2006-01-02 15:04:05"))
            return panCakeUpdatedAt
        } else {
            log.Printf("[Bridge] CẢNH BÁO: Không tìm thấy panCakeUpdatedAt trong conversation")
            // Debug: log toàn bộ conversation để xem structure
            log.Printf("[Bridge] Conversation structure: %+v", conversation)
        }
    }
    
    log.Printf("[Bridge] Không thể parse conversation từ FolkForm")
    return 0
}
```

**Lưu ý về FolkForm API:**
- Endpoint: `GET /facebook/conversation/sort-by-api-update`
- Sắp xếp: `SetSort(bson.D{{Key: "panCakeUpdatedAt", Value: -1}})` → giảm dần
- `items[0]` = conversation mới nhất (panCakeUpdatedAt lớn nhất)
- Field `panCakeUpdatedAt` là Unix timestamp (giây)

---

### Bước 3: Sửa `Sync_NewMessagesOfPage` - Dùng `since`/`until`

**File:** `app/integrations/bridge.go`

```go
func Sync_NewMessagesOfPage(page_id string, page_username string) (resultErr error) {
    log.Printf("[Bridge] Bắt đầu sync conversations mới cho page_id: %s", page_id)
    
    // Bước 1: Lấy panCakeUpdatedAt cuối cùng từ FolkForm
    lastUpdatedAt := getLastPanCakeUpdatedAt(page_id)
    
    // Nếu không tìm thấy conversation nào → sync từ đầu (lastUpdatedAt = 0)
    if lastUpdatedAt == 0 {
        log.Printf("[Bridge] Không tìm thấy conversation nào trong FolkForm, sẽ sync từ đầu")
    } else {
        log.Printf("[Bridge] Tìm thấy panCakeUpdatedAt cuối cùng: %d (Unix timestamp)", lastUpdatedAt)
        // Convert sang time để log dễ đọc
        lastUpdatedTime := time.Unix(lastUpdatedAt, 0)
        log.Printf("[Bridge] Thời gian tương ứng: %s", lastUpdatedTime.Format("2006-01-02 15:04:05"))
    }
    
    // Bước 2: Tính since và until
    since := lastUpdatedAt
    until := time.Now().Unix()
    
    // Edge case: since >= until
    if since >= until {
        log.Printf("[Bridge] since (%d) >= until (%d), không có conversations mới", since, until)
        return nil
    }
    
    log.Printf("[Bridge] Sync conversations từ %d đến %d (khoảng thời gian: %d giây)", 
        since, until, until-since)
    
    // Bước 3: Sync conversations trong khoảng thời gian
    last_conversation_id := ""
    conversationCount := 0
    
    for {
        // Gọi API với since/until
        resultGetConversations, err := Pancake_GetConversations_v2(page_id, last_conversation_id, since, until)
        if err != nil {
            logError("[Bridge] Lỗi khi lấy danh sách hội thoại: %v", err)
            break
        }
        
        if resultGetConversations["conversations"] != nil {
            conversations := resultGetConversations["conversations"].([]interface{})
            if len(conversations) == 0 {
                log.Printf("[Bridge] Không còn conversations nào trong khoảng thời gian")
                break
            }
            
            log.Printf("[Bridge] Lấy được %d conversations từ Pancake", len(conversations))
            
            // Xử lý từng conversation
            for _, conversation := range conversations {
                conversationMap := conversation.(map[string]interface{})
                conversation_id := conversationMap["id"].(string)
                customerId := ""
                if cid, ok := conversationMap["customer_id"].(string); ok {
                    customerId = cid
                }
                
                // Tạo/update conversation trong FolkForm
                _, err = FolkForm_CreateConversation(page_id, page_username, conversation)
                if err != nil {
                    logError("[Bridge] Lỗi khi tạo/cập nhật hội thoại: %v", err)
                    continue
                }
                
                conversationCount++
                
                // Sync messages của conversation này
                err = bridge_SyncMessageOfConversation(page_id, page_username, conversation_id, customerId)
                if err != nil {
                    logError("[Bridge] Lỗi khi đồng bộ tin nhắn: %v", err)
                    continue
                }
                
                // Dừng nửa giây trước khi tiếp tục
                time.Sleep(100 * time.Millisecond)
            }
            
            // Cập nhật last_conversation_id để pagination
            new_last_conversation_id := conversations[len(conversations)-1].(map[string]interface{})["id"].(string)
            if new_last_conversation_id != last_conversation_id {
                last_conversation_id = new_last_conversation_id
                continue
            } else {
                log.Printf("[Bridge] Không còn conversations mới, dừng pagination")
                break
            }
        } else {
            log.Printf("[Bridge] Không có conversations nào trong response")
            break
        }
    }
    
    log.Printf("[Bridge] Đồng bộ conversations mới thành công cho page_id: %s, tổng cộng: %d conversations", 
        page_id, conversationCount)
    
    return nil
}
```

**Thay đổi chính:**
1. ✅ Dùng `getLastPanCakeUpdatedAt()` thay vì lấy `conversationId`
2. ✅ Tính `since` = `lastUpdatedAt`, `until` = `now`
3. ✅ Truyền `since`/`until` vào `Pancake_GetConversations_v2()`
4. ✅ Bỏ logic dừng khi gặp `conversation_id_updated`
5. ✅ Dựa vào `since`/`until` để lấy conversations trong khoảng thời gian

---

### Bước 4: Cập Nhật Các Nơi Gọi `Pancake_GetConversations_v2`

**File:** `app/integrations/bridge.go` - Hàm `bridge_SyncConversationsOfPage`

```go
func bridge_SyncConversationsOfPage(page_id string, page_username string) {
    last_conversation_id := ""
    // Sync tất cả → không dùng since/until (truyền 0, 0)
    for {
        result := Pancake_GetConversations_v2(page_id, last_conversation_id, 0, 0)
        // ... rest of code
    }
}
```

**Lưu ý:**
- Sync tất cả không cần `since`/`until` → truyền `0, 0`
- Giữ nguyên logic pagination với `last_conversation_id`

---

## 📊 So Sánh Trước và Sau

### Trước (Dùng conversation_id)

```go
conversation_id_updated := getLastConversationId(page_id)

for {
    result := Pancake_GetConversations_v2(page_id, last_conversation_id)
    if conversation_id == conversation_id_updated {
        return // Dừng → CÓ THỂ BỎ SÓT
    }
}
```

**Vấn đề:**
- ❌ Có thể bỏ sót conversations mới nếu có conversation được insert vào giữa
- ❌ Không track timestamp → không biết đã sync đến đâu

---

### Sau (Dùng since/until)

```go
lastUpdatedAt := getLastPanCakeUpdatedAt(page_id)
since := lastUpdatedAt
until := time.Now().Unix()

for {
    result := Pancake_GetConversations_v2(page_id, last_conversation_id, since, until)
    // ... process tất cả conversations trong khoảng thời gian
}
```

**Lợi ích:**
- ✅ Không bỏ sót - lấy tất cả conversations có `updated_at` trong khoảng
- ✅ Track timestamp chính xác - biết đã sync đến thời điểm nào
- ✅ Có thể resume từ bất kỳ thời điểm nào

---

## 🔍 Chi Tiết Kỹ Thuật

### 1. Format Timestamp

**Pancake API:**
- `since` và `until` là Unix timestamp (giây) - integer
- Ví dụ: `1704067200` (2024-01-01 00:00:00 UTC)

**FolkForm:**
- `panCakeUpdatedAt` là Unix timestamp (giây) - number
- Được extract từ `panCakeData.updated_at` (ISO 8601 string) → convert sang Unix timestamp

**Conversion:**
```go
// Pancake trả về: "2019-08-24T14:15:22.000000" (ISO 8601)
// FolkForm convert sang: 1566656122 (Unix timestamp, giây)
// Dùng trực tiếp: since = 1566656122
```

---

### 2. Edge Cases

#### Case 1: Không có conversation nào trong FolkForm

```go
if lastUpdatedAt == 0 {
    log.Printf("[Bridge] Không có conversation nào, sync từ đầu")
    // Pancake API: since = 0 → không filter (lấy tất cả)
    // Hoặc có thể set since = một thời điểm cụ thể (ví dụ: 1 năm trước)
}
```

---

#### Case 2: `since` >= `until`

```go
if since >= until {
    log.Printf("[Bridge] since (%d) >= until (%d), không có conversations mới", since, until)
    return nil
}
```

---

#### Case 3: Khoảng thời gian quá lớn (Optional)

```go
maxSyncWindow := int64(30 * 24 * 60 * 60) // 30 ngày
if until - since > maxSyncWindow {
    log.Printf("[Bridge] Khoảng thời gian quá lớn (%d giây), giới hạn về 30 ngày", until-since)
    since = until - maxSyncWindow
}
```

---

## 📝 Checklist Implementation

### Priority 1 (Bắt buộc)
- [ ] Sửa `Pancake_GetConversations_v2()` - thêm params `since`, `until`
- [ ] Tạo `getLastPanCakeUpdatedAt()` - lấy timestamp từ FolkForm
- [ ] Sửa `Sync_NewMessagesOfPage()` - dùng `since`/`until`
- [ ] Cập nhật `bridge_SyncConversationsOfPage()` - truyền `0, 0` cho sync tất cả

### Priority 2 (Nên làm)
- [ ] Thêm `order_by=updated_at` cho sync incremental
- [ ] Xử lý edge cases (since = 0, since >= until, khoảng thời gian quá lớn)
- [ ] Cải thiện logging

### Priority 3 (Tùy chọn)
- [ ] Thêm config cho max sync window
- [ ] Thêm metrics để track sync performance

---

## 🎯 Kết Luận

**Logic thêm `since`/`until`:**

1. **Lấy `panCakeUpdatedAt` cuối cùng từ FolkForm** → `lastUpdatedAt`
2. **Tính `since` = `lastUpdatedAt`, `until` = `now`**
3. **Truyền `since`/`until` vào `Pancake_GetConversations_v2()`**
4. **Pancake API sẽ lọc conversations có `updated_at` trong khoảng `since` → `until`**
5. **Sync tất cả conversations trong khoảng thời gian này**

**Lợi ích:**
- ✅ Không bỏ sót conversations mới
- ✅ Track timestamp chính xác
- ✅ Có thể resume từ bất kỳ thời điểm nào
- ✅ Sync incremental hiệu quả
