# Thiết Kế Sync V2 - Phương Án Cuối Cùng

**Ngày:** 2025-01-XX  
**Mục đích:** Tài liệu thiết kế chi tiết cho Sync V2 dựa trên `last_conversation_id` và `order_by=updated_at`

---

## 📋 Tổng Quan Phương Án

### Kiến Trúc

1. **Tạo Bridge V2**: Không phá hỏng luồng cũ
2. **2 Job mới**:
   - `sync_new_data_job`: Sync conversations mới (incremental)
   - `sync_all_data_job`: Sync conversations cũ (full sync)
3. **Dùng `order_by=updated_at`** cho cả 2 job
4. **Không cần checkpoint local**: Dùng `folkform_last/oldest_conversation_id`

---

## 🔍 Phân Tích `order_by=updated_at` Cho Cả 2 Job

### Lý Do Dùng `order_by=updated_at` Cho Cả 2 Job

**User: "Conversation nào mới update thì job sync_new nó hứng rồi"**

**Phân tích:**

#### Job sync_new_data (Incremental)
- ✅ **Đúng**: Dùng `order_by=updated_at` để lấy conversations được update gần đây
- ✅ Conversation cũ được update → Nhảy lên đầu → Job sync_new sẽ hứng
- ✅ Đảm bảo không bỏ sót conversations có thay đổi

#### Job sync_all_data (Full Sync)
- ✅ **Đúng**: Dùng `order_by=updated_at` để sync từ cũ → mới
- ✅ Conversation nào mới update → Job sync_new đã hứng rồi
- ✅ Job sync_all chỉ cần sync những conversations chưa được update gần đây

**Kết luận:** Dùng `order_by=updated_at` cho cả 2 job là hợp lý!

---

## 📊 Job 1: sync_new_data (Incremental Sync)

### Logic Chi Tiết

```
1. DoSyncNewData() gọi SyncBaseAuth() - Các bước phụ:
   - Đăng nhập FolkForm (CheckIn/Login)
   - Sync pages từ Pancake → FolkForm
   - Update page access tokens
   - Sync pages từ FolkForm → Local

2. DoSyncNewData() gọi BridgeV2_SyncNewData():
   - Lấy tất cả pages từ FolkForm (isSync=true)

3. Với mỗi page:
   a. Lấy folkform_last_conversation_id từ FolkForm
      → Conversation được update mới nhất trong FolkForm
      → Nếu không có → Sync từ đầu (last_conversation_id = "")

   b. Gọi Pancake API:
      - last_conversation_id = "" (lần đầu, không truyền param)
      - order_by = "updated_at"
      → Lấy 60 conversations mới nhất (theo updated_at)

   c. Với mỗi conversation trong batch:
      - Nếu conversation.id == folkform_last_conversation_id:
        → Dừng (đã sync rồi)
      - Nếu không:
        → Sync conversation vào FolkForm
        → Sync messages mới (dùng latestInsertedAt)

   d. Nếu chưa gặp folkform_last_conversation_id:
      - last_conversation_id = conversations[59].id (conversation cuối cùng)
      - Gọi API lại với last_conversation_id
      - Lặp lại bước c

   e. Khi gặp folkform_last_conversation_id → Dừng, chuyển sang page tiếp theo
```

### Code Structure

