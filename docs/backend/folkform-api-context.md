# AI Context - Thông Tin Server API cho Frontend Development

## 📝 Changelog

### Version 2.9 - 2025-01-XX

#### 🔄 Customer Separation - Tách Riêng FB Customer và POS Customer

**Customer Architecture Refactoring:**
- **Tách riêng** customer thành 2 collections riêng biệt: `fb_customers` và `pc_pos_customers`
- **Lý do**: Đơn giản hóa logic, dễ maintain, phù hợp với use cases riêng biệt
- **FB Customer**: Dùng cho Facebook conversations, messages (từ Pancake API)
- **POS Customer**: Dùng cho orders, points, loyalty programs (từ Pancake POS API)

**Tính năng:**
- **FB Customer Collection** (`fb_customers`): Quản lý khách hàng từ Pancake API (Facebook)
  - Extract từ `panCakeData` với các field: `customerId`, `psid`, `pageId`, `name`, `phoneNumbers`, `email`, `birthday`, `gender`, `livesIn`
  - Unique indexes: `customerId`, `psid` (sparse)
  - Link với `fb_conversations` và `fb_messages` qua `psid` hoặc `customerId`
- **POS Customer Collection** (`pc_pos_customers`): Quản lý khách hàng từ Pancake POS API
  - Extract từ `posData` với các field: `customerId`, `shopId`, `name`, `phoneNumbers`, `emails`, `point`, `totalOrder`, `totalSpent`, etc.
  - Unique index: `customerId` (UUID string)
  - Link với `pc_pos_orders` qua `customerId`

**Permissions:**
- `FbCustomer.Insert`, `FbCustomer.Read`, `FbCustomer.Update`, `FbCustomer.Delete`
- `PcPosCustomer.Insert`, `PcPosCustomer.Read`, `PcPosCustomer.Update`, `PcPosCustomer.Delete`

**Endpoints:**
- `/api/v1/fb-customer/*` - Full CRUD operations cho FB Customer
- `/api/v1/pc-pos-customer/*` - Full CRUD operations cho POS Customer

**Migration:**
- Collection `customers` cũ vẫn hoạt động (deprecated) để tương thích ngược
- Bot sẽ đồng bộ lại dữ liệu vào 2 collections mới
- Khuyến nghị: Sử dụng endpoints mới cho các tính năng mới

**Lưu ý:**
- Mỗi collection phục vụ use case riêng, không cần merge logic phức tạp
- Nếu cần link giữa 2 collections, có thể dựa trên `phoneNumbers` hoặc `email` matching
- Đơn giản hơn, dễ maintain và mở rộng hơn so với multi-source merge

---

### Version 2.6 - 2025-01-XX (Deprecated)

#### ⚠️ Customer Multi-Source Integration - POS & Pancake (Đã Tách Riêng)

**⚠️ Lưu ý**: Tính năng này đã được tách riêng thành 2 collections trong Version 2.9. Xem phần **FB Customer** và **POS Customer** ở trên.

**Customer Multi-Source Support (Deprecated):**
- ~~Hỗ trợ sync customer từ nhiều nguồn: Pancake API và POS API~~
- ~~Thêm field `customerId` chung để identify customer từ cả 2 nguồn~~
- ~~Conflict resolution với merge strategies: priority, merge_array, keep_existing, overwrite~~

**Endpoints (Deprecated - Vẫn hoạt động để tương thích ngược):**
- `POST /api/v1/customer/upsert-one?filter={"customerId":"xxx"}` - ⚠️ Deprecated, dùng `/fb-customer` hoặc `/pc-pos-customer` thay thế

---

### Version 2.8 - 2025-01-XX

#### 🆕 Pancake POS API Integration - Order Module

**Pancake POS Order Integration:**
- Thêm collection `pc_pos_orders` để quản lý đơn hàng từ Pancake POS API
- Sử dụng CRUD chuẩn với cơ chế extract data tự động qua struct tag
- Model PcPosOrder với các trường extracted: `orderId`, `systemId`, `shopId`, `status`, `statusName`, `billFullName`, `billPhoneNumber`, `billEmail`, `customerId`, `warehouseId`, `shippingFee`, `totalDiscount`, `note`, `pageId`, `postId`, `insertedAt`, `posUpdatedAt`, `paidAt`, `orderItems`, `shippingAddress`, `warehouseInfo`, `customerInfo`
- Text indexes trên `orderId`, `shopId`, `billFullName`, `billPhoneNumber`, `billEmail`, `customerId`, `warehouseId`, `pageId`, `postId` để hỗ trợ tìm kiếm

**Tính năng:**
- **Tự động extract**: Data extraction tự động từ `posData` vào các trường typed
- **Upsert với filter**: Dùng `upsert-one` với filter để sync từ Pancake POS API
- **CRUD chuẩn**: Tất cả các CRUD endpoints chuẩn đều có sẵn
- **Order management**: Quản lý đơn hàng từ POS với đầy đủ thông tin billing, shipping, items, và customer

**Permissions:**
- `PcPosOrder.Insert`, `PcPosOrder.Read`, `PcPosOrder.Update`, `PcPosOrder.Delete`

**Endpoints:**
- `/api/v1/pancake-pos/order/*` - Full CRUD operations cho Order

**Lưu ý:**
- Order là core module trong hệ thống POS, cần thiết cho quản lý bán hàng và báo cáo
- Hỗ trợ đầy đủ thông tin đơn hàng: billing, shipping address, order items, warehouse info, customer info

---

### Version 2.7 - 2025-01-XX

#### 🆕 Pancake POS API Integration - Products Modules

**Pancake POS Product Integration:**
- Thêm collection `pc_pos_products` để quản lý sản phẩm từ Pancake POS API
- Sử dụng CRUD chuẩn với cơ chế extract data tự động qua struct tag
- Model PcPosProduct với các trường extracted: `productId`, `shopId`, `name`, `categoryIds`, `tagIds`, `isHide`, `noteProduct`, `productAttributes`
- Text indexes trên `productId`, `shopId`, `name` để hỗ trợ tìm kiếm

**Pancake POS Variation Integration:**
- Thêm collection `pc_pos_variations` để quản lý biến thể sản phẩm từ Pancake POS API
- Sử dụng CRUD chuẩn với cơ chế extract data tự động qua struct tag
- Model PcPosVariation với các trường extracted: `variationId`, `productId`, `shopId`, `sku`, `retailPrice`, `priceAtCounter`, `quantity`, `weight`, `fields`, `images`
- Unique index: `{variationId: 1}` để đảm bảo không duplicate variation
- Text indexes trên `variationId`, `productId`, `shopId`, `sku` để hỗ trợ tìm kiếm

**Pancake POS Category Integration:**
- Thêm collection `pc_pos_categories` để quản lý danh mục sản phẩm từ Pancake POS API
- Sử dụng CRUD chuẩn với cơ chế extract data tự động qua struct tag
- Model PcPosCategory với các trường extracted: `categoryId`, `shopId`, `name`
- Text indexes trên `categoryId`, `shopId`, `name` để hỗ trợ tìm kiếm

**Tính năng:**
- **Tự động extract**: Data extraction tự động từ `posData` vào các trường typed
- **Upsert với filter**: Dùng `upsert-one` với filter để sync từ Pancake POS API
- **CRUD chuẩn**: Tất cả các CRUD endpoints chuẩn đều có sẵn

**Permissions:**
- `PcPosProduct.Insert`, `PcPosProduct.Read`, `PcPosProduct.Update`, `PcPosProduct.Delete`
- `PcPosVariation.Insert`, `PcPosVariation.Read`, `PcPosVariation.Update`, `PcPosVariation.Delete`
- `PcPosCategory.Insert`, `PcPosCategory.Read`, `PcPosCategory.Update`, `PcPosCategory.Delete`

**Endpoints:**
- `/api/v1/pancake-pos/product/*` - Full CRUD operations cho Product
- `/api/v1/pancake-pos/variation/*` - Full CRUD operations cho Variation
- `/api/v1/pancake-pos/category/*` - Full CRUD operations cho Category

**Lưu ý:**
- Product, Variation và Category được đồng bộ từ Pancake POS API thông qua endpoint `upsert-one` với filter
- Đây là các module core trong hệ thống POS, cần thiết cho quản lý tồn kho và bán hàng

---

### Version 2.5 - 2025-01-XX

#### 🆕 Pancake POS API Integration - Shop & Warehouse Modules

**Pancake POS Shop Integration:**
- Thêm collection `pc_pos_shops` để quản lý cửa hàng từ Pancake POS API
- Sử dụng CRUD chuẩn với cơ chế extract data tự động qua struct tag
- Model PcPosShop với các trường extracted: `shopId`, `name`, `avatarUrl`, `pages`
- Unique index: `{shopId: 1}` để đảm bảo không duplicate shop

**Pancake POS Warehouse Integration:**
- Thêm collection `pc_pos_warehouses` để quản lý kho hàng từ Pancake POS API
- Sử dụng CRUD chuẩn với cơ chế extract data tự động qua struct tag
- Model PcPosWarehouse với các trường extracted: `warehouseId`, `shopId`, `name`, `phoneNumber`, `fullAddress`, `provinceId`, `districtId`, `communeId`
- Text indexes trên `warehouseId`, `shopId`, `name` để hỗ trợ tìm kiếm

**Tính năng:**
- **Tự động extract**: Data extraction tự động từ `panCakeData` vào các trường typed
- **Upsert với filter**: Dùng `upsert-one` với filter để sync từ Pancake POS API
- **CRUD chuẩn**: Tất cả các CRUD endpoints chuẩn đều có sẵn

**Permissions:**
- `PcPosShop.Insert`, `PcPosShop.Read`, `PcPosShop.Update`, `PcPosShop.Delete`
- `PcPosWarehouse.Insert`, `PcPosWarehouse.Read`, `PcPosWarehouse.Update`, `PcPosWarehouse.Delete`

**Endpoints:**
- `/api/v1/pancake-pos/shop/*` - Full CRUD operations cho Shop
- `/api/v1/pancake-pos/warehouse/*` - Full CRUD operations cho Warehouse

**Lưu ý:**
- Shop và Warehouse được đồng bộ từ Pancake POS API thông qua endpoint `upsert-one` với filter
- Đây là các module đầu tiên trong kế hoạch tích hợp đầy đủ Pancake POS API

---

### Version 2.4 - 2025-01-XX

#### 🆕 Customer API Module

**Customer Integration:**
- Thêm collection `customers` để quản lý khách hàng từ Pancake API
- Sử dụng CRUD chuẩn với cơ chế extract data tự động qua struct tag
- Model Customer với các trường extracted: `psid`, `pageId`, `name`, `phoneNumbers`, `email`, `birthday`, `gender`, `livesIn`
- Unique index: `{psid: 1, pageId: 1}` để đảm bảo không duplicate customer theo page

**Tính năng:**
- **Tự động extract**: Data extraction tự động từ `panCakeData` vào các trường typed
- **Upsert với filter**: Dùng `upsert-one` với filter `{"psid": "xxx", "pageId": "yyy"}` để sync từ Pancake
- **CRUD chuẩn**: Tất cả các CRUD endpoints chuẩn đều có sẵn

**Permissions:**
- `Customer.Insert` - Quyền tạo customer
- `Customer.Read` - Quyền đọc customer
- `Customer.Update` - Quyền cập nhật customer
- `Customer.Delete` - Quyền xóa customer

**Lưu ý:**
- Customer được đồng bộ từ Pancake API thông qua endpoint `upsert-one` với filter
- **Đã mở rộng**: Version 2.6 đã thêm hỗ trợ multi-source (POS + Pancake) với endpoint `/upsert-from-pos`

---

### Version 2.3 - 2025-12-16

#### 🆕 CRUD Endpoints Cho Message Items

**Facebook Message Item Integration:**
- Thêm đầy đủ CRUD endpoints cho collection `fb_message_items`
- `GET /api/v1/facebook/message-item/find-by-conversation/:conversationId` - Lấy message items theo conversationId với phân trang
- `GET /api/v1/facebook/message-item/find-by-message-id/:messageId` - Tìm message item theo messageId
- Tất cả các endpoint CRUD chuẩn: insert-one, insert-many, find, find-one, find-by-id, update-one, update-many, delete-one, delete-many, count, distinct, exists

**DTO mới:**
- `FbMessageItemCreateInput`: DTO cho tạo mới message item
- `FbMessageItemUpdateInput`: DTO cho cập nhật message item

**Permissions:**
- `FbMessageItem.Insert` - Quyền tạo message items
- `FbMessageItem.Read` - Quyền đọc message items
- `FbMessageItem.Update` - Quyền cập nhật message items
- `FbMessageItem.Delete` - Quyền xóa message items

**Lưu ý:**
- CRUD endpoints cho phép quản lý message items thủ công nếu cần
- Collection vẫn được quản lý tự động bởi endpoint `/upsert-messages` khi sync từ Pancake API
- Endpoint đặc biệt `/find-by-conversation/:conversationId` hỗ trợ phân trang với query params: `page` (default: 1), `limit` (default: 50, max: 100)

---

### Version 2.2 - 2025-12-16

#### 🆕 Endpoint Đặc Biệt Mới: Upsert Messages

**Facebook Message Integration:**
- `POST /api/v1/facebook/message/upsert-messages` - Upsert messages với logic tự động tách messages vào collection riêng

**Tính năng mới:**
- **Tự động tách messages**: Endpoint này tự động tách `messages[]` ra khỏi `panCakeData` và lưu vào 2 collections:
  - `fb_messages`: Metadata (không có messages[])
  - `fb_message_items`: Từng message riêng lẻ (mỗi message là 1 document)
- **Bulk upsert**: Tự động upsert nhiều messages cùng lúc, tránh duplicate theo `messageId`
- **Tự động cập nhật**: Cập nhật `totalMessages` và `lastSyncedAt` tự động
- **Tương thích ngược**: API bên ngoài vẫn gửi `panCakeData` đầy đủ (bao gồm messages[]), server tự động xử lý

**Model mới:**
- `FbMessageItem`: Model cho collection `fb_message_items` (từng message riêng lẻ)
- Cập nhật `FbMessage`: Thêm fields `lastSyncedAt`, `totalMessages`, `hasMore`

**DTO mới:**
- `FbMessageUpsertMessagesInput`: DTO riêng cho endpoint upsert-messages (có field `hasMore`)

**Lưu ý:**
- CRUD routes (`/insert-one`, `/update-one`, ...) vẫn hoạt động bình thường, không có logic tách messages
- Endpoint đặc biệt `/upsert-messages` hoàn toàn tách biệt với CRUD routes
- Dùng endpoint này khi sync messages từ Pancake API để tối ưu performance và scalability

#### 📚 Tài Liệu Mới
- Thêm mô tả chi tiết về endpoint `upsert-messages`
- Thêm model `FbMessageItem` và collection `fb_message_items`
- Thêm DTO `FbMessageUpsertMessagesInput`
- Cập nhật section về FbMessage Collection với kiến trúc mới (2 collections)

---

### Version 2.1 - 2025-12-12

#### ✅ Routes Đặc Biệt Mới Được Thêm

**Facebook Integration:**
- `GET /api/v1/facebook/page/find-by-page-id/:id` - Tìm page theo Facebook PageID
- `PUT /api/v1/facebook/page/update-token` - Cập nhật Page Access Token
- `GET /api/v1/facebook/post/find-by-post-id/:id` - Tìm post theo Facebook PostID
- `PUT /api/v1/facebook/post/update-token` - Cập nhật token của post

**RBAC Module:**
- `PUT /api/v1/user-role/update-user-roles` - Cập nhật hàng loạt roles cho user
- `GET /api/v1/permission/by-category/:category` - Lấy permissions theo category
- `GET /api/v1/permission/by-group/:group` - Lấy permissions theo group

#### 📚 Tài Liệu Mới
- Cập nhật endpoints đặc biệt cho Facebook Page và Post
- Thêm hướng dẫn sử dụng endpoint update-user-roles
- Thêm endpoints lọc permissions theo category và group

---

### Version 2.0 - 2025-12-12

#### 🔄 Thay Đổi Quan Trọng

**1. Scope Definition - Thay Đổi Cơ Bản:**
- **CŨ**: Scope có 3 mức (0: Read, 1: Write, 2: Delete)
- **MỚI**: Scope chỉ còn 2 mức về phạm vi tổ chức:
  - `scope = 0` (Default): Chỉ tổ chức role thuộc về
  - `scope = 1`: Tổ chức đó và tất cả các tổ chức con
- **Lý do**: Đơn giản hóa logic phân quyền, scope giờ chỉ ảnh hưởng đến phạm vi dữ liệu, không ảnh hưởng đến loại thao tác (Read/Insert/Update/Delete)
- **Migration**: Tất cả role permissions hiện có với scope = 0 giữ nguyên, scope = 1 hoặc 2 được chuyển thành scope = 1

**2. User Model - Cập Nhật:**
- Thêm field `tokens` (array): Danh sách tokens cho nhiều thiết bị (mỗi hwid có một token)
- Thêm field `isBlock`: Trạng thái bị khóa
- Thêm field `blockNote`: Ghi chú về việc bị khóa
- `email` và `phone` giờ là optional với sparse unique index (hỗ trợ Firebase authentication)

**3. Organization Model - Cập Nhật:**
- Thêm type `system` (Level -1): Tổ chức hệ thống cấp cao nhất, chứa Administrator
- Level -1: System (root organization)
- Level 0: Group (Tập đoàn)
- Level 1: Company (Công ty)
- Level 2: Department (Phòng ban)
- Level 3: Division (Bộ phận)
- Level 4+: Team

**4. RolePermission Model - Cập Nhật:**
- Thêm fields: `createdByRoleId`, `createdByUserId` (optional)
- Scope definition đã thay đổi (xem mục 1)

**5. Permissions - Danh Sách Đầy Đủ:**
- Thêm danh sách đầy đủ 50+ permissions được phân loại theo Category (Auth, Pancake) và Group
- Mỗi permission có mô tả bằng tiếng Việt

**6. UI Design Guide:**
- Thêm section "Hướng Dẫn Thiết Kế UI cho Phân Quyền" với 6 màn hình cụ thể
- Layout đề xuất, ví dụ code TypeScript, best practices

#### ✅ Đã Sửa

- Cập nhật tất cả comments và documentation về scope
- Thêm danh sách đầy đủ permissions
- Cập nhật User model với tokens, isBlock, blockNote
- Cập nhật Organization model với system type
- Thêm hướng dẫn UI design chi tiết

#### ⚠️ Breaking Changes

- **Scope values**: Nếu frontend đang sử dụng scope = 1 hoặc 2 với ý nghĩa cũ (Write/Delete), cần cập nhật logic để hiểu scope mới (phạm vi tổ chức)

#### 📚 Tài Liệu Mới

- Section "Hướng Dẫn Thiết Kế UI cho Phân Quyền"
- Danh sách đầy đủ permissions với mô tả
- Ví dụ code TypeScript cho permission management

---

### Version 1.0 - 2025-12-10

- Tài liệu ban đầu
- Mô tả cơ bản về API, models, endpoints

---

## 📋 Tổng Quan Hệ Thống

### Thông Tin Cơ Bản
- **Framework Backend**: Go (Golang) với Fiber v3
- **Database**: MongoDB
- **Base URL**: `http://localhost:8080/api/v1`
- **Authentication**: Firebase Authentication + JWT Token (Bearer Token)
- **Response Format**: JSON

### Mục Đích Hệ Thống
Hệ thống **FolkForm Auth Backend** là một hệ thống quản lý xác thực và phân quyền (RBAC) với các tính năng:
- **Firebase Authentication**: Đăng nhập bằng Firebase (Email, Phone OTP, Google, Facebook)
- Cấp quyền theo vai trò (Role-Based Access Control)
- Quản lý tổ chức (Organization) theo cấu trúc cây
- Tích hợp với Facebook (quản lý pages, posts, conversations, messages)
- Tích hợp với Pancake (quản lý đơn hàng)
- Quản lý Agent (trợ lý tự động) với check-in/check-out

---

## 🔐 Authentication & Authorization

### Cách Xác Thực
Tất cả các API (trừ auth endpoints) yêu cầu header:
```
Authorization: Bearer <token>
```

**Firebase Authentication Flow:**
1. Frontend sử dụng Firebase Client SDK để đăng nhập (Email/Password, Phone OTP, Google, Facebook)
2. Firebase trả về **Firebase ID Token**
3. Frontend gửi Firebase ID Token đến backend endpoint `/auth/login/firebase`
4. Backend verify Firebase ID Token và trả về **JWT Token** của hệ thống
5. Lưu JWT Token để sử dụng cho các request tiếp theo

**Lưu ý:** User được tạo tự động trong MongoDB khi đăng nhập lần đầu với Firebase.

### Permission System
Hệ thống sử dụng RBAC (Role-Based Access Control):
- **Permission**: Quyền cụ thể (ví dụ: `User.Read`, `Role.Update`)
- **Role**: Vai trò chứa nhiều permissions, thuộc về một Organization
- **User**: Người dùng có nhiều roles
- **Scope**: Phạm vi áp dụng quyền theo tổ chức (0: Chỉ tổ chức role thuộc về, 1: Tổ chức đó và tất cả các tổ chức con)

**Format permission:** `<Module>.<Action>`
- **Module**: User, Role, Permission, Agent, FbPage, FbPost, Organization, etc.
- **Action**: Read, Insert, Update, Delete, Block, CheckIn, CheckOut, etc.

**Ví dụ permissions:**
- `User.Read` - Đọc thông tin user
- `User.Insert` - Tạo user mới
- `User.Update` - Cập nhật user
- `User.Delete` - Xóa user
- `User.Block` - Khóa/mở khóa user
- `Role.Read` - Đọc thông tin role
- `Role.Update` - Cập nhật role
- `Permission.Read` - Đọc danh sách permissions
- `Organization.Read` - Đọc thông tin tổ chức
- `Organization.Update` - Cập nhật tổ chức
- `Agent.CheckIn` - Check-in agent
- `Agent.CheckOut` - Check-out agent
- `FbPage.Read` - Đọc thông tin Facebook page
- `FbPost.Read` - Đọc thông tin Facebook post

---

## 📡 Cấu Trúc Response

### Response Thành Công
```json
{
  "code": 200,
  "message": "Thao tác thành công",
  "data": { /* dữ liệu trả về */ },
  "status": "success"
}
```

### Response Lỗi
```json
{
  "code": "AUTH_001",
  "message": "Thông báo lỗi",
  "details": { /* chi tiết lỗi (nếu có) */ },
  "status": "error"
}
```

### HTTP Status Codes
- `200` - Thành công
- `201` - Tạo mới thành công
- `400` - Yêu cầu không hợp lệ
- `401` - Chưa xác thực
- `403` - Không có quyền truy cập
- `404` - Không tìm thấy
- `409` - Xung đột dữ liệu
- `500` - Lỗi server

---

## 📚 Mô Tả Collections & Tính Năng

### 1. Authentication Module (BẮT BUỘC)

#### User Collection
**Ý nghĩa**: Quản lý thông tin người dùng trong hệ thống
**Tính năng**:
- Đăng ký, đăng nhập, đăng xuất
- Quản lý profile (xem, cập nhật)
- Đổi mật khẩu
- Quản lý tokens (mỗi thiết bị có một token riêng dựa trên HWID)
- Block/Unblock user (chỉ admin)

**Cần thiết**: ⭐⭐⭐⭐⭐ (BẮT BUỘC - Core của hệ thống)

**Model:**
```typescript
interface Token {
  hwid: string;      // Hardware ID (unique per device)
  token: string;     // JWT token cho thiết bị này
  createdAt: number; // Thời gian tạo token
}

interface User {
  id: string;
  firebaseUid: string;      // Firebase User ID (unique, primary key)
  name: string;
  email?: string;            // Optional - có thể đăng nhập bằng phone (sparse unique index)
  emailVerified: boolean;
  phone?: string;            // Optional - có thể đăng nhập bằng email (sparse unique index)
  phoneVerified: boolean;
  avatarUrl?: string;        // URL avatar từ Firebase
  token: string;             // JWT token hiện tại (latest token)
  tokens: Token[];          // Danh sách tokens cho nhiều thiết bị (mỗi hwid có một token)
  isBlock: boolean;         // Trạng thái bị khóa (chỉ admin mới thấy)
  blockNote?: string;       // Ghi chú về việc bị khóa (chỉ admin mới thấy)
  createdAt: number;
  updatedAt: number;
}
```

