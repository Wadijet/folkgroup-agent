# 🤖 AI Context Documentation

Thư mục này chứa tài liệu context chi tiết, đầy đủ để cung cấp cho AI (như ChatGPT, Claude, Cursor AI, v.v.) để xây dựng frontend application tích hợp với FolkForm Auth Backend API.

## 📋 Mục Đích

Tài liệu trong thư mục này được thiết kế đặc biệt để:
- Cung cấp đầy đủ context cho AI assistants
- Giúp AI hiểu rõ hệ thống backend và cách tích hợp
- Cung cấp code examples và best practices
- Hỗ trợ AI generate code chính xác và phù hợp

## 📚 Cấu Trúc Tài Liệu

### 1. [Frontend API Context](./frontend-api-context.md) ⭐ **BẮT ĐẦU TỪ ĐÂY**
Tài liệu chính, đầy đủ về:
- Tổng quan hệ thống
- Authentication & Authorization
- Tất cả Collections và Models
- API Endpoints chi tiết
- Error Handling
- Validation Rules

**Đây là file chính bạn nên cung cấp cho AI khi bắt đầu dự án frontend.**

### 2. [TypeScript Types & Interfaces](./types-and-interfaces.md)
Tất cả TypeScript interfaces và types được định nghĩa rõ ràng:
- User types
- RBAC types
- Facebook integration types
- Pancake integration types
- Agent types
- API Response types
- Error types

**Sử dụng file này khi cần generate TypeScript types.**

### 3. [Frontend Implementation Guide](./frontend-implementation-guide.md)
Hướng dẫn implementation chi tiết:
- API Client setup
- Auth Service implementation
- CRUD operations
- Error handling patterns
- State management recommendations
- Best practices

**Sử dụng file này khi cần hướng dẫn AI implement code.**

### 4. [Code Examples](./examples.md)
Các ví dụ code thực tế:
- Complete API Client implementation
- Auth flow examples
- CRUD operation examples
- Error handling examples
- React/Vue/Angular examples (nếu có)

**Sử dụng file này khi cần examples cụ thể.**

## 🚀 Cách Sử Dụng

### Cho AI Assistant (ChatGPT, Claude, Cursor AI, v.v.)

1. **Bắt đầu với file chính:**
   ```
   Đọc file: docs/09-ai-context/frontend-api-context.md
   ```

2. **Khi cần types:**
   ```
   Tham khảo: docs/09-ai-context/types-and-interfaces.md
   ```

3. **Khi cần implementation:**
   ```
   Tham khảo: docs/09-ai-context/frontend-implementation-guide.md
   ```

4. **Khi cần examples:**
   ```
   Tham khảo: docs/09-ai-context/examples.md
   ```

### Prompt Mẫu Cho AI

```
Tôi muốn xây dựng frontend application tích hợp với FolkForm Auth Backend API.

Vui lòng đọc và hiểu tài liệu context tại:
- docs/09-ai-context/frontend-api-context.md (file chính)
- docs/09-ai-context/types-and-interfaces.md (types)
- docs/09-ai-context/frontend-implementation-guide.md (implementation)

Sau đó giúp tôi:
1. Setup API Client
2. Implement Authentication flow
3. Implement CRUD operations cho User
4. Implement error handling
```

## 📝 Lưu Ý

- Tất cả tài liệu trong thư mục này được viết bằng **Tiếng Việt** (theo yêu cầu dự án)
- Tài liệu được cập nhật thường xuyên khi có thay đổi API
- Code examples sử dụng TypeScript và modern JavaScript
- Tất cả endpoints và models đều được mô tả chi tiết

## 🔄 Cập Nhật

Khi có thay đổi về:
- API endpoints mới
- Models/Interfaces mới
- Business logic mới
- Error codes mới

Vui lòng cập nhật các file tương ứng trong thư mục này.

## 📞 Hỗ Trợ

Nếu có câu hỏi hoặc cần bổ sung thông tin, vui lòng:
- Tạo issue trong repository
- Liên hệ team backend
- Cập nhật tài liệu và tạo pull request

---

**Phiên bản**: 1.0  
**Cập nhật lần cuối**: 2025-12-10