```go
// File: app/integrations/bridge_v2.go
func BridgeV2_SyncNewData() error {
    // Lưu ý: SyncBaseAuth() đã được gọi trong DoSyncNewData() rồi
    // Không cần gọi lại ở đây
    
    // Bước 2: Lấy tất cả pages từ FolkForm
    limit := 50
    page := 1
    
    for {
        resultPages, err := FolkForm_GetFbPages(page, limit)
        if err != nil {
            return err
        }
        
        items, itemCount, err := parseResponseData(resultPages)
        if err != nil || itemCount == 0 {
            break
        }
        
        // Bước 3: Với mỗi page
        for _, item := range items {
            pageMap := item.(map[string]interface{})
            pageId := pageMap["pageId"].(string)
            pageUsername := pageMap["pageUsername"].(string)
            isSync := pageMap["isSync"].(bool)
            
            if !isSync {
                continue // Bỏ qua page không sync
            }
            
            // Lấy conversation mới nhất từ FolkForm
            lastConversationId, _ := FolkForm_GetLastConversationId(pageId)
            
            // Sync conversations mới
            last_conversation_id := ""
            
            // Sử dụng adaptive rate limiter để tránh rate limit
            rateLimiter := apputility.GetPancakeRateLimiter()
            
            for {
                // ✅ Áp dụng Rate Limiter: Gọi Wait() trước mỗi API call
                rateLimiter.Wait()
                
                // Gọi Pancake API (đã có retry logic sẵn trong Pancake_GetConversations_v2)
                conversations := Pancake_GetConversations_v2(
                    page_id=pageId,
                    last_conversation_id=last_conversation_id,
                    since=0,
                    until=0,
                    order_by="updated_at"  // ✅ Dùng updated_at
                )
                
                if len(conversations) == 0 {
                    break // Hết conversations
                }
                
                foundLastConversation := false
                
                // Sync từng conversation
                for _, conv := range conversations {
                    convMap := conv.(map[string]interface{})
                    convId := convMap["id"].(string)
                    customerId := ""
                    if cid, ok := convMap["customer_id"].(string); ok {
                        customerId = cid
                    }
                    
                    // Kiểm tra: Đã gặp conversation cuối cùng chưa?
                    if convId == lastConversationId {
                        foundLastConversation = true
                        log.Printf("Đã gặp folkform_last_conversation_id (%s), dừng sync", lastConversationId)
                        break
                    }
                    
                    // Sync conversation
                    FolkForm_CreateConversation(pageId, pageUsername, conv)
                    
                    // Sync messages mới
                    // Lưu ý: bridge_SyncMessageOfConversation đã có rate limiter bên trong
                    bridge_SyncMessageOfConversation(
                        pageId, pageUsername, convId, customerId,
                        isFullSync=false  // Chỉ sync messages mới
                    )
                }
                
                if foundLastConversation {
                    break // Dừng
                }
                
                // Cập nhật last_conversation_id để pagination
                last_conversation_id = conversations[len(conversations)-1].(map[string]interface{})["id"].(string)
            }
        }
        
        page++
    }
    
    return nil
}

// File: app/jobs/sync_new_data_job.go
func DoSyncNewData() error {
    // Thực hiện xác thực và đồng bộ dữ liệu cơ bản
    SyncBaseAuth()

    // Đồng bộ dữ liệu mới nhất
    log.Println("Bắt đầu đồng bộ dữ liệu mới nhất...")
    err := integrations.BridgeV2_SyncNewData()
    if err != nil {
        log.Printf("❌ Lỗi khi đồng bộ dữ liệu mới: %v", err)
        return err
    }
    log.Println("Đồng bộ dữ liệu mới nhất thành công")
    return nil
}
```

---

## 📊 Job 2: sync_all_data (Full Sync)

### Logic Chi Tiết