**Lưu ý:**
- `email` và `phone` là optional vì hệ thống sử dụng Firebase authentication
- Mỗi thiết bị (hwid) có một token riêng trong mảng `tokens`
- `token` field chứa token mới nhất
- `isBlock` và `blockNote` không được trả về cho user thường, chỉ admin mới thấy

**Endpoints:**
- `/api/v1/user/*` - CRUD operations (Read-only cho user thường)
- `/api/v1/auth/login/firebase` - Đăng nhập bằng Firebase ID Token
- `/api/v1/auth/logout` - Đăng xuất
- `/api/v1/auth/profile` - Xem/Cập nhật profile
- `/api/v1/auth/roles` - Lấy danh sách roles của user

---

### 2. RBAC Module (BẮT BUỘC)

#### Permission Collection
**Ý nghĩa**: Định nghĩa các quyền trong hệ thống (tương ứng với các API endpoints)
**Tính năng**:
- Quản lý danh sách quyền (Read-only, được tạo tự động khi khởi tạo hệ thống)
- Mỗi quyền có format: `Module.Action` (ví dụ: `User.Read`, `Role.Update`)
- Phân loại theo Category và Group để dễ quản lý và hiển thị trong UI

**Cần thiết**: ⭐⭐⭐⭐⭐ (BẮT BUỘC - Core của hệ thống phân quyền)

**Model:**
```typescript
interface Permission {
  id: string;
  name: string;        // Format: "Module.Action" (ví dụ: "User.Read")
  describe: string;   // Mô tả quyền bằng tiếng Việt
  category: string;   // "Auth" hoặc "Pancake" - Phân loại module
  group: string;       // "User", "Role", "FbPage", etc. - Nhóm quyền
  createdAt: number;
  updatedAt: number;
}
```

**Danh sách đầy đủ các Permissions:**

**AUTH MODULE (Category: "Auth"):**

**User Management (Group: "User"):**
- `User.Insert` - Quyền tạo người dùng
- `User.Read` - Quyền xem danh sách người dùng
- `User.Update` - Quyền cập nhật thông tin người dùng
- `User.Delete` - Quyền xóa người dùng
- `User.Block` - Quyền khóa/mở khóa người dùng
- `User.SetRole` - Quyền phân quyền cho người dùng (gán roles)

**Organization Management (Group: "Organization"):**
- `Organization.Insert` - Quyền tạo tổ chức
- `Organization.Read` - Quyền xem danh sách tổ chức
- `Organization.Update` - Quyền cập nhật tổ chức
- `Organization.Delete` - Quyền xóa tổ chức

**Role Management (Group: "Role"):**
- `Role.Insert` - Quyền tạo vai trò
- `Role.Read` - Quyền xem danh sách vai trò
- `Role.Update` - Quyền cập nhật vai trò
- `Role.Delete` - Quyền xóa vai trò

**Permission Management (Group: "Permission"):**
- `Permission.Insert` - Quyền tạo quyền
- `Permission.Read` - Quyền xem danh sách quyền
- `Permission.Update` - Quyền cập nhật quyền
- `Permission.Delete` - Quyền xóa quyền

**RolePermission Management (Group: "RolePermission"):**
- `RolePermission.Insert` - Quyền tạo phân quyền cho vai trò
- `RolePermission.Read` - Quyền xem phân quyền của vai trò
- `RolePermission.Update` - Quyền cập nhật phân quyền của vai trò
- `RolePermission.Delete` - Quyền xóa phân quyền của vai trò

**UserRole Management (Group: "UserRole"):**
- `UserRole.Insert` - Quyền phân công vai trò cho người dùng
- `UserRole.Read` - Quyền xem vai trò của người dùng
- `UserRole.Update` - Quyền cập nhật vai trò của người dùng
- `UserRole.Delete` - Quyền xóa vai trò của người dùng

**Agent Management (Group: "Agent"):**
- `Agent.Insert` - Quyền tạo đại lý
- `Agent.Read` - Quyền xem danh sách đại lý
- `Agent.Update` - Quyền cập nhật thông tin đại lý
- `Agent.Delete` - Quyền xóa đại lý
- `Agent.CheckIn` - Quyền check-in đại lý
- `Agent.CheckOut` - Quyền check-out đại lý

**PANCAKE MODULE (Category: "Pancake"):**

**AccessToken Management (Group: "AccessToken"):**
- `AccessToken.Insert` - Quyền tạo token truy cập Pancake
- `AccessToken.Read` - Quyền xem danh sách token
- `AccessToken.Update` - Quyền cập nhật token
- `AccessToken.Delete` - Quyền xóa token

**Facebook Page Management (Group: "FbPage"):**
- `FbPage.Insert` - Quyền tạo trang Facebook
- `FbPage.Read` - Quyền xem danh sách trang Facebook
- `FbPage.Update` - Quyền cập nhật thông tin trang Facebook
- `FbPage.Delete` - Quyền xóa trang Facebook
- `FbPage.UpdateToken` - Quyền cập nhật token trang Facebook

**Facebook Conversation Management (Group: "FbConversation"):**
- `FbConversation.Insert` - Quyền tạo cuộc trò chuyện
- `FbConversation.Read` - Quyền xem danh sách cuộc trò chuyện
- `FbConversation.Update` - Quyền cập nhật cuộc trò chuyện
- `FbConversation.Delete` - Quyền xóa cuộc trò chuyện

**Facebook Message Management (Group: "FbMessage"):**
- `FbMessage.Insert` - Quyền tạo tin nhắn
- `FbMessage.Read` - Quyền xem danh sách tin nhắn
- `FbMessage.Update` - Quyền cập nhật tin nhắn
- `FbMessage.Delete` - Quyền xóa tin nhắn

**Facebook Post Management (Group: "FbPost"):**
- `FbPost.Insert` - Quyền tạo bài viết
- `FbPost.Read` - Quyền xem danh sách bài viết
- `FbPost.Update` - Quyền cập nhật bài viết
- `FbPost.Delete` - Quyền xóa bài viết

**Pancake Order Management (Group: "PcOrder"):**
- `PcOrder.Insert` - Quyền tạo đơn hàng
- `PcOrder.Read` - Quyền xem danh sách đơn hàng
- `PcOrder.Update` - Quyền cập nhật đơn hàng
- `PcOrder.Delete` - Quyền xóa đơn hàng

**Customer Management (Group: "Customer") - ⚠️ Deprecated:**
- `Customer.Insert` - Quyền tạo khách hàng (Deprecated - dùng FbCustomer hoặc PcPosCustomer)
- `Customer.Read` - Quyền xem danh sách khách hàng (Deprecated - dùng FbCustomer hoặc PcPosCustomer)
- `Customer.Update` - Quyền cập nhật thông tin khách hàng (Deprecated - dùng FbCustomer hoặc PcPosCustomer)
- `Customer.Delete` - Quyền xóa khách hàng (Deprecated - dùng FbCustomer hoặc PcPosCustomer)

**Facebook Customer Management (Group: "FbCustomer"):**
- `FbCustomer.Insert` - Quyền tạo khách hàng Facebook
- `FbCustomer.Read` - Quyền xem danh sách khách hàng Facebook
- `FbCustomer.Update` - Quyền cập nhật thông tin khách hàng Facebook
- `FbCustomer.Delete` - Quyền xóa khách hàng Facebook

**POS Customer Management (Group: "PcPosCustomer"):**
- `PcPosCustomer.Insert` - Quyền tạo khách hàng POS
- `PcPosCustomer.Read` - Quyền xem danh sách khách hàng POS
- `PcPosCustomer.Update` - Quyền cập nhật thông tin khách hàng POS
- `PcPosCustomer.Delete` - Quyền xóa khách hàng POS

**Pancake POS Shop Management (Group: "PcPosShop"):**
- `PcPosShop.Insert` - Quyền tạo cửa hàng từ Pancake POS
- `PcPosShop.Read` - Quyền xem danh sách cửa hàng từ Pancake POS
- `PcPosShop.Update` - Quyền cập nhật thông tin cửa hàng từ Pancake POS
- `PcPosShop.Delete` - Quyền xóa cửa hàng từ Pancake POS

**Pancake POS Warehouse Management (Group: "PcPosWarehouse"):**
- `PcPosWarehouse.Insert` - Quyền tạo kho hàng từ Pancake POS
- `PcPosWarehouse.Read` - Quyền xem danh sách kho hàng từ Pancake POS
- `PcPosWarehouse.Update` - Quyền cập nhật thông tin kho hàng từ Pancake POS
- `PcPosWarehouse.Delete` - Quyền xóa kho hàng từ Pancake POS

**Pancake POS Product Management (Group: "PcPosProduct"):**
- `PcPosProduct.Insert` - Quyền tạo sản phẩm từ Pancake POS
- `PcPosProduct.Read` - Quyền xem danh sách sản phẩm từ Pancake POS
- `PcPosProduct.Update` - Quyền cập nhật thông tin sản phẩm từ Pancake POS
- `PcPosProduct.Delete` - Quyền xóa sản phẩm từ Pancake POS

**Pancake POS Variation Management (Group: "PcPosVariation"):**
- `PcPosVariation.Insert` - Quyền tạo biến thể sản phẩm từ Pancake POS
- `PcPosVariation.Read` - Quyền xem danh sách biến thể sản phẩm từ Pancake POS
- `PcPosVariation.Update` - Quyền cập nhật thông tin biến thể sản phẩm từ Pancake POS
- `PcPosVariation.Delete` - Quyền xóa biến thể sản phẩm từ Pancake POS

**Pancake POS Category Management (Group: "PcPosCategory"):**
- `PcPosCategory.Insert` - Quyền tạo danh mục sản phẩm từ Pancake POS
- `PcPosCategory.Read` - Quyền xem danh sách danh mục sản phẩm từ Pancake POS
- `PcPosCategory.Update` - Quyền cập nhật thông tin danh mục sản phẩm từ Pancake POS
- `PcPosCategory.Delete` - Quyền xóa danh mục sản phẩm từ Pancake POS

**Pancake POS Order Management (Group: "PcPosOrder"):**
- `PcPosOrder.Insert` - Quyền tạo đơn hàng từ Pancake POS
- `PcPosOrder.Read` - Quyền xem danh sách đơn hàng từ Pancake POS
- `PcPosOrder.Update` - Quyền cập nhật thông tin đơn hàng từ Pancake POS
- `PcPosOrder.Delete` - Quyền xóa đơn hàng từ Pancake POS

**Gợi ý thiết kế UI cho Frontend:**

1. **Hiển thị danh sách Permissions:**
   - Nhóm theo Category (Auth, Pancake) → Tab hoặc Accordion
   - Trong mỗi Category, nhóm theo Group (User, Role, Organization, etc.) → Section hoặc Card
   - Hiển thị checkbox để chọn permissions khi gán cho role
   - Hiển thị tooltip với mô tả (`describe`) khi hover

2. **Gán Permissions cho Role:**
   - Tree view hoặc nested list theo Category → Group → Permissions
   - Checkbox "Select All" cho từng Group
   - Hiển thị scope selector (0 hoặc 1) cho mỗi permission được chọn
   - Preview tổng số permissions đã chọn

3. **Phân quyền theo Scope:**
   - Radio buttons hoặc Toggle: "Chỉ tổ chức này" (0) vs "Tổ chức và các tổ chức con" (1)
   - Tooltip giải thích rõ ràng sự khác biệt
   - Mặc định chọn "Chỉ tổ chức này" (0)
   - Hiển thị icon hoặc badge để phân biệt scope

4. **Validation:**
   - Kiểm tra user có quyền `RolePermission.Insert` hoặc `RolePermission.Update` trước khi cho phép gán
   - Hiển thị warning nếu gán scope = 1 cho role không thuộc root organization

**Endpoints:**
- `/api/v1/permission/*` - CRUD operations (Read-only)
- GET `/api/v1/permission` - Lấy danh sách tất cả permissions (có thể filter theo category, group)
- `GET /api/v1/permission/by-category/:category` - **Đặc biệt**: Lấy permissions theo category (Permission: `Permission.Read`)
- `GET /api/v1/permission/by-group/:group` - **Đặc biệt**: Lấy permissions theo group (Permission: `Permission.Read`)

---

#### Role Collection
**Ý nghĩa**: Định nghĩa các vai trò trong hệ thống, mỗi role thuộc về một Organization
**Tính năng**:
- Tạo, sửa, xóa vai trò
- Mỗi role thuộc về một Organization (bắt buộc)
- Tên role phải unique trong mỗi Organization
- Gán permissions cho role thông qua RolePermission

**Cần thiết**: ⭐⭐⭐⭐⭐ (BẮT BUỘC - Core của hệ thống phân quyền)

**Model:**
```typescript
interface Role {
  id: string;
  name: string;
  describe: string;
  organizationId: string; // BẮT BUỘC - Role thuộc Organization nào
  createdAt: number;
  updatedAt: number;
}
```

**Endpoints:**
- `/api/v1/role/*` - Full CRUD operations

---

#### RolePermission Collection
**Ý nghĩa**: Liên kết giữa Role và Permission, định nghĩa quyền của từng role và phạm vi áp dụng theo tổ chức
**Tính năng**:
- Gán permissions cho role với scope (phạm vi tổ chức)
- Cập nhật hàng loạt permissions của một role
- Quản lý quyền chi tiết cho từng role
- Kiểm soát phạm vi áp dụng quyền theo cấu trúc tổ chức

**Cần thiết**: ⭐⭐⭐⭐⭐ (BẮT BUỘC - Core của hệ thống phân quyền)

**Model:**
```typescript
interface RolePermission {
  id: string;
  roleId: string;              // ID của role
  permissionId: string;        // ID của permission
  scope: number;               // Phạm vi áp dụng quyền (0 hoặc 1)
  createdByRoleId?: string;    // ID của role tạo quyền này
  createdByUserId?: string;     // ID của user tạo quyền này
  createdAt: number;
  updatedAt: number;
}
```

**Scope Values (Phạm vi áp dụng quyền):**
- **`scope = 0`** (Default): **Chỉ tổ chức role thuộc về**
  - Quyền chỉ áp dụng cho tổ chức mà role thuộc về
  - User với role này chỉ có thể thao tác trên dữ liệu của tổ chức đó
  - Không thể truy cập dữ liệu của các tổ chức con
  - **Ví dụ**: Manager của "Phòng Kinh Doanh" chỉ quản lý được dữ liệu trong phòng đó
  
- **`scope = 1`**: **Tổ chức đó và tất cả các tổ chức con**
  - Quyền áp dụng cho tổ chức mà role thuộc về VÀ tất cả các tổ chức con
  - User với role này có thể thao tác trên dữ liệu của tổ chức đó và tất cả tổ chức con
  - **Ví dụ**: Director của "Công ty A" có thể quản lý dữ liệu của "Công ty A" và tất cả các phòng ban, bộ phận, team thuộc công ty đó
  - **Thường dùng cho**: Administrator role (thuộc root organization), Director, Manager cấp cao

**Lưu ý quan trọng:**
- Scope mặc định là `0` (zero value của number trong TypeScript/JavaScript)
- Khi tạo role permission mới, nếu không chỉ định scope, mặc định sẽ là `0`
- Administrator role thường có scope = 1 để quản lý toàn bộ hệ thống
- Scope chỉ ảnh hưởng đến phạm vi dữ liệu có thể truy cập, không ảnh hưởng đến loại thao tác (Read/Insert/Update/Delete)

**Endpoints:**
- `/api/v1/role-permission/*` - Full CRUD operations
- `/api/v1/role-permission/update-role` - Cập nhật hàng loạt permissions của role

**Ví dụ sử dụng trong Frontend:**
```typescript
// Tạo role permission với scope = 0 (chỉ tổ chức)
const createRolePermission = {
  roleId: "role123",
  permissionId: "permission456",
  scope: 0  // Chỉ tổ chức role thuộc về
};

// Tạo role permission với scope = 1 (tổ chức + các tổ chức con)
const createAdminPermission = {
  roleId: "adminRoleId",
  permissionId: "permission456",
  scope: 1  // Tổ chức + tất cả các tổ chức con
};

// UI nên hiển thị:
// - Checkbox hoặc Radio: "Chỉ tổ chức này" (scope = 0) vs "Tổ chức này và các tổ chức con" (scope = 1)
// - Tooltip giải thích rõ ràng sự khác biệt
// - Mặc định chọn "Chỉ tổ chức này" (scope = 0)
```

---

#### UserRole Collection
**Ý nghĩa**: Liên kết giữa User và Role, định nghĩa user có những roles nào
**Tính năng**:
- Gán roles cho user
- Một user có thể có nhiều roles
- Quản lý vai trò của từng user

**Cần thiết**: ⭐⭐⭐⭐⭐ (BẮT BUỘC - Core của hệ thống phân quyền)

**Model:**
```typescript
interface UserRole {
  id: string;
  userId: string;
  roleId: string;
  createdAt: number;
  updatedAt: number;
}
```

**Endpoints:**
- `/api/v1/user-role/*` - Full CRUD operations
  - `POST /api/v1/user-role/insert-one` - Tạo user role mới
  - `GET /api/v1/user-role/find` - Tìm user roles
  - `GET /api/v1/user-role/find-one` - Tìm một user role
  - `GET /api/v1/user-role/find-by-id/:id` - Tìm user role theo ID
  - `POST /api/v1/user-role/find-by-ids` - Tìm nhiều user roles theo IDs
  - `GET /api/v1/user-role/find-with-pagination` - Tìm với phân trang
  - `PUT /api/v1/user-role/update-one` - Cập nhật một user role
  - `PUT /api/v1/user-role/update-many` - Cập nhật nhiều user roles
  - `PUT /api/v1/user-role/update-by-id/:id` - Cập nhật user role theo ID
  - `DELETE /api/v1/user-role/delete-one` - Xóa một user role
  - `DELETE /api/v1/user-role/delete-many` - Xóa nhiều user roles
  - `DELETE /api/v1/user-role/delete-by-id/:id` - Xóa user role theo ID
  - `GET /api/v1/user-role/count` - Đếm số lượng user roles
  - `GET /api/v1/user-role/distinct` - Lấy danh sách giá trị duy nhất
  - `GET /api/v1/user-role/exists` - Kiểm tra user role có tồn tại không
- `PUT /api/v1/user-role/update-user-roles` - **Đặc biệt**: Cập nhật hàng loạt roles cho user (Permission: `UserRole.Update`)

**Request Body cho update-user-roles:**
```json
{
  "userId": "user-id-objectid",
  "roleIDs": ["role-id-1", "role-id-2", "role-id-3"]
}
```

**Lưu ý:** 
- Endpoint `update-user-roles` sẽ tự động xóa các roles cũ và thêm các roles mới cho user
- Đây là cách tiện lợi nhất để cập nhật roles cho user

---

## 🎨 Hướng Dẫn Thiết Kế UI cho Phân Quyền

### 1. Màn Hình Quản Lý Permissions

**Mục đích**: Hiển thị danh sách tất cả permissions trong hệ thống để tham khảo

**Layout đề xuất:**
```
┌─────────────────────────────────────────────────┐
│  Permissions Management                         │
├─────────────────────────────────────────────────┤
│  [Tab: Auth] [Tab: Pancake]                    │
├─────────────────────────────────────────────────┤
│  📁 Auth Module                                 │
│    ├─ 📁 User Management                        │
│    │   ├─ ☐ User.Insert (Quyền tạo người dùng) │
│    │   ├─ ☐ User.Read (Quyền xem...)           │
│    │   └─ ...                                   │
│    ├─ 📁 Role Management                        │
│    └─ ...                                       │
└─────────────────────────────────────────────────┘
```

**Tính năng:**
- Tab hoặc Accordion để phân loại theo Category (Auth, Pancake)
- Tree view hoặc nested list theo Category → Group → Permissions
- Hiển thị tooltip với mô tả (`describe`) khi hover vào permission
- Search/Filter để tìm kiếm nhanh
- Read-only (không cho phép edit vì permissions được tạo tự động)

### 2. Màn Hình Gán Permissions cho Role

**Mục đích**: Gán permissions cho một role với scope tương ứng

**Layout đề xuất:**
```
┌─────────────────────────────────────────────────────────────┐
│  Gán Quyền cho Role: "Manager"                              │
│  Tổ chức: "Công ty A"                                       │
├─────────────────────────────────────────────────────────────┤
│  [Tab: Auth] [Tab: Pancake]                                │
├─────────────────────────────────────────────────────────────┤
│  📁 Auth Module                                             │
│    ├─ 📁 User Management [Select All]                       │
│    │   ├─ ☑ User.Read                                       │
│    │   │   └─ Scope: ○ Chỉ tổ chức này                     │
│    │   │              ● Tổ chức và các tổ chức con           │
│    │   ├─ ☑ User.Update                                     │
│    │   │   └─ Scope: ● Chỉ tổ chức này                      │
│    │   └─ ☐ User.Delete                                     │
│    └─ ...                                                   │
├─────────────────────────────────────────────────────────────┤
│  Đã chọn: 15 permissions                                    │
│  [Hủy] [Lưu]                                                │
└─────────────────────────────────────────────────────────────┘
```

**Tính năng:**
- Tree view với checkbox cho mỗi permission
- "Select All" cho từng Group
- Scope selector (Radio buttons) cho mỗi permission được chọn:
  - "Chỉ tổ chức này" (scope = 0) - Mặc định
  - "Tổ chức và các tổ chức con" (scope = 1)
- Tooltip giải thích scope khi hover
- Preview tổng số permissions đã chọn
- Validation: Kiểm tra quyền `RolePermission.Insert`/`Update` trước khi cho phép

**Ví dụ code:**
```typescript
interface PermissionWithScope {
  permissionId: string;
  permissionName: string;
  scope: 0 | 1;  // 0: Chỉ tổ chức, 1: Tổ chức + con
  selected: boolean;
}

// Component state
const [selectedPermissions, setSelectedPermissions] = useState<PermissionWithScope[]>([]);

// Khi chọn permission
const handlePermissionToggle = (permissionId: string) => {
  // Toggle selection
};

// Khi thay đổi scope
const handleScopeChange = (permissionId: string, scope: 0 | 1) => {
  // Update scope
};

// Submit
const handleSubmit = async () => {
  const rolePermissions = selectedPermissions
    .filter(p => p.selected)
    .map(p => ({
      roleId: currentRoleId,
      permissionId: p.permissionId,
      scope: p.scope
    }));
  
  await api.post('/role-permission/update-role', {
    roleId: currentRoleId,
    permissionIds: rolePermissions.map(rp => rp.permissionId)
    // Note: Scope sẽ được set mặc định = 0, cần update riêng nếu muốn scope = 1
  });
};
```

### 3. Màn Hình Xem Permissions của Role

**Mục đích**: Hiển thị danh sách permissions đã được gán cho role

**Layout đề xuất:**
```
┌─────────────────────────────────────────────────────────────┐
│  Permissions của Role: "Manager"                            │
│  Tổ chức: "Công ty A"                                       │
├─────────────────────────────────────────────────────────────┤
│  📁 Auth Module                                             │
│    ├─ 📁 User Management                                    │
│    │   ├─ ✓ User.Read [Scope: Chỉ tổ chức này]            │
│    │   ├─ ✓ User.Update [Scope: Chỉ tổ chức này]           │
│    │   └─ ✓ User.Block [Scope: Tổ chức và các con]         │
│    └─ ...                                                   │
├─────────────────────────────────────────────────────────────┤
│  Tổng: 15 permissions                                       │
│  [Chỉnh sửa]                                                │
└─────────────────────────────────────────────────────────────┘
```

**Tính năng:**
- Hiển thị permissions đã được gán với scope tương ứng
- Badge hoặc icon để phân biệt scope:
  - 🏢 "Chỉ tổ chức này" (scope = 0)
  - 🌳 "Tổ chức và các tổ chức con" (scope = 1)
- Filter theo Category, Group
- Search permissions
- Nút "Chỉnh sửa" để vào màn hình gán permissions

### 4. Màn Hình Gán Roles cho User

**Mục đích**: Gán một hoặc nhiều roles cho user

