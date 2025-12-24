# Đề Xuất: Tách Chiều Đồng Bộ Ngược (Verify) Ra Job Riêng

**Ngày:** 2025-01-XX  
**Mục đích:** Phân tích việc tách logic verify conversations từ FolkForm ra job riêng

---

## 📊 Tình Trạng Hiện Tại

### Logic Hiện Tại trong `BridgeV2_SyncNewData()`

**Bước 1:** Sync unseen conversations từ Pancake → FolkForm  
**Bước 2:** Sync read conversations mới hơn lastConversationId từ Pancake → FolkForm  
**Bước 3:** Verify unseen conversations từ FolkForm → Pancake (đồng bộ ngược)

**Tần suất:** Tất cả 3 bước chạy mỗi 1 phút

---

## ⚖️ Phân Tích: Tách Ra Job Riêng vs Giữ Nguyên

### ✅ Ưu Điểm Tách Ra Job Riêng

#### 1. **Tách Biệt Concerns (Separation of Concerns)**
- **Sync từ Pancake:** Đồng bộ dữ liệu mới từ Pancake về FolkForm
- **Verify từ FolkForm:** Đảm bảo đồng bộ 2 chiều, sửa lỗi không đồng bộ
- **Lợi ích:** Mỗi job có trách nhiệm rõ ràng, dễ maintain

#### 2. **Tần Suất Chạy Linh Hoạt**
- **Sync từ Pancake:** Cần chạy thường xuyên (mỗi 1 phút) để đảm bảo real-time
- **Verify từ FolkForm:** Có thể chạy ít hơn (mỗi 5-10 phút) vì:
  - Không cần real-time như sync chính
  - Tốn nhiều API calls (lấy từ FolkForm + verify với Pancake)
  - Chủ yếu để sửa lỗi không đồng bộ, không phải sync dữ liệu mới

#### 3. **Hiệu Suất và Tài Nguyên**
- **Sync từ Pancake:** Nhanh, ít API calls (chỉ gọi Pancake)
- **Verify từ FolkForm:** Chậm hơn, nhiều API calls:
  - Lấy conversations từ FolkForm (1 API call/page)
  - Verify với Pancake (nhiều API calls để tìm conversations)
- **Lợi ích:** Tách ra giúp sync chính không bị chậm bởi verify

#### 4. **Độc Lập Về Lỗi**
- Nếu verify lỗi → không ảnh hưởng đến sync chính
- Có thể retry verify độc lập
- Dễ debug và monitor riêng

#### 5. **Monitoring và Logging**
- Có thể monitor riêng:
  - Số lượng conversations được verify
  - Số lượng conversations được cập nhật
  - Thời gian thực thi của từng job
- Dễ phát hiện vấn đề

#### 6. **Scalability**
- Có thể scale độc lập:
  - Sync job: Cần nhiều resources để sync nhanh
  - Verify job: Có thể chạy trên instance khác, tần suất thấp hơn

---

### ❌ Nhược Điểm Tách Ra Job Riêng

#### 1. **Tăng Số Lượng Jobs**
- Hiện tại: 2 jobs (incremental + backfill)
- Sau khi tách: 3 jobs (incremental + backfill + verify)
- **Nhược điểm:** Quản lý nhiều jobs hơn

#### 2. **Phức Tạp Hơn**
- Cần tạo job mới
- Cần quản lý schedule riêng
- **Nhược điểm:** Code phức tạp hơn một chút

#### 3. **Có Thể Trễ Hơn**
- Nếu verify chạy ít hơn (5-10 phút) → có thể trễ hơn trong việc sửa lỗi không đồng bộ
- **Nhược điểm:** Nhưng không ảnh hưởng đến sync dữ liệu mới

---

## 🎯 So Sánh Chi Tiết

| Tiêu Chí | Giữ Nguyên (Hiện Tại) | Tách Ra Job Riêng |
|---------|----------------------|-------------------|
| **Tần suất sync chính** | Mỗi 1 phút | Mỗi 1 phút |
| **Tần suất verify** | Mỗi 1 phút | Mỗi 5-10 phút (đề xuất) |
| **Thời gian thực thi** | Lâu hơn (3 bước) | Nhanh hơn (sync chính) |
| **API calls** | Nhiều (cả sync + verify) | Ít hơn cho sync chính |
| **Độc lập lỗi** | ❌ Lỗi verify ảnh hưởng sync | ✅ Độc lập |
| **Dễ maintain** | ⚠️ Tất cả trong 1 hàm | ✅ Tách biệt rõ ràng |
| **Monitoring** | ⚠️ Khó tách biệt | ✅ Dễ monitor riêng |
| **Scalability** | ⚠️ Khó scale độc lập | ✅ Dễ scale độc lập |