```
1. DoSyncAllData() gọi SyncBaseAuth() - Các bước phụ:
   - Đăng nhập FolkForm (CheckIn/Login)
   - Sync pages từ Pancake → FolkForm
   - Update page access tokens
   - Sync pages từ FolkForm → Local

2. DoSyncAllData() gọi BridgeV2_SyncAllData():
   - Lấy tất cả pages từ FolkForm (isSync=true)

3. Với mỗi page:
   a. Lấy folkform_oldest_conversation_id từ FolkForm
      → Conversation được update cũ nhất trong FolkForm
      → Nếu không có → Sync từ đầu (last_conversation_id = "")

   b. Gọi Pancake API:
      - last_conversation_id = folkform_oldest_conversation_id
      - order_by = "updated_at"
      → Lấy 60 conversations cũ hơn (theo updated_at)

   c. Nếu len(conversations) == 0:
      → Dừng (đã sync hết, không còn conversations cũ hơn)

   d. Với mỗi conversation trong batch:
      → Sync conversation vào FolkForm
      → Sync TẤT CẢ messages (dùng oldestInsertedAt để xác định điểm dừng)

   e. last_conversation_id = conversations[59].id (conversation cuối cùng)
      - Gọi API lại với last_conversation_id
      - Lặp lại bước b-d

   f. Khi len(conversations) == 0 → Dừng, chuyển sang page tiếp theo
```

### Code Structure

```go
// File: app/integrations/bridge_v2.go
func BridgeV2_SyncAllData() error {
    // Lưu ý: SyncBaseAuth() đã được gọi trong DoSyncAllData() rồi
    // Không cần gọi lại ở đây
    
    // Bước 2: Lấy tất cả pages từ FolkForm
    limit := 50
    page := 1
    
    for {
        resultPages, err := FolkForm_GetFbPages(page, limit)
        if err != nil {
            return err
        }
        
        items, itemCount, err := parseResponseData(resultPages)
        if err != nil || itemCount == 0 {
            break
        }
        
        // Bước 3: Với mỗi page
        for _, item := range items {
            pageMap := item.(map[string]interface{})
            pageId := pageMap["pageId"].(string)
            pageUsername := pageMap["pageUsername"].(string)
            isSync := pageMap["isSync"].(bool)
            
            if !isSync {
                continue // Bỏ qua page không sync
            }
            
            // Lấy conversation cũ nhất từ FolkForm
            oldestConversationId, _ := FolkForm_GetOldestConversationId(pageId)
            
            // Sync conversations cũ
            last_conversation_id := oldestConversationId
            
            // Sử dụng adaptive rate limiter để tránh rate limit
            rateLimiter := apputility.GetPancakeRateLimiter()
            
            for {
                // ✅ Áp dụng Rate Limiter: Gọi Wait() trước mỗi API call
                rateLimiter.Wait()
                
                // Gọi Pancake API (đã có retry logic sẵn trong Pancake_GetConversations_v2)
                conversations := Pancake_GetConversations_v2(
                    page_id=pageId,
                    last_conversation_id=last_conversation_id,
                    since=0,
                    until=0,
                    order_by="updated_at"  // ✅ Dùng updated_at
                )
                
                if len(conversations) == 0 {
                    log.Printf("Không còn conversations cũ hơn, dừng sync")
                    break // Đã sync hết
                }
                
                // Sync từng conversation
                for _, conv := range conversations {
                    convMap := conv.(map[string]interface{})
                    convId := convMap["id"].(string)
                    customerId := ""
                    if cid, ok := convMap["customer_id"].(string); ok {
                        customerId = cid
                    }
                    
                    // Sync conversation
                    FolkForm_CreateConversation(pageId, pageUsername, conv)
                    
                    // Sync TẤT CẢ messages
                    // Lưu ý: bridge_SyncMessageOfConversation đã có rate limiter bên trong
                    bridge_SyncMessageOfConversation(
                        pageId, pageUsername, convId, customerId,
                        isFullSync=true  // Sync tất cả messages
                    )
                }
                
                // Cập nhật last_conversation_id để pagination
                last_conversation_id = conversations[len(conversations)-1].(map[string]interface{})["id"].(string)
            }
        }
        
        page++
    }
    
    return nil
}

// File: app/jobs/sync_all_data_job.go
func DoSyncAllData() error {
    // Thực hiện xác thực và đồng bộ dữ liệu cơ bản
    SyncBaseAuth()

    // Công việc cần thực hiện
    log.Println("Bắt đầu đồng bộ tất cả dữ liệu...")
    err := integrations.BridgeV2_SyncAllData()
    if err != nil {
        log.Printf("❌ Lỗi khi đồng bộ tất cả dữ liệu: %v", err)
        return err
    }
    log.Println("Đồng bộ tất cả dữ liệu thành công")
    return nil
}
```

