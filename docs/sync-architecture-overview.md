# Kiến Trúc Đồng Bộ Dữ Liệu - Tổng Quan

**Ngày:** 2025-01-XX  
**Mục đích:** Tổng hợp toàn cảnh về dữ liệu cần sync và kiến trúc 2 chiều (incremental + full sync)

---

## 📊 Dữ Liệu Cần Đồng Bộ

### ✅ Đã Đồng Bộ (3/4)

| Loại Dữ Liệu | Pancake API | FolkForm Collection | Trạng Thái | Hàm Đồng Bộ |
|-------------|-------------|---------------------|------------|-------------|
| **Pages** | `GET /v1/pages` | `FbPage` | ✅ Đã sync | `Bridge_SyncPages()` |
| **Conversations** | `GET /pages/{page_id}/conversations` | `FbConversation` | ✅ Đã sync | `Bridge_SyncConversationsFromCloud()`, `Sync_NewMessagesOfPage()` |
| **Messages** | `GET /pages/{page_id}/conversations/{conversation_id}/messages` | `FbMessage`, `FbMessageItem` | ✅ Đã sync | `Bridge_SyncMessages()`, `bridge_SyncMessageOfConversation()` |

### ⚠️ Cần Đồng Bộ (1/4)

| Loại Dữ Liệu | Pancake API | FolkForm Collection | Trạng Thái | Độ Ưu Tiên |
|-------------|-------------|---------------------|------------|------------|
| **Posts** | `GET /pages/{page_id}/posts` | `FbPost` | ❌ Chưa sync | ⭐⭐⭐ Rất cao |

---

## 🏗️ Kiến Trúc 2 Chiều

### Chiều 1: Incremental Sync (Mới → Cũ) - ⚡ **NHANH**

**Mục đích:** Đồng bộ dữ liệu mới nhất, đảm bảo real-time

**Đặc điểm:**
- ✅ Chạy thường xuyên (mỗi 5 phút)
- ✅ Chỉ sync dữ liệu mới (từ lần sync cuối)
- ✅ Nhanh, ít tốn tài nguyên
- ✅ Đảm bảo dữ liệu mới luôn được cập nhật

**Logic:**
1. Lấy timestamp cuối cùng đã sync (`lastUpdatedAt`, `latestInsertedAt`)
2. Sync từ `lastUpdatedAt` → `now`
3. Chỉ lấy dữ liệu mới hơn timestamp đó

**Dữ liệu sync:**
- ✅ Conversations mới (dùng `since`/`until` với `panCakeUpdatedAt`)
- ✅ Messages mới (dùng `latestInsertedAt` để so sánh)
- ⚠️ Posts mới (chưa implement)

**Job:** `SyncNewJob` - Chạy mỗi 5 phút

---

### Chiều 2: Full Sync (Cũ → Mới) - 🐢 **CHẬM NHƯNG ĐẦY ĐỦ**

**Mục đích:** Đồng bộ toàn bộ lịch sử, đảm bảo không bỏ sót

**Đặc điểm:**
- ✅ Chạy nền, không giới hạn thời gian
- ✅ Sync từ đầu đến cuối (hoặc từ checkpoint)
- ✅ Có thể dừng giữa chừng và tiếp tục
- ✅ Đảm bảo dữ liệu đầy đủ

**Logic:**
1. Bắt đầu từ đầu (hoặc từ checkpoint nếu có)
2. Sync từng batch, lưu checkpoint sau mỗi batch
3. Tiếp tục cho đến khi hết dữ liệu
4. Có thể dừng và resume từ checkpoint

**Dữ liệu sync:**
- ✅ Conversations toàn bộ (không dùng `since`/`until`)
- ✅ Messages toàn bộ (dùng `current_count` pagination)
- ⚠️ Posts toàn bộ (chưa implement)

**Job:** `SyncAllDataJob` - Chạy mỗi ngày lúc 00:00:00 (hoặc chạy nền liên tục)

---

## 📋 Bố Trí Các Job

### Job 1: SyncNewJob (Incremental Sync)

**File:** `app/jobs/sync_new_job.go`

**Lịch chạy:** Mỗi 5 phút (`0 */5 * * * *`)

**Chức năng:**
```go
DoSyncNew() {
    SyncBaseAuth()  // Đăng nhập, sync pages
    
    // Sync conversations mới (dùng since/until)
    Sync_NewMessagesOfAllPages() {
        // Với mỗi page:
        Sync_NewMessagesOfPage() {
            // 1. Lấy lastUpdatedAt từ FolkForm
            // 2. Sync conversations từ lastUpdatedAt → now
            // 3. Với mỗi conversation:
            //    - Sync messages mới (dùng latestInsertedAt)
        }
    }
}
```

**Đặc điểm:**
- ⚡ Nhanh: Chỉ sync dữ liệu mới
- 🔄 Thường xuyên: Mỗi 5 phút
- 📊 Real-time: Đảm bảo dữ liệu mới luôn được cập nhật