**Layout đề xuất:**
```
┌─────────────────────────────────────────────────────────────┐
│  Gán Roles cho User: "Nguyễn Văn A"                          │
├─────────────────────────────────────────────────────────────┤
│  Tổ chức: "Công ty A"                                       │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Danh sách Roles:                                    │   │
│  │  ☑ Manager (Công ty A)                                │   │
│  │  ☐ Director (Công ty A)                               │   │
│  │  ☐ Employee (Phòng Kinh Doanh)                       │   │
│  └─────────────────────────────────────────────────────┘   │
│  [Hủy] [Lưu]                                                │
└─────────────────────────────────────────────────────────────┘
```

**Tính năng:**
- Hiển thị danh sách roles có thể gán (filter theo organization nếu cần)
- Checkbox để chọn nhiều roles
- Hiển thị organization của mỗi role
- Validation: Kiểm tra quyền `UserRole.Insert`/`Update`

### 5. Màn Hình Xem Roles và Permissions của User

**Mục đích**: Hiển thị tất cả roles và permissions mà user có

**Layout đề xuất:**
```
┌─────────────────────────────────────────────────────────────┐
│  Roles và Permissions của User: "Nguyễn Văn A"              │
├─────────────────────────────────────────────────────────────┤
│  📋 Roles:                                                  │
│    • Manager (Công ty A)                                    │
│    • Employee (Phòng Kinh Doanh)                            │
├─────────────────────────────────────────────────────────────┤
│  🔑 Permissions (tổng hợp từ tất cả roles):                │
│    📁 Auth Module                                           │
│      ├─ ✓ User.Read                                        │
│      ├─ ✓ User.Update                                      │
│      └─ ✓ Organization.Read                                │
│    📁 Pancake Module                                        │
│      └─ ✓ FbPage.Read                                      │
├─────────────────────────────────────────────────────────────┤
│  [Chỉnh sửa Roles]                                          │
└─────────────────────────────────────────────────────────────┘
```

**Tính năng:**
- Hiển thị danh sách roles của user
- Hiển thị tổng hợp tất cả permissions từ các roles
- Group permissions theo Category và Group
- Highlight permissions có scope = 1 (tổ chức + con)
- Nút "Chỉnh sửa Roles" để vào màn hình gán roles

### 6. Best Practices cho UI

1. **Scope Selector:**
   - Luôn hiển thị rõ ràng 2 options với tooltip
   - Mặc định chọn scope = 0
   - Disable scope = 1 nếu role không thuộc root organization (trừ admin)

2. **Permission Tree:**
   - Sử dụng tree view hoặc nested list
   - Collapse/Expand cho từng Group
   - "Select All" cho từng Group và Category

3. **Visual Indicators:**
   - Icon/badge để phân biệt scope
   - Color coding: scope = 0 (blue), scope = 1 (green)
   - Tooltip giải thích khi hover

4. **Validation & Feedback:**
   - Kiểm tra quyền trước khi hiển thị form
   - Hiển thị error message rõ ràng
   - Loading state khi submit
   - Success notification sau khi lưu

5. **Responsive Design:**
   - Mobile: Accordion thay vì tabs
   - Tablet: Sidebar với tree view
   - Desktop: Full layout với tất cả tính năng

---

#### Organization Collection
**Ý nghĩa**: Quản lý cấu trúc tổ chức theo dạng cây (System → Tập đoàn → Công ty → Phòng ban → Bộ phận → Team)
**Tính năng**:
- Quản lý cấu trúc tổ chức phân cấp
- Hỗ trợ 6 loại: System, Group, Company, Department, Division, Team
- Mỗi organization có parent (null nếu là System root)
- Lưu path và level để truy vấn nhanh
- Roles thuộc về Organization
- System organization (Level -1) là cấp cao nhất, chứa Administrator, không thể xóa

**Cần thiết**: ⭐⭐⭐⭐ (RẤT QUAN TRỌNG - Nếu hệ thống cần phân quyền theo tổ chức)

**Model:**
```typescript
type OrganizationType = "system" | "group" | "company" | "department" | "division" | "team";

interface Organization {
  id: string;
  name: string;
  code: string;        // Unique code
  type: OrganizationType; // Loại tổ chức
  parentId?: string;    // ID của organization cha (null nếu là System root)
  path: string;         // Đường dẫn cây (ví dụ: "/system/root_group/company1/dept1")
  level: number;        // Cấp độ (-1 = System, 0 = Group, 1 = Company, 2 = Department, ...)
  isActive: boolean;
  createdAt: number;
  updatedAt: number;
}
```

**Organization Types và Levels:**
- **System** (type: "system", level: -1): Tổ chức hệ thống, cấp cao nhất, chứa Administrator role, không thể xóa
- **Group** (type: "group", level: 0): Tập đoàn
- **Company** (type: "company", level: 1): Công ty
- **Department** (type: "department", level: 2): Phòng ban
- **Division** (type: "division", level: 3): Bộ phận
- **Team** (type: "team", level: 4+): Team

**Lưu ý:**
- System organization được tạo tự động khi khởi tạo hệ thống
- Administrator role luôn thuộc về System organization
- Khi tạo organization mới, level được tính tự động dựa trên parent

**Endpoints:**
- `/api/v1/organization/*` - Full CRUD operations

---

### 3. Agent Module (TÙY CHỌN - Nếu cần tự động hóa)

#### Agent Collection
**Ý nghĩa**: Quản lý các trợ lý tự động (AI Agent) thực hiện các tác vụ tự động
**Tính năng**:
- Tạo, quản lý agent
- Agent được gán thông tin đăng nhập của user để thực hiện hành động
- Check-in/Check-out để cập nhật trạng thái hoạt động
- Quản lý trạng thái (offline/online) và lệnh điều khiển (stop/play)
- Gán users cho agent
- Lưu config data cho agent

**Cần thiết**: ⭐⭐⭐ (TÙY CHỌN - Chỉ cần nếu hệ thống có tính năng tự động hóa)

**Model:**
```typescript
interface Agent {
  id: string;
  name: string;
  describe: string;
  status: number; // 0: offline, 1: online
  command: number; // 0: stop, 1: play
  assignedUsers: string[]; // Array of user IDs
  configData: Record<string, any>; // Cấu hình agent
  createdAt: number;
  updatedAt: number;
}
```

**Endpoints:**
- `/api/v1/agent/*` - Full CRUD operations
- `/api/v1/agent/check-in/:id` - Check-in agent (cập nhật trạng thái online)
- `/api/v1/agent/check-out/:id` - Check-out agent (cập nhật trạng thái offline)

**Lưu ý**: Agent cần check-in thường xuyên (mỗi 5 phút) để duy trì trạng thái online. Nếu không check-in sau 5 phút, hệ thống tự động chuyển về offline.

---

### 4. Facebook Integration Module (TÙY CHỌN - Nếu cần tích hợp Facebook)

#### AccessToken Collection
**Ý nghĩa**: Quản lý các access tokens để truy cập vào các hệ thống bên ngoài (Facebook, Pancake, etc.)
**Tính năng**:
- Lưu trữ access tokens cho các hệ thống khác
- Gán tokens cho users
- Quản lý trạng thái active/inactive

**Cần thiết**: ⭐⭐⭐ (TÙY CHỌN - Chỉ cần nếu tích hợp với hệ thống bên ngoài)

**Model:**
```typescript
interface AccessToken {
  id: string;
  name: string; // Unique name
  describe: string;
  system: string; // Hệ thống (Facebook, Pancake, etc.)
  value: string; // Token value
  assignedUsers: string[]; // Array of user IDs
  status: number; // 0: active, 1: inactive
  createdAt: number;
  updatedAt: number;
}
```

**Endpoints:**
- `/api/v1/access-token/*` - Full CRUD operations

---

#### FbPage Collection
**Ý nghĩa**: Quản lý các Facebook Pages được kết nối với hệ thống
**Tính năng**:
- Lưu thông tin Facebook Pages
- Quản lý Page Access Token
- Đồng bộ dữ liệu từ Pancake (panCakeData)
- Quản lý trạng thái đồng bộ (isSync)

**Cần thiết**: ⭐⭐⭐ (TÙY CHỌN - Chỉ cần nếu tích hợp Facebook)

**Model:**
```typescript
interface FbPage {
  id: string;
  pageName: string;
  pageUsername: string;
  pageId: string; // Facebook Page ID (unique)
  isSync: boolean; // Trạng thái đồng bộ
  accessToken: string;
  pageAccessToken: string; // Page Access Token
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API
  createdAt: number;
  updatedAt: number;
}
```

**Endpoints:**
- `/api/v1/facebook/page/*` - Full CRUD operations
- `GET /api/v1/facebook/page/find-by-page-id/:id` - **Đặc biệt**: Tìm page theo PageID (Permission: `FbPage.Read`)
- `PUT /api/v1/facebook/page/update-token` - **Đặc biệt**: Cập nhật Page Access Token (Permission: `FbPage.Update`)

---

#### FbPost Collection
**Ý nghĩa**: Quản lý các Facebook Posts từ các Pages
**Tính năng**:
- Lưu thông tin các bài viết trên Facebook
- Liên kết với FbPage
- Đồng bộ dữ liệu từ Pancake

**Cần thiết**: ⭐⭐ (TÙY CHỌN - Chỉ cần nếu cần quản lý Facebook Posts)

**Model:**
```typescript
interface FbPost {
  id: string;
  pageId: string; // Reference to FbPage (tự động extract từ panCakeData.page_id)
  postId: string; // Facebook Post ID (unique, tự động extract từ panCakeData.id)
  insertedAt: number; // Thời gian insert bài viết (tự động extract từ panCakeData.inserted_at, convert sang Unix timestamp)
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API
  createdAt: number;
  updatedAt: number;
}
```

**Lưu ý về Data Extraction:**
- Hệ thống tự động extract các trường từ `panCakeData` khi insert/update:
  - `pageId` ← `panCakeData.page_id` (required)
  - `postId` ← `panCakeData.id` (required)
  - `insertedAt` ← `panCakeData.inserted_at` (convert từ ISO 8601 string sang Unix timestamp milliseconds)
- Khi gửi request, chỉ cần gửi `panCakeData`, các trường khác sẽ được extract tự động
- Format `panCakeData.inserted_at`: `"2006-01-02T15:04:05"` (ISO 8601)

**Endpoints:**
- `/api/v1/facebook/post/*` - Full CRUD operations
- `GET /api/v1/facebook/post/find-by-post-id/:id` - **Đặc biệt**: Tìm post theo PostID (Permission: `FbPost.Read`)
- `PUT /api/v1/facebook/post/update-token` - **Đặc biệt**: Cập nhật token của post (Permission: `FbPost.Update`)

---

#### FbConversation Collection
**Ý nghĩa**: Quản lý các cuộc trò chuyện (conversations) trên Facebook Messenger
**Tính năng**:
- Lưu thông tin conversations từ Facebook Pages
- Theo dõi thời gian cập nhật từ API (panCakeUpdatedAt)
- Liên kết với FbPage và Customer
- Endpoint đặc biệt để lấy conversations sắp xếp theo thời gian cập nhật API

**Cần thiết**: ⭐⭐⭐ (TÙY CHỌN - Chỉ cần nếu cần quản lý Facebook Conversations)

**Model:**
```typescript
interface FbConversation {
  id: string;                    // MongoDB ObjectID
  pageId: string;                // Reference to FbPage
  pageUsername: string;           // Tên người dùng của trang
  conversationId: string;         // Facebook Conversation ID từ Pancake (unique, tự động extract từ panCakeData.id)
  customerId: string;             // Facebook Customer ID (tự động extract từ panCakeData.customer_id, optional)
  panCakeData: Record<string, any>; // Dữ liệu gốc từ Pancake API
  panCakeUpdatedAt: number;      // Thời gian cập nhật từ Pancake API (tự động extract từ panCakeData.updated_at)
  createdAt: number;             // Thời gian tạo trong MongoDB
  updatedAt: number;             // Thời gian cập nhật trong MongoDB
}
```

**Lưu ý về Data Extraction:**
- Hệ thống tự động extract các trường từ `panCakeData` khi insert/update:
  - `conversationId` ← `panCakeData.id` (required)
  - `customerId` ← `panCakeData.customer_id` (optional)
  - `panCakeUpdatedAt` ← `panCakeData.updated_at` (convert từ ISO 8601 string sang Unix timestamp, optional)
- Khi gửi request, chỉ cần gửi `panCakeData`, các trường khác sẽ được extract tự động
- Format `panCakeData.updated_at`: `"2006-01-02T15:04:05.000000"` (ISO 8601)

**Endpoints:**
- `/api/v1/facebook/conversation/*` - Full CRUD operations
  - `POST /api/v1/facebook/conversation/insert-one` - Tạo conversation mới
  - `GET /api/v1/facebook/conversation/find` - Tìm conversations
  - `GET /api/v1/facebook/conversation/find-one` - Tìm một conversation
  - `GET /api/v1/facebook/conversation/find-by-id/:id` - Tìm conversation theo ID
  - `POST /api/v1/facebook/conversation/find-by-ids` - Tìm nhiều conversations theo IDs
  - `GET /api/v1/facebook/conversation/find-with-pagination` - Tìm với phân trang
  - `GET /api/v1/facebook/conversation/sort-by-api-update` - **Đặc biệt**: Lấy danh sách conversations sắp xếp theo thời gian cập nhật API (panCakeUpdatedAt) (Permission: `FbConversation.Read`)
  - `PUT /api/v1/facebook/conversation/update-one` - Cập nhật một conversation
  - `PUT /api/v1/facebook/conversation/update-many` - Cập nhật nhiều conversations
  - `PUT /api/v1/facebook/conversation/update-by-id/:id` - Cập nhật conversation theo ID
  - `DELETE /api/v1/facebook/conversation/delete-one` - Xóa một conversation
  - `DELETE /api/v1/facebook/conversation/delete-many` - Xóa nhiều conversations
  - `DELETE /api/v1/facebook/conversation/delete-by-id/:id` - Xóa conversation theo ID
  - `GET /api/v1/facebook/conversation/count` - Đếm số lượng conversations
  - `GET /api/v1/facebook/conversation/distinct` - Lấy danh sách giá trị duy nhất
  - `GET /api/v1/facebook/conversation/exists` - Kiểm tra conversation có tồn tại không

**Endpoint Đặc Biệt: `sort-by-api-update`**

**Mục đích:** Lấy danh sách conversations sắp xếp theo thời gian cập nhật từ Pancake API (panCakeUpdatedAt), hữu ích cho việc đồng bộ dữ liệu từ Pancake.

**Query Parameters:**
- `page` (integer, optional): Số trang (mặc định: 1)
- `limit` (integer, optional): Số lượng mỗi trang (mặc định: 10)
- `pageId` (string, optional): Lọc theo page ID

**Response:**
```json
{
  "code": 200,
  "status": "success",
  "data": {
    "page": 1,
    "limit": 10,
    "itemCount": 5,
    "total": 50,
    "totalPage": 5,
    "items": [
      {
        "id": "conversation_mongodb_id",
        "pageId": "facebook_page_id",
        "pageUsername": "page_username",
        "conversationId": "pancake_conversation_id",
        "customerId": "customer_id",
        "panCakeData": {
          "id": "pancake_conversation_id",
          "type": "INBOX",
          "updated_at": "2019-08-24T14:15:22Z",
          "tags": ["tag1", "tag2"]
        },
        "panCakeUpdatedAt": 1234567890,
        "createdAt": 1234567890,
        "updatedAt": 1234567890
      }
    ]
  }
}
```

**Lưu ý:**
- Conversations được sắp xếp theo `panCakeUpdatedAt` giảm dần (cũ nhất trước)
- Hữu ích để lấy conversations cần đồng bộ lại từ Pancake API
- `conversationId` được tự động extract từ `panCakeData.id` khi insert/update

---

#### FbMessage Collection
**Ý nghĩa**: Quản lý metadata của conversations trên Facebook Messenger (không lưu messages[])
**Tính năng**:
- Lưu metadata của conversations (panCakeData không có messages[])
- Liên kết với FbPage và FbConversation
- Đồng bộ dữ liệu từ Pancake
- Tracking: `lastSyncedAt`, `totalMessages`, `hasMore`

**Cần thiết**: ⭐⭐ (TÙY CHỌN - Chỉ cần nếu cần quản lý chi tiết Facebook Messages)

**Model:**
```typescript
interface FbMessage {
  id: string;                    // MongoDB ObjectID
  pageId: string;                // Reference to FbPage
  pageUsername: string;           // Tên người dùng của trang
  conversationId: string;         // Facebook Conversation ID (unique, tự động extract từ panCakeData.conversation_id)
  customerId: string;             // Facebook Customer ID
  panCakeData: Record<string, any>; // Dữ liệu gốc từ Pancake API (KHÔNG có messages[])
  lastSyncedAt: number;          // Thời gian sync cuối cùng
  totalMessages: number;         // Tổng số messages trong fb_message_items
  hasMore: boolean;              // Còn messages để sync không
  createdAt: number;             // Thời gian tạo trong MongoDB
  updatedAt: number;             // Thời gian cập nhật trong MongoDB
}
```

**Lưu ý về Data Extraction:**
- Hệ thống tự động extract `conversationId` từ `panCakeData.conversation_id` khi insert/update
- Khi gửi request, chỉ cần gửi `panCakeData`, `conversationId` sẽ được extract tự động
- **Quan trọng**: `panCakeData` trong `fb_messages` KHÔNG chứa `messages[]` (messages được lưu riêng trong `fb_message_items`)

**Endpoints:**
- `/api/v1/facebook/message/*` - Full CRUD operations (Logic chung - không tách messages)
  - `POST /api/v1/facebook/message/insert-one` - Tạo message mới (Permission: `FbMessage.Insert`)
  - `GET /api/v1/facebook/message/find` - Tìm messages (Permission: `FbMessage.Read`)
  - `GET /api/v1/facebook/message/find-one` - Tìm một message (Permission: `FbMessage.Read`)
  - `GET /api/v1/facebook/message/find-by-id/:id` - Tìm message theo ID (Permission: `FbMessage.Read`)
  - `POST /api/v1/facebook/message/find-by-ids` - Tìm nhiều messages theo IDs (Permission: `FbMessage.Read`)
  - `GET /api/v1/facebook/message/find-with-pagination` - Tìm với phân trang (Permission: `FbMessage.Read`)
  - `PUT /api/v1/facebook/message/update-one` - Cập nhật một message (Permission: `FbMessage.Update`)
  - `PUT /api/v1/facebook/message/update-many` - Cập nhật nhiều messages (Permission: `FbMessage.Update`)
  - `PUT /api/v1/facebook/message/update-by-id/:id` - Cập nhật message theo ID (Permission: `FbMessage.Update`)
  - `PUT /api/v1/facebook/message/find-one-and-update` - Tìm và cập nhật message (Permission: `FbMessage.Update`)
  - `DELETE /api/v1/facebook/message/delete-one` - Xóa một message (Permission: `FbMessage.Delete`)
  - `DELETE /api/v1/facebook/message/delete-many` - Xóa nhiều messages (Permission: `FbMessage.Delete`)
  - `DELETE /api/v1/facebook/message/delete-by-id/:id` - Xóa message theo ID (Permission: `FbMessage.Delete`)
  - `DELETE /api/v1/facebook/message/find-one-and-delete` - Tìm và xóa message (Permission: `FbMessage.Delete`)
  - `GET /api/v1/facebook/message/count` - Đếm số lượng messages (Permission: `FbMessage.Read`)
  - `GET /api/v1/facebook/message/distinct` - Lấy danh sách giá trị duy nhất (Permission: `FbMessage.Read`)
  - `GET /api/v1/facebook/message/exists` - Kiểm tra message có tồn tại không (Permission: `FbMessage.Read`)
- `POST /api/v1/facebook/message/upsert-messages` - **Đặc biệt**: Upsert messages với logic tự động tách (Permission: `FbMessage.Update`)

---

#### FbMessageItem Collection
**Ý nghĩa**: Quản lý từng message riêng lẻ trong conversations (mỗi message là 1 document)
**Tính năng**:
- Lưu từng message riêng lẻ (mỗi message là 1 document)
- Liên kết với conversation qua `conversationId`
- Tự động tránh duplicate theo `messageId` (unique)
- Hỗ trợ query và phân trang hiệu quả

**Cần thiết**: ⭐⭐ (TÙY CHỌN - Chỉ cần nếu cần quản lý chi tiết Facebook Messages)

**Model:**
```typescript
interface FbMessageItem {
  id: string;                    // MongoDB ObjectID
  conversationId: string;         // Facebook Conversation ID (không unique, nhiều messages cùng conversationId)
  messageId: string;              // Message ID từ Pancake (unique, tự động extract từ messageData.id)
  messageData: Record<string, any>; // Toàn bộ dữ liệu của message
  insertedAt: number;            // Thời gian insert message (Unix timestamp, extract từ messageData.inserted_at)
  createdAt: number;             // Thời gian tạo document
  updatedAt: number;             // Thời gian cập nhật document
}
```

**Lưu ý:**
- Mỗi message là 1 document riêng để tránh document quá lớn (giới hạn MongoDB 16MB)
- `messageId` là unique để tự động tránh duplicate khi upsert
- `insertedAt` được extract từ `messageData.inserted_at` (format: `2006-01-02T15:04:05.000000`)
- Index: `conversationId` + `insertedAt` (compound) để query nhanh, `messageId` (unique)

**Endpoints:**
- `/api/v1/facebook/message-item/*` - Full CRUD operations (Permission: `FbMessageItem.*`)
  - `POST /api/v1/facebook/message-item/insert-one` - Tạo message item mới (Permission: `FbMessageItem.Insert`)
  - `POST /api/v1/facebook/message-item/insert-many` - Tạo nhiều message items (Permission: `FbMessageItem.Insert`)
  - `GET /api/v1/facebook/message-item/find` - Tìm message items (Permission: `FbMessageItem.Read`)
  - `GET /api/v1/facebook/message-item/find-one` - Tìm một message item (Permission: `FbMessageItem.Read`)
  - `GET /api/v1/facebook/message-item/find-by-id/:id` - Tìm message item theo ID (Permission: `FbMessageItem.Read`)
  - `POST /api/v1/facebook/message-item/find-by-ids` - Tìm nhiều message items theo IDs (Permission: `FbMessageItem.Read`)
  - `GET /api/v1/facebook/message-item/find-with-pagination` - Tìm với phân trang (Permission: `FbMessageItem.Read`)
  - `PUT /api/v1/facebook/message-item/update-one` - Cập nhật một message item (Permission: `FbMessageItem.Update`)
  - `PUT /api/v1/facebook/message-item/update-many` - Cập nhật nhiều message items (Permission: `FbMessageItem.Update`)
  - `PUT /api/v1/facebook/message-item/update-by-id/:id` - Cập nhật message item theo ID (Permission: `FbMessageItem.Update`)
  - `PUT /api/v1/facebook/message-item/find-one-and-update` - Tìm và cập nhật message item (Permission: `FbMessageItem.Update`)
  - `DELETE /api/v1/facebook/message-item/delete-one` - Xóa một message item (Permission: `FbMessageItem.Delete`)
  - `DELETE /api/v1/facebook/message-item/delete-many` - Xóa nhiều message items (Permission: `FbMessageItem.Delete`)
  - `DELETE /api/v1/facebook/message-item/delete-by-id/:id` - Xóa message item theo ID (Permission: `FbMessageItem.Delete`)
  - `DELETE /api/v1/facebook/message-item/find-one-and-delete` - Tìm và xóa message item (Permission: `FbMessageItem.Delete`)
  - `GET /api/v1/facebook/message-item/count` - Đếm số lượng message items (Permission: `FbMessageItem.Read`)
  - `GET /api/v1/facebook/message-item/distinct` - Lấy danh sách giá trị duy nhất (Permission: `FbMessageItem.Read`)
  - `GET /api/v1/facebook/message-item/exists` - Kiểm tra message item có tồn tại không (Permission: `FbMessageItem.Read`)
- **Endpoints đặc biệt:**
  - `GET /api/v1/facebook/message-item/find-by-conversation/:conversationId` - Lấy message items theo conversationId với phân trang (Permission: `FbMessageItem.Read`)
    - Query params: `page` (default: 1), `limit` (default: 50, max: 100)
    - Response: `{ data: FbMessageItem[], pagination: { page, limit, total } }`
  - `GET /api/v1/facebook/message-item/find-by-message-id/:messageId` - Tìm message item theo messageId (Permission: `FbMessageItem.Read`)