---

## 🔧 Helper Functions

### 1. Lấy Last Conversation ID (Mới Nhất)

**File:** `app/integrations/folkform.go`

**Cách 1: Dùng endpoint `sort-by-api-update` (Đơn giản, đã có sẵn)**

```go
// FolkForm_GetLastConversationId lấy conversation mới nhất từ FolkForm
// Sử dụng endpoint sort-by-api-update (sort desc - mới nhất trước)
// Endpoint này tự động filter theo pageId và sort theo panCakeUpdatedAt desc
func FolkForm_GetLastConversationId(pageId string) (conversationId string, err error) {
    log.Printf("[FolkForm] Lấy conversation mới nhất - pageId: %s", pageId)
    
    // Endpoint: GET /facebook/conversation/sort-by-api-update?page=1&limit=1&pageId={pageId}
    // Tự động filter theo pageId và sort theo panCakeUpdatedAt desc (mới nhất trước)
    result, err := FolkForm_GetConversationsWithPageId(1, 1, pageId)
    if err != nil {
        return "", err
    }
    
    // Parse response
    var items []interface{}
    if dataMap, ok := result["data"].(map[string]interface{}); ok {
        if itemsArray, ok := dataMap["items"].([]interface{}); ok {
            items = itemsArray
        }
    }
    
    if len(items) == 0 {
        log.Printf("[FolkForm] Không tìm thấy conversation nào - pageId: %s", pageId)
        return "", nil // Không có conversation → trả về empty
    }
    
    // items[0] = conversation mới nhất (panCakeUpdatedAt lớn nhất)
    firstItem := items[0]
    if conversation, ok := firstItem.(map[string]interface{}); ok {
        if convId, ok := conversation["conversationId"].(string); ok {
            log.Printf("[FolkForm] Tìm thấy conversation mới nhất - conversationId: %s", convId)
            return convId, nil
        }
    }
    
    return "", nil
}
```

**Cách 2: Dùng Find API với filter và sort (Nếu cần linh hoạt hơn)**

```go
// FolkForm_GetLastConversationId - Dùng Find API
// GET /facebook/conversation/find?filter={"pageId":"..."}&options={"sort":{"panCakeUpdatedAt":-1},"limit":1}
func FolkForm_GetLastConversationId(pageId string) (conversationId string, err error) {
    log.Printf("[FolkForm] Lấy conversation mới nhất - pageId: %s", pageId)
    
    if err := checkApiToken(); err != nil {
        return "", err
    }
    
    client := createAuthorizedClient(defaultTimeout)
    
    // Dùng GET với query string
    params := map[string]string{
        "filter":  fmt.Sprintf(`{"pageId":"%s"}`, pageId),
        "options": `{"sort":{"panCakeUpdatedAt":-1},"limit":1}`, // Sort desc (mới nhất trước)
    }
    
    result, err := executeGetRequest(
        client,
        "/facebook/conversation/find",
        params,
        "Lấy conversation mới nhất thành công",
    )
    
    if err != nil {
        return "", err
    }
    
    // Parse response tương tự như trên
    // ...
}
```

**Khuyến nghị:** Dùng Cách 1 (endpoint `sort-by-api-update`) vì đơn giản và đã có sẵn

---

### 2. Lấy Oldest Conversation ID (Cũ Nhất)

**Option A: Dùng Find API với Filter và Sort**

**File:** `app/integrations/folkform.go`

