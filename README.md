# FolkForm Sync Agent

Hệ thống đồng bộ dữ liệu tự động giữa Pancake API và FolkForm Backend, được xây dựng bằng Go với scheduler và job system.

## 📋 Tổng Quan

FolkForm Sync Agent là một service chạy nền (background service) cung cấp các tính năng:

- 🔄 **Đồng Bộ Conversations**: Tự động sync conversations từ Pancake Pages API sang FolkForm
- 📨 **Đồng Bộ Messages**: Sync messages và message items từ Facebook
- 🛒 **Đồng Bộ Pancake POS**: Sync dữ liệu từ Pancake POS (shops, warehouses, products, orders, customers)
- ⏰ **Scheduler System**: Hệ thống lên lịch chạy jobs tự động
- ✅ **Verify & Recovery**: Kiểm tra và khôi phục dữ liệu đã sync

## 🚀 Bắt Đầu Nhanh

### Yêu Cầu Hệ Thống

- Go 1.23+
- MongoDB (để kết nối với FolkForm Backend)
- Pancake API credentials
- FolkForm Backend API đang chạy

### Cài Đặt

1. **Clone repository:**
```bash
git clone <repository-url>
cd folkgroup-agent
```

2. **Cài đặt dependencies:**
```bash
go mod download
```

3. **Cấu hình môi trường:**
```bash
# Copy file cấu hình mẫu
cp .env.example .env

# Chỉnh sửa các biến môi trường cần thiết
# - Pancake API credentials
# - FolkForm Backend API URL
# - MongoDB connection (nếu cần)
```

4. **Chạy agent:**
```bash
go run main.go
```

Agent sẽ tự động chạy các jobs theo lịch đã cấu hình.

## 📁 Cấu Trúc Dự Án

```
folkgroup-agent/
├── main.go                    # Entry point
├── app/
│   ├── jobs/                 # Các job sync
│   │   ├── sync_conversations.go
│   │   ├── sync_messages.go
│   │   ├── sync_pancake_pos.go
│   │   └── ...
│   └── scheduler/            # Scheduler system
├── config/                    # Configuration
├── global/                    # Global variables
├── utility/                   # Utility functions
│   ├── httpclient/           # HTTP client
│   ├── logger/               # Logging
│   └── hwid/                 # Hardware ID
└── docs/                     # Tài liệu
```

## 🔧 Cấu Hình

### Biến Môi Trường Quan Trọng

| Biến | Mô Tả | Ví Dụ |
|------|-------|-------|
| `PANCAKE_API_URL` | Pancake API base URL | `https://api.pancake.vn` |
| `PANCAKE_API_KEY` | Pancake API key | `your-api-key` |
| `FOLKFORM_API_URL` | FolkForm Backend API URL | `http://localhost:8080/api/v1` |
| `FOLKFORM_API_KEY` | FolkForm API key (nếu cần) | `your-api-key` |

Xem chi tiết tại [docs/README.md](docs/README.md)

## 📚 Tài Liệu

### Tài Liệu Chính

- [📖 Tổng Quan Tài Liệu](docs/README.md) - Index của tất cả tài liệu
- [🔄 Sync Implementation Guide](docs/sync-implementation-guide.md) - Hướng dẫn implement sync
- [🏗️ Sync Architecture](docs/sync-architecture-overview.md) - Kiến trúc hệ thống sync
- [📊 Sync Coverage Analysis](docs/sync-coverage-analysis.md) - Phân tích dữ liệu đã sync
- [🐛 Sync Issues Analysis](docs/sync-issues-analysis.md) - Phân tích các vấn đề

### Tài Liệu API (Workspace-level)

Tài liệu về API được quản lý tập trung tại workspace-level:

- [FolkForm API Context](../../docs/ai-context/folkform-api-context.md) - Tài liệu FolkForm API
- [FolkForm API Context](../../docs/ai-context/folkform-api-context.md) - Chi tiết FolkForm API
- [Pancake API Context](../../docs/ai-context/pancake-api-context.md) - Chi tiết Pancake API
- [Pancake POS API Context](../../docs/ai-context/pancake-pos-api-context.md) - Chi tiết Pancake POS API

## 🔄 Các Jobs Chính

### 1. Sync Incremental Conversations
- **Tên:** `sync-incremental-conversations-job`
- **Lịch:** Chạy mỗi 30 giây
- **Mục đích:** Sync conversations mới/cập nhật gần đây
- **Logic:** Incremental sync với `order_by=updated_at`, dừng khi gặp `lastConversationId`

### 2. Sync Backfill Conversations
- **Tên:** `sync-backfill-conversations-job`
- **Lịch:** Chạy mỗi 3 phút
- **Mục đích:** Sync conversations cũ hơn `oldestConversationId`
- **Logic:** Backfill sync để đảm bảo không bỏ sót dữ liệu

### 3. Sync Verify Conversations
- **Tên:** `sync-verify-conversations-job`
- **Lịch:** Chạy mỗi 30 giây
- **Mục đích:** Verify conversations đã sync
- **Logic:** So sánh dữ liệu giữa FolkForm và Pancake

### 4. Sync Pancake POS
- **Tên:** `sync-pancake-pos-*`
- **Lịch:** Tùy cấu hình
- **Mục đích:** Sync dữ liệu Pancake POS (shops, warehouses, products, orders, customers)

Xem chi tiết tại [docs/sync-implementation-guide.md](docs/sync-implementation-guide.md)

## 🛠️ Công Nghệ Sử Dụng

- **Language**: Go 1.23+
- **Scheduler**: robfig/cron
- **HTTP Client**: net/http
- **Logging**: log package (standard library)

## 📝 Ghi Chú

- Agent chạy như một background service
- Tất cả jobs được quản lý bởi scheduler
- Logs được ghi ra stdout để dễ theo dõi
- Agent tự động retry khi có lỗi

## 🔗 Liên Kết

- [Workspace Docs](../../docs/README.md) - Tài liệu workspace
- [Backend Docs](../../folkgroup-backend/docs/README.md) - Tài liệu Backend
- [Frontend Docs](../../folkgroup-frontend/docs/README.md) - Tài liệu Frontend

---

**Lưu ý**: Đây là tài liệu tổng quan. Để biết chi tiết, vui lòng xem các tài liệu trong thư mục `docs/`.