**Lưu ý:**
- Collection này cũng được quản lý tự động bởi endpoint `/api/v1/facebook/message/upsert-messages` (tự động tách messages từ panCakeData)
- CRUD endpoints cho phép quản lý message items thủ công nếu cần

---

### 5. PcPosShop Collection (Quản Lý Cửa Hàng từ Pancake POS)

**Ý nghĩa**: Quản lý thông tin cửa hàng từ Pancake POS API
**Tính năng**:
- Lưu thông tin cửa hàng từ Pancake POS API
- Đồng bộ dữ liệu đầy đủ từ Pancake POS API (panCakeData)
- Tự động extract các trường quan trọng từ panCakeData
- Liên kết với các module khác (Warehouse, Orders, Products, etc.)

**Cần thiết**: ⭐⭐⭐⭐⭐ (Cần thiết cho tích hợp Pancake POS - Entity cơ bản)

**Model:**
```typescript
interface PcPosShop {
  id: string;                    // MongoDB ObjectID
  shopId: number;                // ID của shop trên Pancake POS (extract từ panCakeData.id, unique)
  name: string;                  // Tên cửa hàng (extract từ panCakeData.name)
  avatarUrl: string;             // Link hình đại diện (extract từ panCakeData.avatar_url)
  pages: any[];                  // Thông tin các pages được gộp trong shop (extract từ panCakeData.pages)
  panCakeData: Record<string, any>; // Dữ liệu gốc từ Pancake POS API
  createdAt: number;             // Thời gian tạo
  updatedAt: number;             // Thời gian cập nhật
}
```

**Indexes:**
- Unique: `shopId` - Đảm bảo không duplicate shop
- Text indexes: `shopId`, `name` - Hỗ trợ tìm kiếm

**Data Extraction (Tự động ở Backend):**
- **Lưu ý quan trọng**: Client chỉ cần gửi `panCakeData` trong DTO, backend tự động extract các field sau:
  - `shopId` ← `panCakeData.id` (required, convert to int64)
  - `name` ← `panCakeData.name` (optional)
  - `avatarUrl` ← `panCakeData.avatar_url` (optional)
  - `pages` ← `panCakeData.pages` (optional)
- **Client không cần extract hoặc gửi các field này**, chỉ cần gửi `panCakeData` đầy đủ từ Pancake POS API

**Endpoints:**
- `/api/v1/pancake-pos/shop/*` - Full CRUD operations (Permission: `PcPosShop.*`)
  - `POST /api/v1/pancake-pos/shop/insert-one` - Tạo shop mới (Permission: `PcPosShop.Insert`)
  - `POST /api/v1/pancake-pos/shop/upsert-one?filter={...}` - Upsert shop (dùng cho sync từ Pancake POS) (Permission: `PcPosShop.Update`)
  - `GET /api/v1/pancake-pos/shop/find` - Tìm shops (Permission: `PcPosShop.Read`)
  - `GET /api/v1/pancake-pos/shop/find-one` - Tìm một shop (Permission: `PcPosShop.Read`)
  - `GET /api/v1/pancake-pos/shop/find-by-id/:id` - Tìm shop theo ID (Permission: `PcPosShop.Read`)
  - `POST /api/v1/pancake-pos/shop/find-by-ids` - Tìm nhiều shops theo IDs (Permission: `PcPosShop.Read`)
  - `GET /api/v1/pancake-pos/shop/find-with-pagination` - Tìm với phân trang (Permission: `PcPosShop.Read`)
  - `PUT /api/v1/pancake-pos/shop/update-one` - Cập nhật một shop (Permission: `PcPosShop.Update`)
  - `PUT /api/v1/pancake-pos/shop/update-many` - Cập nhật nhiều shops (Permission: `PcPosShop.Update`)
  - `PUT /api/v1/pancake-pos/shop/update-by-id/:id` - Cập nhật shop theo ID (Permission: `PcPosShop.Update`)
  - `DELETE /api/v1/pancake-pos/shop/delete-one` - Xóa một shop (Permission: `PcPosShop.Delete`)
  - `DELETE /api/v1/pancake-pos/shop/delete-many` - Xóa nhiều shops (Permission: `PcPosShop.Delete`)
  - `DELETE /api/v1/pancake-pos/shop/delete-by-id/:id` - Xóa shop theo ID (Permission: `PcPosShop.Delete`)
  - `GET /api/v1/pancake-pos/shop/count` - Đếm số lượng shops (Permission: `PcPosShop.Read`)
  - `GET /api/v1/pancake-pos/shop/distinct` - Lấy danh sách giá trị duy nhất (Permission: `PcPosShop.Read`)
  - `GET /api/v1/pancake-pos/shop/exists` - Kiểm tra shop có tồn tại không (Permission: `PcPosShop.Read`)

**Ví dụ sử dụng:**

**Upsert Shop từ Pancake POS:**
```bash
POST /api/v1/pancake-pos/shop/upsert-one?filter={"shopId":123}
Authorization: Bearer <token>
Content-Type: application/json

{
  "panCakeData": {
    "id": 123,
    "name": "Cửa hàng ABC",
    "avatar_url": "https://example.com/avatar.jpg",
    "pages": [
      {
        "id": "page_123",
        "name": "Page Name"
      }
    ]
  }
}
```

---

### 6. PcPosWarehouse Collection (Quản Lý Kho Hàng từ Pancake POS)

**Ý nghĩa**: Quản lý thông tin kho hàng từ Pancake POS API
**Tính năng**:
- Lưu thông tin kho hàng từ Pancake POS API
- Đồng bộ dữ liệu đầy đủ từ Pancake POS API (panCakeData)
- Tự động extract các trường quan trọng từ panCakeData
- Liên kết với Shop và các module khác (Orders, Products, etc.)

**Cần thiết**: ⭐⭐⭐⭐ (Cần thiết nếu quản lý tồn kho)

**Model:**
```typescript
interface PcPosWarehouse {
  id: string;                    // MongoDB ObjectID
  warehouseId: string;           // ID của warehouse trên Pancake POS (extract từ panCakeData.id, UUID string)
  shopId: number;                // ID của shop (extract từ panCakeData.shop_id)
  name: string;                  // Tên kho hàng (extract từ panCakeData.name)
  phoneNumber: string;           // Số điện thoại kho hàng (extract từ panCakeData.phone_number)
  fullAddress: string;           // Địa chỉ đầy đủ (extract từ panCakeData.full_address)
  provinceId: string;            // ID tỉnh/thành phố (extract từ panCakeData.province_id)
  districtId: string;            // ID quận/huyện (extract từ panCakeData.district_id)
  communeId: string;             // ID phường/xã (extract từ panCakeData.commune_id)
  panCakeData: Record<string, any>; // Dữ liệu gốc từ Pancake POS API
  createdAt: number;             // Thời gian tạo
  updatedAt: number;             // Thời gian cập nhật
}
```

**Indexes:**
- Text indexes: `warehouseId`, `shopId`, `name` - Hỗ trợ tìm kiếm

**Data Extraction (Tự động ở Backend):**
- **Lưu ý quan trọng**: Client chỉ cần gửi `panCakeData` trong DTO, backend tự động extract các field sau:
  - `warehouseId` ← `panCakeData.id` (required, convert to string - UUID)
  - `shopId` ← `panCakeData.shop_id` (optional, convert to int64)
  - `name` ← `panCakeData.name` (optional)
  - `phoneNumber` ← `panCakeData.phone_number` (optional)
  - `fullAddress` ← `panCakeData.full_address` (optional)
  - `provinceId` ← `panCakeData.province_id` (optional)
  - `districtId` ← `panCakeData.district_id` (optional)
  - `communeId` ← `panCakeData.commune_id` (optional)
- **Client không cần extract hoặc gửi các field này**, chỉ cần gửi `panCakeData` đầy đủ từ Pancake POS API

**Endpoints:**
- `/api/v1/pancake-pos/warehouse/*` - Full CRUD operations (Permission: `PcPosWarehouse.*`)
  - `POST /api/v1/pancake-pos/warehouse/insert-one` - Tạo warehouse mới (Permission: `PcPosWarehouse.Insert`)
  - `POST /api/v1/pancake-pos/warehouse/upsert-one?filter={...}` - Upsert warehouse (dùng cho sync từ Pancake POS) (Permission: `PcPosWarehouse.Update`)
  - `GET /api/v1/pancake-pos/warehouse/find` - Tìm warehouses (Permission: `PcPosWarehouse.Read`)
  - `GET /api/v1/pancake-pos/warehouse/find-one` - Tìm một warehouse (Permission: `PcPosWarehouse.Read`)
  - `GET /api/v1/pancake-pos/warehouse/find-by-id/:id` - Tìm warehouse theo ID (Permission: `PcPosWarehouse.Read`)
  - `POST /api/v1/pancake-pos/warehouse/find-by-ids` - Tìm nhiều warehouses theo IDs (Permission: `PcPosWarehouse.Read`)
  - `GET /api/v1/pancake-pos/warehouse/find-with-pagination` - Tìm với phân trang (Permission: `PcPosWarehouse.Read`)
  - `PUT /api/v1/pancake-pos/warehouse/update-one` - Cập nhật một warehouse (Permission: `PcPosWarehouse.Update`)
  - `PUT /api/v1/pancake-pos/warehouse/update-many` - Cập nhật nhiều warehouses (Permission: `PcPosWarehouse.Update`)
  - `PUT /api/v1/pancake-pos/warehouse/update-by-id/:id` - Cập nhật warehouse theo ID (Permission: `PcPosWarehouse.Update`)
  - `DELETE /api/v1/pancake-pos/warehouse/delete-one` - Xóa một warehouse (Permission: `PcPosWarehouse.Delete`)
  - `DELETE /api/v1/pancake-pos/warehouse/delete-many` - Xóa nhiều warehouses (Permission: `PcPosWarehouse.Delete`)
  - `DELETE /api/v1/pancake-pos/warehouse/delete-by-id/:id` - Xóa warehouse theo ID (Permission: `PcPosWarehouse.Delete`)
  - `GET /api/v1/pancake-pos/warehouse/count` - Đếm số lượng warehouses (Permission: `PcPosWarehouse.Read`)
  - `GET /api/v1/pancake-pos/warehouse/distinct` - Lấy danh sách giá trị duy nhất (Permission: `PcPosWarehouse.Read`)
  - `GET /api/v1/pancake-pos/warehouse/exists` - Kiểm tra warehouse có tồn tại không (Permission: `PcPosWarehouse.Read`)

**Ví dụ sử dụng:**

**Upsert Warehouse từ Pancake POS:**
```bash
POST /api/v1/pancake-pos/warehouse/upsert-one?filter={"warehouseId":"uuid-here"}
Authorization: Bearer <token>
Content-Type: application/json

{
  "panCakeData": {
    "id": "uuid-here",
    "shop_id": 123,
    "name": "Kho hàng chính",
    "phone_number": "0912345678",
    "full_address": "123 Đường ABC, Quận XYZ",
    "province_id": "717",
    "district_id": "71705",
    "commune_id": "7170510"
  }
}
```

---

### 7. PcPosProduct Collection (Quản Lý Sản Phẩm từ Pancake POS)

**Ý nghĩa**: Quản lý thông tin sản phẩm từ Pancake POS API
**Tính năng**:
- Lưu thông tin sản phẩm từ Pancake POS API
- Đồng bộ dữ liệu đầy đủ từ Pancake POS (panCakeData)
- Tự động extract các trường quan trọng từ panCakeData
- Text indexes trên `productId`, `shopId`, `name` để hỗ trợ tìm kiếm

**Model Structure:**
```typescript
interface PcPosProduct {
  id: string;                    // MongoDB ObjectID
  productId: number;             // ID của product trên Pancake POS (extract từ panCakeData.id)
  shopId: number;                 // ID của shop (extract từ panCakeData.shop_id)
  name: string;                   // Tên sản phẩm (extract từ panCakeData.name)
  categoryIds: number[];          // Danh sách ID danh mục (extract từ panCakeData.category_ids)
  tagIds: number[];               // Danh sách ID tags (extract từ panCakeData.tags)
  isHide: boolean;                // Trạng thái ẩn/hiện (extract từ panCakeData.is_hide)
  noteProduct: string;            // Ghi chú sản phẩm (extract từ panCakeData.note_product)
  productAttributes: any[];       // Thuộc tính sản phẩm (extract từ panCakeData.product_attributes)
  panCakeData: object;            // Dữ liệu gốc từ Pancake POS API
  createdAt: number;              // Thời gian tạo (timestamp)
  updatedAt: number;              // Thời gian cập nhật (timestamp)
}
```

**Data Extraction:**
- Backend tự động extract các field từ `panCakeData`:
  - `productId` ← `panCakeData.id` (required, convert to int64)
  - `shopId` ← `panCakeData.shop_id` (optional, convert to int64)
  - `name` ← `panCakeData.name` (optional)
  - `categoryIds` ← `panCakeData.category_ids` (optional, array)
  - `tagIds` ← `panCakeData.tags` (optional, array)
  - `isHide` ← `panCakeData.is_hide` (optional, convert to bool)
  - `noteProduct` ← `panCakeData.note_product` (optional)
  - `productAttributes` ← `panCakeData.product_attributes` (optional, array)
- **Client không cần extract hoặc gửi các field này**, chỉ cần gửi `panCakeData` đầy đủ từ Pancake POS API

**Endpoints:**
- `/api/v1/pancake-pos/product/*` - Full CRUD operations (Permission: `PcPosProduct.*`)
  - `POST /api/v1/pancake-pos/product/insert-one` - Tạo product mới (Permission: `PcPosProduct.Insert`)
  - `POST /api/v1/pancake-pos/product/upsert-one?filter={...}` - Upsert product (dùng cho sync từ Pancake POS) (Permission: `PcPosProduct.Update`)
  - `GET /api/v1/pancake-pos/product/find` - Tìm products (Permission: `PcPosProduct.Read`)
  - `GET /api/v1/pancake-pos/product/find-one` - Tìm một product (Permission: `PcPosProduct.Read`)
  - `GET /api/v1/pancake-pos/product/find-by-id/:id` - Tìm product theo ID (Permission: `PcPosProduct.Read`)
  - `POST /api/v1/pancake-pos/product/find-by-ids` - Tìm nhiều products theo IDs (Permission: `PcPosProduct.Read`)
  - `GET /api/v1/pancake-pos/product/find-with-pagination` - Tìm với phân trang (Permission: `PcPosProduct.Read`)
  - `PUT /api/v1/pancake-pos/product/update-one` - Cập nhật một product (Permission: `PcPosProduct.Update`)
  - `PUT /api/v1/pancake-pos/product/update-many` - Cập nhật nhiều products (Permission: `PcPosProduct.Update`)
  - `PUT /api/v1/pancake-pos/product/update-by-id/:id` - Cập nhật product theo ID (Permission: `PcPosProduct.Update`)
  - `DELETE /api/v1/pancake-pos/product/delete-one` - Xóa một product (Permission: `PcPosProduct.Delete`)
  - `DELETE /api/v1/pancake-pos/product/delete-many` - Xóa nhiều products (Permission: `PcPosProduct.Delete`)
  - `DELETE /api/v1/pancake-pos/product/delete-by-id/:id` - Xóa product theo ID (Permission: `PcPosProduct.Delete`)
  - `GET /api/v1/pancake-pos/product/count` - Đếm số lượng products (Permission: `PcPosProduct.Read`)
  - `GET /api/v1/pancake-pos/product/distinct` - Lấy danh sách giá trị duy nhất (Permission: `PcPosProduct.Read`)
  - `GET /api/v1/pancake-pos/product/exists` - Kiểm tra product có tồn tại không (Permission: `PcPosProduct.Read`)

**Ví dụ sử dụng:**

**Upsert Product từ Pancake POS:**
```bash
POST /api/v1/pancake-pos/product/upsert-one?filter={"productId":123,"shopId":456}
Authorization: Bearer <token>
Content-Type: application/json

{
  "posData": {
    "id": 123,
    "shop_id": 456,
    "name": "Áo thun nam",
    "category_ids": [1, 2],
    "tags": [10, 20],
    "is_hide": false,
    "note_product": "Sản phẩm bán chạy",
    "product_attributes": [
      {
        "name": "Màu",
        "values": ["Đen", "Trắng", "Đỏ"]
      },
      {
        "name": "Size",
        "values": ["S", "M", "L"]
      }
    ]
  }
}
```

---

### 8. PcPosVariation Collection (Quản Lý Biến Thể Sản Phẩm từ Pancake POS)

**Ý nghĩa**: Quản lý thông tin biến thể sản phẩm từ Pancake POS API
**Tính năng**:
- Lưu thông tin biến thể sản phẩm từ Pancake POS API
- Đồng bộ dữ liệu đầy đủ từ Pancake POS (panCakeData)
- Tự động extract các trường quan trọng từ panCakeData
- Unique index: `{variationId: 1}` để đảm bảo không duplicate variation
- Text indexes trên `variationId`, `productId`, `shopId`, `sku` để hỗ trợ tìm kiếm

**Model Structure:**
```typescript
interface PcPosVariation {
  id: string;                     // MongoDB ObjectID
  variationId: string;             // ID của variation trên Pancake POS (extract từ posData.id, UUID string)
  productId: number;               // ID của product (extract từ posData.product_id)
  shopId: number;                  // ID của shop (extract từ posData.shop_id)
  sku: string;                     // Mã SKU (extract từ posData.sku)
  retailPrice: number;             // Giá bán lẻ (extract từ posData.retail_price)
  priceAtCounter: number;           // Giá tại quầy (extract từ posData.price_at_counter)
  quantity: number;                // Số lượng tồn kho (extract từ posData.quantity)
  weight: number;                  // Trọng lượng (extract từ posData.weight)
  fields: any[];                   // Các trường thuộc tính (extract từ posData.fields)
  images: string[];                // Danh sách hình ảnh (extract từ posData.images)
  posData: object;                 // Dữ liệu gốc từ Pancake POS API
  createdAt: number;               // Thời gian tạo (timestamp)
  updatedAt: number;               // Thời gian cập nhật (timestamp)
}
```

**Data Extraction:**
- Backend tự động extract các field từ `posData`:
  - `variationId` ← `posData.id` (required, convert to string - UUID)
  - `productId` ← `posData.product_id` (optional, convert to int64)
  - `shopId` ← `posData.shop_id` (optional, convert to int64)
  - `sku` ← `posData.sku` (optional)
  - `retailPrice` ← `posData.retail_price` (optional, convert to number)
  - `priceAtCounter` ← `posData.price_at_counter` (optional, convert to number)
  - `quantity` ← `posData.quantity` (optional, convert to int64)
  - `weight` ← `posData.weight` (optional, convert to number)
  - `fields` ← `posData.fields` (optional, array)
  - `images` ← `posData.images` (optional, array of strings)
- **Client không cần extract hoặc gửi các field này**, chỉ cần gửi `posData` đầy đủ từ Pancake POS API

**Endpoints:**
- `/api/v1/pancake-pos/variation/*` - Full CRUD operations (Permission: `PcPosVariation.*`)
  - `POST /api/v1/pancake-pos/variation/insert-one` - Tạo variation mới (Permission: `PcPosVariation.Insert`)
  - `POST /api/v1/pancake-pos/variation/upsert-one?filter={...}` - Upsert variation (dùng cho sync từ Pancake POS) (Permission: `PcPosVariation.Update`)
  - `GET /api/v1/pancake-pos/variation/find` - Tìm variations (Permission: `PcPosVariation.Read`)
  - `GET /api/v1/pancake-pos/variation/find-one` - Tìm một variation (Permission: `PcPosVariation.Read`)
  - `GET /api/v1/pancake-pos/variation/find-by-id/:id` - Tìm variation theo ID (Permission: `PcPosVariation.Read`)
  - `POST /api/v1/pancake-pos/variation/find-by-ids` - Tìm nhiều variations theo IDs (Permission: `PcPosVariation.Read`)
  - `GET /api/v1/pancake-pos/variation/find-with-pagination` - Tìm với phân trang (Permission: `PcPosVariation.Read`)
  - `PUT /api/v1/pancake-pos/variation/update-one` - Cập nhật một variation (Permission: `PcPosVariation.Update`)
  - `PUT /api/v1/pancake-pos/variation/update-many` - Cập nhật nhiều variations (Permission: `PcPosVariation.Update`)
  - `PUT /api/v1/pancake-pos/variation/update-by-id/:id` - Cập nhật variation theo ID (Permission: `PcPosVariation.Update`)
  - `DELETE /api/v1/pancake-pos/variation/delete-one` - Xóa một variation (Permission: `PcPosVariation.Delete`)
  - `DELETE /api/v1/pancake-pos/variation/delete-many` - Xóa nhiều variations (Permission: `PcPosVariation.Delete`)
  - `DELETE /api/v1/pancake-pos/variation/delete-by-id/:id` - Xóa variation theo ID (Permission: `PcPosVariation.Delete`)
  - `GET /api/v1/pancake-pos/variation/count` - Đếm số lượng variations (Permission: `PcPosVariation.Read`)
  - `GET /api/v1/pancake-pos/variation/distinct` - Lấy danh sách giá trị duy nhất (Permission: `PcPosVariation.Read`)
  - `GET /api/v1/pancake-pos/variation/exists` - Kiểm tra variation có tồn tại không (Permission: `PcPosVariation.Read`)

**Ví dụ sử dụng:**

**Upsert Variation từ Pancake POS:**
```bash
POST /api/v1/pancake-pos/variation/upsert-one?filter={"variationId":"uuid-here"}
Authorization: Bearer <token>
Content-Type: application/json

{
  "posData": {
    "id": "uuid-here",
    "product_id": 123,
    "shop_id": 456,
    "sku": "SKU-001",
    "retail_price": 100000,
    "price_at_counter": 90000,
    "quantity": 100,
    "weight": 0.5,
    "fields": [
      {"name": "Màu", "value": "Đen"},
      {"name": "Size", "value": "M"}
    ],
    "images": ["https://example.com/image1.jpg", "https://example.com/image2.jpg"]
  }
}
```

---

### 9. PcPosCategory Collection (Quản Lý Danh Mục Sản Phẩm từ Pancake POS)

**Ý nghĩa**: Quản lý thông tin danh mục sản phẩm từ Pancake POS API
**Tính năng**:
- Lưu thông tin danh mục sản phẩm từ Pancake POS API
- Đồng bộ dữ liệu đầy đủ từ Pancake POS (panCakeData)
- Tự động extract các trường quan trọng từ panCakeData
- Text indexes trên `categoryId`, `shopId`, `name` để hỗ trợ tìm kiếm

**Model Structure:**
```typescript
interface PcPosCategory {
  id: string;                      // MongoDB ObjectID
  categoryId: number;              // ID của category trên Pancake POS (extract từ posData.id)
  shopId: number;                  // ID của shop (extract từ posData.shop_id)
  name: string;                    // Tên danh mục (extract từ posData.name)
  posData: object;                  // Dữ liệu gốc từ Pancake POS API
  createdAt: number;               // Thời gian tạo (timestamp)
  updatedAt: number;               // Thời gian cập nhật (timestamp)
}
```

**Data Extraction:**
- Backend tự động extract các field từ `posData`:
  - `categoryId` ← `posData.id` (required, convert to int64)
  - `shopId` ← `posData.shop_id` (optional, convert to int64)
  - `name` ← `posData.name` (optional)
- **Client không cần extract hoặc gửi các field này**, chỉ cần gửi `posData` đầy đủ từ Pancake POS API