```go
// FolkForm_GetOldestConversationId lấy conversation cũ nhất từ FolkForm
// Filter theo pageId và sort theo panCakeUpdatedAt asc (cũ nhất trước)
func FolkForm_GetOldestConversationId(pageId string) (conversationId string, err error) {
    log.Printf("[FolkForm] Lấy conversation cũ nhất - pageId: %s", pageId)
    
    if err := checkApiToken(); err != nil {
        return "", err
    }
    
    client := createAuthorizedClient(defaultTimeout)
    
    // Dùng GET với query string
    // GET /facebook/conversation/find?filter={"pageId":"..."}&options={"sort":{"panCakeUpdatedAt":1},"limit":1}
    params := map[string]string{
        "filter":  fmt.Sprintf(`{"pageId":"%s"}`, pageId),  // ✅ Filter theo pageId
        "options": `{"sort":{"panCakeUpdatedAt":1},"limit":1}`, // ✅ Sort asc (cũ nhất trước)
    }
    
    result, err := executeGetRequest(
        client,
        "/facebook/conversation/find",
        params,
        "Lấy conversation cũ nhất thành công",
    )
    
    if err != nil {
        return "", err
    }
    
    // Parse response
    var items []interface{}
    if dataMap, ok := result["data"].(map[string]interface{}); ok {
        if itemsArray, ok := dataMap["items"].([]interface{}); ok {
            items = itemsArray
        } else if itemsArray, ok := dataMap["data"].([]interface{}); ok {
            items = itemsArray
        }
    } else if itemsArray, ok := result["data"].([]interface{}); ok {
        items = itemsArray
    }
    
    if len(items) == 0 {
        log.Printf("[FolkForm] Không tìm thấy conversation nào - pageId: %s", pageId)
        return "", nil // Không có conversation → trả về empty
    }
    
    // items[0] = conversation cũ nhất (panCakeUpdatedAt nhỏ nhất)
    firstItem := items[0]
    if conversation, ok := firstItem.(map[string]interface{}); ok {
        if convId, ok := conversation["conversationId"].(string); ok {
            log.Printf("[FolkForm] Tìm thấy conversation cũ nhất - conversationId: %s", convId)
            return convId, nil
        }
    }
    
    return "", nil
}
```

**Lưu ý:**
- ✅ **Filter theo `pageId`**: Chỉ lấy conversations của page cụ thể
- ✅ **Sort theo `panCakeUpdatedAt`**: 
  - `1` = asc (cũ nhất trước) → Dùng cho `GetOldestConversationId`
  - `-1` = desc (mới nhất trước) → Dùng cho `GetLastConversationId`

**Option B: Lấy Page Cuối Cùng (Nếu Find API không hỗ trợ sort)**

```go
// FolkForm_GetOldestConversationId - Lấy conversation cũ nhất bằng cách lấy page cuối cùng
func FolkForm_GetOldestConversationId(pageId string) (conversationId string, err error) {
    log.Printf("[FolkForm] Lấy conversation cũ nhất - pageId: %s", pageId)
    
    // Bước 1: Đếm tổng số conversations
    result, err := FolkForm_GetConversationsWithPageId(1, 1, pageId)
    if err != nil {
        return "", err
    }
    
    var totalPages float64 = 1
    if dataMap, ok := result["data"].(map[string]interface{}); ok {
        if tp, ok := dataMap["totalPage"].(float64); ok {
            totalPages = tp
        }
    }
    
    if totalPages == 0 {
        return "", nil // Không có conversations
    }
    
    // Bước 2: Lấy page cuối cùng (cũ nhất)
    result, err = FolkForm_GetConversationsWithPageId(int(totalPages), 1, pageId)
    if err != nil {
        return "", err
    }
    
    // Parse response
    var items []interface{}
    if dataMap, ok := result["data"].(map[string]interface{}); ok {
        if itemsArray, ok := dataMap["items"].([]interface{}); ok {
            items = itemsArray
        }
    }
    
    if len(items) == 0 {
        return "", nil
    }
    
    // items[0] = conversation cũ nhất (trong page cuối cùng)
    firstItem := items[0]
    if conversation, ok := firstItem.(map[string]interface{}); ok {
        if convId, ok := conversation["conversationId"].(string); ok {
            log.Printf("[FolkForm] Tìm thấy conversation cũ nhất - conversationId: %s", convId)
            return convId, nil
        }
    }
    
    return "", nil
}
```