---

## 💡 Đề Xuất

### ✅ **NÊN TÁCH RA JOB RIÊNG**

**Lý do:**
1. **Tách biệt concerns:** Sync và verify là 2 mục đích khác nhau
2. **Tần suất khác nhau:** Verify không cần real-time như sync
3. **Hiệu suất tốt hơn:** Sync chính không bị chậm bởi verify
4. **Dễ maintain:** Mỗi job có trách nhiệm rõ ràng
5. **Dễ scale:** Có thể scale độc lập

### 📋 Kiến Trúc Đề Xuất

#### Job 1: Sync Incremental Conversations (Từ Pancake)
- **Tên:** `sync-incremental-conversations-job`
- **Tần suất:** Mỗi 1 phút
- **Logic:**
  - Bước 1: Sync unseen conversations từ Pancake
  - Bước 2: Sync read conversations mới hơn lastConversationId từ Pancake
- **Mục đích:** Đồng bộ dữ liệu mới từ Pancake về FolkForm

#### Job 2: Sync Backfill Conversations (Từ Pancake)
- **Tên:** `sync-backfill-conversations-job`
- **Tần suất:** Mỗi 1 phút
- **Logic:** Sync conversations cũ hơn oldestConversationId
- **Mục đích:** Đồng bộ dữ liệu cũ từ Pancake về FolkForm

#### Job 3: Verify Conversations (Từ FolkForm) - **MỚI**
- **Tên:** `verify-conversations-job`
- **Tần suất:** Mỗi 5-10 phút (đề xuất: 5 phút)
- **Logic:**
  - Bước 1: Verify unseen conversations từ FolkForm với Pancake
  - Bước 2: Verify read conversations từ FolkForm với Pancake (nếu cần)
- **Mục đích:** Đảm bảo đồng bộ 2 chiều, sửa lỗi không đồng bộ

---

## 🔧 Implementation Plan

### Bước 1: Tạo Job Mới
- Tạo `SyncVerifyConversationsJob` trong `app/jobs/`
- Tạo hàm `BridgeV2_VerifyConversations()` trong `bridge_v2.go`

### Bước 2: Tách Logic Verify
- Di chuyển logic verify từ `BridgeV2_SyncNewData()` sang `BridgeV2_VerifyConversations()`
- Giữ lại chỉ sync từ Pancake trong `BridgeV2_SyncNewData()`

### Bước 3: Đăng Ký Job Mới
- Thêm job vào scheduler với tần suất 5 phút
- Cập nhật `main.go`

### Bước 4: Testing
- Test sync job (không có verify)
- Test verify job độc lập
- Test cả 2 jobs chạy cùng lúc

---

## 📊 Lợi Ích Cụ Thể

### 1. Hiệu Suất
- **Sync job:** Giảm thời gian thực thi từ ~30s xuống ~20s (ước tính)
- **Verify job:** Chạy độc lập, không ảnh hưởng sync

### 2. Tài Nguyên
- **API calls:** Giảm số lượng API calls cho sync job
- **Memory:** Tách biệt, dễ quản lý memory

### 3. Monitoring
- Có thể monitor riêng:
  - Sync job: Số conversations sync được
  - Verify job: Số conversations verify được, số conversations được cập nhật

### 4. Debugging
- Dễ debug hơn:
  - Nếu sync lỗi → chỉ cần xem sync job
  - Nếu verify lỗi → chỉ cần xem verify job

---

## ⚠️ Lưu Ý

### 1. Tần Suất Verify
- **Đề xuất:** Mỗi 5 phút
- **Lý do:** 
  - Đủ để sửa lỗi không đồng bộ
  - Không tốn quá nhiều tài nguyên
  - Có thể điều chỉnh sau

### 2. Thứ Tự Chạy
- Sync job và verify job có thể chạy song song
- Không cần đợi sync xong mới verify
- Upsert sẽ tự động xử lý conflict

### 3. Error Handling
- Mỗi job có error handling riêng
- Lỗi của job này không ảnh hưởng job kia

---

## ✅ Kết Luận

**Khuyến nghị:** ✅ **NÊN TÁCH RA JOB RIÊNG**

**Lý do chính:**
1. Tách biệt concerns rõ ràng
2. Tần suất chạy linh hoạt hơn
3. Hiệu suất tốt hơn
4. Dễ maintain và scale

**Next Steps:**
1. Implement job mới
2. Test kỹ lưỡng
3. Monitor và điều chỉnh tần suất nếu cần

