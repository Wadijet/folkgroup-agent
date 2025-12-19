# Pancake API - Tài liệu AI Context

## 📚 Liên kết tham khảo

- **Tài liệu chính thức:** https://developer.pancake.biz/
- **Overview:** https://developer.pancake.biz/#/
- **Schemas:** https://developer.pancake.biz/#/schemas

### Links theo từng mục:

#### Pages
- List Pages: https://developer.pancake.biz/#/paths/pages/get
- Generate Page Access Token: https://developer.pancake.biz/#/paths/pages-page_id--generate_page_access_token/post

#### Conversations
- List Conversations: https://developer.pancake.biz/#/paths/pages-page_id--conversations/get
- Tag Conversation: https://developer.pancake.biz/#/paths/pages-page_id--conversations-conversation_id--tags/post
- Assign Conversation: https://developer.pancake.biz/#/paths/pages-page_id--conversations-conversation_id--assign/post
- Mark as Read: https://developer.pancake.biz/#/paths/pages-page_id--conversations-conversation_id--read/post
- Mark as Unread: https://developer.pancake.biz/#/paths/pages-page_id--conversations-conversation_id--unread/post

#### Messages
- Get Messages: https://developer.pancake.biz/#/paths/pages-page_id--conversations-conversation_id--messages/get
- Send Message: https://developer.pancake.biz/#/paths/pages-page_id--conversations-conversation_id--messages/post

#### Statistics
- Ads Campaign Statistics: https://developer.pancake.biz/#/paths/pages-page_id--statistics-pages_campaign/get
- Ads Statistics: https://developer.pancake.biz/#/paths/pages-page_id--statistics-ads/get
- Customer Engagement Statistics: https://developer.pancake.biz/#/paths/pages-page_id--statistics-customer_engagements/get
- Page Statistics: https://developer.pancake.biz/#/paths/pages-page_id--statistics-pages/get
- Tag Statistics: https://developer.pancake.biz/#/paths/pages-page_id--statistics-tags/get
- User Statistics: https://developer.pancake.biz/#/paths/pages-page_id--statistics-users/get

#### Customers
- Get Page Customers: https://developer.pancake.biz/#/paths/pages-page_id--page_customers/get
- Update Customer: https://developer.pancake.biz/#/paths/pages-page_id--page_customers-page_customer_id/put
- Add Customer Note: https://developer.pancake.biz/#/paths/pages-page_id--page_customers-page_customer_id--notes/post
- Update Customer Note: https://developer.pancake.biz/#/paths/pages-page_id--page_customers-page_customer_id--notes/put
- Delete Customer Note: https://developer.pancake.biz/#/paths/pages-page_id--page_customers-page_customer_id--notes/delete

#### Export Data
- Export Conversations from Ads: https://developer.pancake.biz/#/paths/pages-page_id--export_data/get

#### Call Logs
- Retrieve Call Logs: https://developer.pancake.biz/#/paths/pages-page_id--sip_call_logs/get

#### Tags
- Get List Tags: https://developer.pancake.biz/#/paths/pages-page_id--tags/get

#### Posts
- Get Posts: https://developer.pancake.biz/#/paths/pages-page_id--posts/get

#### Users
- Get List of Users: https://developer.pancake.biz/#/paths/pages-page_id--users/get
- Update Round Robin Users: https://developer.pancake.biz/#/paths/pages-page_id--round_robin_users/post

#### Page's Contents
- Upload Media Content: https://developer.pancake.biz/#/paths/pages-page_id--upload_contents/post

---

## 📑 Mục lục