**Khuyến nghị:** Thử Option A trước (Find API với sort), nếu không được thì dùng Option B

---

## 🔄 Logic Dừng Khi Gặp Conversation Đã Có

### Job sync_new_data

**Nguyên tắc:** "Cứ chạy sync, nếu gặp conversation đó thì dừng lại"

```go
folkform_last_conversation_id := FolkForm_GetLastConversationId(pageId)

for {
    conversations := Pancake_GetConversations(...)
    
    for _, conv := range conversations {
        // ✅ Cứ sync bình thường
        FolkForm_CreateConversation(...)
        bridge_SyncMessageOfConversation(...)
        
        // ✅ Nếu gặp conversation đã có → Dừng
        if conv.id == folkform_last_conversation_id {
            log.Printf("Đã gặp folkform_last_conversation_id (%s), dừng sync", folkform_last_conversation_id)
            return nil // Dừng ngay
        }
    }
    
    // Tiếp tục pagination
    last_conversation_id = conversations[len(conversations)-1].id
}
```

**Lưu ý:**
- Không cần kiểm tra trước khi sync
- Cứ sync bình thường, khi gặp conversation đã có thì dừng
- Đơn giản, dễ hiểu

---

## ⚠️ Edge Cases

### 1. Conversation Bị Xóa Trong Pancake

**Kịch bản:**
- `folkform_oldest_conversation_id = "conv_123"`
- Gọi API với `last_conversation_id = "conv_123"`
- Pancake trả về 60 conversations cũ hơn
- Nhưng `conv_123` không có trong response (đã bị xóa)

**Xử lý:** "Chạy tiếp"

```go
oldestConversationId := FolkForm_GetOldestConversationId(pageId)

for {
    conversations := Pancake_GetConversations(last_conversation_id=oldestConversationId)
    
    if len(conversations) == 0 {
        break // Hết conversations
    }
    
    // ✅ Không cần kiểm tra oldestConversationId có trong response không
    // ✅ Cứ sync bình thường, chạy tiếp
    
    for _, conv := range conversations {
        FolkForm_CreateConversation(...)
        bridge_SyncMessageOfConversation(...)
    }
    
    last_conversation_id = conversations[len(conversations)-1].id
}
```

**Lý do:**
- Conversation đã bị xóa → Không cần quan tâm
- Cứ sync tiếp các conversations còn lại
- Đơn giản, không phức tạp

---

### 2. Không Có Conversations Trong FolkForm

**Job sync_new_data:**
```go
lastConversationId := FolkForm_GetLastConversationId(pageId)
if lastConversationId == "" {
    // Chưa có conversation nào → Sync từ đầu
    last_conversation_id = ""
}
```

**Job sync_all_data:**
```go
oldestConversationId := FolkForm_GetOldestConversationId(pageId)
if oldestConversationId == "" {
    // Chưa có conversation nào → Sync từ đầu
    last_conversation_id = ""
}
```

---

### 3. `folkform_last_conversation_id` Không Có Trong 60 Conversations Đầu

**Xử lý:** Tiếp tục pagination cho đến khi gặp

```go
for {
    conversations := Pancake_GetConversations(...)
    
    found := false
    for _, conv := range conversations {
        if conv.id == folkform_last_conversation_id {
            found = true
            break // Dừng
        }
        // Sync conversation
    }
    
    if found {
        break // Dừng
    }
    
    // Tiếp tục pagination
    last_conversation_id = conversations[len(conversations)-1].id
}
```

---

## 📝 Implementation Checklist