**Endpoints:**
- `/api/v1/pancake-pos/category/*` - Full CRUD operations (Permission: `PcPosCategory.*`)
  - `POST /api/v1/pancake-pos/category/insert-one` - Tạo category mới (Permission: `PcPosCategory.Insert`)
  - `POST /api/v1/pancake-pos/category/upsert-one?filter={...}` - Upsert category (dùng cho sync từ Pancake POS) (Permission: `PcPosCategory.Update`)
  - `GET /api/v1/pancake-pos/category/find` - Tìm categories (Permission: `PcPosCategory.Read`)
  - `GET /api/v1/pancake-pos/category/find-one` - Tìm một category (Permission: `PcPosCategory.Read`)
  - `GET /api/v1/pancake-pos/category/find-by-id/:id` - Tìm category theo ID (Permission: `PcPosCategory.Read`)
  - `POST /api/v1/pancake-pos/category/find-by-ids` - Tìm nhiều categories theo IDs (Permission: `PcPosCategory.Read`)
  - `GET /api/v1/pancake-pos/category/find-with-pagination` - Tìm với phân trang (Permission: `PcPosCategory.Read`)
  - `PUT /api/v1/pancake-pos/category/update-one` - Cập nhật một category (Permission: `PcPosCategory.Update`)
  - `PUT /api/v1/pancake-pos/category/update-many` - Cập nhật nhiều categories (Permission: `PcPosCategory.Update`)
  - `PUT /api/v1/pancake-pos/category/update-by-id/:id` - Cập nhật category theo ID (Permission: `PcPosCategory.Update`)
  - `DELETE /api/v1/pancake-pos/category/delete-one` - Xóa một category (Permission: `PcPosCategory.Delete`)
  - `DELETE /api/v1/pancake-pos/category/delete-many` - Xóa nhiều categories (Permission: `PcPosCategory.Delete`)
  - `DELETE /api/v1/pancake-pos/category/delete-by-id/:id` - Xóa category theo ID (Permission: `PcPosCategory.Delete`)
  - `GET /api/v1/pancake-pos/category/count` - Đếm số lượng categories (Permission: `PcPosCategory.Read`)
  - `GET /api/v1/pancake-pos/category/distinct` - Lấy danh sách giá trị duy nhất (Permission: `PcPosCategory.Read`)
  - `GET /api/v1/pancake-pos/category/exists` - Kiểm tra category có tồn tại không (Permission: `PcPosCategory.Read`)

**Ví dụ sử dụng:**

**Upsert Category từ Pancake POS:**
```bash
POST /api/v1/pancake-pos/category/upsert-one?filter={"categoryId":123,"shopId":456}
Authorization: Bearer <token>
Content-Type: application/json

{
  "posData": {
    "id": 123,
    "shop_id": 456,
    "name": "Áo thun"
  }
}
```

---

### 10. PcPosOrder Collection (Quản Lý Đơn Hàng từ Pancake POS)

**Ý nghĩa**: Quản lý thông tin đơn hàng từ Pancake POS API
**Tính năng**:
- Lưu thông tin đơn hàng từ Pancake POS API
- Đồng bộ dữ liệu đầy đủ từ Pancake POS (posData)
- Tự động extract các trường quan trọng từ posData
- Text indexes trên `orderId`, `shopId`, `billFullName`, `billPhoneNumber`, `billEmail`, `customerId`, `warehouseId`, `pageId`, `postId` để hỗ trợ tìm kiếm
- Quản lý đơn hàng với đầy đủ thông tin: billing, shipping, order items, warehouse, customer

**Cần thiết**: ⭐⭐⭐⭐⭐ (Core module - Cần thiết cho quản lý bán hàng và báo cáo)

**Model Structure:**
```typescript
interface PcPosOrder {
  id: string;                      // MongoDB ObjectID
  orderId: number;                 // ID của order trên Pancake POS (extract từ posData.id, required)
  systemId: number;                // System ID (extract từ posData.system_id)
  shopId: number;                  // ID của shop (extract từ posData.shop_id)
  status: number;                  // Trạng thái đơn hàng (extract từ posData.status)
  statusName: string;              // Tên trạng thái (extract từ posData.status_name)
  billFullName: string;            // Tên người thanh toán (extract từ posData.bill_full_name)
  billPhoneNumber: string;         // Số điện thoại người thanh toán (extract từ posData.bill_phone_number)
  billEmail: string;               // Email người thanh toán (extract từ posData.bill_email)
  customerId: string;              // ID khách hàng (extract từ posData.customer.id, UUID string)
  warehouseId: string;             // ID kho hàng (extract từ posData.warehouse_id, UUID string)
  shippingFee: number;             // Phí vận chuyển (extract từ posData.shipping_fee)
  totalDiscount: number;           // Tổng giảm giá (extract từ posData.total_discount)
  note: string;                    // Ghi chú đơn hàng (extract từ posData.note)
  pageId: string;                   // Facebook Page ID (extract từ posData.page_id)
  postId: string;                   // Facebook Post ID (extract từ posData.post_id)
  insertedAt: number;              // Thời gian tạo đơn hàng (extract từ posData.inserted_at, timestamp)
  posUpdatedAt: number;            // Thời gian cập nhật từ POS (extract từ posData.updated_at, timestamp)
  paidAt: number;                  // Thời gian thanh toán (extract từ posData.paid_at, timestamp)
  orderItems: any[];               // Danh sách sản phẩm trong đơn hàng (extract từ posData.order_items)
  shippingAddress: object;         // Địa chỉ giao hàng (extract từ posData.shipping_address)
  warehouseInfo: object;           // Thông tin kho hàng (extract từ posData.warehouse_info)
  customerInfo: object;            // Thông tin khách hàng (extract từ posData.customer)
  posData: object;                 // Dữ liệu gốc từ Pancake POS API
  createdAt: number;               // Thời gian tạo (timestamp)
  updatedAt: number;               // Thời gian cập nhật (timestamp)
}
```

**Data Extraction:**
- Backend tự động extract các field từ `posData`:
  - `orderId` ← `posData.id` (required, convert to int64)
  - `systemId` ← `posData.system_id` (optional, convert to int64)
  - `shopId` ← `posData.shop_id` (optional, convert to int64)
  - `status` ← `posData.status` (optional, convert to int)
  - `statusName` ← `posData.status_name` (optional)
  - `billFullName` ← `posData.bill_full_name` (optional)
  - `billPhoneNumber` ← `posData.bill_phone_number` (optional)
  - `billEmail` ← `posData.bill_email` (optional)
  - `customerId` ← `posData.customer.id` (optional, convert to string - UUID)
  - `warehouseId` ← `posData.warehouse_id` (optional, convert to string - UUID)
  - `shippingFee` ← `posData.shipping_fee` (optional, convert to number)
  - `totalDiscount` ← `posData.total_discount` (optional, convert to number)
  - `note` ← `posData.note` (optional)
  - `pageId` ← `posData.page_id` (optional)
  - `postId` ← `posData.post_id` (optional)
  - `insertedAt` ← `posData.inserted_at` (optional, convert to timestamp, format: "2006-01-02T15:04:05Z")
  - `posUpdatedAt` ← `posData.updated_at` (optional, convert to timestamp, format: "2006-01-02T15:04:05Z")
  - `paidAt` ← `posData.paid_at` (optional, convert to timestamp, format: "2006-01-02T15:04:05Z")
  - `orderItems` ← `posData.order_items` (optional, array)
  - `shippingAddress` ← `posData.shipping_address` (optional, object)
  - `warehouseInfo` ← `posData.warehouse_info` (optional, object)
  - `customerInfo` ← `posData.customer` (optional, object)
- **Client không cần extract hoặc gửi các field này**, chỉ cần gửi `posData` đầy đủ từ Pancake POS API

**Endpoints:**
- `/api/v1/pancake-pos/order/*` - Full CRUD operations (Permission: `PcPosOrder.*`)
  - `POST /api/v1/pancake-pos/order/insert-one` - Tạo order mới (Permission: `PcPosOrder.Insert`)
  - `POST /api/v1/pancake-pos/order/upsert-one?filter={...}` - Upsert order (dùng cho sync từ Pancake POS) (Permission: `PcPosOrder.Update`)
  - `GET /api/v1/pancake-pos/order/find` - Tìm orders (Permission: `PcPosOrder.Read`)
  - `GET /api/v1/pancake-pos/order/find-one` - Tìm một order (Permission: `PcPosOrder.Read`)
  - `GET /api/v1/pancake-pos/order/find-by-id/:id` - Tìm order theo ID (Permission: `PcPosOrder.Read`)
  - `POST /api/v1/pancake-pos/order/find-by-ids` - Tìm nhiều orders theo IDs (Permission: `PcPosOrder.Read`)
  - `GET /api/v1/pancake-pos/order/find-with-pagination` - Tìm với phân trang (Permission: `PcPosOrder.Read`)
  - `PUT /api/v1/pancake-pos/order/update-one` - Cập nhật một order (Permission: `PcPosOrder.Update`)
  - `PUT /api/v1/pancake-pos/order/update-many` - Cập nhật nhiều orders (Permission: `PcPosOrder.Update`)
  - `PUT /api/v1/pancake-pos/order/update-by-id/:id` - Cập nhật order theo ID (Permission: `PcPosOrder.Update`)
  - `DELETE /api/v1/pancake-pos/order/delete-one` - Xóa một order (Permission: `PcPosOrder.Delete`)
  - `DELETE /api/v1/pancake-pos/order/delete-many` - Xóa nhiều orders (Permission: `PcPosOrder.Delete`)
  - `DELETE /api/v1/pancake-pos/order/delete-by-id/:id` - Xóa order theo ID (Permission: `PcPosOrder.Delete`)
  - `GET /api/v1/pancake-pos/order/count` - Đếm số lượng orders (Permission: `PcPosOrder.Read`)
  - `GET /api/v1/pancake-pos/order/distinct` - Lấy danh sách giá trị duy nhất (Permission: `PcPosOrder.Read`)
  - `GET /api/v1/pancake-pos/order/exists` - Kiểm tra order có tồn tại không (Permission: `PcPosOrder.Read`)

**Ví dụ sử dụng:**

**Upsert Order từ Pancake POS:**
```bash
POST /api/v1/pancake-pos/order/upsert-one?filter={"orderId":123,"shopId":456}
Authorization: Bearer <token>
Content-Type: application/json

{
  "posData": {
    "id": 123,
    "system_id": 1,
    "shop_id": 456,
    "inserted_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T12:00:00Z",
    "status": 1,
    "status_name": "Đã xác nhận",
    "bill_full_name": "Nguyễn Văn A",
    "bill_phone_number": "0999999999",
    "bill_email": "email@example.com",
    "page_id": "104438181227821",
    "post_id": "185187094667903_477083092110915",
    "shipping_fee": 10000,
    "total_discount": 50000,
    "note": "Ghi chú đơn hàng",
    "warehouse_id": "uuid-warehouse",
    "warehouse_info": {
      "name": "Tên kho",
      "phone_number": "0999999999",
      "full_address": "Địa chỉ đầy đủ",
      "province_id": "717",
      "district_id": "71705",
      "commune_id": "7170510"
    },
    "customer": {
      "id": "uuid-customer",
      "name": "Tên khách hàng",
      "phone_number": "0999999999",
      "email": "email@example.com"
    },
    "order_items": [
      {
        "id": 1,
        "product_id": 1,
        "product_name": "Tên sản phẩm",
        "variation_id": "uuid-variation",
        "quantity": 2,
        "price": 100000,
        "total": 200000
      }
    ],
    "shipping_address": {
      "full_name": "Tên người nhận",
      "phone_number": "0999999999",
      "full_address": "Địa chỉ đầy đủ",
      "province_id": "717",
      "district_id": "71705",
      "commune_id": "7170510"
    }
  }
}
```

**Tìm orders theo shop và status:**
```bash
GET /api/v1/pancake-pos/order/find?filter={"shopId":456,"status":1}
Authorization: Bearer <token>
```

**Tìm orders theo customer:**
```bash
GET /api/v1/pancake-pos/order/find?filter={"customerId":"uuid-customer"}
Authorization: Bearer <token>
```

---

### 11. FB Customer Collection (Quản Lý Khách Hàng Facebook)

**Ý nghĩa**: Quản lý thông tin khách hàng từ Pancake API (Facebook)
**Tính năng**:
- Lưu thông tin khách hàng từ Pancake API (Facebook conversations, messages)
- Tự động extract các trường từ `panCakeData`
- Link với `fb_conversations` và `fb_messages` qua `psid` hoặc `customerId`
- Hiển thị thông tin khách hàng trong Facebook conversations

**Cần thiết**: ⭐⭐⭐⭐⭐ (Cần thiết nếu tích hợp với Pancake để quản lý Facebook customers)

**Model:**
```typescript
interface FbCustomer {
  id: string;                    // MongoDB ObjectID
  
  // ===== IDENTIFIERS =====
  customerId: string;            // Pancake Customer ID (extract từ panCakeData.id, unique)
  psid: string;                  // Page Scoped ID (Facebook, unique, sparse)
  pageId: string;                // Facebook Page ID (extract từ panCakeData.page_id)
  
  // ===== BASIC INFO =====
  name: string;                  // Tên khách hàng (extract từ panCakeData.name)
  phoneNumbers: string[];        // Số điện thoại (extract từ panCakeData.phone_numbers, array)
  email: string;                 // Email (extract từ panCakeData.email)
  
  // ===== ADDITIONAL INFO =====
  birthday: string;              // Ngày sinh (extract từ panCakeData.birthday)
  gender: string;                // Giới tính (extract từ panCakeData.gender)
  livesIn: string;               // Nơi ở (extract từ panCakeData.lives_in)
  
  // ===== SOURCE DATA =====
  panCakeData: Record<string, any>; // Dữ liệu gốc từ Pancake API
  
  // ===== METADATA =====
  panCakeUpdatedAt: number;      // Thời gian cập nhật từ Pancake (extract từ panCakeData.updated_at)
  createdAt: number;             // Thời gian tạo
  updatedAt: number;             // Thời gian cập nhật
}
```

**Indexes:**
- Unique: `customerId` - Đảm bảo không duplicate customer theo Pancake Customer ID
- Unique, sparse: `psid` - Đảm bảo không duplicate customer theo PSID (không phải customer nào cũng có PSID)
- Text indexes: `customerId`, `psid`, `pageId`, `name`, `phoneNumbers`, `email` - Hỗ trợ tìm kiếm

**Data Extraction (Tự động ở Backend):**
- `customerId` ← `panCakeData.id` (converter=string)
- `psid` ← `panCakeData.psid` (converter=string, optional)
- `pageId` ← `panCakeData.page_id` (converter=string, optional)
- `name` ← `panCakeData.name` (converter=string, optional)
- `phoneNumbers` ← `panCakeData.phone_numbers` (optional, array)
- `email` ← `panCakeData.email` (converter=string, optional)
- `birthday` ← `panCakeData.birthday` (converter=string, optional)
- `gender` ← `panCakeData.gender` (converter=string, optional)
- `livesIn` ← `panCakeData.lives_in` (converter=string, optional)
- `panCakeUpdatedAt` ← `panCakeData.updated_at` (converter=time, format=2006-01-02T15:04:05.000000, optional)

**Endpoints:**
- `/api/v1/fb-customer/*` - Full CRUD operations (Permission: `FbCustomer.*`)
  - `POST /api/v1/fb-customer/upsert-one?filter={"customerId":"xxx"}` - Upsert FB customer (Permission: `FbCustomer.Update`)
  - `GET /api/v1/fb-customer/find` - Tìm FB customers (Permission: `FbCustomer.Read`)
  - Tất cả các CRUD endpoints chuẩn khác

**Ví dụ sử dụng:**

**1. Upsert FB Customer từ Pancake:**
```bash
POST /api/v1/fb-customer/upsert-one?filter={"customerId":"600208cc-136b-4000-8fde-9572e45787a0"}
Authorization: Bearer <token>
Content-Type: application/json

{
  "panCakeData": {
    "id": "600208cc-136b-4000-8fde-9572e45787a0",
    "psid": "25149177694676594",
    "page_id": "page_123",
    "name": "Mai Thao Nguyen",
    "phone_numbers": ["0903154539"],
    "email": "user@example.com",
    "birthday": "1990-01-01",
    "gender": "male",
    "lives_in": "Thành phố Hồ Chí Minh",
    "updated_at": "2025-12-07T10:23:23.000000"
  }
}
```

**2. Tìm FB Customer theo PSID:**
```bash
GET /api/v1/fb-customer/find-one?filter={"psid":"25149177694676594"}
Authorization: Bearer <token>
```

---

### 12. POS Customer Collection (Quản Lý Khách Hàng POS)

**Ý nghĩa**: Quản lý thông tin khách hàng từ Pancake POS API
**Tính năng**:
- Lưu thông tin khách hàng từ Pancake POS API
- Tự động extract các trường từ `posData`
- Link với `pc_pos_orders` qua `customerId`
- Quản lý điểm tích lũy, loyalty programs, customer segmentation

**Cần thiết**: ⭐⭐⭐⭐⭐ (Cần thiết nếu tích hợp với Pancake POS để quản lý POS customers)

**Model:**
```typescript
interface PcPosCustomer {
  id: string;                    // MongoDB ObjectID
  
  // ===== IDENTIFIERS =====
  customerId: string;            // POS Customer ID (UUID string, extract từ posData.id, unique)
  shopId: number;                 // Shop ID (extract từ posData.shop_id)
  
  // ===== BASIC INFO =====
  name: string;                  // Tên khách hàng (extract từ posData.name)
  phoneNumbers: string[];        // Số điện thoại (extract từ posData.phone_numbers, array)
  emails: string[];              // Email (extract từ posData.emails, array - POS có thể có nhiều emails)
  
  // ===== ADDITIONAL INFO =====
  dateOfBirth: string;           // Ngày sinh (extract từ posData.date_of_birth)
  gender: string;                // Giới tính (extract từ posData.gender)
  
  // ===== POS-SPECIFIC FIELDS =====
  customerLevelId: string;       // Customer Level ID (UUID string, extract từ posData.level_id)
  point: number;                 // Điểm tích lũy (extract từ posData.reward_point)
  totalOrder: number;            // Tổng đơn hàng (extract từ posData.order_count)
  totalSpent: number;            // Tổng tiền đã mua (extract từ posData.purchased_amount)
  succeedOrderCount: number;     // Số đơn hàng thành công (extract từ posData.succeed_order_count)
  tagIds: any[];                 // Tags (extract từ posData.tags, array)
  lastOrderAt: number;           // Thời gian đơn hàng cuối (extract từ posData.last_order_at)
  addresses: any[];              // Địa chỉ (extract từ posData.shop_customer_address, array)
  referralCode: string;          // Mã giới thiệu (extract từ posData.referral_code)
  isBlock: boolean;              // Trạng thái block (extract từ posData.is_block)
  
  // ===== SOURCE DATA =====
  posData: Record<string, any>;   // Dữ liệu gốc từ POS API
  
  // ===== METADATA =====
  posUpdatedAt: number;          // Thời gian cập nhật từ POS (extract từ posData.updated_at)
  createdAt: number;             // Thời gian tạo
  updatedAt: number;             // Thời gian cập nhật
}
```

**Indexes:**
- Unique: `customerId` - Đảm bảo không duplicate customer theo POS Customer ID (UUID string)
- Text indexes: `customerId`, `shopId`, `name`, `phoneNumbers`, `emails` - Hỗ trợ tìm kiếm

**Data Extraction (Tự động ở Backend):**
- `customerId` ← `posData.id` (converter=string, UUID)
- `shopId` ← `posData.shop_id` (converter=int64, optional)
- `name` ← `posData.name` (converter=string, optional)
- `phoneNumbers` ← `posData.phone_numbers` (optional, array)
- `emails` ← `posData.emails` (optional, array)
- `dateOfBirth` ← `posData.date_of_birth` (converter=string, optional)
- `gender` ← `posData.gender` (converter=string, optional)
- `customerLevelId` ← `posData.level_id` (converter=string, optional)
- `point` ← `posData.reward_point` (converter=int64, optional)
- `totalOrder` ← `posData.order_count` (converter=int64, optional)
- `totalSpent` ← `posData.purchased_amount` (converter=number, optional)
- `succeedOrderCount` ← `posData.succeed_order_count` (converter=int64, optional)
- `tagIds` ← `posData.tags` (optional, array)
- `lastOrderAt` ← `posData.last_order_at` (converter=time, format=2006-01-02T15:04:05Z, optional)
- `addresses` ← `posData.shop_customer_address` (optional, array)
- `referralCode` ← `posData.referral_code` (converter=string, optional)
- `isBlock` ← `posData.is_block` (converter=bool, optional)
- `posUpdatedAt` ← `posData.updated_at` (converter=time, format=2006-01-02T15:04:05Z, optional)

**Endpoints:**
- `/api/v1/pc-pos-customer/*` - Full CRUD operations (Permission: `PcPosCustomer.*`)
  - `POST /api/v1/pc-pos-customer/upsert-one?filter={"customerId":"xxx"}` - Upsert POS customer (Permission: `PcPosCustomer.Update`)
  - `GET /api/v1/pc-pos-customer/find` - Tìm POS customers (Permission: `PcPosCustomer.Read`)
  - Tất cả các CRUD endpoints chuẩn khác

**Ví dụ sử dụng:**

**1. Upsert POS Customer:**
```bash
POST /api/v1/pc-pos-customer/upsert-one?filter={"customerId":"b0110315-b102-436b-8b3b-ed8d16740327"}
Authorization: Bearer <token>
Content-Type: application/json

{
  "posData": {
    "id": "b0110315-b102-436b-8b3b-ed8d16740327",
    "shop_id": 860225178,
    "name": "Trần Văn Hoàng",
    "gender": "male",
    "emails": ["thudo@gmail.com"],
    "phone_numbers": ["0999999999"],
    "date_of_birth": "1999-09-01",
    "reward_point": 10,
    "level_id": "uuid-here",
    "order_count": 108,
    "purchased_amount": 5000000,
    "succeed_order_count": 8,
    "last_order_at": "2020-04-01T10:18:41Z",
    "referral_code": "1nw4geGA",
    "is_block": false,
    "updated_at": "2025-01-15T10:18:41Z"
  }
}
```

**2. Tìm POS Customer theo Customer ID:**
```bash
GET /api/v1/pc-pos-customer/find-one?filter={"customerId":"b0110315-b102-436b-8b3b-ed8d16740327"}
Authorization: Bearer <token>
```

---

### 11.1. Customer Collection (Deprecated - Không Khuyến Nghị Sử Dụng)

**⚠️ Deprecated**: Collection này đã được tách riêng thành `fb_customers` và `pc_pos_customers` trong Version 2.9. Vui lòng sử dụng các collections mới.

**Ý nghĩa**: Quản lý thông tin khách hàng từ các nguồn (Pancake, POS, ...) - **Đã tách riêng**
**Trạng thái**: ⚠️ Deprecated - Vẫn hoạt động để tương thích ngược, nhưng không khuyến nghị sử dụng cho tính năng mới

**Khuyến nghị:**
- Sử dụng `/api/v1/fb-customer/*` cho Facebook customers
- Sử dụng `/api/v1/pc-pos-customer/*` cho POS customers

**Model:**
```typescript
interface Customer {
  id: string;                    // MongoDB ObjectID
  
  // ===== COMMON FIELDS (Extract từ nhiều nguồn với conflict resolution) =====
  name: string;                   // Tên khách hàng (ưu tiên POS hơn Pancake)
  phoneNumbers: string[];         // Danh sách số điện thoại (merge từ tất cả nguồn)
  email: string;                  // Email khách hàng (ưu tiên POS hơn Pancake)
  birthday: string;               // Ngày sinh (ưu tiên POS hơn Pancake)
  gender: string;                 // Giới tính (ưu tiên POS hơn Pancake)
  
  // ===== COMMON IDENTIFIER =====
  customerId: string;             // ID chung để identify customer từ cả 2 nguồn (dùng cho filter khi upsert)
                                  // Extract từ: posData.id (ưu tiên) hoặc panCakeData.id
                                  // Unique, sparse index
  
  // ===== SOURCE-SPECIFIC IDENTIFIERS =====
  panCakeCustomerId: string;     // Pancake Customer ID (extract từ panCakeData.id)
  psid: string;                  // PSID từ Pancake (Page Scoped ID)
  pageId: string;                 // Page ID từ Pancake
  posCustomerId: string;          // POS Customer ID (extract từ posData.id, UUID string, unique, sparse)
  
  // ===== SOURCE-SPECIFIC DATA =====
  panCakeData: Record<string, any>; // Dữ liệu gốc từ Pancake API
  posData: Record<string, any>;     // Dữ liệu gốc từ POS API
  
  // ===== EXTRACTED FIELDS (Từ các nguồn) =====
  // Pancake-specific
  livesIn: string;                // Nơi ở (extract từ panCakeData.lives_in)
  panCakeUpdatedAt: number;       // Thời gian cập nhật từ Pancake (extract từ panCakeData.updated_at)
  
  // POS-specific
  customerLevelId: string;        // Customer Level ID (extract từ posData.level_id, UUID string)
  point: number;                   // Điểm tích lũy (extract từ posData.reward_point)
  totalOrder: number;              // Tổng đơn hàng (extract từ posData.order_count)
  totalSpent: number;              // Tổng tiền đã mua (extract từ posData.purchased_amount)
  succeedOrderCount: number;        // Số đơn hàng thành công (extract từ posData.succeed_order_count)
  tagIds: any[];                   // Tags (extract từ posData.tags)
  posLastOrderAt: number;          // Thời gian đơn hàng cuối (extract từ posData.last_order_at)
  posAddresses: any[];             // Địa chỉ (extract từ posData.shop_customer_address)
  posReferralCode: string;         // Mã giới thiệu (extract từ posData.referral_code)
  posIsBlock: boolean;             // Trạng thái block (extract từ posData.is_block)
  
  // ===== METADATA =====
  sources: string[];               // ["pancake", "pos"] - Track nguồn dữ liệu
  createdAt: number;              // Thời gian tạo
  updatedAt: number;              // Thời gian cập nhật
}
```

