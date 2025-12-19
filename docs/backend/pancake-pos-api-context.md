# Pancake POS API - Tài liệu AI Context

## 📚 Thông tin chung

- **Tên API:** Pancake POS Open API
- **Phiên bản:** 1.0.0
- **OpenAPI Version:** 3.1.0
- **Base URL:** `https://pos.pages.fm/api/v1`
- **Mô tả:** API documentation for POS system (Tài liệu API cho hệ thống POS)

---

## 📑 Mục lục

1. [Tổng quan](#tổng-quan)
2. [Xác thực (Authentication)](#xác-thực-authentication)
3. [Cấu trúc Endpoints](#cấu-trúc-endpoints)
   - [1. Shop (Cửa hàng)](#1-shop-cửa-hàng)
   - [2. Địa lý (Geo)](#2-địa-lý-geo)
   - [3. Kho hàng (Warehouses)](#3-kho-hàng-warehouses)
   - [4. Đơn hàng (Orders)](#4-đơn-hàng-orders)
   - [5. Khách hàng (Customers)](#5-khách-hàng-customers)
   - [6. Sản phẩm (Products)](#6-sản-phẩm-products)
   - [7. Nhập hàng (Purchases)](#7-nhập-hàng-purchases)
   - [8. Chuyển kho (Transfers)](#8-chuyển-kho-transfers)
   - [9. Kiểm kê (Stocktakings)](#9-kiểm-kê-stocktakings)
   - [10. Khuyến mãi (Promotions)](#10-khuyến-mãi-promotions)
   - [11. Voucher](#11-voucher)
   - [12. Combo Products](#12-combo-products)
   - [13. Phân tích (Analytics)](#13-phân-tích-analytics)
   - [14. Người dùng (Users)](#14-người-dùng-users)
   - [15. CRM](#15-crm)
   - [16. Các API khác](#16-các-api-khác)
4. [Data Schemas](#data-schemas)
5. [Trạng thái đơn hàng (Order Status)](#trạng-thái-đơn-hàng-order-status)
6. [Best Practices](#best-practices)

---

## Tổng quan

Pancake POS API là một hệ thống API RESTful cho phép quản lý toàn bộ hoạt động của hệ thống POS (Point of Sale) bao gồm:
- Quản lý cửa hàng và kho hàng
- Quản lý đơn hàng và khách hàng
- Quản lý sản phẩm và tồn kho
- Quản lý nhập hàng, chuyển kho, kiểm kê
- Quản lý khuyến mãi và voucher
- Phân tích và báo cáo
- CRM và quản lý khách hàng

**Tất cả các API đều yêu cầu xác thực bằng API Key.**

---

## Xác thực (Authentication)

### Cách tạo API Key

1. Đăng nhập vào hệ thống Pancake POS
2. Vào **Cấu hình -> Nâng cao -> Kết nối bên thứ 3 -> Webhook/API**
3. Trong khung `API KEY`, click `Thêm mới` (Create)
4. Copy API key được tạo

### Sử dụng API Key

API Key được truyền qua query parameter `api_key` trong mọi request:

```
GET https://pos.pages.fm/api/v1/shops?api_key=YOUR_API_KEY
```

**Lưu ý:** 
- API Key phải được truyền trong mọi request
- Không chia sẻ API Key với người khác
- Nếu API Key bị lộ, hãy tạo lại ngay lập tức

---

## Cấu trúc Endpoints

Tất cả endpoints đều có format: `/shops/{SHOP_ID}/...`

Trong đó `SHOP_ID` là mã cửa hàng (integer).

### Pagination

Hầu hết các API list đều hỗ trợ phân trang:
- `page_size`: Số lượng items mỗi trang (mặc định: 30)
- `page_number`: Số trang (mặc định: 1)

---

### 1. Shop (Cửa hàng)

#### Lấy thông tin cửa hàng
```
GET /shops
```

**Response:** Danh sách các shop với thông tin:
- `id`: Mã cửa hàng
- `name`: Tên cửa hàng
- `avatar_url`: Link hình đại diện
- `pages`: Thông tin các pages được gộp trong shop
- `link_post_marketer`: Thông tin liên kết bài viết/nguồn đơn/TK QC với marketer

#### Lấy thông tin chi tiết shop
```
GET /shops/{SHOP_ID}
```

---

### 2. Địa lý (Geo)

#### Lấy danh sách tỉnh/thành phố
```
GET /geo/provinces
```

#### Lấy danh sách quận/huyện
```
GET /geo/districts?province_id={PROVINCE_ID}
```

#### Lấy danh sách phường/xã
```
GET /geo/communes?district_id={DISTRICT_ID}
```

---

### 3. Kho hàng (Warehouses)

#### Lấy danh sách kho hàng
```
GET /shops/{SHOP_ID}/warehouses
```

#### Lấy thông tin chi tiết kho hàng
```
GET /shops/{SHOP_ID}/warehouses/{WAREHOUSE_ID}
```

#### Lấy lịch sử tồn kho
```
GET /shops/{SHOP_ID}/inventory_histories
```

**Query parameters:**
- `variation_id`: Mã biến thể sản phẩm
- `warehouse_id`: Mã kho hàng
- `page_size`, `page_number`: Phân trang

---

### 4. Đơn hàng (Orders)

#### Lấy danh sách đơn hàng
```
GET /shops/{SHOP_ID}/orders
```

**Query parameters:**
- `page_size`: Kích thước trang (mặc định: 30)
- `page_number`: Số trang (mặc định: 1)
- `search`: Tìm kiếm theo số điện thoại, tên khách hàng, ghi chú...
- `filter_status[]`: Lọc theo trạng thái đơn hàng (array of integers)
- `include_removed`: Bao gồm đơn đã xóa (0 hoặc 1)
- `updateStatus`: Sắp xếp theo thời gian (inserted_at, updated_at, paid_at, etc.)

**Ví dụ:**
```
GET /shops/4/orders?page_size=50&page_number=1&filter_status[]=1&filter_status[]=2&search=0999999999
```

#### Lấy thông tin chi tiết đơn hàng
```
GET /shops/{SHOP_ID}/orders/{ORDER_ID}
```

**Response:** Object `Order` với đầy đủ thông tin đơn hàng

#### Lấy nguồn đơn hàng
```
GET /shops/{SHOP_ID}/order_source
```

#### Lấy tags của đơn hàng
```
GET /shops/{SHOP_ID}/orders/tags
```

#### Lấy URL tracking đơn hàng
```
GET /shops/{SHOP_ID}/orders/get_tracking_url?order_id={ORDER_ID}
```

#### Lấy khuyến mãi đang áp dụng
```
GET /shops/{SHOP_ID}/orders/get_promotion_advance_active
```

#### Lấy đơn hàng đã trả
```
GET /shops/{SHOP_ID}/orders_returned
```

**Query parameters:** Tương tự như list orders

---

### 5. Khách hàng (Customers)

#### Lấy danh sách khách hàng
```
GET /shops/{SHOP_ID}/customers
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm theo tên, số điện thoại, email
- `customer_level_id`: Lọc theo cấp độ khách hàng
- `tag_ids[]`: Lọc theo tags

#### Lấy thông tin chi tiết khách hàng
```
GET /shops/{SHOP_ID}/customers/{CUSTOMER_ID}
```

#### Lấy lịch sử điểm tích lũy
```
GET /shops/{SHOP_ID}/customers/point_logs
```

**Query parameters:**
- `customer_id`: Mã khách hàng
- `page_size`, `page_number`: Phân trang

#### Lấy ghi chú khách hàng
```
GET /shops/{SHOP_ID}/customers/{CUSTOMER_ID}/load_customer_notes
```

#### Tạo ghi chú khách hàng
```
POST /shops/{SHOP_ID}/customers/{CUSTOMER_ID}/create_note
```

**Body:**
```json
{
  "note": "Nội dung ghi chú"
}
```

#### Lấy danh sách cấp độ khách hàng
```
GET /shops/{SHOP_ID}/customer_levels
```

---

### 6. Sản phẩm (Products)

#### Lấy danh sách sản phẩm
```
GET /shops/{SHOP_ID}/products
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm theo tên, SKU
- `category_ids[]`: Lọc theo danh mục
- `tag_ids[]`: Lọc theo tags
- `is_hide`: Lọc sản phẩm ẩn/hiện (0 hoặc 1)

#### Tạo sản phẩm
```
POST /shops/{SHOP_ID}/products
```

**Body:**
```json
{
  "product": {
    "name": "Tên sản phẩm",
    "category_ids": [1290021044, 201250699],
    "note_product": "Ghi chú sản phẩm",
    "product_attributes": [
      {
        "name": "Màu",
        "values": ["Đen", "Trắng", "Đỏ"]
      },
      {
        "name": "Size",
        "values": ["S", "M", "L"]
      }
    ],
    "tags": [193, 51],
    "variations": [
      {
        "fields": [
          {"name": "Màu", "value": "Trắng"},
          {"name": "Size", "value": "M"}
        ],
        "images": ["https://example.com/image.jpg"],
        "last_imported_price": 30000,
        "retail_price": 140000,
        "price_at_counter": 123000,
        "weight": 0,
        "sku": "SKU-001"
      }
    ]
  }
}
```

#### Lấy thông tin chi tiết sản phẩm
```
GET /shops/{SHOP_ID}/products/{PRODUCT_ID}
```

#### Lấy sản phẩm theo SKU
```
GET /shops/{SHOP_ID}/products/{PRODUCT_SKU}
```

#### Cập nhật số lượng tồn kho (một biến thể)
```
PUT /shops/{SHOP_ID}/variations/{VARIATION_ID}/update_quantity
```

**Body:**
```json
{
  "warehouse_id": "uuid",
  "quantity": 100
}
```

#### Cập nhật số lượng tồn kho (nhiều biến thể)
```
PUT /shops/{SHOP_ID}/variations/update_quantity
```

**Body:**
```json
{
  "variations": [
    {
      "variation_id": "uuid",
      "warehouse_id": "uuid",
      "quantity": 100
    }
  ]
}
```

#### Cập nhật sản phẩm composite
```
PUT /shops/{SHOP_ID}/variations/update_composite_product
```

#### Lấy danh sách biến thể sản phẩm
```
GET /shops/{SHOP_ID}/products/variations
```

**Query parameters:**
- `product_id`: Mã sản phẩm
- `warehouse_id`: Mã kho hàng
- `page_size`, `page_number`: Phân trang

#### Cập nhật trạng thái ẩn/hiện sản phẩm
```
PUT /shops/{SHOP_ID}/products/update_hide
```

**Body:**
```json
{
  "product_ids": [1, 2, 3],
  "is_hide": 1
}
```

#### Lấy tags sản phẩm
```
GET /shops/{SHOP_ID}/tags_products
```

#### Lấy danh sách danh mục
```
GET /shops/{SHOP_ID}/categories
```

#### Lấy nguyên liệu sản phẩm
```
GET /shops/{SHOP_ID}/materials_products
```

#### Lấy đơn vị đo lường
```
GET /shops/{SHOP_ID}/product_measurements/get_measure
```

---

### 7. Nhập hàng (Purchases)

#### Lấy danh sách phiếu nhập
```
GET /shops/{SHOP_ID}/purchases
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm
- `supplier_id`: Lọc theo nhà cung cấp
- `warehouse_id`: Lọc theo kho

#### Lấy thông tin chi tiết phiếu nhập
```
GET /shops/{SHOP_ID}/purchases/{PURCHASE_ID}
```

#### Tách phiếu nhập
```
POST /shops/{SHOP_ID}/purchases/separate
```

#### Lấy danh sách nhà cung cấp
```
GET /shops/{SHOP_ID}/supplier
```

---

### 8. Chuyển kho (Transfers)

#### Lấy danh sách phiếu chuyển kho
```
GET /shops/{SHOP_ID}/transfers
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `from_warehouse_id`: Lọc theo kho nguồn
- `to_warehouse_id`: Lọc theo kho đích
- `status`: Lọc theo trạng thái

#### Tạo phiếu chuyển kho (nhiều sản phẩm)
```
POST /shops/{SHOP_ID}/transfers/multi
```

#### Lấy thông tin chi tiết phiếu chuyển kho
```
GET /shops/{SHOP_ID}/transfers/{TRANSFER_ID}
```

#### Lấy lịch sử trạng thái chuyển kho
```
GET /shops/{SHOP_ID}/transfers/get_status_history/{TRANSFER_ID}
```

---

### 9. Kiểm kê (Stocktakings)

#### Lấy danh sách phiếu kiểm kê
```
GET /shops/{SHOP_ID}/stocktakings
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `warehouse_id`: Lọc theo kho
- `status`: Lọc theo trạng thái

#### Lấy thông tin chi tiết phiếu kiểm kê
```
GET /shops/{SHOP_ID}/stocktakings/{STOCKTAKING_ID}
```

---

### 10. Khuyến mãi (Promotions)

#### Lấy danh sách khuyến mãi
```
GET /shops/{SHOP_ID}/promotion_advance
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm
- `status`: Lọc theo trạng thái

#### Lấy thông tin chi tiết khuyến mãi
```
GET /shops/{SHOP_ID}/promotion_advance/{PROMOTION_ID}
```

#### Tạo nhiều khuyến mãi cùng lúc
```
POST /shops/{SHOP_ID}/promotion_advance/create_multi
```

#### Xóa nhiều khuyến mãi
```
POST /shops/{SHOP_ID}/promotion_advance/delete_multi
```

**Body:**
```json
{
  "promotion_ids": [1, 2, 3]
}
```

---

### 11. Voucher

#### Lấy danh sách voucher
```
GET /shops/{SHOP_ID}/vouchers
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm
- `status`: Lọc theo trạng thái

#### Lấy thông tin chi tiết voucher
```
GET /shops/{SHOP_ID}/vouchers/{VOUCHER_ID}
```

#### Tạo nhiều voucher cùng lúc
```
POST /shops/{SHOP_ID}/vouchers/create_multi
```

---

### 12. Combo Products

#### Lấy danh sách combo sản phẩm
```
GET /shops/{SHOP_ID}/combo_products
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm

---

### 13. Phân tích (Analytics)

#### Phân tích bán hàng
```
GET /shops/{SHOP_ID}/analytics/sale
```

**Query parameters:**
- `from_date`: Ngày bắt đầu (YYYY-MM-DD)
- `to_date`: Ngày kết thúc (YYYY-MM-DD)
- `group_by`: Nhóm theo (day, week, month, year)

#### Lấy danh sách công thức phân tích
```
GET /shops/{SHOP_ID}/analytics/get_list_formula
```

#### Lấy các trường phân tích
```
GET /shops/{SHOP_ID}/analytics/get_analytic_fields
```

#### Phân tích tồn kho
```
GET /shops/{SHOP_ID}/inventory_analytics/inventory
```

#### Phân tích tồn kho theo sản phẩm
```
GET /shops/{SHOP_ID}/inventory_analytics/inventory_by_product
```

---

### 14. Người dùng (Users)

#### Lấy danh sách người dùng
```
GET /shops/{SHOP_ID}/users
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm theo tên, email

---

### 15. CRM

#### Lấy danh sách bảng CRM
```
GET /shops/{SHOP_ID}/crm/tables
```

#### Lấy profile CRM
```
GET /shops/{SHOP_ID}/crm/profile
```

#### Lấy records từ bảng CRM
```
GET /shops/{SHOP_ID}/crm/{TABLE_NAME}/records
```

**Query parameters:**
- `page_size`, `page_number`: Phân trang
- `search`: Tìm kiếm

#### Lấy lịch sử bảng CRM
```
GET /shops/{SHOP_ID}/crm/{TABLE_NAME}/history
```

---

### 16. Các API khác

#### Lấy tài liệu vận chuyển logistics
```
GET /shops/{SHOP_ID}/products/get_logistics_shipping_document
```

#### Lấy danh sách thanh toán ngân hàng
```
GET /shops/{SHOP_ID}/bank_payments
```

#### Lấy đơn hàng gọi lại sau
```
GET /shops/{SHOP_ID}/order_call_laters
```

#### Lấy công nợ
```
GET /shops/{SHOP_ID}/debt
```

#### Lấy giao dịch
```
GET /shops/{SHOP_ID}/transactions
```

#### Lấy chi phí quảng cáo
```
GET /shops/{SHOP_ID}/adv_costs
```

#### Lấy lịch sử thanh toán
```
GET /shops/{SHOP_ID}/payment_accounts/get_payment_histories
```

#### Xuất dữ liệu
```
GET /shops/{SHOP_ID}/export
```

**Query parameters:**
- `type`: Loại export (orders, products, customers, etc.)
- `format`: Định dạng (xlsx, csv)

#### Lấy thông tin tài khoản marketplace
```
GET /shops/{SHOP_ID}/marketplace/get_account_info
```

#### Đánh giá Shopee
```
POST /shops/{SHOP_ID}/shopee/evaluate
```

#### Đảo ngược đơn hàng Shopee
```
POST /shops/{SHOP_ID}/shopee/reverse_order
```

#### Lấy danh sách đối tác
```
GET /shops/{SHOP_ID}/partners
```

#### Lấy danh sách hóa đơn điện tử
```
GET /shops/{SHOP_ID}/list_einvoices/
```

---

## Data Schemas

### Order Schema

Object `Order` chứa các thông tin chính:

```json
{
  "id": 1,
  "system_id": 1,
  "shop_id": 1,
  "inserted_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z",
  "status": 1,
  "status_name": "Đã xác nhận",
  "bill_full_name": "Tên khách hàng",
  "bill_phone_number": "0999999999",
  "bill_email": "email@example.com",
  "page_id": "104438181227821",
  "post_id": "185187094667903_477083092110915",
  "shipping_fee": 10000,
  "partner_fee": 5000,
  "fee_marketplace": 3000,
  "customer_pay_fee": true,
  "is_free_shipping": false,
  "total_discount": 50000,
  "note": "Ghi chú đơn hàng",
  "warehouse_id": "uuid",
  "warehouse_info": {
    "name": "Tên kho",
    "phone_number": "0999999999",
    "full_address": "Địa chỉ đầy đủ",
    "province_id": "717",
    "district_id": "71705",
    "commune_id": "7170510"
  },
  "customer": {
    "id": 1,
    "name": "Tên khách hàng",
    "phone_number": "0999999999",
    "email": "email@example.com"
  },
  "order_items": [
    {
      "id": 1,
      "product_id": 1,
      "product_name": "Tên sản phẩm",
      "variation_id": "uuid",
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
```

### Product Schema

```json
{
  "id": 1,
  "name": "Tên sản phẩm",
  "category_ids": [1, 2],
  "note_product": "Ghi chú",
  "product_attributes": [
    {
      "name": "Màu",
      "values": ["Đen", "Trắng"]
    }
  ],
  "tags": [1, 2],
  "variations": [
    {
      "id": "uuid",
      "fields": [
        {"name": "Màu", "value": "Đen"}
      ],
      "images": ["https://example.com/image.jpg"],
      "retail_price": 100000,
      "price_at_counter": 90000,
      "sku": "SKU-001",
      "quantity": 100
    }
  ]
}
```

### Customer Schema

```json
{
  "id": 1,
  "name": "Tên khách hàng",
  "phone_number": "0999999999",
  "email": "email@example.com",
  "customer_level_id": 1,
  "point": 1000,
  "total_order": 10,
  "total_spent": 1000000,
  "tags": [1, 2]
}
```

---

## Trạng thái đơn hàng (Order Status)

Các trạng thái đơn hàng được định nghĩa bằng số nguyên:

| Status | Tên tiếng Việt | Tên tiếng Anh |
|--------|----------------|---------------|
| 0 | Mới | New |
| 17 | Chờ xác nhận | Waiting for confirmation |
| 11 | Chờ hàng | Restocking |
| 12 | Chờ in | Wait for printing |
| 13 | Đã in | Printed |
| 20 | Đã đặt hàng | Purchased |
| 1 | Đã xác nhận | Confirmed |
| 8 | Đang đóng hàng | Packaging |
| 9 | Chờ lấy hàng | Waiting for pick up |
| 2 | Đã giao hàng | Shipped |
| 3 | Đã nhận hàng | Received |
| 16 | Đã thu tiền | Collected money |
| 4 | Đang trả hàng | Returning |
| 15 | Trả hàng một phần | Partial return |
| 5 | Đã trả hàng | Returned |
| 6 | Đã hủy | Canceled |
| 7 | Đã xóa gần đây | Deleted recently |

---

## Best Practices

### 1. Xử lý Pagination

Luôn sử dụng pagination cho các API list để tránh timeout và giảm tải server:

```javascript
// Ví dụ: Lấy đơn hàng với pagination
const pageSize = 50;
let pageNumber = 1;
let allOrders = [];

while (true) {
  const response = await fetch(
    `https://pos.pages.fm/api/v1/shops/${shopId}/orders?api_key=${apiKey}&page_size=${pageSize}&page_number=${pageNumber}`
  );
  const data = await response.json();
  
  if (!data.orders || data.orders.length === 0) break;
  
  allOrders = allOrders.concat(data.orders);
  pageNumber++;
  
  // Giới hạn số trang để tránh vòng lặp vô hạn
  if (pageNumber > 100) break;
}
```

### 2. Xử lý Rate Limiting

API có thể có giới hạn số request mỗi giây. Nên implement retry logic với exponential backoff:

```javascript
async function fetchWithRetry(url, options, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await fetch(url, options);
      if (response.status === 429) {
        // Rate limit exceeded
        const delay = Math.pow(2, i) * 1000; // Exponential backoff
        await new Promise(resolve => setTimeout(resolve, delay));
        continue;
      }
      return response;
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      await new Promise(resolve => setTimeout(resolve, 1000 * (i + 1)));
    }
  }
}
```

### 3. Xử lý Lỗi

Luôn kiểm tra status code và xử lý lỗi phù hợp:

```javascript
const response = await fetch(url);
if (!response.ok) {
  const error = await response.json();
  console.error('API Error:', error);
  // Xử lý lỗi cụ thể
  if (response.status === 401) {
    // API Key không hợp lệ
  } else if (response.status === 404) {
    // Không tìm thấy resource
  } else if (response.status === 500) {
    // Lỗi server
  }
}
```

### 4. Cache dữ liệu

Cache các dữ liệu ít thay đổi như danh mục, tags, cấp độ khách hàng:

```javascript
const cache = new Map();
const CACHE_TTL = 3600000; // 1 giờ

async function getCategories(shopId, apiKey) {
  const cacheKey = `categories_${shopId}`;
  const cached = cache.get(cacheKey);
  
  if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
    return cached.data;
  }
  
  const response = await fetch(
    `https://pos.pages.fm/api/v1/shops/${shopId}/categories?api_key=${apiKey}`
  );
  const data = await response.json();
  
  cache.set(cacheKey, {
    data,
    timestamp: Date.now()
  });
  
  return data;
}
```

### 5. Batch Operations

Khi cần cập nhật nhiều items, sử dụng các API batch thay vì gọi từng API:

```javascript
// ❌ Không tốt: Gọi từng API
for (const productId of productIds) {
  await updateProductHide(shopId, productId, true);
}

// ✅ Tốt: Sử dụng batch API
await updateProductsHide(shopId, productIds, true);
```

### 6. Xử lý Date/Time

API sử dụng ISO 8601 format cho datetime. Luôn convert đúng format:

```javascript
// Convert date sang format API yêu cầu
const fromDate = new Date('2024-01-01').toISOString().split('T')[0]; // YYYY-MM-DD
const toDate = new Date('2024-01-31').toISOString().split('T')[0];
```

### 7. Validate Input

Luôn validate input trước khi gọi API:

```javascript
function validateOrderStatus(status) {
  const validStatuses = [0, 17, 11, 12, 13, 20, 1, 8, 9, 2, 3, 16, 4, 15, 5, 6, 7];
  return validStatuses.includes(status);
}

if (!validateOrderStatus(status)) {
  throw new Error('Invalid order status');
}
```

---

## Lưu ý quan trọng

1. **API Key Security**: Không commit API key vào code, sử dụng environment variables
2. **Shop ID**: Luôn validate Shop ID trước khi gọi API
3. **Error Handling**: Luôn xử lý các trường hợp lỗi có thể xảy ra
4. **Data Validation**: Validate dữ liệu trước khi gửi request
5. **Testing**: Test kỹ trên môi trường development trước khi deploy production
6. **Documentation**: Tham khảo file OpenAPI JSON gốc để biết chi tiết về request/response schemas

---

## Tài liệu tham khảo

- File OpenAPI gốc: `api-1.json`
- Base URL: `https://pos.pages.fm/api/v1`
- Tất cả endpoints đều yêu cầu `api_key` trong query parameter