### Phase 1: Helper Functions
- [ ] `FolkForm_GetLastConversationId()` - Lấy conversation mới nhất (dùng sort-by-api-update)
- [ ] `FolkForm_GetOldestConversationId()` - Lấy conversation cũ nhất (thử Find API với sort trước, nếu không được thì lấy page cuối cùng)
- [ ] Test 2 hàm này hoạt động đúng

### Phase 2: Sửa Pancake API
- [ ] Thêm param `order_by` vào `Pancake_GetConversations_v2()`
- [ ] Test với `order_by=updated_at`

### Phase 3: Tạo Bridge V2
- [ ] Tạo file `bridge_v2.go`
- [ ] Implement `BridgeV2_SyncNewData()` - Lấy pages, sync conversations + messages
  - [ ] **Áp dụng Rate Limiter**: Gọi `rateLimiter.Wait()` trước mỗi Pancake API call
  - [ ] **Áp dụng Retry Logic**: Sử dụng các hàm Pancake API đã có retry logic sẵn
  - [ ] **Error Handling**: Log lỗi và record failure vào rate limiter
- [ ] Implement `BridgeV2_SyncAllData()` - Lấy pages, sync conversations + messages
  - [ ] **Áp dụng Rate Limiter**: Gọi `rateLimiter.Wait()` trước mỗi Pancake API call
  - [ ] **Áp dụng Retry Logic**: Sử dụng các hàm Pancake API đã có retry logic sẵn
  - [ ] **Error Handling**: Log lỗi và record failure vào rate limiter
- [ ] Test logic pagination và dừng
- [ ] Test rate limiter hoạt động đúng
- [ ] Test retry logic khi có lỗi

### Phase 4: Tạo Jobs Mới

**Logic cần lấy từ job cũ:**

#### 1. ExecuteInternal Wrapper (Từ sync_new_job.go và sync_all_data_job.go)
- ✅ Format log đẹp với emoji và separator
- ✅ Log thời gian bắt đầu/kết thúc
- ✅ Log duration (thời gian thực thi)
- ✅ Log lỗi chi tiết khi thất bại
- ✅ Return error để scheduler xử lý

#### 2. Error Handling Pattern
- ✅ Log lỗi với format: `❌ Lỗi khi đồng bộ...`
- ✅ Return error để job bị đánh dấu thất bại
- ✅ Log thành công: `✅ Đồng bộ... thành công`

#### 3. Structure Pattern
- ✅ Struct với `*scheduler.BaseJob`
- ✅ Constructor `NewXXXJob(name, schedule)`
- ✅ `ExecuteInternal(ctx)` - Wrapper với logging
- ✅ `DoXXX()` - Logic thực sự, có thể gọi độc lập

#### 4. Log Messages (Tiếng Việt)
- ✅ `"Bắt đầu đồng bộ..."` - Trước khi sync
- ✅ `"Đồng bộ... thành công"` - Khi thành công
- ✅ `"❌ Lỗi khi đồng bộ..."` - Khi có lỗi

#### 5. Rate Limiter (Từ bridge.go và pancake.go)
- ✅ **Pancake API**: Dùng `apputility.GetPancakeRateLimiter()`
  - Gọi `rateLimiter.Wait()` trước mỗi API call
  - Gọi `rateLimiter.RecordFailure(statusCode, errorCode)` khi có lỗi
  - Gọi `rateLimiter.RecordResponse(statusCode, success, errorCode)` sau mỗi response
- ✅ **FolkForm API**: Dùng `apputility.GetFolkFormRateLimiter()`
  - Gọi `rateLimiter.Wait()` trước mỗi API call
  - Gọi `rateLimiter.RecordFailure(statusCode, errorCode)` khi có lỗi
  - Gọi `rateLimiter.RecordResponse(statusCode, success, errorCode)` sau mỗi response