**Indexes:**
- Unique, sparse: `customerId` - Đảm bảo không duplicate customer theo ID chung (dùng cho filter khi upsert)
- Unique, sparse: `posCustomerId` - Đảm bảo không duplicate customer theo POS Customer ID
- Text indexes: `customerId`, `panCakeCustomerId`, `psid`, `pageId`, `posCustomerId`, `name`, `phoneNumbers`, `email` - Hỗ trợ tìm kiếm

**Data Extraction (Tự động ở Backend - Multi-Source):**
- **Lưu ý quan trọng**: Client chỉ cần gửi `panCakeData` và/hoặc `posData` trong DTO, backend tự động extract các field với conflict resolution

**Common Fields (Multi-Source với Priority):**
- `customerId` ← `posData.id` (priority=1) hoặc `panCakeData.id` (priority=2) - ID chung để identify (unique, sparse)
- `name` ← `posData.name` (priority=1) hoặc `panCakeData.name` (priority=2) - Ưu tiên POS
- `phoneNumbers` ← Merge từ `posData.phone_numbers` (priority=1) và `panCakeData.phone_numbers` (priority=2) - Merge array
- `email` ← `posData.emails[0]` (priority=1) hoặc `panCakeData.email` (priority=2) - Ưu tiên POS
- `birthday` ← `posData.date_of_birth` (priority=1) hoặc `panCakeData.birthday` (priority=2) - Ưu tiên POS
- `gender` ← `posData.gender` (priority=1) hoặc `panCakeData.gender` (priority=2) - Ưu tiên POS

**Pancake-specific Fields:**
- `panCakeCustomerId` ← `panCakeData.id` (optional)
- `psid` ← `panCakeData.psid` (optional)
- `pageId` ← `panCakeData.page_id` (optional)
- `livesIn` ← `panCakeData.lives_in` (optional, merge=keep_existing)
- `panCakeUpdatedAt` ← `panCakeData.updated_at` (converted to timestamp, optional)

**POS-specific Fields:**
- `posCustomerId` ← `posData.id` (optional, UUID string)
- `customerLevelId` ← `posData.level_id` (optional, UUID string, merge=overwrite)
- `point` ← `posData.reward_point` (optional, convert to int64, merge=overwrite)
- `totalOrder` ← `posData.order_count` (optional, convert to int64, merge=overwrite)
- `totalSpent` ← `posData.purchased_amount` (optional, convert to number, merge=overwrite)
- `succeedOrderCount` ← `posData.succeed_order_count` (optional, convert to int64, merge=overwrite)
- `tagIds` ← `posData.tags` (optional, merge=overwrite)
- `posLastOrderAt` ← `posData.last_order_at` (optional, convert to timestamp, merge=overwrite)
- `posAddresses` ← `posData.shop_customer_address` (optional, merge=overwrite)
- `posReferralCode` ← `posData.referral_code` (optional, merge=overwrite)
- `posIsBlock` ← `posData.is_block` (optional, convert to bool, merge=overwrite)

**Merge Strategies:**
- `priority`: Chọn giá trị từ nguồn có priority nhỏ nhất (ưu tiên cao nhất)
- `merge_array`: Merge tất cả giá trị vào array, loại bỏ duplicate
- `keep_existing`: Giữ giá trị hiện có nếu đã có, nếu không lấy từ nguồn
- `overwrite`: Luôn ghi đè bằng giá trị mới

**Client không cần extract hoặc gửi các field này**, chỉ cần gửi `panCakeData` và/hoặc `posData` đầy đủ từ các API

**Endpoints:**
- `/api/v1/customer/*` - Full CRUD operations (Permission: `Customer.*`)
  - `POST /api/v1/customer/insert-one` - Tạo customer mới (Permission: `Customer.Insert`)
  - `POST /api/v1/customer/upsert-one?filter={...}` - Upsert customer (dùng cho sync từ cả 2 nguồn) (Permission: `Customer.Update`)
  - `GET /api/v1/customer/find` - Tìm customers (Permission: `Customer.Read`)
  - `GET /api/v1/customer/find-one` - Tìm một customer (Permission: `Customer.Read`)
  - `GET /api/v1/customer/find-by-id/:id` - Tìm customer theo ID (Permission: `Customer.Read`)
  - `POST /api/v1/customer/find-by-ids` - Tìm nhiều customers theo IDs (Permission: `Customer.Read`)
  - `GET /api/v1/customer/find-with-pagination` - Tìm với phân trang (Permission: `Customer.Read`)
  - `PUT /api/v1/customer/update-one` - Cập nhật một customer (Permission: `Customer.Update`)
  - `PUT /api/v1/customer/update-many` - Cập nhật nhiều customers (Permission: `Customer.Update`)
  - `PUT /api/v1/customer/update-by-id/:id` - Cập nhật customer theo ID (Permission: `Customer.Update`)
  - `DELETE /api/v1/customer/delete-one` - Xóa một customer (Permission: `Customer.Delete`)
  - `DELETE /api/v1/customer/delete-many` - Xóa nhiều customers (Permission: `Customer.Delete`)
  - `DELETE /api/v1/customer/delete-by-id/:id` - Xóa customer theo ID (Permission: `Customer.Delete`)
  - `GET /api/v1/customer/count` - Đếm số lượng customers (Permission: `Customer.Read`)
  - `GET /api/v1/customer/distinct` - Lấy danh sách giá trị duy nhất (Permission: `Customer.Read`)
  - `GET /api/v1/customer/exists` - Kiểm tra customer có tồn tại không (Permission: `Customer.Read`)

**Ví dụ sử dụng:**

**1. Upsert Customer từ Pancake (Dùng filter customerId):**
```bash
POST /api/v1/customer/upsert-one?filter={"customerId":"600208cc-136b-4000-8fde-9572e45787a0"}
Authorization: Bearer <token>
Content-Type: application/json

{
  "panCakeData": {
    "id": "600208cc-136b-4000-8fde-9572e45787a0", // customerId sẽ extract từ id này
    "psid": "25149177694676594",
    "page_id": "page_123",
    "name": "Mai Thao Nguyen",
    "phone_numbers": ["0903154539"],
    "email": "user@example.com",
    "birthday": "1990-01-01",
    "gender": "male",
    "lives_in": "Thành phố Hồ Chí Minh",
    "updated_at": "2025-12-07T10:23:23.000000"
  }
}
```

**2. Upsert Customer từ POS (Dùng filter customerId):**
```bash
POST /api/v1/customer/upsert-one?filter={"customerId":"b0110315-b102-436b-8b3b-ed8d16740327"}
Authorization: Bearer <token>
Content-Type: application/json

{
  "posData": {
    "id": "b0110315-b102-436b-8b3b-ed8d16740327", // customerId sẽ extract từ id này
    "name": "Trần Văn Hoàng",
    "gender": "male",
    "emails": ["thudo@gmail.com"],
    "phone_numbers": ["0999999999"],
    "date_of_birth": "1999-09-01",
    "reward_point": 10,
    "level_id": "uuid-here",
    "order_count": 108,
    "purchased_amount": 5000000,
    "succeed_order_count": 8,
    "last_order_at": "2020-04-01T10:18:41Z",
    "referral_code": "1nw4geGA",
    "is_block": false
  }
}
```

**3. Upsert Customer từ cả 2 nguồn (POS + Pancake):**
```bash
POST /api/v1/customer/upsert-one?filter={"customerId":"b0110315-b102-436b-8b3b-ed8d16740327"}
Authorization: Bearer <token>
Content-Type: application/json

{
  "posData": {
    "id": "b0110315-b102-436b-8b3b-ed8d16740327", // customerId sẽ extract từ id này (ưu tiên)
    "name": "Trần Văn Hoàng",
    "phone_numbers": ["0999999999"],
    "emails": ["thudo@gmail.com"],
    ...
  },
  "panCakeData": {
    "id": "600208cc-136b-4000-8fde-9572e45787a0", // Nếu posData.id không có thì dùng id này
    "psid": "25149177694676594",
    "name": "Mai Thao Nguyen",
    ...
  }
}
```

**Tìm Customer:**

**Theo Customer ID (Khuyến nghị - ID chung từ cả 2 nguồn):**
```bash
GET /api/v1/customer/find-one?filter={"customerId":"b0110315-b102-436b-8b3b-ed8d16740327"}
Authorization: Bearer <token>
```

**Theo POS Customer ID:**
```bash
GET /api/v1/customer/find-one?filter={"posCustomerId":"b0110315-b102-436b-8b3b-ed8d16740327"}
Authorization: Bearer <token>
```

**Theo Pancake Customer ID:**
```bash
GET /api/v1/customer/find-one?filter={"panCakeCustomerId":"600208cc-136b-4000-8fde-9572e45787a0"}
Authorization: Bearer <token>
```

**Theo PSID và Page ID:**
```bash
GET /api/v1/customer/find-one?filter={"psid":"25149177694676594","pageId":"page_123"}
Authorization: Bearer <token>
```

**Theo Phone:**
```bash
GET /api/v1/customer/find?filter={"phoneNumbers":"0999999999"}
Authorization: Bearer <token>
```

**Theo Email:**
```bash
GET /api/v1/customer/find-one?filter={"email":"thudo@gmail.com"}
Authorization: Bearer <token>
```

**⚠️ Lưu ý quan trọng:**
- **Collection này đã deprecated**: Không khuyến nghị sử dụng cho tính năng mới
- **Sử dụng collections mới**: Dùng `fb_customers` và `pc_pos_customers` thay thế
- **Tương thích ngược**: Endpoints vẫn hoạt động để đảm bảo tương thích với code cũ
- **Migration**: Bot sẽ đồng bộ lại dữ liệu vào 2 collections mới

---

### 6. Pancake Integration Module (TÙY CHỌN - Nếu cần tích hợp Pancake)

#### PcOrder Collection
**Ý nghĩa**: Quản lý đơn hàng từ hệ thống Pancake
**Tính năng**:
- Lưu thông tin đơn hàng từ Pancake
- Đồng bộ dữ liệu đầy đủ từ Pancake API (panCakeData)
- Quản lý trạng thái đơn hàng

**Cần thiết**: ⭐⭐ (TÙY CHỌN - Chỉ cần nếu tích hợp với hệ thống Pancake)

**Model:**
```typescript
interface PcOrder {
  id: string;
  pancakeOrderId: string; // Pancake Order ID (unique)
  status: number; // 0: active, 1: inactive
  panCakeData: Record<string, any>; // Full data from Pancake API
  createdAt: number;
  updatedAt: number;
}
```

**Endpoints:**
- `/api/v1/pancake/order/*` - Full CRUD operations
  - `POST /api/v1/pancake/order/insert-one` - Tạo order mới
  - `GET /api/v1/pancake/order/find` - Tìm orders
  - `GET /api/v1/pancake/order/find-one` - Tìm một order
  - `GET /api/v1/pancake/order/find-by-id/:id` - Tìm order theo ID
  - `POST /api/v1/pancake/order/find-by-ids` - Tìm nhiều orders theo IDs
  - `GET /api/v1/pancake/order/find-with-pagination` - Tìm với phân trang
  - `PUT /api/v1/pancake/order/update-one` - Cập nhật một order
  - `PUT /api/v1/pancake/order/update-many` - Cập nhật nhiều orders
  - `PUT /api/v1/pancake/order/update-by-id/:id` - Cập nhật order theo ID
  - `DELETE /api/v1/pancake/order/delete-one` - Xóa một order
  - `DELETE /api/v1/pancake/order/delete-many` - Xóa nhiều orders
  - `DELETE /api/v1/pancake/order/delete-by-id/:id` - Xóa order theo ID
  - `GET /api/v1/pancake/order/count` - Đếm số lượng orders
  - `GET /api/v1/pancake/order/distinct` - Lấy danh sách giá trị duy nhất
  - `GET /api/v1/pancake/order/exists` - Kiểm tra order có tồn tại không

---

## 🔄 Data Extraction từ PanCakeData

Hệ thống hỗ trợ tự động extract dữ liệu từ nested object `panCakeData` vào các trường riêng biệt thông qua struct tags.

### Cách Hoạt Động

Khi insert hoặc update một document có field `panCakeData`, hệ thống sẽ tự động:
1. Parse struct tags `extract` trong model
2. Extract giá trị từ `panCakeData` theo path được chỉ định
3. Convert giá trị nếu có converter (time, number, int64, bool, string)
4. Gán vào field tương ứng

### Format Extract Tag

```
extract:"PanCakeData\\.field_path[,converter=name][,format=value][,optional|required]"
```

**Ví dụ:**
- `extract:"PanCakeData\\.id"` - Extract từ `panCakeData.id`
- `extract:"PanCakeData\\.customer_id,optional"` - Extract từ `panCakeData.customer_id` (optional)
- `extract:"PanCakeData\\.updated_at,converter=time,format=2006-01-02T15:04:05.000000,optional"` - Extract và convert thời gian

### Converters Hỗ Trợ

- `time` - Convert ISO 8601 string sang Unix timestamp (giây)
- `number` - Convert sang number
- `int64` - Convert sang int64
- `bool` - Convert sang boolean
- `string` - Convert sang string (mặc định)

### Ví Dụ Sử Dụng

**FbConversation:**
```typescript
// Request body khi insert
{
  "pageId": "page_123",
  "pageUsername": "my_page",
  "panCakeData": {
    "id": "conv_123456",
    "customer_id": "customer_789",
    "updated_at": "2019-08-24T14:15:22.000000",
    "type": "INBOX"
  }
}

// Sau khi insert, hệ thống tự động extract:
{
  "id": "...",
  "pageId": "page_123",
  "pageUsername": "my_page",
  "conversationId": "conv_123456",        // ← Từ panCakeData.id
  "customerId": "customer_789",           // ← Từ panCakeData.customer_id
  "panCakeUpdatedAt": 1566656122,         // ← Từ panCakeData.updated_at (converted)
  "panCakeData": { ... },
  "createdAt": 1234567890,
  "updatedAt": 1234567890
}
```

**FbMessage:**
```typescript
// Request body khi insert
{
  "pageId": "page_123",
  "pageUsername": "my_page",
  "customerId": "customer_789",
  "panCakeData": {
    "conversation_id": "conv_123456",
    "message": "Hello",
    "from": { "id": "user_123", "name": "John" }
  }
}

// Sau khi insert, hệ thống tự động extract:
{
  "id": "...",
  "pageId": "page_123",
  "pageUsername": "my_page",
  "conversationId": "conv_123456",        // ← Từ panCakeData.conversation_id
  "customerId": "customer_789",
  "panCakeData": { ... },
  "createdAt": 1234567890,
  "updatedAt": 1234567890
}
```

**FbPost:**
```typescript
// Request body khi insert
{
  "panCakeData": {
    "id": "256469571178082_1719461745119729",
    "page_id": "256469571178082",
    "from": {
      "id": "5460527857372996",
      "name": "Djamel Belkessa"
    },
    "message": "edit review là 1 nghệ thuật",
    "type": "rating",
    "inserted_at": "2022-08-22T03:09:27",
    "comment_count": 0,
    "reactions": {
      "like_count": 111,
      "love_count": 14
    }
  }
}

// Sau khi insert, hệ thống tự động extract:
{
  "id": "...",
  "pageId": "256469571178082",            // ← Từ panCakeData.page_id
  "postId": "256469571178082_1719461745119729", // ← Từ panCakeData.id
  "insertedAt": 1661130567000,            // ← Từ panCakeData.inserted_at (converted)
  "panCakeData": { ... },
  "createdAt": 1234567890,
  "updatedAt": 1234567890
}
```

**Customer:**
```typescript
// Request body khi upsert từ Pancake (khuyến nghị dùng panCakeCustomerId)
// LƯU Ý: Client chỉ cần gửi panCakeData, không cần extract hoặc gửi các field extracted
POST /api/v1/customer/upsert-one?filter={"panCakeCustomerId":"600208cc-136b-4000-8fde-9572e45787a0"}
{
  "panCakeData": {
    "id": "600208cc-136b-4000-8fde-9572e45787a0",
    "psid": "25149177694676594",
    "page_id": "page_123",
    "name": "Mai Thao Nguyen",
    "phone_numbers": ["0903154539"],
    "email": "user@example.com",
    "birthday": "1990-01-01",
    "gender": "male",
    "lives_in": "Thành phố Hồ Chí Minh",
    "updated_at": "2025-12-07T10:23:23.000000"
  }
}

// Sau khi upsert, BACKEND TỰ ĐỘNG extract các field sau (client không cần làm gì):
{
  "id": "...",
  "panCakeCustomerId": "600208cc-136b-4000-8fde-9572e45787a0", // ← Backend extract từ panCakeData.id
  "psid": "25149177694676594",                    // ← Backend extract từ panCakeData.psid
  "pageId": "page_123",                           // ← Backend extract từ panCakeData.page_id
  "name": "Mai Thao Nguyen",                      // ← Backend extract từ panCakeData.name
  "phoneNumbers": ["0903154539"],                 // ← Backend extract từ panCakeData.phone_numbers
  "email": "user@example.com",                    // ← Backend extract từ panCakeData.email
  "birthday": "1990-01-01",                       // ← Backend extract từ panCakeData.birthday
  "gender": "male",                               // ← Backend extract từ panCakeData.gender
  "livesIn": "Thành phố Hồ Chí Minh",             // ← Backend extract từ panCakeData.lives_in
  "panCakeUpdatedAt": 1733555003000,              // ← Backend extract từ panCakeData.updated_at (converted)
  "panCakeData": { ... },                         // ← Giữ nguyên dữ liệu gốc
  "createdAt": 1766039204906,
  "updatedAt": 1766039204906
}
```

### Lưu Ý

1. **Path Syntax:** Sử dụng `\\.` để escape dấu chấm trong path (ví dụ: `PanCakeData\\.id`)
2. **Optional Fields:** Nếu field là optional và không tìm thấy trong `panCakeData`, field sẽ là `null` hoặc empty
3. **Required Fields:** Nếu field là required và không tìm thấy, sẽ trả về lỗi validation
4. **Time Format:** Format mặc định cho time converter là `2006-01-02T15:04:05` (Go time format)
5. **Nested Path:** Có thể extract từ nested path (ví dụ: `PanCakeData\\.from\\.id`)

---

## 📡 API Endpoints Chi Tiết

### 1. System Routes

