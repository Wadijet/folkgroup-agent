# TypeScript Types & Interfaces

Tài liệu này chứa tất cả TypeScript interfaces và types được sử dụng trong frontend application tích hợp với FolkForm Auth Backend API.

## 📋 Mục Lục

- [API Response Types](#api-response-types)
- [Authentication Types](#authentication-types)
- [User Types](#user-types)
- [RBAC Types](#rbac-types)
- [Organization Types](#organization-types)
- [Agent Types](#agent-types)
- [Facebook Integration Types](#facebook-integration-types)
- [Pancake Integration Types](#pancake-integration-types)
- [Error Types](#error-types)
- [Utility Types](#utility-types)

---

## API Response Types

### ApiResponse<T>
Response format chuẩn của tất cả API endpoints.

```typescript
interface ApiResponse<T> {
  code: number | string;      // HTTP status code hoặc error code
  message: string;            // Thông báo
  data: T;                    // Dữ liệu trả về (generic)
  status: 'success' | 'error'; // Trạng thái
}
```

### PaginatedResponse<T>
Response format cho các API có phân trang.

```typescript
interface PaginatedResponse<T> {
  page: number;        // Trang hiện tại (bắt đầu từ 1)
  limit: number;        // Số items mỗi trang
  itemCount: number;   // Tổng số items
  items: T[];          // Danh sách items
}
```

### HealthCheckResponse
Response từ health check endpoint.

```typescript
interface HealthCheckResponse {
  status: 'healthy' | 'unhealthy';
  timestamp: string;    // ISO 8601 format
  services: {
    api: 'ok' | 'error';
    database: 'ok' | 'error';
  };
}
```

---

## Authentication Types

### FirebaseLoginInput
Input cho Firebase login endpoint.

```typescript
interface FirebaseLoginInput {
  idToken: string;  // Firebase ID Token từ Firebase Client SDK
  hwid: string;     // Hardware ID duy nhất cho mỗi thiết bị
}
```

### LogoutInput
Input cho logout endpoint.

```typescript
interface LogoutInput {
  hwid: string;  // Hardware ID
}
```

### UpdateProfileInput
Input cho update profile endpoint.

```typescript
interface UpdateProfileInput {
  name?: string;  // Tên mới (optional)
}
```

**Lưu ý:** Email và phone được quản lý bởi Firebase, không thể thay đổi qua API này.

---

## User Types

### User
Model chính cho User collection.

```typescript
interface User {
  id: string;                    // MongoDB ObjectID
  firebaseUid: string;           // Firebase User ID (unique)
  name: string;                 // Tên người dùng
  email?: string;                // Email (optional - có thể đăng nhập bằng phone)
  emailVerified: boolean;        // Email đã được verify chưa
  phone?: string;                // Số điện thoại (optional - có thể đăng nhập bằng email)
  phoneVerified: boolean;        // Phone đã được verify chưa
  avatarUrl?: string;            // URL avatar từ Firebase
  token: string;                 // JWT token hiện tại
  createdAt: number;            // Unix timestamp
  updatedAt: number;            // Unix timestamp
}
```

### UserWithRoles
User kèm theo danh sách roles (khi cần).

```typescript
interface UserWithRoles extends User {
  roles: Role[];  // Danh sách roles của user
}
```

---

## RBAC Types

### Permission
Model cho Permission collection.

```typescript
interface Permission {
  id: string;           // MongoDB ObjectID
  name: string;        // Format: "Module.Action" (ví dụ: "User.Read")
  describe: string;    // Mô tả quyền
  category: string;    // Category (Auth, Pancake, etc.)
  group: string;       // Group (User, Role, FbPage, etc.)
  createdAt: number;   // Unix timestamp
  updatedAt: number;  // Unix timestamp
}
```

**Permission Format:**
- Format: `<Module>.<Action>`
- Module: User, Role, Permission, Agent, FbPage, FbPost, etc.
- Action: Read, Insert, Update, Delete, Block, CheckIn, CheckOut, etc.

**Ví dụ:**
- `User.Read` - Đọc thông tin user
- `Role.Update` - Cập nhật role
- `Agent.CheckIn` - Check-in agent

### Role
Model cho Role collection.

```typescript
interface Role {
  id: string;              // MongoDB ObjectID
  name: string;           // Tên role (unique trong mỗi Organization)
  describe: string;       // Mô tả role
  organizationId: string; // BẮT BUỘC - Role thuộc Organization nào
  createdAt: number;     // Unix timestamp
  updatedAt: number;     // Unix timestamp
}
```

### RolePermission
Model cho RolePermission collection (liên kết Role-Permission).

```typescript
interface RolePermission {
  id: string;              // MongoDB ObjectID
  roleId: string;          // Reference to Role
  permissionId: string;    // Reference to Permission
  scope: number;           // Phạm vi áp dụng quyền: 0 = Chỉ tổ chức role thuộc về, 1 = Tổ chức đó và tất cả các tổ chức con
  createdByRoleId?: string; // ID của role tạo quyền này
  createdByUserId?: string; // ID của user tạo quyền này
  createdAt: number;       // Unix timestamp
  updatedAt: number;       // Unix timestamp
}
```

**Scope Values (Phạm vi áp dụng quyền):**
- **`0`** (Default): **Chỉ tổ chức role thuộc về**
  - Quyền chỉ áp dụng cho tổ chức mà role thuộc về
  - User với role này chỉ có thể thao tác trên dữ liệu của tổ chức đó
  - Không thể truy cập dữ liệu của các tổ chức con
  - **Ví dụ UI**: Hiển thị checkbox/radio "Chỉ tổ chức này" với tooltip "Quyền chỉ áp dụng cho tổ chức mà role thuộc về"
  
- **`1`**: **Tổ chức đó và tất cả các tổ chức con**
  - Quyền áp dụng cho tổ chức mà role thuộc về VÀ tất cả các tổ chức con
  - User với role này có thể thao tác trên dữ liệu của tổ chức đó và tất cả tổ chức con
  - **Ví dụ UI**: Hiển thị checkbox/radio "Tổ chức này và các tổ chức con" với tooltip "Quyền áp dụng cho tổ chức này và tất cả các tổ chức con (phòng ban, bộ phận, team)"
  - **Thường dùng cho**: Administrator role, Director, Manager cấp cao

**Lưu ý cho Frontend:**
- Scope mặc định là `0` - không cần set khi tạo mới
- UI nên có 2 options rõ ràng với tooltip giải thích
- Mặc định chọn scope = 0
- Scope chỉ ảnh hưởng đến phạm vi dữ liệu, không ảnh hưởng đến loại thao tác (Read/Insert/Update/Delete)

### UserRole
Model cho UserRole collection (liên kết User-Role).

```typescript
interface UserRole {
  id: string;        // MongoDB ObjectID
  userId: string;   // Reference to User
  roleId: string;   // Reference to Role
  createdAt: number; // Unix timestamp
  updatedAt: number; // Unix timestamp
}
```

### RoleWithPermissions
Role kèm theo danh sách permissions (khi cần).

```typescript
interface RoleWithPermissions extends Role {
  permissions: Permission[];  // Danh sách permissions của role
}
```

---

## Organization Types

### Organization
Model cho Organization collection.

```typescript
interface Organization {
  id: string;         // MongoDB ObjectID
  name: string;       // Tên organization
  code: string;       // Unique code
  type: OrganizationType; // Loại organization
  parentId?: string;  // ID của organization cha (null nếu là root)
  path: string;       // Đường dẫn cây (ví dụ: "/root_group/company1/dept1")
  level: number;      // Cấp độ (0 = root, 1, 2, ...)
  isActive: boolean;  // Trạng thái active
  createdAt: number; // Unix timestamp
  updatedAt: number; // Unix timestamp
}
```

### OrganizationType
Enum cho các loại organization.

```typescript
type OrganizationType = 
  | 'group'      // Tập đoàn
  | 'company'    // Công ty
  | 'department' // Phòng ban
  | 'division'   // Bộ phận
  | 'team';      // Team
```

### OrganizationWithChildren
Organization kèm theo danh sách children (khi cần).

```typescript
interface OrganizationWithChildren extends Organization {
  children: Organization[];  // Danh sách organization con
}
```

---

## Agent Types

### Agent
Model cho Agent collection.

```typescript
interface Agent {
  id: string;                // MongoDB ObjectID
  name: string;              // Tên agent
  describe: string;          // Mô tả agent
  status: AgentStatus;      // Trạng thái agent
  command: AgentCommand;    // Lệnh điều khiển
  assignedUsers: string[];  // Array of user IDs được gán cho agent
  configData: Record<string, any>; // Cấu hình agent (flexible)
  createdAt: number;        // Unix timestamp
  updatedAt: number;        // Unix timestamp
}
```

### AgentStatus
Enum cho trạng thái agent.

```typescript
type AgentStatus = 0 | 1;  // 0 = offline, 1 = online
```

### AgentCommand
Enum cho lệnh điều khiển agent.

```typescript
type AgentCommand = 0 | 1;  // 0 = stop, 1 = play
```

**Lưu ý:** Agent cần check-in thường xuyên (mỗi 5 phút) để duy trì trạng thái online. Nếu không check-in sau 5 phút, hệ thống tự động chuyển về offline.

---

## Facebook Integration Types

### AccessToken
Model cho AccessToken collection.

```typescript
interface AccessToken {
  id: string;              // MongoDB ObjectID
  name: string;           // Unique name
  describe: string;       // Mô tả
  system: string;         // Hệ thống (Facebook, Pancake, etc.)
  value: string;          // Token value
  assignedUsers: string[]; // Array of user IDs
  status: TokenStatus;   // Trạng thái token
  createdAt: number;     // Unix timestamp
  updatedAt: number;     // Unix timestamp
}
```

### TokenStatus
Enum cho trạng thái token.

```typescript
type TokenStatus = 0 | 1;  // 0 = active, 1 = inactive
```

### FbPage
Model cho FbPage collection.

```typescript
interface FbPage {
  id: string;                    // MongoDB ObjectID
  pageName: string;             // Tên Facebook Page
  pageUsername: string;         // Username của Page
  pageId: string;               // Facebook Page ID (unique)
  isSync: boolean;              // Trạng thái đồng bộ
  accessToken: string;          // Access token
  pageAccessToken: string;      // Page Access Token
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API
  createdAt: number;           // Unix timestamp
  updatedAt: number;          // Unix timestamp
}
```

### FbPost
Model cho FbPost collection.

```typescript
interface FbPost {
  id: string;                    // MongoDB ObjectID
  pageId: string;               // Reference to FbPage
  postId: string;                // Facebook Post ID (unique)
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API
  createdAt: number;           // Unix timestamp
  updatedAt: number;          // Unix timestamp
}
```

### FbConversation
Model cho FbConversation collection.

```typescript
interface FbConversation {
  id: string;                    // MongoDB ObjectID
  pageId: string;               // Reference to FbPage
  pageUsername: string;         // Username của Page
  conversationId: string;       // Facebook Conversation ID (unique)
  customerId: string;           // Facebook Customer ID
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API
  panCakeUpdatedAt: number;     // Thời gian cập nhật từ Pancake API
  createdAt: number;           // Unix timestamp
  updatedAt: number;          // Unix timestamp
}
```

### FbMessage
Model cho FbMessage collection.

```typescript
interface FbMessage {
  id: string;                    // MongoDB ObjectID
  pageId: string;               // Reference to FbPage
  pageUsername: string;         // Username của Page
  conversationId: string;       // Reference to FbConversation
  customerId: string;           // Facebook Customer ID
  panCakeData: Record<string, any>; // Dữ liệu từ Pancake API
  createdAt: number;           // Unix timestamp
  updatedAt: number;          // Unix timestamp
}
```

---

## Pancake Integration Types

### PcOrder
Model cho PcOrder collection.

```typescript
interface PcOrder {
  id: string;                    // MongoDB ObjectID
  pancakeOrderId: string;        // Pancake Order ID (unique)
  status: OrderStatus;          // Trạng thái đơn hàng
  panCakeData: Record<string, any>; // Full data from Pancake API
  createdAt: number;           // Unix timestamp
  updatedAt: number;          // Unix timestamp
}
```

### OrderStatus
Enum cho trạng thái đơn hàng.

```typescript
type OrderStatus = 0 | 1;  // 0 = active, 1 = inactive
```

---

## Error Types

### ApiError
Custom error class cho API errors.

```typescript
class ApiError extends Error {
  constructor(
    message: string,
    public code: string,        // Error code (ví dụ: "AUTH_001")
    public statusCode: number   // HTTP status code
  ) {
    super(message);
    this.name = 'ApiError';
  }
}
```

### ErrorResponse
Format response khi có lỗi.

```typescript
interface ErrorResponse {
  code: string;      // Error code
  message: string;   // Thông báo lỗi
  details?: any;     // Chi tiết lỗi (nếu có)
  status: 'error';  // Luôn là 'error'
}
```

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

---

## Utility Types

### FilterOptions
Options cho filter queries.

```typescript
interface FilterOptions {
  sort?: Record<string, 1 | -1>;  // Sort: { field: 1 (asc) | -1 (desc) }
  limit?: number;                  // Số items tối đa
  skip?: number;                   // Số items bỏ qua
  projection?: Record<string, 0 | 1>; // Chọn fields: { field: 1 (include) | 0 (exclude) }
}
```

### MongoDBFilter
Filter query theo MongoDB syntax.

```typescript
type MongoDBFilter = Record<string, any>;
```

**Ví dụ:**
```typescript
const filter: MongoDBFilter = {
  email: "user@example.com",
  name: { $regex: "John" },
  createdAt: { $gte: 1609459200 }
};
```

### PaginationParams
Parameters cho pagination.

```typescript
interface PaginationParams {
  page: number;    // Trang hiện tại (bắt đầu từ 1)
  limit: number;    // Số items mỗi trang
  filter?: MongoDBFilter; // Filter query (optional)
}
```

---

## 📝 Sử Dụng

### Import Types

```typescript
// Import tất cả types
import type {
  User,
  Role,
  Permission,
  Organization,
  Agent,
  FbPage,
  PcOrder,
  ApiResponse,
  PaginatedResponse,
  ApiError
} from './types';

// Hoặc import từng file riêng
import type { User, UserWithRoles } from './types/user';
import type { Role, RolePermission } from './types/rbac';
```

### Type Guards

```typescript
// Kiểm tra ApiError
function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

// Kiểm tra response thành công
function isSuccessResponse<T>(
  response: ApiResponse<T>
): response is ApiResponse<T> & { status: 'success' } {
  return response.status === 'success';
}
```

### Type Assertions

```typescript
// Assert response type
const userResponse = await apiClient.request<{ data: User }>('/auth/profile');
const user: User = userResponse.data;

// Assert paginated response
const usersResponse = await apiClient.findWithPagination<User>('user', 1, 10);
const users: User[] = usersResponse.items;
```

---

**Lưu ý:** Tất cả timestamps sử dụng Unix timestamp (number), không phải Date object. Cần convert khi hiển thị:

```typescript
const date = new Date(user.createdAt * 1000); // Convert Unix timestamp to Date
```

---

**Cập nhật lần cuối**: 2025-12-10