- ✅ **Vị trí áp dụng**:
  - Trước khi gọi `Pancake_GetConversations_v2()`
  - Trước khi gọi `Pancake_GetMessages()`
  - Trong các helper functions `executeGetRequest`, `executePostRequest`, `executePutRequest` (đã có sẵn)

#### 6. Retry Logic (Từ pancake.go và folkform.go)
- ✅ **Pancake API**: Retry loop với max 5 lần
  - Retry khi status code != 200
  - Retry khi có lỗi network
  - Retry khi `success != true` trong response
  - Log chi tiết mỗi lần retry
- ✅ **FolkForm API**: Retry loop với `maxRetries = 5`
  - Retry khi status code != 200
  - Retry khi có lỗi network
  - Retry khi `status != "success"` trong response
  - Log chi tiết mỗi lần retry
- ✅ **Lưu ý**: Retry logic đã được implement trong các helper functions (`executeGetRequest`, `executePostRequest`, `executePutRequest`), chỉ cần sử dụng các hàm này

#### 7. Error Handling Pattern
- ✅ Log lỗi chi tiết với format: `[System] [Bước X/5] ❌ LỖI: ...`
- ✅ Record failure vào rate limiter để tự động điều chỉnh
- ✅ Continue retry nếu chưa vượt maxRetries
- ✅ Return error khi vượt maxRetries

**Implementation:**
- [ ] Tạo `sync_new_data_job.go` (copy structure từ sync_new_job.go)
  - [ ] Struct `SyncNewDataJob` với `*scheduler.BaseJob`
  - [ ] `NewSyncNewDataJob(name, schedule)`
  - [ ] `ExecuteInternal(ctx)` - **Copy nguyên format log từ sync_new_job.go**
  - [ ] `DoSyncNewData()` - Gọi `BridgeV2_SyncNewData()` với log messages
- [ ] Tạo `sync_all_data_job.go` (copy structure từ sync_all_data_job.go)
  - [ ] Struct `SyncAllDataJob` với `*scheduler.BaseJob`
  - [ ] `NewSyncAllDataJob(name, schedule)`
  - [ ] `ExecuteInternal(ctx)` - **Copy nguyên format log từ sync_all_data_job.go**
  - [ ] `DoSyncAllData()` - Gọi `BridgeV2_SyncAllData()` với log messages
- [ ] Cấu hình scheduler trong `main.go`:
  ```go
  // Job sync_new_data - Chạy mỗi 5 phút
  syncNewDataJob := jobs.NewSyncNewDataJob("sync-new-data-job", "0 */5 * * * *")
  s.AddJobObject(syncNewDataJob)
  
  // Job sync_all_data - Chạy nền liên tục (hoặc mỗi ngày lúc 00:00:00)
  syncAllDataJob := jobs.NewSyncAllDataJob("sync-all-data-job", "0 0 0 * * *")
  s.AddJobObject(syncAllDataJob)
  ```

### Phase 5: Testing
- [ ] Test sync_new_data: Dừng đúng khi gặp last_conversation_id
- [ ] Test sync_all_data: Resume đúng từ oldest_conversation_id
- [ ] Test edge cases: Conversation bị xóa, không có conversations, etc.
- [ ] Test SyncBaseAuth: Đăng nhập, sync pages, etc.

---

## 🎯 Kết Luận

**Thiết kế cuối cùng:**
1. ✅ Dùng `order_by=updated_at` cho cả 2 job
2. ✅ Logic dừng: Cứ sync, gặp conversation đã có thì dừng
3. ✅ Edge case: Conversation bị xóa → Chạy tiếp
4. ✅ Không cần checkpoint local: Dùng `folkform_last/oldest_conversation_id`

**Lợi ích:**
- ✅ Đơn giản, dễ hiểu
- ✅ Tự động resume
- ✅ Không cần quản lý file checkpoint
- ✅ Conversation nào mới update → Job sync_new sẽ hứng