#### Health Check
```
GET /api/v1/system/health
```
**Không cần authentication**

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-12-09T21:02:38Z",
  "services": {
    "api": "ok",
    "database": "ok"
  }
}
```

---

### 2. Authentication Routes (Không cần token)

#### Đăng Nhập Bằng Firebase
```
POST /api/v1/auth/login/firebase
```

**Request Body:**
```json
{
  "idToken": "firebase-id-token-from-client-sdk",
  "hwid": "hardware-id-unique"
}
```

**Response:**
```json
{
  "code": 200,
  "message": "Thao tác thành công",
  "data": {
    "id": "507f1f77bcf86cd799439011",
    "firebaseUid": "firebase-user-uid",
    "name": "Tên người dùng",
    "email": "user@example.com",
    "emailVerified": true,
    "phone": "+84123456789",
    "phoneVerified": true,
    "avatarUrl": "https://example.com/avatar.jpg",
    "token": "jwt-token-string",
    "createdAt": 1702147200,
    "updatedAt": 1702147200
  },
  "status": "success"
}
```

**Lưu ý:** 
- Lưu `token` để sử dụng cho các request tiếp theo
- User được tạo tự động trong MongoDB nếu chưa tồn tại
- Nếu là user đầu tiên và chưa có admin, user này sẽ tự động trở thành Administrator

#### Đăng Xuất
```
POST /api/v1/auth/logout
```
**Cần authentication**

**Request Body:**
```json
{
  "hwid": "hardware-id-unique"
}
```

#### Lấy Thông Tin Profile
```
GET /api/v1/auth/profile
```
**Cần authentication**

#### Cập Nhật Profile
```
PUT /api/v1/auth/profile
```
**Cần authentication**

**Request Body:**
```json
{
  "name": "Tên mới"
}
```

**Lưu ý:** Email và phone được quản lý bởi Firebase, không thể thay đổi qua API này.

#### Lấy Danh Sách Roles Của User
```
GET /api/v1/auth/roles
```
**Cần authentication**

---

### 3. CRUD Operations Pattern

Hệ thống sử dụng pattern CRUD thống nhất cho tất cả các collections. Các endpoints có format:

#### Create Operations
- `POST /api/v1/{collection}/insert-one` - Tạo một document
- `POST /api/v1/{collection}/insert-many` - Tạo nhiều documents

#### Read Operations
- `GET /api/v1/{collection}/find` - Tìm tất cả (có filter)
- `GET /api/v1/{collection}/find-one` - Tìm một document
- `GET /api/v1/{collection}/find-by-id/:id` - Tìm theo ID
- `POST /api/v1/{collection}/find-by-ids` - Tìm nhiều documents theo IDs
- `GET /api/v1/{collection}/find-with-pagination` - Tìm có phân trang
- `GET /api/v1/{collection}/count` - Đếm số documents
- `GET /api/v1/{collection}/distinct` - Lấy giá trị distinct
- `GET /api/v1/{collection}/exists` - Kiểm tra document tồn tại

#### Update Operations
- `PUT /api/v1/{collection}/update-one` - Cập nhật một document
- `PUT /api/v1/{collection}/update-many` - Cập nhật nhiều documents
- `PUT /api/v1/{collection}/update-by-id/:id` - Cập nhật theo ID
- `PUT /api/v1/{collection}/find-one-and-update` - Tìm và cập nhật
- `POST /api/v1/{collection}/upsert-one` - Upsert một document
- `POST /api/v1/{collection}/upsert-many` - Upsert nhiều documents

#### Delete Operations
- `DELETE /api/v1/{collection}/delete-one` - Xóa một document
- `DELETE /api/v1/{collection}/delete-many` - Xóa nhiều documents
- `DELETE /api/v1/{collection}/delete-by-id/:id` - Xóa theo ID
- `DELETE /api/v1/{collection}/find-one-and-delete` - Tìm và xóa

#### Query Parameters cho Find Operations

**Filter (query string):**
```
GET /api/v1/user/find?filter={"email":"user@example.com"}
```

**Options (query string):**
```
GET /api/v1/user/find?options={"sort":{"createdAt":-1},"limit":10,"skip":0}
```

**Pagination:**
```
GET /api/v1/user/find-with-pagination?page=1&limit=10&filter={"name":"John"}
```

**Response Pagination:**
```json
{
  "code": 200,
  "message": "Thao tác thành công",
  "data": {
    "page": 1,
    "limit": 10,
    "itemCount": 5,
    "items": [ /* danh sách items */ ]
  },
  "status": "success"
}
```

---

### 4. Facebook Message Endpoint Đặc Biệt

#### Upsert Messages (Tự Động Tách Messages)
```
POST /api/v1/facebook/message/upsert-messages
```
**Permission:** `FbMessage.Update`

**Mục đích:** Upsert messages từ Pancake API với logic tự động tách `messages[]` ra khỏi `panCakeData` và lưu vào 2 collections riêng biệt để tối ưu performance và scalability.

**Request Body:**
```json
{
  "conversationId": "157725629736743_9350439438393456",
  "pageId": "157725629736743",
  "pageUsername": "Folkformint",
  "customerId": "8b168fa9-4836-4648-a3fd-799c227675a1",
  "panCakeData": {
    "conv_from": {
      "id": "user_123",
      "name": "John Doe"
    },
    "read_watermarks": [
      {
        "user_id": "user_123",
        "timestamp": "2025-12-16T15:22:45.000000"
      }
    ],
    "activities": [
      {
        "type": "message",
        "timestamp": "2025-12-16T15:22:45.000000"
      }
    ],
    "messages": [
      {
        "id": "m_xxx1",
        "conversation_id": "157725629736743_9350439438393456",
        "message": "<div>Message 1</div>",
        "inserted_at": "2025-12-16T15:22:45.000000",
        "from": {
          "id": "user_123",
          "name": "John Doe"
        },
        "attachments": []
      },
      {
        "id": "m_xxx2",
        "conversation_id": "157725629736743_9350439438393456",
        "message": "<div>Message 2</div>",
        "inserted_at": "2025-12-16T15:23:45.000000",
        "from": {
          "id": "user_456",
          "name": "Jane Smith"
        },
        "attachments": []
      }
    ]
  },
  "hasMore": true
}
```

**Response:**
```json
{
  "code": 200,
  "message": "Thao tác thành công",
  "data": {
    "id": "507f1f77bcf86cd799439011",
    "pageId": "157725629736743",
    "pageUsername": "Folkformint",
    "conversationId": "157725629736743_9350439438393456",
    "customerId": "8b168fa9-4836-4648-a3fd-799c227675a1",
    "panCakeData": {
      "conv_from": {
        "id": "user_123",
        "name": "John Doe"
      },
      "read_watermarks": [...],
      "activities": [...]
      // Lưu ý: KHÔNG có messages[] (đã được tách ra)
    },
    "lastSyncedAt": 1765898960082,
    "totalMessages": 2,
    "hasMore": true,
    "createdAt": 1765898960082,
    "updatedAt": 1765898960082
  },
  "status": "success"
}
```

**Logic Xử Lý Nội Bộ:**

1. **Tách messages[] ra khỏi panCakeData:**
   - Extract `messages[]` từ `panCakeData.messages`
   - Tạo `metadataPanCakeData` (copy `panCakeData` nhưng bỏ `messages[]`)

2. **Upsert metadata vào `fb_messages`:**
   - Upsert theo `conversationId` (unique)
   - Lưu metadata (panCakeData không có messages[])
   - Cập nhật `lastSyncedAt`, `hasMore`

3. **Upsert messages vào `fb_message_items`:**
   - Bulk upsert từng message theo `messageId` (unique)
   - Tự động tránh duplicate (nếu message đã tồn tại → update, chưa có → insert)
   - Extract `insertedAt` từ `messageData.inserted_at` (convert sang Unix timestamp)

4. **Cập nhật totalMessages:**
   - Count messages trong `fb_message_items` theo `conversationId`
   - Update vào `fb_messages.totalMessages`

**Lưu ý quan trọng:**

- ✅ **API bên ngoài không cần thay đổi**: Vẫn gửi `panCakeData` đầy đủ (bao gồm `messages[]`)
- ✅ **Logic tách tự động**: Server tự động tách và lưu vào 2 collections
- ✅ **Tự động tránh duplicate**: Upsert theo `messageId`, không tạo duplicate
- ✅ **Scalable**: Không có giới hạn số lượng messages (mỗi message là 1 document riêng)
- ✅ **Performance tốt**: Query nhanh với index trên `conversationId` + `insertedAt`
- ⚠️ **Khác với CRUD routes**: CRUD routes (`/insert-one`, `/update-one`) không có logic tách messages, lưu nguyên `panCakeData`

**Khi nào dùng:**

- ✅ Sync messages từ Pancake API (khuyến nghị)
- ✅ Đồng bộ dữ liệu tự động
- ✅ Xử lý số lượng messages lớn

**Khi nào KHÔNG dùng:**

- ❌ Tạo/cập nhật message thủ công → Dùng CRUD routes (`/insert-one`, `/update-one`)
- ❌ Import dữ liệu từ nguồn khác → Dùng CRUD routes

**Ví dụ sử dụng trong Frontend:**

```typescript
// Sync messages từ Pancake API
const syncMessages = async (
  conversationId: string,
  pageId: string,
  pageUsername: string,
  customerId: string,
  panCakeData: any, // PanCakeData đầy đủ (bao gồm messages[])
  hasMore: boolean = false
) => {
  const response = await apiClient.request<{ data: FbMessage }>(
    '/facebook/message/upsert-messages',
    {
      method: 'POST',
      body: JSON.stringify({
        conversationId,
        pageId,
        pageUsername,
        customerId,
        panCakeData, // Gửi đầy đủ, server tự động tách
        hasMore
      })
    }
  );
  
  // Response chứa metadata (không có messages[])
  // Messages đã được lưu riêng trong fb_message_items
  return response.data;
};
```

---

## 📝 Input Structs và Request Parameters

Tất cả các endpoints đều sử dụng các DTO (Data Transfer Object) structs để định nghĩa input. Dưới đây là danh sách đầy đủ các input structs cho từng module.

### Authentication Module

#### FirebaseLoginInput
**Endpoint:** `POST /api/v1/auth/login/firebase`

```typescript
interface FirebaseLoginInput {
  idToken: string;  // Firebase ID token (required)
  hwid: string;     // Device hardware ID (required)
}
```

#### UserLogoutInput
**Endpoint:** `POST /api/v1/auth/logout`

```typescript
interface UserLogoutInput {
  hwid: string;     // Device hardware ID (required)
}
```

#### UserChangeInfoInput
**Endpoint:** `PUT /api/v1/auth/profile`

```typescript
interface UserChangeInfoInput {
  name?: string;    // Tên người dùng (optional)
}
```

#### UserCreateInput
**Endpoint:** `POST /api/v1/user/insert-one`

**Lưu ý:** User được tạo tự động từ Firebase, không cần tạo thủ công. DTO này chỉ dùng cho CRUD operations.

```typescript
interface UserCreateInput {
  name: string;     // Tên người dùng (required)
  email: string;    // Email người dùng (required)
}
```

#### BlockUserInput
**Endpoint:** `POST /api/v1/admin/user/block`

```typescript
interface BlockUserInput {
  email: string;    // Email người dùng cần chặn (required)
  note: string;     // Lý do chặn (required)
}
```

#### UnBlockUserInput
**Endpoint:** `POST /api/v1/admin/user/unblock`

```typescript
interface UnBlockUserInput {
  email: string;    // Email người dùng cần bỏ chặn (required)
}
```

---

### RBAC Module

#### RoleCreateInput
**Endpoint:** `POST /api/v1/role/insert-one`

```typescript
interface RoleCreateInput {
  name: string;     // Tên vai trò (required)
  describe: string; // Mô tả vai trò (required)
}
```

#### RoleUpdateInput
**Endpoint:** `PUT /api/v1/role/update-by-id/:id`

```typescript
interface RoleUpdateInput {
  name?: string;     // Tên vai trò (optional)
  describe?: string; // Mô tả vai trò (optional)
}
```

#### PermissionCreateInput
**Endpoint:** `POST /api/v1/permission/insert-one`

```typescript
interface PermissionCreateInput {
  name: string;     // Tên quyền (required, format: "Module.Action")
  describe: string; // Mô tả quyền (required)
  category: string; // Danh mục quyền (required, ví dụ: "Auth", "Pancake")
  group: string;     // Nhóm quyền (required, ví dụ: "User", "Role")
}
```

#### PermissionUpdateInput
**Endpoint:** `PUT /api/v1/permission/update-by-id/:id`

```typescript
interface PermissionUpdateInput {
  name?: string;     // Tên quyền (optional)
  describe?: string; // Mô tả quyền (optional)
  category?: string; // Danh mục quyền (optional)
  group?: string;     // Nhóm quyền (optional)
}
```

#### OrganizationCreateInput
**Endpoint:** `POST /api/v1/organization/insert-one`

```typescript
interface OrganizationCreateInput {
  name: string;     // Tên tổ chức (required)
  code: string;     // Mã tổ chức (required, unique)
  type: string;     // Loại tổ chức (required): "system" | "group" | "company" | "department" | "division" | "team"
  parentId?: string; // ID tổ chức cha (optional, string ObjectID)
  isActive?: boolean; // Trạng thái hoạt động (optional, default: true)
}
```

#### OrganizationUpdateInput
**Endpoint:** `PUT /api/v1/organization/update-by-id/:id`

```typescript
interface OrganizationUpdateInput {
  name?: string;     // Tên tổ chức (optional)
  code?: string;     // Mã tổ chức (optional, unique)
  type?: string;     // Loại tổ chức (optional)
  parentId?: string; // ID tổ chức cha (optional, string ObjectID)
  isActive?: boolean; // Trạng thái hoạt động (optional, dùng để phân biệt false và không cập nhật)
}
```

#### RolePermissionCreateInput
**Endpoint:** `POST /api/v1/role-permission/insert-one`

```typescript
interface RolePermissionCreateInput {
  roleId: string;       // ID vai trò (required)
  permissionId: string; // ID quyền (required)
  scope?: number;       // Phạm vi quyền (optional, default: 0)
  // 0: Chỉ tổ chức role thuộc về
  // 1: Tổ chức đó và tất cả các tổ chức con
}
```

#### RolePermissionUpdateInput
**Endpoint:** `PUT /api/v1/role-permission/update-role`

```typescript
interface RolePermissionUpdateItem {
  permissionId: string; // ID quyền (required)
  scope: number;        // Phạm vi quyền (0 hoặc 1)
}

interface RolePermissionUpdateInput {
  roleId: string;                    // ID vai trò (required)
  permissions: RolePermissionUpdateItem[]; // Danh sách quyền với scope (required)
}
```

#### UserRoleCreateInput
**Endpoint:** `POST /api/v1/user-role/insert-one`

```typescript
interface UserRoleCreateInput {
  userId: string; // ID người dùng (required)
  roleId: string; // ID vai trò (required)
}
```

#### UserRoleUpdateInput
**Endpoint:** `PUT /api/v1/user-role/update-user-roles`

```typescript
interface UserRoleUpdateInput {
  userId: string;   // ID người dùng (required)
  roleIds: string[]; // Danh sách ID vai trò (required, min: 1)
}
```

---

### Agent Module

#### AgentCreateInput
**Endpoint:** `POST /api/v1/agent/insert-one`

```typescript
interface AgentCreateInput {
  name: string;                    // Tên agent (required)
  describe: string;                // Mô tả agent (required)
  assignedUsers?: string[];        // Danh sách user IDs được gán (optional)
  configData?: Record<string, any>; // Dữ liệu cấu hình (optional)
}
```

#### AgentUpdateInput
**Endpoint:** `PUT /api/v1/agent/update-by-id/:id`

```typescript
interface AgentUpdateInput {
  name?: string;                    // Tên agent (optional)
  describe?: string;                // Mô tả agent (optional)
  status?: number;                  // Trạng thái (optional, 0: offline, 1: online)
  command?: number;                 // Lệnh điều khiển (optional, 0: stop, 1: play)
  assignedUsers?: string[];         // Danh sách user IDs (optional)
  configData?: Record<string, any>;  // Dữ liệu cấu hình (optional)
}
```

---

### Facebook Integration Module

#### FbPageCreateInput
**Endpoint:** `POST /api/v1/facebook/page/insert-one`

```typescript
interface FbPageCreateInput {
  accessToken: string;              // Access token (required)
  panCakeData: Record<string, any>;  // Dữ liệu từ Pancake API (required)
}
```

#### FbPageUpdateTokenInput
**Endpoint:** `PUT /api/v1/facebook/page/update-token`

```typescript
interface FbPageUpdateTokenInput {
  pageId: string;          // Facebook Page ID (required)
  pageAccessToken: string; // Page Access Token mới (required)
}
```

#### FbPostCreateInput
**Endpoint:** `POST /api/v1/facebook/post/insert-one`

```typescript
interface FbPostCreateInput {
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API (required)
}
```

#### FbPostUpdateTokenInput
**Endpoint:** `PUT /api/v1/facebook/post/update-token`

```typescript
interface FbPostUpdateTokenInput {
  postId: string;                   // Facebook Post ID (required)
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API (required)
}
```

#### FbConversationCreateInput
**Endpoint:** `POST /api/v1/facebook/conversation/insert-one`

```typescript
interface FbConversationCreateInput {
  pageId: string;                   // Facebook Page ID (required)
  pageUsername: string;             // Tên người dùng của trang (required)
  panCakeData?: Record<string, any>; // Dữ liệu từ Pancake API (optional)
  // Lưu ý: conversationId, customerId, panCakeUpdatedAt sẽ được extract tự động từ panCakeData
}
```

#### FbMessageCreateInput
**Endpoint:** `POST /api/v1/facebook/message/insert-one` (CRUD Route - Logic chung)

```typescript
interface FbMessageCreateInput {
  pageId: string;                   // Facebook Page ID (required)
  pageUsername: string;             // Tên người dùng của trang (required)
  conversationId: string;            // Conversation ID (required)
  customerId: string;                // Customer ID (required)
  panCakeData: Record<string, any>;  // Dữ liệu từ Pancake API (required, có thể có messages[])
  // Lưu ý: conversationId sẽ được extract tự động từ panCakeData.conversation_id nếu có
  // Lưu ý: CRUD routes KHÔNG tự động tách messages[], lưu nguyên panCakeData
}
```

#### FbMessageUpsertMessagesInput
**Endpoint:** `POST /api/v1/facebook/message/upsert-messages` (Endpoint Đặc Biệt - Logic tách messages)

```typescript
interface FbMessageUpsertMessagesInput {
  pageId: string;                   // Facebook Page ID (required)
  pageUsername: string;             // Tên người dùng của trang (required)
  conversationId: string;            // Conversation ID (required)
  customerId: string;                // Customer ID (required)
  panCakeData: Record<string, any>;  // Dữ liệu từ Pancake API (required, đầy đủ bao gồm messages[])
  hasMore: boolean;                 // Còn messages để sync không (optional, default: false)
  // Lưu ý: Endpoint này tự động tách messages[] ra khỏi panCakeData và lưu vào 2 collections:
  // - Metadata (panCakeData không có messages[]) → fb_messages
  // - Messages (từng message riêng lẻ) → fb_message_items
}
```

#### FbMessageItemCreateInput
**Endpoint:** `POST /api/v1/facebook/message-item/insert-one` (CRUD Route)

```typescript
interface FbMessageItemCreateInput {
  conversationId: string;            // ID của cuộc hội thoại (required)
  messageId: string;                  // ID của message từ Pancake (required, unique)
  messageData: Record<string, any>;  // Toàn bộ dữ liệu của message (required)
  insertedAt?: number;                // Thời gian insert message - Unix timestamp (optional, có thể extract từ messageData.inserted_at)
}
```

#### FbMessageItemUpdateInput
**Endpoint:** `PUT /api/v1/facebook/message-item/update-one` (CRUD Route)

```typescript
interface FbMessageItemUpdateInput {
  conversationId?: string;            // ID của cuộc hội thoại (optional)
  messageId?: string;                  // ID của message từ Pancake (optional)
  messageData?: Record<string, any>;  // Toàn bộ dữ liệu của message (optional)
  insertedAt?: number;                // Thời gian insert message - Unix timestamp (optional)
}
```

---

### Customer Module

#### CustomerCreateInput
**Endpoint:** `POST /api/v1/customer/insert-one` hoặc `POST /api/v1/customer/upsert-one?filter={...}`

```typescript
interface CustomerCreateInput {
  panCakeData?: Record<string, any>; // Dữ liệu từ Pancake API (optional)
  posData?: Record<string, any>;     // Dữ liệu từ POS API (optional)
  // LƯU Ý QUAN TRỌNG: Client chỉ cần gửi panCakeData và/hoặc posData, không cần extract hoặc gửi các field extracted
  // Ít nhất 1 trong 2 nguồn phải có
  // Backend tự động extract các field sau:
  // - customerId ← posData.id (ưu tiên) hoặc panCakeData.id
  // - panCakeCustomerId ← panCakeData.id
  // - posCustomerId ← posData.id
  // - psid ← panCakeData.psid
  // - pageId ← panCakeData.page_id
  // - name ← posData.name (ưu tiên) hoặc panCakeData.name
  // - phoneNumbers ← merge từ posData.phone_numbers và panCakeData.phone_numbers
  // - email ← posData.emails[0] (ưu tiên) hoặc panCakeData.email
  // - birthday ← posData.date_of_birth (ưu tiên) hoặc panCakeData.birthday
  // - gender ← posData.gender (ưu tiên) hoặc panCakeData.gender
  // - livesIn ← panCakeData.lives_in
  // - panCakeUpdatedAt ← panCakeData.updated_at
  // - point, totalOrder, totalSpent, etc. ← posData.*
}
```

**Ví dụ sử dụng với upsert-one:**
```typescript
// Upsert customer từ Pancake (khuyến nghị dùng customerId)
// LƯU Ý: Client chỉ cần gửi panCakeData, backend tự động extract các field
POST /api/v1/customer/upsert-one?filter={"customerId":"600208cc-136b-4000-8fde-9572e45787a0"}
{
  "panCakeData": {
    "id": "600208cc-136b-4000-8fde-9572e45787a0", // customerId sẽ extract từ id này
    "psid": "25149177694676594",
    "page_id": "page_123",
    "name": "Mai Thao Nguyen",
    "phone_numbers": ["0903154539"],
    "email": "user@example.com",
    "birthday": "1990-01-01",
    "gender": "male",
    "lives_in": "Thành phố Hồ Chí Minh",
    "updated_at": "2025-12-07T10:23:23.000000"
  }
}

// Upsert customer từ POS
POST /api/v1/customer/upsert-one?filter={"customerId":"b0110315-b102-436b-8b3b-ed8d16740327"}
{
  "posData": {
    "id": "b0110315-b102-436b-8b3b-ed8d16740327", // customerId sẽ extract từ id này
    "name": "Trần Văn Hoàng",
    "phone_numbers": ["0999999999"],
    "emails": ["thudo@gmail.com"],
    ...
  }
}

// Upsert customer từ cả 2 nguồn
POST /api/v1/customer/upsert-one?filter={"customerId":"b0110315-b102-436b-8b3b-ed8d16740327"}
{
  "posData": { ... },
  "panCakeData": { ... }
}
```

---

### Pancake Integration Module

```typescript
interface FbMessageItemUpdateInput {
  conversationId: string;            // ID của cuộc hội thoại (required)
  messageId: string;                  // ID của message từ Pancake (required, unique)
  messageData: Record<string, any>;  // Toàn bộ dữ liệu của message (required)
  insertedAt?: number;                // Thời gian insert message - Unix timestamp (optional, có thể extract từ messageData.inserted_at)
}
```

**Lưu ý về FbMessageItem DTOs:**
- `messageId` phải unique trong toàn bộ collection
- `insertedAt` có thể được extract tự động từ `messageData.inserted_at` nếu không được cung cấp
- `messageData` chứa toàn bộ dữ liệu của message từ Pancake API

---

### Customer Module

#### CustomerCreateInput
**Endpoint:** `POST /api/v1/customer/insert-one` hoặc `POST /api/v1/customer/upsert-one?filter={...}`

```typescript
interface CustomerCreateInput {
  panCakeData?: Record<string, any>; // Dữ liệu từ Pancake API (optional)
  posData?: Record<string, any>;     // Dữ liệu từ POS API (optional)
  // LƯU Ý QUAN TRỌNG: Client chỉ cần gửi panCakeData và/hoặc posData, không cần extract hoặc gửi các field extracted
  // Ít nhất 1 trong 2 nguồn phải có
  // Backend tự động extract các field sau:
  // - customerId ← posData.id (ưu tiên) hoặc panCakeData.id
  // - panCakeCustomerId ← panCakeData.id
  // - posCustomerId ← posData.id
  // - psid ← panCakeData.psid
  // - pageId ← panCakeData.page_id
  // - name ← posData.name (ưu tiên) hoặc panCakeData.name
  // - phoneNumbers ← merge từ posData.phone_numbers và panCakeData.phone_numbers
  // - email ← posData.emails[0] (ưu tiên) hoặc panCakeData.email
  // - birthday ← posData.date_of_birth (ưu tiên) hoặc panCakeData.birthday
  // - gender ← posData.gender (ưu tiên) hoặc panCakeData.gender
  // - livesIn ← panCakeData.lives_in
  // - panCakeUpdatedAt ← panCakeData.updated_at
  // - point, totalOrder, totalSpent, etc. ← posData.*
}
```

**Ví dụ sử dụng với upsert-one (Khuyến nghị dùng customerId):**
```typescript
// Upsert customer từ Pancake (khuyến nghị dùng customerId)
// LƯU Ý: Client chỉ cần gửi panCakeData, backend tự động extract các field
POST /api/v1/customer/upsert-one?filter={"customerId":"600208cc-136b-4000-8fde-9572e45787a0"}
{
  "panCakeData": {
    "id": "600208cc-136b-4000-8fde-9572e45787a0", // customerId sẽ extract từ id này
    "psid": "25149177694676594",
    "page_id": "page_123",
    "name": "Mai Thao Nguyen",
    "phone_numbers": ["0903154539"],
    "email": "user@example.com",
    "birthday": "1990-01-01",
    "gender": "male",
    "lives_in": "Thành phố Hồ Chí Minh",
    "updated_at": "2025-12-07T10:23:23.000000"
  }
}

// Hoặc dùng psid + pageId nếu không có panCakeCustomerId
POST /api/v1/customer/upsert-one?filter={"psid":"25149177694676594","pageId":"page_123"}
{
  "panCakeData": { ... }
}
```

**Lưu ý về Customer DTO:**
- **Client chỉ cần gửi `panCakeData` và/hoặc `posData`**: DTO có 2 field optional `panCakeData` và `posData` (dữ liệu gốc từ các API), không cần gửi các field extracted như `customerId`, `panCakeCustomerId`, `posCustomerId`, `psid`, `name`, etc.
- **Backend tự động extract**: Hệ thống tự động extract các field từ `panCakeData` và/hoặc `posData` qua struct tag `extract` với conflict resolution khi insert/update, client không cần xử lý gì
- **Khuyến nghị**: Dùng `upsert-one` với filter `{"customerId": "xxx"}` để sync customer từ cả 2 nguồn (đơn giản và chính xác nhất)
- `customerId` được extract từ `posData.id` (ưu tiên) hoặc `panCakeData.id`
- Có thể dùng filter `{"panCakeCustomerId": "xxx"}`, `{"posCustomerId": "xxx"}`, `{"psid": "xxx", "pageId": "yyy"}` nếu cần
- Unique index `customerId` (sparse) đảm bảo không duplicate customer theo ID chung
- Unique index `posCustomerId` (sparse) đảm bảo không duplicate customer theo POS Customer ID

---

### Pancake Integration Module

#### AccessTokenCreateInput
**Endpoint:** `POST /api/v1/access-token/insert-one`

```typescript
interface AccessTokenCreateInput {
  name: string;          // Tên token (required)
  describe: string;      // Mô tả token (required)
  system: string;        // Hệ thống (required, ví dụ: "Facebook", "Pancake")
  value: string;         // Giá trị token (required)
  assignedUsers?: string[]; // Danh sách user IDs được gán (optional)
}
```

#### AccessTokenUpdateInput
**Endpoint:** `PUT /api/v1/access-token/update-by-id/:id`

```typescript
interface AccessTokenUpdateInput {
  name?: string;          // Tên token (optional)
  describe?: string;      // Mô tả token (optional)
  system?: string;        // Hệ thống (optional)
  value?: string;         // Giá trị token (optional)
  assignedUsers?: string[]; // Danh sách user IDs (optional)
}
```

#### PcOrderCreateInput
**Endpoint:** `POST /api/v1/pancake/order/insert-one`

```typescript
interface PcOrderCreateInput {
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API (required)
}
```

---

### Query Parameters cho Find Operations

#### Filter Parameter
**Format:** Query string với JSON

```typescript
// Ví dụ: GET /api/v1/user/find?filter={"email":"user@example.com"}
const filter = {
  email: "user@example.com"
};