---

### Job 2: SyncAllDataJob (Full Sync)

**File:** `app/jobs/sync_all_data_job.go`

**Lịch chạy:** Mỗi ngày lúc 00:00:00 (`0 0 0 * * *`) - **HOẶC** chạy nền liên tục

**Chức năng:**
```go
DoSyncAllData() {
    SyncBaseAuth()  // Đăng nhập, sync pages
    
    // Sync messages toàn bộ (từ đầu đến cuối)
    Bridge_SyncMessages() {
        // 1. Lấy tất cả conversations từ FolkForm
        // 2. Với mỗi conversation:
        //    - Đọc checkpoint (nếu có)
        //    - Sync messages từ checkpoint → đầu tiên
        //    - Lưu checkpoint sau mỗi batch
        //    - Có thể dừng và resume
    }
}
```

**Đặc điểm:**
- 🐢 Chậm: Sync toàn bộ lịch sử
- 🔄 Chạy nền: Không giới hạn thời gian
- 💾 Checkpoint: Có thể dừng và tiếp tục
- 📊 Đầy đủ: Đảm bảo không bỏ sót dữ liệu

---

## 🔄 Flow Hoạt Động

### Incremental Sync (Mỗi 5 phút)

```
┌─────────────────────────────────────────┐
│  SyncNewJob (Mỗi 5 phút)                │
├─────────────────────────────────────────┤
│  1. SyncBaseAuth()                       │
│     - Đăng nhập                          │
│     - Sync pages                         │
│                                          │
│  2. Sync_NewMessagesOfAllPages()        │
│     ┌──────────────────────────────────┐  │
│     │ Với mỗi page:                   │  │
│     │                                  │  │
│     │ Sync_NewMessagesOfPage()        │  │
│     │  1. Lấy lastUpdatedAt           │  │
│     │  2. Sync conversations mới       │  │
│     │     (since=lastUpdatedAt,        │  │
│     │      until=now)                  │  │
│     │                                  │  │
│     │  3. Với mỗi conversation:        │  │
│     │     - Lấy latestInsertedAt      │  │
│     │     - Sync messages mới         │  │
│     │       (chỉ messages mới hơn)    │  │
│     └──────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

### Full Sync (Chạy nền)

```
┌─────────────────────────────────────────┐
│  SyncAllDataJob (Chạy nền)             │
├─────────────────────────────────────────┤
│  1. SyncBaseAuth()                      │
│     - Đăng nhập                         │
│     - Sync pages                        │
│                                         │
│  2. Bridge_SyncMessages()               │
│     ┌─────────────────────────────────┐ │
│     │ Lấy tất cả conversations        │ │
│     │                                 │ │
│     │ Với mỗi conversation:          │ │
│     │   1. Đọc checkpoint (nếu có)   │ │
│     │      → current_count = X        │ │
│     │                                 │ │
│     │   2. Sync messages từ đầu       │ │
│     │      (current_count = 0)        │ │
│     │      HOẶC                        │ │
│     │      (current_count = X nếu có  │ │
│     │       checkpoint)                │ │
│     │                                 │ │
│     │   3. Sau mỗi batch:             │ │
│     │      - Lưu checkpoint           │ │
│     │      - Cập nhật current_count   │ │
│     │                                 │ │
│     │   4. Tiếp tục cho đến hết       │ │
│     │                                 │ │
│     │   5. Khi hoàn thành:            │ │
│     │      - Xóa checkpoint            │ │
│     └─────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

---

## 🎯 So Sánh 2 Chiều

| Tiêu chí | Incremental Sync | Full Sync |
|----------|-----------------|-----------|
| **Tần suất** | Mỗi 5 phút | Mỗi ngày / Chạy nền |
| **Tốc độ** | ⚡ Nhanh | 🐢 Chậm |
| **Dữ liệu** | Chỉ mới | Toàn bộ |
| **Mục đích** | Real-time update | Đảm bảo đầy đủ |
| **Checkpoint** | ❌ Không cần | ✅ Có |
| **Resume** | ❌ Không cần | ✅ Có thể dừng/tiếp tục |
| **Tài nguyên** | Ít | Nhiều |

---

## 📝 Implementation Plan

### Bước 1: Cải Thiện Full Sync với Checkpoint

**File:** `app/integrations/checkpoint.go` (mới)

```go
// Lưu checkpoint vào file JSON
type SyncCheckpoint struct {
    ConversationId string `json:"conversationId"`
    CurrentCount   int    `json:"currentCount"`
    LastSyncedAt   int64  `json:"lastSyncedAt"`
}

// Helper functions
func LoadCheckpoint(conversationId string) (*SyncCheckpoint, error)
func SaveCheckpoint(checkpoint *SyncCheckpoint) error
func DeleteCheckpoint(conversationId string) error
```

**File:** `app/integrations/bridge.go`