1. [Tổng quan](#tổng-quan)
2. [Base URLs](#base-urls)
3. [Xác thực (Authentication)](#xác-thực-authentication)
4. [Cấu trúc Endpoints](#cấu-trúc-endpoints)
   - [1. Pages (Quản lý Trang)](#1-pages-quản-lý-trang)
   - [2. Conversations (Cuộc hội thoại)](#2-conversations-cuộc-hội-thoại)
   - [3. Messages (Tin nhắn)](#3-messages-tin-nhắn)
   - [4. Statistics (Thống kê)](#4-statistics-thống-kê)
   - [5. Customers (Khách hàng)](#5-customers-khách-hàng)
   - [6. Export Data (Xuất dữ liệu)](#6-export-data-xuất-dữ-liệu)
   - [7. Call Logs (Nhật ký cuộc gọi)](#7-call-logs-nhật-ký-cuộc-gọi)
   - [8. Tags (Thẻ)](#8-tags-thẻ)
   - [9. Posts (Bài đăng)](#9-posts-bài-đăng)
   - [10. Users (Người dùng)](#10-users-người-dùng)
   - [11. Page's Contents (Nội dung Trang)](#11-pages-contents-nội-dung-trang)
5. [Data Schemas](#data-schemas)
6. [Các loại dữ liệu quan trọng](#các-loại-dữ-liệu-quan-trọng)
7. [Workflow và Best Practices](#workflow-và-best-practices)

---

## Tổng quan

Pancake API là một hệ thống API RESTful cho phép truy xuất dữ liệu trang, tạo access token và quản lý các cuộc hội thoại trên nền tảng Pancake. API này được thiết kế để tích hợp với các hệ thống quản lý trang Facebook và các nền tảng mạng xã hội khác.

**Phiên bản API:** v1.0.0

**Tài liệu gốc:** https://developer.pancake.biz/

## Base URLs

API Pancake có 3 base URL khác nhau tùy theo loại API:

1. **User's API:** `https://pages.fm/api/v1`
   - Sử dụng cho các API liên quan đến người dùng và quản lý tài khoản

2. **Page's API v1:** `https://pages.fm/api/public_api/v1`
   - Phiên bản 1 của API công khai cho trang

3. **Page's API v2:** `https://pages.fm/api/public_api/v2`
   - Phiên bản 2 của API công khai cho trang (phiên bản mới nhất)

## Xác thực (Authentication)

API sử dụng **API Key** để xác thực. Có hai loại token:

### 1. User Access Token (`access_token`)
- Token của người dùng Pancake
- Sử dụng để xác thực các API của User's API
- Được truyền qua query parameter `access_token`

### 2. Page Access Token (`page_access_token`)
- Token dành riêng cho từng trang (Page)
- Được tạo từ User Access Token của admin trang
- Sử dụng để xác thực các API công khai của trang
- Token này không hết hạn trừ khi bị xóa thủ công hoặc được làm mới
- Được truyền qua query parameter `page_access_token`

**Lưu ý:** Admin của trang có thể lấy token này từ giao diện Pancake: Page's settings → Tools

## Cấu trúc Endpoints

### 1. Pages (Quản lý Trang)

#### 1.1. List Pages
**GET** `/pages`

Lấy danh sách các trang của tài khoản đã xác thực.

**Request:**
- **Query Parameters:**
  - `access_token` (string, required): Pancake user access token để xác thực

**Response 200:**
```json
{
  "pages": [
    {
      "id": "string",
      "platform": "string",
      "name": "string",
      "avatar_url": "http://example.com"
    }
  ]
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/v1/pages?access_token=YOUR_ACCESS_TOKEN' \
  --header 'Accept: application/json'
```

#### 1.2. Generate Page Access Token
**POST** `/pages/{page_id}/generate_page_access_token`

Tạo hoặc làm mới `page_access_token` bằng `access_token` của admin trang.

**Mô tả:** Page Access Token được sử dụng để xác thực các API công khai thay mặt cho một Trang. Token này không hết hạn trừ khi bị xóa thủ công hoặc được làm mới. Khi gọi API, cần bao gồm token này trong query parameter có tên `page_access_token`.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang khách hàng

- **Query Parameters:**
  - `access_token` (string, required): Pancake user access token với quyền admin của trang

**Response 200:**
- Token được tạo thành công

**Ví dụ cURL:**
```bash
curl --request POST \
  --url 'https://pages.fm/api/v1/pages/{page_id}/generate_page_access_token?access_token=YOUR_ACCESS_TOKEN' \
  --header 'Content-Type: application/json'
```

### 2. Conversations (Cuộc hội thoại)

#### 2.1. List Conversations
**GET** `/pages/{page_id}/conversations`

Lấy danh sách 60 cuộc hội thoại mới nhất. Sử dụng tham số `last_conversation_id` để lấy thêm cuộc hội thoại (pagination).

**Lưu ý:** Endpoint này sử dụng API v2 (`/api/public_api/v2`)

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID duy nhất của trang khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Access token của trang để xác thực
  - `last_conversation_id` (string, optional): ID của cuộc hội thoại cuối cùng từ lần gọi trước. Nếu không cung cấp, hệ thống trả về 60 cuộc hội thoại được cập nhật gần nhất. Nếu cung cấp ID của cuộc hội thoại cuối cùng từ lần gọi trước, hệ thống trả về 60 cuộc hội thoại cũ hơn tiếp theo.
  - `order_by` (string, optional): Sắp xếp theo thời gian chèn hoặc cập nhật. Giá trị cho phép: `inserted_at`, `updated_at`
  - `post_ids` (array[string], optional): Lọc theo post IDs (cho các cuộc hội thoại dựa trên comment)
  - `since` (integer, optional): Lọc từ một timestamp cụ thể (tính bằng giây)
  - `tags` (string, optional): Lọc cuộc hội thoại theo tag IDs (phân cách bằng dấu phẩy)
  - `type` (array[string], optional): Lọc theo loại cuộc hội thoại (ví dụ: INBOX, COMMENT)
  - `unread_first` (boolean, optional): Ưu tiên các cuộc hội thoại chưa đọc
  - `until` (integer, optional): Lọc đến một timestamp cụ thể (tính bằng giây)

**Response 200:**
```json
{
  "conversations": [
    {
      "id": "string",
      "type": "INBOX",
      "page_uid": "string",
      "updated_at": "2019-08-24T14:15:22Z",
      "inserted_at": "2019-08-24T14:15:22Z",
      "tags": ["string"],
      "last_message": {
        "text": "string",
        "sender": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "participants": [
        {
          "name": "string",
          "email": "string",
          "phone": "string"
        }
      ]
    }
  ]
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/public_api/v2/pages/{page_id}/conversations?page_access_token=YOUR_PAGE_ACCESS_TOKEN&last_conversation_id=conv_123&order_by=updated_at&type[]=INBOX&unread_first=true' \
  --header 'Accept: application/json'
```

#### 2.2. Conversation's Tag
**POST** `/pages/{page_id}/conversations/{conversation_id}/tags`

Gán tag cho cuộc hội thoại.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `conversation_id` (string, required): ID của cuộc hội thoại

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

#### 2.3. Assign Conversation
**POST** `/pages/{page_id}/conversations/{conversation_id}/assign`

Gán cuộc hội thoại cho một người dùng cụ thể.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `conversation_id` (string, required): ID của cuộc hội thoại

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

#### 2.4. Mark Conversation as Read
**POST** `/pages/{page_id}/conversations/{conversation_id}/read`

Đánh dấu cuộc hội thoại là đã đọc.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `conversation_id` (string, required): ID của cuộc hội thoại

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

#### 2.5. Mark Conversation as Unread
**POST** `/pages/{page_id}/conversations/{conversation_id}/unread`

Đánh dấu cuộc hội thoại là chưa đọc.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `conversation_id` (string, required): ID của cuộc hội thoại

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

### 3. Messages (Tin nhắn)

#### 3.1. Get Messages
**GET** `/pages/{page_id}/conversations/{conversation_id}/messages`

Lấy danh sách tin nhắn trong một cuộc hội thoại. Sử dụng tham số `current_count` để lấy thêm tin nhắn (pagination).

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang để lấy dữ liệu
  - `conversation_id` (string, required): ID của cuộc hội thoại để lấy tin nhắn

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token để xác thực
  - `current_count` (number, optional): Vị trí index để lấy tin nhắn. Trả về 30 tin nhắn trước index này.

**Response 200:**
```json
{
  "messages": [
    {
      "conversation_id": "string",
      "from": {
        "email": "string",
        "id": "string",
        "name": "string"
      },
      "has_phone": true,
      "inserted_at": "string",
      "is_hidden": true,
      "is_removed": true,
      "message": "string",
      "page_id": "string",
      "type": "string"
    }
  ]
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/conversations/{conversation_id}/messages?page_access_token=YOUR_PAGE_ACCESS_TOKEN&current_count=30' \
  --header 'Accept: application/json'
```

#### 3.2. Send a Message
**POST** `/pages/{page_id}/conversations/{conversation_id}/messages`

Gửi tin nhắn (private reply, inbox message, hoặc comment reply).

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `conversation_id` (string, required): ID của cuộc hội thoại

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

- **Body:** Một trong các loại sau:
  - `PrivateReply` - Để gửi private reply
  - `InboxMessage` - Để gửi inbox message
  - `ReplyComment` - Để reply comment

**Response 200:**
- Tin nhắn được gửi thành công

**Ví dụ cURL:**
```bash
curl --request POST \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/conversations/{conversation_id}/messages?page_access_token=YOUR_PAGE_ACCESS_TOKEN' \
  --header 'Content-Type: application/json' \
  --data '{
    "action": "reply_inbox",
    "message": "Xin chào, cảm ơn bạn đã liên hệ!"
  }'
```

### 4. Statistics (Thống kê)

Các endpoint để lấy thống kê và báo cáo cho trang.

#### 4.1. Ads Campaign Statistics
**GET** `/pages/{page_id}/statistics/pages_campaign`

Lấy thống kê về chiến dịch quảng cáo của trang.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

#### 4.2. Ads Statistics
**GET** `/pages/{page_id}/statistics/ads`

Lấy thống kê về quảng cáo của trang.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

#### 4.3. Customer Engagement Statistics
**GET** `/pages/{page_id}/statistics/customer_engagements`

Lấy thống kê về tương tác khách hàng.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

#### 4.4. Page Statistics
**GET** `/pages/{page_id}/statistics/pages`

Lấy thống kê tổng quan về trang.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

#### 4.5. Tag Statistics
**GET** `/pages/{page_id}/statistics/tags`

Lấy thống kê về tags.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

#### 4.6. User Statistics
**GET** `/pages/{page_id}/statistics/users`

Lấy thống kê về người dùng.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

### 5. Customers (Khách hàng)

#### 5.1. Get Page Customers Information
**GET** `/pages/{page_id}/page_customers`

Lấy thông tin về các khách hàng của trang trong một khoảng thời gian cụ thể. Hỗ trợ pagination và sắp xếp.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Access token của trang (có thể tạo trong page settings)
  - `page_number` (integer, required): Số trang hiện tại (tối thiểu là 1)
  - `page_size` (integer, optional): Kích thước mỗi trang (tối đa 100). Mặc định không giới hạn
  - `since` (integer<int64>, required): Thời gian bắt đầu (UNIX timestamp, UTC+0)
  - `until` (integer<int64>, required): Thời gian kết thúc (UNIX timestamp, UTC+0)
  - `order_by` (string, optional): Sắp xếp theo thứ tự giảm dần. Giá trị cho phép: `inserted_at`, `updated_at`. Mặc định: `inserted_at`

**Response 200:**
```json
{
  "total": 500,
  "customers": [
    {
      "birthday": "2019-08-24",
      "gender": "string",
      "inserted_at": "2019-08-24T14:15:22Z",
      "lives_in": "string",
      "name": "string",
      "phone_numbers": ["string"],
      "psid": "string",
      "notes": [
        {
          "created_at": -9007199254740991,
          "created_by": {
            "fb_id": "string",
            "fb_name": "string",
            "uid": "string"
          },
          "edit_history": [{}],
          "id": "string",
          "images": ["string"],
          "links": ["string"],
          "message": "string",
          "order_id": "string",
          "removed_at": -9007199254740991,
          "updated_at": -9007199254740991
        }
      ]
    }
  ],
  "success": true
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/page_customers?page_access_token=YOUR_PAGE_ACCESS_TOKEN&page_number=1&page_size=100&since=1672531200&until=1675219599&order_by=inserted_at' \
  --header 'Accept: application/json'
```

#### 5.2. Update Customer Information
**PUT** `/pages/{page_id}/page_customers/{page_customer_id}`

Cập nhật thông tin của một khách hàng.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `page_customer_id` (string, required): ID của khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

- **Body:** Thông tin khách hàng cần cập nhật

**Response 200:**
- Cập nhật thành công

#### 5.3. Add a New Customer Note
**POST** `/pages/{page_id}/page_customers/{page_customer_id}/notes`

Thêm ghi chú mới cho khách hàng.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `page_customer_id` (string, required): ID của khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

- **Body:** Nội dung ghi chú

**Response 200:**
- Ghi chú được thêm thành công

#### 5.4. Update a Customer Note
**PUT** `/pages/{page_id}/page_customers/{page_customer_id}/notes`

Cập nhật một ghi chú của khách hàng.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `page_customer_id` (string, required): ID của khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

- **Body:** Nội dung ghi chú cần cập nhật

**Response 200:**
- Cập nhật thành công

#### 5.5. Delete a Customer Note
**DELETE** `/pages/{page_id}/page_customers/{page_customer_id}/notes`

Xóa một ghi chú của khách hàng.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang
  - `page_customer_id` (string, required): ID của khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

**Response 200:**
- Xóa thành công

### 6. Export Data (Xuất dữ liệu)

#### 6.1. Export Conversations from Ads
**GET** `/pages/{page_id}/export_data`

Xuất các cuộc hội thoại đến từ quảng cáo trong một khoảng thời gian cụ thể. Mỗi request trả về tối đa 60 cuộc hội thoại bắt đầu từ offset đã cho.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Access token của trang (có thể lấy từ page settings)
  - `action` (string, required): Loại action, phải là `conversations_from_ads`
  - `since` (integer<int64>, required): Thời gian bắt đầu (UNIX timestamp, UTC+0)
  - `until` (integer<int64>, required): Thời gian kết thúc (UNIX timestamp, UTC+0)
  - `offset` (integer, optional): Offset cho pagination. Mặc định là 0. Mỗi lần gọi trả về tối đa 60 records.

**Response 200:**
```json
{
  "data": [
    {
      "id": "string",
      "tags": ["string"],
      "from": {
        "email": "user@example.com",
        "id": "string",
        "name": "string"
      },
      "inserted_at": "2019-08-24T14:15:22Z",
      "updated_at": "2019-08-24T14:15:22Z",
      "customers": [
        {
          "fb_id": "string",
          "id": "string",
          "name": "string"
        }
      ],
      "recent_phone_numbers": ["string"],
      "recent_seen_users": [
        {
          "fb_id": "string",
          "fb_name": "string",
          "seen_at": "2019-08-24T14:15:22Z"
        }
      ],
      "thread_key": "string",
      "psid": "string",
      "ad_clicks": ["string"],
      "is_banned": true,
      "assignees": [
        {
          "id": "string",
          "name": "string"
        }
      ]
    }
  ],
  "success": true
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/export_data?action=conversations_from_ads&page_access_token=YOUR_PAGE_ACCESS_TOKEN&since=1672531200&until=1675219599&offset=0' \
  --header 'Accept: application/json'
```

### 7. Call Logs (Nhật ký cuộc gọi)

#### 7.1. Retrieve Call Logs
**GET** `/pages/{page_id}/sip_call_logs`

Lấy danh sách lịch sử cuộc gọi cho một trang cụ thể. Yêu cầu page ID và access token.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token (có thể tạo trong page settings)
  - `id` (string, required): ID của gói SIP
  - `page_number` (integer, required): Số trang hiện tại (tối thiểu là 1)
  - `page_size` (integer, required): Số bản ghi mỗi trang (tối đa là 30)
  - `since` (integer<int64>, optional): Thời gian bắt đầu (Unix timestamp, giây, UTC+0)
  - `until` (integer<int64>, optional): Thời gian kết thúc (Unix timestamp, giây, UTC+0)

**Response 200:**
```json
{
  "data": [
    {
      "call_id": "string",
      "caller": "string",
      "callee": "string",
      "start_time": "2019-08-24T14:15:22Z",
      "duration": 0,
      "status": "string"
    }
  ],
  "success": true
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/sip_call_logs?page_access_token=YOUR_PAGE_ACCESS_TOKEN&id=SIP_PACKAGE_ID&page_number=1&page_size=30&since=1672531200&until=1675219599' \
  --header 'Accept: application/json'
```

### 8. Tags (Thẻ)

#### 8.1. Get List Tags
**GET** `/pages/{page_id}/tags`

Lấy danh sách các tag của một trang cụ thể.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token (có thể tạo trong page settings)

**Response 200:**
```json
{
  "tags": [
    {
      "id": 0,
      "text": "Kiểm hàng",
      "color": "#4b5577",
      "lighten_color": "#c9ccd6"
    }
  ]
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/tags?page_access_token=YOUR_PAGE_ACCESS_TOKEN' \
  --header 'Accept: application/json'
```

### 9. Posts (Bài đăng)

#### 9.1. Get Posts
**GET** `/pages/{page_id}/posts`

Lấy danh sách các bài đăng của một trang cụ thể.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token (có thể tạo trong page settings)
  - `page_number` (integer, required): Số trang hiện tại (tối thiểu là 1)
  - `page_size` (integer, required): Kích thước trang (tối đa 30)
  - `since` (integer, required): Thời gian bắt đầu (Unix timestamp tính bằng giây, UTC+0)
  - `until` (integer, required): Thời gian kết thúc (Unix timestamp tính bằng giây, UTC+0)
  - `type` (string, optional): Lọc bài đăng theo loại. Giá trị cho phép: `video`, `photo`, `text`, `livestream`

**Response 200:**
```json
{
  "success": true,
  "total": 200,
  "posts": [
    {
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
        "angry_count": 1,
        "care_count": 2,
        "haha_count": 1,
        "like_count": 111,
        "love_count": 14,
        "sad_count": 12,
        "wow_count": 17
      },
      "phone_number_count": 0
    }
  ]
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/posts?page_access_token=YOUR_PAGE_ACCESS_TOKEN&page_number=1&page_size=30&since=1672531200&until=1675219599&type=video' \
  --header 'Accept: application/json'
```

### 10. Users (Người dùng)

#### 10.1. Get List of Users
**GET** `/pages/{page_id}/users`

Lấy danh sách người dùng của một trang cụ thể.

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang khách hàng

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token (có thể tạo trong page settings)

**Response 200:**
```json
{
  "success": true,
  "users": [
    {
      "id": "c4bafd84-7b96-4f28-b59a-031f17c32ddf",
      "name": "Anh Ngoc Nguyen",
      "status": "available",
      "fb_id": "116256249766099",
      "page_permissions": {
        "permissions": [100, 71, 81]
      },
      "status_in_page": "active",
      "is_online": false
    }
  ],
  "disabled_users": [
    {
      "id": "69586d78-dd37-4d25-ad2b-0716697b1c34",
      "name": "Khanh khanh",
      "fb_id": "1736243166628197"
    }
  ],
  "round_robin_users": {
    "comment": ["79d4e769-ac31-4821-8304-d6e251d532e9"],
    "inbox": ["fb5ff8ed-434b-4d4b-a213-b595b242b81a"]
  }
}
```

**Ví dụ cURL:**
```bash
curl --request GET \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/users?page_access_token=YOUR_PAGE_ACCESS_TOKEN' \
  --header 'Accept: application/json'
```

#### 10.2. Update Round Robin Users
**POST** `/pages/{page_id}/round_robin_users`

Cập nhật danh sách người dùng cho round robin (phân phối tự động cuộc hội thoại).

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của trang

- **Query Parameters:**
  - `page_access_token` (string, required): Page access token

- **Body:** Danh sách user IDs cho round robin

**Response 200:**
- Cập nhật thành công

### 11. Page's Contents (Nội dung Trang)

#### 11.1. Upload Media Content
**POST** `/pages/{page_id}/upload_contents`

Upload file (ví dụ: hình ảnh, video) lên một Trang. Request phải bao gồm `page_access_token` hợp lệ trong query string.

**Giới hạn kích thước video:**
- **Shopee**: tối đa 30MB
- **Whatsapp Official**: tối đa 16MB
- **Lazada**: tối đa 100MB
- **Khác**: tối đa 25MB

**Request:**
- **Path Parameters:**
  - `page_id` (string, required): ID của Trang để upload nội dung

- **Query Parameters:**
  - `page_access_token` (string, required): Page Access Token lấy từ Settings → Tools

- **Body (multipart/form-data):**
  - `file` (binary, required): File cần upload

**Response 200:**
```json
{
  "id": "HXrxioWFIc5DFwffhmOVHspLuMwpWCXfWDoBxiov6DLa3MvakLeGpLQAly7oHDvZT66VEhnYG4zQEi2MhEzhlg",
  "attachment_type": "PHOTO",
  "success": true
}
```

**Lưu ý:** 
- `id` trả về là `content_id` được sử dụng trong `InboxMessage` schema khi gửi tin nhắn có attachment
- Các loại attachment được hỗ trợ: PHOTO, VIDEO, DOCUMENT, AUDIO_ATTACHMENT_ID

**Ví dụ cURL:**
```bash
curl --request POST \
  --url 'https://pages.fm/api/public_api/v1/pages/{page_id}/upload_contents?page_access_token=YOUR_PAGE_ACCESS_TOKEN' \
  --header 'Accept: application/json' \
  --header 'Content-Type: multipart/form-data' \
  --form 'file=@/path/to/file.jpg'
```

## Các loại dữ liệu quan trọng

### Conversation Types (Loại cuộc hội thoại)

- **INBOX**: Cuộc hội thoại trong hộp thư đến (inbox messages)
- **COMMENT**: Cuộc hội thoại từ comment trên bài đăng
- **LIVESTREAM**: Cuộc hội thoại từ livestream

### Message Types (Loại tin nhắn)

- **text**: Tin nhắn văn bản
- **image**: Tin nhắn hình ảnh
- **system**: Tin nhắn hệ thống
- Các loại khác tùy theo nền tảng

### Attachment Types (Loại file đính kèm)

- **PHOTO**: Hình ảnh
- **VIDEO**: Video
- **DOCUMENT**: Tài liệu
- **AUDIO_ATTACHMENT_ID**: Audio

## Data Schemas (Cấu trúc dữ liệu)

### Page Schema

Đại diện cho một trang trong hệ thống Pancake.

```typescript
interface Page {
  id: string;              // ID duy nhất của trang
  platform: string;        // Nền tảng (ví dụ: "facebook")
  name: string;           // Tên trang
  avatar_url: string;      // URL của avatar trang
}
```

**Ví dụ:**
```json
{
  "id": "123456789",
  "platform": "facebook",
  "name": "My Page",
  "avatar_url": "https://example.com/avatar.jpg"
}
```

### Conversation Schema

Đại diện cho một cuộc hội thoại.

```typescript
interface Conversation {
  id: string;                    // ID duy nhất của cuộc hội thoại
  type: string;                  // Loại cuộc hội thoại: "INBOX" | "COMMENT" | "LIVESTREAM"
  page_uid: string;              // UID của trang
  updated_at: string;            // Thời gian cập nhật cuối cùng (ISO 8601 format)
  inserted_at: string;           // Thời gian tạo cuộc hội thoại (ISO 8601 format)
  tags: string[];                // Danh sách các tag của cuộc hội thoại
  last_message: {                // Tin nhắn cuối cùng trong cuộc hội thoại
    text: string;                // Nội dung tin nhắn
    sender: string;              // Người gửi
    created_at: string;          // Thời gian tạo (ISO 8601 format)
  };
  participants: {                // Danh sách người tham gia
    name: string;                // Tên người tham gia
    email: string;               // Email người tham gia
    phone: string;               // Số điện thoại người tham gia
  }[];
}
```

**Ví dụ:**
```json
{
  "id": "conv_123456",
  "type": "INBOX",
  "page_uid": "page_123",
  "updated_at": "2019-08-24T14:15:22Z",
  "inserted_at": "2019-08-24T10:00:00Z",
  "tags": ["urgent", "customer-support"],
  "last_message": {
    "text": "Xin chào, tôi cần hỗ trợ",
    "sender": "customer_789",
    "created_at": "2019-08-24T14:15:22Z"
  },
  "participants": [
    {
      "name": "John Doe",
      "email": "john@example.com",
      "phone": "+84123456789"
    }
  ]
}
```

### Message Schema

Đại diện cho một tin nhắn trong cuộc hội thoại.

```typescript
interface Message {
  conversation_id: string;        // ID của cuộc hội thoại mà tin nhắn này thuộc về
  from: {                         // Thông tin về người gửi tin nhắn
    email: string;                // Địa chỉ email của người gửi
    id: string;                   // ID duy nhất của người gửi
    name: string;                 // Tên hiển thị của người gửi
  };
  has_phone: boolean;              // Có số điện thoại liên kết hay không
  inserted_at: string;            // Thời gian tin nhắn được chèn vào (ISO 8601 format)
  is_hidden: boolean;             // Tin nhắn có bị ẩn hay không
  is_removed: boolean;            // Tin nhắn có bị xóa hay không
  message: string;                // Nội dung của tin nhắn
  page_id: string;                // ID của trang liên quan đến tin nhắn này
  type: string;                   // Loại tin nhắn (ví dụ: "text", "image", "system")
}
```

**Ví dụ:**
```json
{
  "conversation_id": "conv_123456",
  "from": {
    "email": "user@example.com",
    "id": "user_789",
    "name": "John Doe"
  },
  "has_phone": true,
  "inserted_at": "2019-08-24T14:15:22Z",
  "is_hidden": false,
  "is_removed": false,
  "message": "Xin chào, tôi cần hỗ trợ",
  "page_id": "page_123",
  "type": "text"
}
```

### Private Reply Schema

Đại diện cho một phản hồi riêng tư (private reply) cho comment trên bài đăng.

```typescript
interface PrivateReply {
  action: "private_replies";     // Loại action (bắt buộc phải là "private_replies")
  post_id: string;               // ID của bài đăng chứa comment (required)
  message_id: string;            // ID của comment bạn muốn gửi tin nhắn từ đó (required)
  from_id?: string;             // ID duy nhất của người gửi (from.id) (optional)
  message: string;               // Nội dung tin nhắn (required)
}
```

**Ví dụ:**
```json
{
  "action": "private_replies",
  "post_id": "post_123456",
  "message_id": "comment_789",
  "from_id": "user_456",
  "message": "Cảm ơn bạn đã quan tâm! Tôi sẽ liên hệ với bạn qua tin nhắn riêng."
}
```

### Inbox Message Schema

Đại diện cho một tin nhắn trong hộp thư đến (inbox message).

```typescript
interface InboxMessage {
  action: "reply_inbox";         // Loại action (bắt buộc phải là "reply_inbox")
  message: string;               // Nội dung tin nhắn inbox (required)
  name?: string;                 // Tên file (optional)
  mime_type?: string;            // MIME type của file (image, etc.) (optional)
  content_ids?: string[];        // Danh sách content_ids bạn muốn gửi. Content_id được tạo từ content upload API (optional)
  attachment_type?: string;       // Loại attachment (PHOTO, VIDEO, DOCUMENT, AUDIO_ATTACHMENT_ID) (optional)
}
```

**Lưu ý:** `content_ids` được tạo từ [content upload API](https://developer.pancake.biz/#/paths/pages-page_id--upload_contents/post)

**Ví dụ:**
```json
{
  "action": "reply_inbox",
  "message": "Xin chào! Chúng tôi đã nhận được yêu cầu của bạn.",
  "name": "image.jpg",
  "mime_type": "image/jpeg",
  "content_ids": ["content_123", "content_456"],
  "attachment_type": "PHOTO"
}
```

### Reply Comment Schema

Đại diện cho một phản hồi bình luận (reply comment).

```typescript
interface ReplyComment {
  action: "reply_comment";       // Loại action (bắt buộc phải là "reply_comment")
  message_id: string;            // ID của comment bạn muốn reply (required)
  message: string;               // Nội dung tin nhắn reply (required)
  content_url?: string;          // URL của hình ảnh (optional)
  mentions?: {                   // Danh sách mentions (optional)
    psid: string;                // PSID của khách hàng
    name: string;                // Tên khách hàng
    offset: number;              // Vị trí offset trong message
    length: number;               // Độ dài của mention
  }[];
}
```

**Ví dụ:**
```json
{
  "action": "reply_comment",
  "message_id": "comment_123",
  "message": "Cảm ơn @John Doe đã phản hồi!",
  "content_url": "https://example.com/image.jpg",
  "mentions": [
    {
      "psid": "psid_123456",
      "name": "John Doe",
      "offset": 8,
      "length": 8
    }
  ]
}
```

### Tag Schema

Đại diện cho một tag trong hệ thống.

```typescript
interface Tag {
  id: number;                    // ID duy nhất của tag
  text: string;                  // Tên của tag
  color: string;                 // Mã màu chính của tag (hex color)
  lighten_color: string;         // Phiên bản màu sáng hơn của tag (hex color)
}
```

**Ví dụ:**
```json
{
  "id": 0,
  "text": "Kiểm hàng",
  "color": "#4b5577",
  "lighten_color": "#c9ccd6"
}
```

### User Schema

Đại diện cho một người dùng trong hệ thống.

```typescript
interface User {
  id: string;                    // UUID của người dùng
  name: string;                  // Tên người dùng
  status: string;               // Trạng thái khả dụng (ví dụ: "available")
  fb_id: string;                // Facebook ID của người dùng
  page_permissions?: {          // Quyền của người dùng trong trang
    permissions: number[];       // Danh sách mã quyền
  } | null;
  status_in_page: string;       // Trạng thái người dùng trong trang (ví dụ: "active")
  is_online: boolean;           // Người dùng có đang online hay không
}
```

**Ví dụ:**
```json
{
  "id": "c4bafd84-7b96-4f28-b59a-031f17c32ddf",
  "name": "Anh Ngoc Nguyen",
  "status": "available",
  "fb_id": "116256249766099",
  "page_permissions": {
    "permissions": [100, 71, 81]
  },
  "status_in_page": "active",
  "is_online": false
}
```

### Round Robin Users Schema

Đại diện cho cấu hình round robin users (phân phối tự động cuộc hội thoại).

```typescript
interface RoundRobinUsers {
  comment: string[];            // Danh sách user IDs cho round robin comment
  inbox: string[];              // Danh sách user IDs cho round robin inbox
}
```

**Ví dụ:**
```json
{
  "comment": ["79d4e769-ac31-4821-8304-d6e251d532e9"],
  "inbox": ["fb5ff8ed-434b-4d4b-a213-b595b242b81a"]
}
```

### Upload Content Response Schema

Đại diện cho response khi upload content thành công.

```typescript
interface UploadContentResponse {
  id: string;                   // Content ID (sử dụng trong InboxMessage.content_ids)
  attachment_type: string;      // Loại attachment (PHOTO, VIDEO, DOCUMENT, AUDIO_ATTACHMENT_ID)
  success: boolean;             // Trạng thái thành công
}
```

**Ví dụ:**
```json
{
  "id": "HXrxioWFIc5DFwffhmOVHspLuMwpWCXfWDoBxiov6DLa3MvakLeGpLQAly7oHDvZT66VEhnYG4zQEi2MhEzhlg",
  "attachment_type": "PHOTO",
  "success": true
}
```

## Quy trình làm việc cơ bản

### Bước 1: Lấy User Access Token
- Người dùng đăng nhập vào Pancake và lấy `access_token`

### Bước 2: Lấy danh sách Pages
```bash
GET https://pages.fm/api/v1/pages?access_token=USER_ACCESS_TOKEN
```

### Bước 3: Tạo Page Access Token
```bash
POST https://pages.fm/api/v1/pages/{page_id}/generate_page_access_token?access_token=USER_ACCESS_TOKEN
```

### Bước 4: Sử dụng Page Access Token cho các API công khai
```bash
GET https://pages.fm/api/public_api/v1/pages/{page_id}/conversations?page_access_token=PAGE_ACCESS_TOKEN
```

## Headers

Tất cả các request nên bao gồm:

- `Accept: application/json` - Cho request GET
- `Content-Type: application/json` - Cho request POST/PUT/PATCH

## Mã lỗi và xử lý

API trả về các mã HTTP status code chuẩn:

- **200 OK:** Request thành công
- **400 Bad Request:** Request không hợp lệ
- **401 Unauthorized:** Token không hợp lệ hoặc thiếu quyền
- **404 Not Found:** Tài nguyên không tồn tại
- **500 Internal Server Error:** Lỗi server

## Best Practices

1. **Bảo mật Token:**
   - Không commit token vào code hoặc repository
   - Sử dụng biến môi trường để lưu trữ token
   - Rotate token định kỳ

2. **Rate Limiting:**
   - Tuân thủ giới hạn rate limit của API
   - Implement retry logic với exponential backoff

3. **Error Handling:**
   - Luôn kiểm tra status code của response
   - Xử lý các lỗi một cách graceful
   - Log lỗi để debug

4. **Performance:**
   - Sử dụng pagination khi lấy danh sách lớn
   - Cache dữ liệu khi có thể
   - Sử dụng async/await cho các request không đồng bộ

## Ví dụ tích hợp

### Ví dụ với JavaScript/TypeScript

```typescript
// Lấy danh sách pages
async function getPages(accessToken: string) {
  const response = await fetch(
    `https://pages.fm/api/v1/pages?access_token=${accessToken}`,
    {
      method: 'GET',
      headers: {
        'Accept': 'application/json'
      }
    }
  );
  
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  
  const data = await response.json();
  return data.pages;
}

// Tạo page access token
async function generatePageAccessToken(
  pageId: string, 
  userAccessToken: string
) {
  const response = await fetch(
    `https://pages.fm/api/v1/pages/${pageId}/generate_page_access_token?access_token=${userAccessToken}`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      }
    }
  );
  
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  
  return response.json();
}
```

### Ví dụ với Go

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "io/ioutil"
)

type Page struct {
    ID        string `json:"id"`
    Platform  string `json:"platform"`
    Name      string `json:"name"`
    AvatarURL string `json:"avatar_url"`
}

type PagesResponse struct {
    Pages []Page `json:"pages"`
}

func GetPages(accessToken string) ([]Page, error) {
    url := fmt.Sprintf("https://pages.fm/api/v1/pages?access_token=%s", accessToken)
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Accept", "application/json")
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("HTTP error! status: %d", resp.StatusCode)
    }
    
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
    var result PagesResponse
    err = json.Unmarshal(body, &result)
    if err != nil {
        return nil, err
    }
    
    return result.Pages, nil
}
```

## Tài liệu tham khảo

- **Tài liệu chính thức:** https://developer.pancake.biz/
- **API Base URLs:**
  - User's API: `https://pages.fm/api/v1`
  - Page's API v1: `https://pages.fm/api/public_api/v1`
  - Page's API v2: `https://pages.fm/api/public_api/v2`

## Ghi chú

- Tài liệu này được tạo dựa trên phiên bản API v1.0.0
- Một số endpoint và schema có thể cần được cập nhật chi tiết hơn khi có thêm thông tin
- Luôn tham khảo tài liệu chính thức tại https://developer.pancake.biz/ để có thông tin mới nhất