// MongoDB query syntax
const filter = {
  name: { $regex: "John", $options: "i" },
  createdAt: { $gte: 1234567890 }
};

// Nested fields
const filter = {
  "panCakeData.type": "INBOX"
};
```

#### Options Parameter
**Format:** Query string với JSON

```typescript
// Ví dụ: GET /api/v1/user/find?options={"sort":{"createdAt":-1},"limit":10,"skip":0}
const options = {
  sort: { createdAt: -1 },  // -1: giảm dần, 1: tăng dần
  limit: 10,                 // Số lượng tối đa
  skip: 0,                   // Số lượng bỏ qua
  projection: { name: 1, email: 1 } // Chỉ lấy các field này
};
```

#### Pagination Parameters
**Format:** Query string riêng lẻ

```typescript
// Ví dụ: GET /api/v1/user/find-with-pagination?page=1&limit=10&filter={"name":"John"}
// Query parameters:
page: number;    // Số trang (bắt đầu từ 1)
limit: number;   // Số lượng mỗi trang (mặc định: 10, tối đa: 100)
filter?: string; // JSON string của MongoDB filter (optional)
```

#### Find By IDs
**Endpoint:** `POST /api/v1/{collection}/find-by-ids`

```typescript
interface FindByIdsInput {
  ids: string[]; // Mảng các ObjectID (required)
}
```

#### Update One/Many
**Endpoint:** `PUT /api/v1/{collection}/update-one` hoặc `update-many`

```typescript
interface UpdateInput {
  filter: Record<string, any>; // MongoDB filter (required)
  update: Record<string, any>; // MongoDB update operation (required)
}

// Ví dụ:
{
  "filter": { "email": "user@example.com" },
  "update": { "$set": { "name": "New Name" } }
}
```

#### Delete One/Many
**Endpoint:** `DELETE /api/v1/{collection}/delete-one` hoặc `delete-many`

```typescript
interface DeleteInput {
  filter: Record<string, any>; // MongoDB filter (required)
}

// Ví dụ:
{
  "filter": { "email": "user@example.com" }
}
```

#### Upsert One/Many
**Endpoint:** `POST /api/v1/{collection}/upsert-one` hoặc `upsert-many`

```typescript
interface UpsertInput {
  filter: Record<string, any>; // MongoDB filter (required)
  update: Record<string, any>; // MongoDB update operation (required)
}

// Ví dụ:
{
  "filter": { "email": "user@example.com" },
  "update": { "$set": { "name": "New Name" } }
}
```

#### Insert Many
**Endpoint:** `POST /api/v1/{collection}/insert-many`

```typescript
interface InsertManyInput {
  items: any[]; // Mảng các documents cần tạo (required)
}

// Ví dụ:
{
  "items": [
    { "name": "User 1", "email": "user1@example.com" },
    { "name": "User 2", "email": "user2@example.com" }
  ]
}
```

---

### Path Parameters

#### Find By ID
**Endpoint:** `GET /api/v1/{collection}/find-by-id/:id`

- `id` (string, required): MongoDB ObjectID

#### Update By ID
**Endpoint:** `PUT /api/v1/{collection}/update-by-id/:id`

- `id` (string, required): MongoDB ObjectID

#### Delete By ID
**Endpoint:** `DELETE /api/v1/{collection}/delete-by-id/:id`

- `id` (string, required): MongoDB ObjectID

#### Special Endpoints

**Find By Page ID:**
- `GET /api/v1/facebook/page/find-by-page-id/:id`
- `id` (string, required): Facebook Page ID (không phải MongoDB ObjectID)

**Find By Post ID:**
- `GET /api/v1/facebook/post/find-by-post-id/:id`
- `id` (string, required): Facebook Post ID (không phải MongoDB ObjectID)

**Check In/Out Agent:**
- `POST /api/v1/agent/check-in/:id`
- `POST /api/v1/agent/check-out/:id`
- `id` (string, required): MongoDB ObjectID của Agent

**Set Administrator:**
- `POST /api/v1/admin/user/set-administrator/:id`
- `POST /api/v1/init/set-administrator/:id`
- `id` (string, required): MongoDB ObjectID của User

**Get Permissions By Category:**
- `GET /api/v1/permission/by-category/:category`
- `category` (string, required): Category name (ví dụ: "Auth", "Pancake")

**Get Permissions By Group:**
- `GET /api/v1/permission/by-group/:group`
- `group` (string, required): Group name (ví dụ: "User", "Role")

---

### 4. Admin Routes

#### Block User
```
POST /api/v1/admin/user/block
```
**Permission:** `User.Block`

**Request Body:**
```json
{
  "email": "user@example.com",
  "note": "Lý do khóa tài khoản"
}
```

#### Unblock User
```
POST /api/v1/admin/user/unblock
```
**Permission:** `User.Block`

**Request Body:**
```json
{
  "email": "user@example.com"
}
```

#### Set Role for User
```
POST /api/v1/admin/user/role
```
**Permission:** `User.SetRole`

**Request Body:**
```json
{
  "email": "user@example.com",
  "roleID": "role-id-objectid"
}
```

#### Set Administrator (Khi đã có admin)
```
POST /api/v1/admin/user/set-administrator/:id
```
**Permission:** `Init.SetAdmin`

**Path Parameter:**
- `id`: User ID cần set làm administrator

**Lưu ý:** Endpoint này chỉ dùng khi hệ thống đã có admin. Nếu chưa có admin, sử dụng `/init/set-administrator/:id`.

---

### 5. Init Routes (Chỉ hoạt động khi chưa có admin)

**⚠️ QUAN TRỌNG:** Tất cả init endpoints sẽ **tự động bị tắt** (404 Not Found) sau khi hệ thống đã có admin và server restart.

#### Kiểm Tra Trạng Thái Init
```
GET /api/v1/init/status
```
**Không cần authentication**

**Response:**
```json
{
  "code": 200,
  "data": {
    "organization": {
      "initialized": true,
      "error": ""
    },
    "permissions": {
      "initialized": true,
      "count": 50,
      "error": ""
    },
    "roles": {
      "initialized": true,
      "error": ""
    },
    "adminUsers": {
      "count": 1,
      "hasAdmin": true
    }
  },
  "status": "success"
}
```

#### Khởi Tạo Organization Root
```
POST /api/v1/init/organization
```
**Không cần authentication** (chỉ khi chưa có admin)

#### Khởi Tạo Permissions
```
POST /api/v1/init/permissions
```
**Không cần authentication** (chỉ khi chưa có admin)

#### Khởi Tạo Roles
```
POST /api/v1/init/roles
```
**Không cần authentication** (chỉ khi chưa có admin)

#### Khởi Tạo Admin User từ Firebase UID
```
POST /api/v1/init/admin-user
```
**Không cần authentication** (chỉ khi chưa có admin)

**Request Body:**
```json
{
  "firebaseUid": "firebase-user-uid"
}
```

#### Khởi Tạo Tất Cả (One-click Setup)
```
POST /api/v1/init/all
```
**Không cần authentication** (chỉ khi chưa có admin)

Khởi tạo Organization, Permissions, và Roles trong một lần gọi.

#### Set Administrator (Khi chưa có admin)
```
POST /api/v1/init/set-administrator/:id
```
**Không cần authentication** (chỉ khi chưa có admin)

**Path Parameter:**
- `id`: User ID cần set làm administrator

**Lưu ý:** 
- Endpoint này chỉ hoạt động khi hệ thống chưa có admin
- Nếu đã có admin, sẽ trả về 403 và hướng dẫn dùng `/admin/user/set-administrator/:id`

---

## 🔍 Query Examples

### Tìm User Theo Email
```
GET /api/v1/user/find-one?filter={"email":"user@example.com"}
```

### Tìm Users Có Phân Trang
```
GET /api/v1/user/find-with-pagination?page=1&limit=10&filter={"name":{"$regex":"John"}}
```

### Tìm Users Với Sort
```
GET /api/v1/user/find?filter={}&options={"sort":{"createdAt":-1},"limit":20}
```

### Cập Nhật User
```
PUT /api/v1/user/update-by-id/507f1f77bcf86cd799439011
Content-Type: application/json

{
  "name": "Tên mới"
}
```

### Xóa User
```
DELETE /api/v1/user/delete-by-id/507f1f77bcf86cd799439011
```

---

## ⚠️ Error Handling

### Common Error Codes

**Authentication Errors:**
- `AUTH_001` - Lỗi token (thiếu, không hợp lệ, hết hạn)
- `AUTH_002` - Lỗi thông tin đăng nhập
- `AUTH_003` - Lỗi quyền truy cập

**Validation Errors:**
- `VAL_001` - Lỗi dữ liệu đầu vào
- `VAL_002` - Lỗi định dạng dữ liệu

**Database Errors:**
- `DB` - Lỗi database chung
- `DB_001` - Lỗi kết nối database
- `DB_002` - Lỗi truy vấn database

**Business Logic Errors:**
- `BIZ_001` - Lỗi trạng thái nghiệp vụ
- `BIZ_002` - Lỗi thao tác nghiệp vụ

### Error Response Format
```json
{
  "code": "AUTH_001",
  "message": "Token không hợp lệ",
  "details": null,
  "status": "error"
}
```

---

## 📝 Validation Rules

### Firebase Login
- **idToken**: Required, Firebase ID Token từ Firebase Client SDK
- **Hwid**: Required, Hardware ID duy nhất cho mỗi thiết bị

### Common Validation
- Tất cả các trường có tag `validate:"required"` là bắt buộc
- Firebase ID Token phải hợp lệ và chưa hết hạn
- Hwid phải là string không rỗng

---

## 🎯 Frontend Implementation Guide

### 1. API Client Setup

```typescript
// apiClient.ts
const API_BASE_URL = 'http://localhost:8080/api/v1';

class ApiClient {
  private token: string | null = null;
  private hwid: string;

  constructor() {
    // Tạo hoặc lấy HWID từ localStorage
    this.hwid = this.getOrCreateHWID();
  }

  private getOrCreateHWID(): string {
    let hwid = localStorage.getItem('hwid');
    if (!hwid) {
      // Tạo HWID duy nhất (có thể dùng thư viện như device-uuid)
      hwid = this.generateHWID();
      localStorage.setItem('hwid', hwid);
    }
    return hwid;
  }

  private generateHWID(): string {
    // Sử dụng device fingerprint hoặc thư viện device-uuid
    // Ví dụ đơn giản:
    return `hwid_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  }

  setToken(token: string) {
    this.token = token;
    localStorage.setItem('auth_token', token);
  }

  getToken(): string | null {
    return this.token || localStorage.getItem('auth_token');
  }

  getHWID(): string {
    return this.hwid;
  }

  clearToken() {
    this.token = null;
    localStorage.removeItem('auth_token');
  }

  async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    const token = this.getToken();
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers,
    });

    const data = await response.json();

    if (!response.ok || data.status === 'error') {
      throw new ApiError(data.message, data.code, response.status);
    }

    return data;
  }

  // CRUD Methods
  async find<T>(collection: string, filter?: any, options?: any): Promise<T[]> {
    const params = new URLSearchParams();
    if (filter) params.append('filter', JSON.stringify(filter));
    if (options) params.append('options', JSON.stringify(options));
    
    const response = await this.request<{ data: T[] }>(
      `/${collection}/find?${params.toString()}`
    );
    return response.data;
  }

  async findOne<T>(collection: string, filter?: any): Promise<T> {
    const params = new URLSearchParams();
    if (filter) params.append('filter', JSON.stringify(filter));
    
    const response = await this.request<{ data: T }>(
      `/${collection}/find-one?${params.toString()}`
    );
    return response.data;
  }

  async findById<T>(collection: string, id: string): Promise<T> {
    const response = await this.request<{ data: T }>(
      `/${collection}/find-by-id/${id}`
    );
    return response.data;
  }

  async insertOne<T>(collection: string, data: any): Promise<T> {
    const response = await this.request<{ data: T }>(
      `/${collection}/insert-one`,
      {
        method: 'POST',
        body: JSON.stringify(data),
      }
    );
    return response.data;
  }

  async updateById<T>(
    collection: string,
    id: string,
    data: any
  ): Promise<T> {
    const response = await this.request<{ data: T }>(
      `/${collection}/update-by-id/${id}`,
      {
        method: 'PUT',
        body: JSON.stringify(data),
      }
    );
    return response.data;
  }

  async deleteById(collection: string, id: string): Promise<void> {
    await this.request(`/${collection}/delete-by-id/${id}`, {
      method: 'DELETE',
    });
  }

  async findWithPagination<T>(
    collection: string,
    page: number = 1,
    limit: number = 10,
    filter?: any
  ): Promise<PaginatedResponse<T>> {
    const params = new URLSearchParams({
      page: page.toString(),
      limit: limit.toString(),
    });
    if (filter) params.append('filter', JSON.stringify(filter));

    const response = await this.request<{ data: PaginatedResponse<T> }>(
      `/${collection}/find-with-pagination?${params.toString()}`
    );
    return response.data;
  }
}

// Types
interface ApiResponse<T> {
  code: number | string;
  message: string;
  data: T;
  status: 'success' | 'error';
}

interface PaginatedResponse<T> {
  page: number;
  limit: number;
  itemCount: number;
  items: T[];
}

class ApiError extends Error {
  constructor(
    message: string,
    public code: string,
    public statusCode: number
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export const apiClient = new ApiClient();
```

### 2. Auth Service

```typescript
// authService.ts
import { apiClient } from './apiClient';

export interface FirebaseLoginInput {
  idToken: string; // Firebase ID Token từ Firebase Client SDK
  hwid: string;
}

export interface User {
  id: string;
  firebaseUid: string;
  name: string;
  email?: string;
  emailVerified: boolean;
  phone?: string;
  phoneVerified: boolean;
  avatarUrl?: string;
  token?: string;
  createdAt: number;
  updatedAt: number;
}

class AuthService {
  /**
   * Đăng nhập bằng Firebase ID Token
   * @param idToken Firebase ID Token từ Firebase Client SDK
   */
  async loginWithFirebase(idToken: string): Promise<User> {
    const hwid = apiClient.getHWID();
    const response = await apiClient.request<{ data: User }>(
      '/auth/login/firebase',
      {
        method: 'POST',
        body: JSON.stringify({
          idToken,
          hwid,
        }),
      }
    );

    if (response.data.token) {
      apiClient.setToken(response.data.token);
    }

    return response.data;
  }

  async logout(): Promise<void> {
    const hwid = apiClient.getHWID();
    await apiClient.request('/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ hwid }),
    });
    apiClient.clearToken();
  }

  async getProfile(): Promise<User> {
    const response = await apiClient.request<{ data: User }>(
      '/auth/profile'
    );
    return response.data;
  }

  async updateProfile(name: string): Promise<User> {
    const response = await apiClient.request<{ data: User }>(
      '/auth/profile',
      {
        method: 'PUT',
        body: JSON.stringify({ name }),
      }
    );
    return response.data;
  }

  /**
   * Lưu ý: Email và phone được quản lý bởi Firebase
   * Để thay đổi email/phone, sử dụng Firebase Client SDK
   */

  async getUserRoles() {
    const response = await apiClient.request<{ data: any[] }>(
      '/auth/roles'
    );
    return response.data;
  }
}

export const authService = new AuthService();
```

### 3. User Management Service

```typescript
// userService.ts
import { apiClient } from './apiClient';

export interface User {
  id: string;
  name: string;
  email: string;
  createdAt: number;
  updatedAt: number;
}

class UserService {
  async findAll(filter?: any): Promise<User[]> {
    return apiClient.find<User>('user', filter);
  }

  async findOne(filter: any): Promise<User> {
    return apiClient.findOne<User>('user', filter);
  }

  async findById(id: string): Promise<User> {
    return apiClient.findById<User>('user', id);
  }

  async findWithPagination(
    page: number = 1,
    limit: number = 10,
    filter?: any
  ) {
    return apiClient.findWithPagination<User>('user', page, limit, filter);
  }
}

export const userService = new UserService();
```

### 4. Error Handling

```typescript
// errorHandler.ts
import { ApiError } from './apiClient';

export function handleApiError(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'AUTH_001':
        return 'Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.';
      case 'AUTH_002':
        return 'Thông tin đăng nhập không chính xác.';
      case 'AUTH_003':
        return 'Bạn không có quyền thực hiện thao tác này.';
      case 'VAL_001':
        return 'Dữ liệu không hợp lệ. Vui lòng kiểm tra lại.';
      case 'DB_002':
        return 'Không tìm thấy dữ liệu.';
      default:
        return error.message || 'Đã xảy ra lỗi. Vui lòng thử lại.';
    }
  }

  if (error instanceof Error) {
    return error.message;
  }

  return 'Đã xảy ra lỗi không xác định.';
}
```

---

## 🔑 Important Notes

1. **Firebase Authentication**: 
   - Sử dụng Firebase Client SDK để đăng nhập (Email, Phone OTP, Google, Facebook)
   - Lấy Firebase ID Token từ Firebase sau khi đăng nhập thành công
   - Gửi Firebase ID Token đến `/auth/login/firebase` để nhận JWT token của hệ thống
   - Lưu JWT token vào localStorage hoặc state management
   - Gửi JWT token trong header `Authorization: Bearer <token>` cho mọi request (trừ auth endpoints)

2. **HWID (Hardware ID)**:
   - Cần tạo và lưu trữ một hardware ID duy nhất cho mỗi thiết bị
   - Sử dụng khi login và logout
   - Có thể sử dụng thư viện như `device-uuid` hoặc tạo từ browser fingerprint

3. **Pagination**:
   - Sử dụng `find-with-pagination` cho danh sách lớn
   - Response có format: `{ page, limit, itemCount, items }`

4. **Filter & Options**:
   - Filter và options được truyền qua query string dưới dạng JSON
   - Sử dụng MongoDB query syntax cho filter
   - Options hỗ trợ: `sort`, `limit`, `skip`, `projection`

5. **Error Handling**:
   - Luôn kiểm tra `status === "error"` trong response
   - Hiển thị message từ response cho user
   - Xử lý 401 để redirect về login page

6. **Permissions**:
   - Mỗi endpoint yêu cầu permission cụ thể
   - Format: `<Module>.<Action>`
   - Nếu không có permission, sẽ nhận 403 Forbidden

7. **Organization & Roles**:
   - Roles phải thuộc về một Organization
   - Tên role phải unique trong mỗi Organization
   - Khi tạo role, bắt buộc phải có `organizationId`

8. **Agent Check-in**:
   - Agent cần check-in mỗi 5 phút để duy trì trạng thái online
   - Nếu không check-in sau 5 phút, hệ thống tự động chuyển về offline

---

## 📊 Tóm Tắt Collections Theo Mức Độ Cần Thiết

### ⭐⭐⭐⭐⭐ BẮT BUỘC (Core System)
- **User** - Quản lý người dùng
- **Permission** - Định nghĩa quyền
- **Role** - Định nghĩa vai trò
- **RolePermission** - Liên kết Role-Permission
- **UserRole** - Liên kết User-Role

### ⭐⭐⭐⭐ RẤT QUAN TRỌNG (Nếu cần phân quyền theo tổ chức)
- **Organization** - Cấu trúc tổ chức

### ⭐⭐⭐ TÙY CHỌN (Tích hợp và tự động hóa)
- **Agent** - Trợ lý tự động
- **AccessToken** - Quản lý tokens
- **FbPage** - Facebook Pages
- **FbConversation** - Facebook Conversations

### ⭐⭐ TÙY CHỌN (Chi tiết)
- **FbPost** - Facebook Posts
- **FbMessage** - Facebook Messages
- **PcOrder** - Pancake Orders

---

## 📚 Additional Resources

- Base URL: `http://localhost:8080/api/v1`
- Health Check: `GET /api/v1/system/health`
- All endpoints require authentication except:
  - `/auth/login/firebase`
  - `/init/status` (chỉ khi chưa có admin)
  - `/init/*` (chỉ khi chưa có admin, sẽ bị tắt sau khi có admin)
  - `/system/health`

---

**Tài liệu này cung cấp đầy đủ thông tin về ý nghĩa, tính năng và mức độ cần thiết của từng collection để phát triển frontend tích hợp với API server này.**

---

## 📋 Tóm Tắt Endpoints Đặc Biệt

### Facebook Integration

1. **FbPage:**
   - `GET /api/v1/facebook/page/find-by-page-id/:id` - Tìm page theo Facebook PageID
   - `PUT /api/v1/facebook/page/update-token` - Cập nhật Page Access Token

2. **FbPost:**
   - `GET /api/v1/facebook/post/find-by-post-id/:id` - Tìm post theo Facebook PostID

3. **FbConversation:**
   - `GET /api/v1/facebook/conversation/sort-by-api-update` - Lấy conversations sắp xếp theo thời gian cập nhật API

4. **FbMessage:**
   - `POST /api/v1/facebook/message/upsert-messages` - Upsert messages với logic tự động tách messages vào collection riêng

### RBAC Module

1. **Permission:**
   - `GET /api/v1/permission/by-category/:category` - Lấy permissions theo category
   - `GET /api/v1/permission/by-group/:group` - Lấy permissions theo group

2. **RolePermission:**
   - `PUT /api/v1/role-permission/update-role` - Cập nhật hàng loạt permissions của role

3. **UserRole:**
   - `PUT /api/v1/user-role/update-user-roles` - Cập nhật hàng loạt roles cho user

### Agent Module

1. **Agent:**
   - `POST /api/v1/agent/check-in/:id` - Check-in agent
   - `POST /api/v1/agent/check-out/:id` - Check-out agent

### Admin Operations

1. **User Management:**
   - `POST /api/v1/admin/user/block` - Chặn user
   - `POST /api/v1/admin/user/unblock` - Bỏ chặn user
   - `POST /api/v1/admin/user/role` - Thiết lập role cho user
   - `POST /api/v1/admin/user/set-administrator/:id` - Thiết lập administrator

---

## 🔧 Best Practices cho Frontend

### 1. Xử Lý Data Extraction

Khi làm việc với FbConversation và FbMessage:

```typescript
// ✅ ĐÚNG: Chỉ cần gửi panCakeData, hệ thống tự động extract
const createConversation = async (panCakeData: any) => {
  return await apiClient.insertOne('facebook/conversation', {
    pageId: 'page_123',
    pageUsername: 'my_page',
    panCakeData: panCakeData  // Hệ thống tự động extract conversationId, customerId, panCakeUpdatedAt
  });
};

// ❌ SAI: Không cần gửi các trường đã được extract tự động
const createConversationWrong = async (panCakeData: any) => {
  return await apiClient.insertOne('facebook/conversation', {
    pageId: 'page_123',
    pageUsername: 'my_page',
    conversationId: panCakeData.id,  // ❌ Không cần, sẽ được extract tự động
    panCakeData: panCakeData
  });
};
```

### 2. Sử Dụng sort-by-api-update

Khi cần đồng bộ conversations từ Pancake:

```typescript
// Lấy conversations cần đồng bộ (sắp xếp theo panCakeUpdatedAt cũ nhất)
const syncConversations = async (pageId?: string) => {
  const params = new URLSearchParams();
  params.append('page', '1');
  params.append('limit', '50');
  if (pageId) {
    params.append('pageId', pageId);
  }
  
  const response = await apiClient.request<PaginatedResponse<FbConversation>>(
    `/facebook/conversation/sort-by-api-update?${params.toString()}`
  );
  
  // Conversations được sắp xếp theo panCakeUpdatedAt giảm dần (cũ nhất trước)
  return response.data.items;
};
```

### 3. Error Handling cho Data Extraction

```typescript
try {
  const conversation = await createConversation(panCakeData);
} catch (error) {
  if (error.code === 'VAL_001') {
    // Có thể là lỗi do thiếu field required trong panCakeData
    console.error('Dữ liệu Pancake không hợp lệ:', error.details);
  }
}
```

---

**Tài liệu này cung cấp đầy đủ thông tin về ý nghĩa, tính năng và mức độ cần thiết của từng collection để phát triển frontend tích hợp với API server này.**