```go
func bridge_SyncMessageOfConversation(...) {
    // 1. Đọc checkpoint (nếu có)
    checkpoint, _ := LoadCheckpoint(conversation_id)
    if checkpoint != nil {
        current_count = checkpoint.CurrentCount
        log.Printf("Resume từ checkpoint: current_count=%d", current_count)
    }
    
    // 2. Vòng lặp sync
    for {
        // ... sync logic ...
        
        // 3. Sau mỗi batch thành công: Lưu checkpoint
        SaveCheckpoint(&SyncCheckpoint{
            ConversationId: conversation_id,
            CurrentCount:   current_count,
            LastSyncedAt:   time.Now().Unix(),
        })
    }
    
    // 4. Khi hoàn thành: Xóa checkpoint
    DeleteCheckpoint(conversation_id)
}
```

---

### Bước 2: Tách Logic Incremental và Full Sync

**Incremental Sync (Mới → Cũ):**
- Dùng `since`/`until` cho conversations
- Dùng `latestInsertedAt` cho messages
- Không cần checkpoint
- Chạy mỗi 5 phút

**Full Sync (Cũ → Mới):**
- Không dùng `since`/`until` (sync toàn bộ)
- Dùng `current_count` pagination
- Có checkpoint để resume
- Chạy nền liên tục

---

### Bước 3: Thêm Sync Posts

**Priority:** ⭐⭐⭐ Rất cao

**Implementation:**
1. Tạo `Pancake_GetPosts()` trong `pancake.go`
2. Tạo `FolkForm_CreateFbPost()` trong `folkform.go`
3. Tạo `Bridge_SyncPosts()` trong `bridge.go`
4. Thêm vào cả 2 job (incremental + full)

---

## ⚙️ Cấu Hình Job

### Option 1: Chạy Full Sync Mỗi Ngày

```go
// main.go
syncAllDataJob := jobs.NewSyncAllDataJob("sync-all-data-job", "0 0 0 * * *")
// Chạy mỗi ngày lúc 00:00:00
```

### Option 2: Chạy Full Sync Nền Liên Tục

```go
// main.go
// Chạy ngay lập tức và chạy nền
go func() {
    for {
        jobs.DoSyncAllData()
        time.Sleep(1 * time.Hour) // Nghỉ 1 giờ rồi chạy lại
    }
}()
```

**Khuyến nghị:** Option 2 (chạy nền liên tục) vì:
- Đảm bảo sync đầy đủ
- Có checkpoint nên có thể dừng/tiếp tục
- Không ảnh hưởng đến incremental sync

---

## 🔍 Monitoring & Logging

### Metrics Cần Theo Dõi

1. **Incremental Sync:**
   - Số conversations mới sync
   - Số messages mới sync
   - Thời gian thực thi
   - Tần suất chạy (mỗi 5 phút)

2. **Full Sync:**
   - Số conversations đã sync
   - Số messages đã sync
   - Checkpoint hiện tại
   - Thời gian thực thi
   - Tỷ lệ hoàn thành

### Log Format

```
[Incremental Sync] ✅ Đã sync 10 conversations mới, 50 messages mới (Thời gian: 30s)
[Full Sync] 📊 Đã sync 100/1000 conversations, 5000/50000 messages (Checkpoint: conversation_123, current_count=5000)
```

---

## ✅ Checklist Implementation

### Phase 1: Checkpoint System
- [ ] Tạo `checkpoint.go` với helper functions
- [ ] Sửa `bridge_SyncMessageOfConversation()` để dùng checkpoint
- [ ] Test resume từ checkpoint
- [ ] Test cleanup checkpoint khi hoàn thành

### Phase 2: Tách Logic 2 Chiều
- [ ] Đảm bảo incremental sync không dùng checkpoint
- [ ] Đảm bảo full sync dùng checkpoint
- [ ] Test cả 2 chiều hoạt động độc lập

### Phase 3: Sync Posts
- [ ] Implement `Pancake_GetPosts()`
- [ ] Implement `FolkForm_CreateFbPost()`
- [ ] Implement `Bridge_SyncPosts()`
- [ ] Thêm vào incremental sync
- [ ] Thêm vào full sync

### Phase 4: Job Configuration
- [ ] Cấu hình incremental sync (mỗi 5 phút)
- [ ] Cấu hình full sync (chạy nền)
- [ ] Test cả 2 job chạy đồng thời

---

## 🎯 Kết Luận

**Kiến trúc 2 chiều:**
1. **Incremental Sync (Mới → Cũ):** Nhanh, thường xuyên, real-time
2. **Full Sync (Cũ → Mới):** Chậm, chạy nền, đầy đủ, có checkpoint

**Lợi ích:**
- ✅ Đảm bảo dữ liệu mới luôn được cập nhật (incremental)
- ✅ Đảm bảo không bỏ sót dữ liệu cũ (full sync)
- ✅ Có thể dừng/tiếp tục (checkpoint)
- ✅ Tối ưu tài nguyên (incremental nhanh, full sync chạy nền)
