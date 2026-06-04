# PRD Technical Backend — Footwear E-Commerce Operations Platform

## 1. Product Overview

**Footwear E-Commerce Operations Platform** adalah sistem backend untuk e-commerce sepatu yang mendukung katalog produk, varian ukuran/warna, cart, checkout, simulated payment, order management, inventory tracking, admin dashboard, dan warehouse-ready fulfillment.

Backend dibangun menggunakan **Golang** sebagai REST API service, dengan **PostgreSQL** sebagai database utama. Untuk MVP, sistem tidak menggunakan payment gateway seperti Midtrans/Xendit dan tidak menggunakan Supabase Storage. Pembayaran dibuat menggunakan **simulated payment flow**, sedangkan file upload memakai **local storage**.

Tujuan backend bukan hanya menyediakan CRUD, tetapi membangun fondasi business logic e-commerce yang realistis dan bisa dikembangkan ke payment gateway, cloud storage, warehouse system, dan reporting di masa depan.

---

## 2. Goals

### 2.1 Business Goals

- Menyediakan backend e-commerce footwear yang realistis untuk portfolio full stack.
- Mendukung flow belanja customer dari product browsing sampai order tracking.
- Mendukung admin dalam mengelola produk, varian, stok, order, dan shipment.
- Menyediakan simulated payment flow yang gateway-ready.
- Menyediakan inventory log agar perubahan stok bisa dilacak.
- Menyediakan struktur API yang mudah dikonsumsi oleh frontend Next.js.

### 2.2 Technical Goals

- Membangun REST API menggunakan Golang.
- Menggunakan PostgreSQL sebagai relational database.
- Menggunakan database transaction untuk checkout dan payment confirmation.
- Mengimplementasikan JWT authentication dan role-based access control.
- Memisahkan layer handler, service, repository, dan DTO.
- Menghindari business logic di handler.
- Menyediakan response format yang konsisten.
- Menyediakan validasi request di backend.
- Menyediakan migration SQL untuk database schema.

---

## 3. Non-Goals

Untuk MVP, backend **tidak membuat**:

- Integrasi real payment gateway Midtrans/Xendit.
- Integrasi Supabase Storage, S3, Cloudinary, atau external storage.
- Real courier API integration.
- Real-time chat customer support.
- Marketplace multi-seller.
- Recommendation engine.
- Loyalty point system.
- Complex stock reservation system.
- Multi-warehouse inventory.
- ERP integration.

Fitur tersebut bisa masuk fase lanjutan setelah core commerce stabil.

---

## 4. Target Users

### 4.1 Customer

Customer adalah user yang membeli produk.

Akses:

- Register dan login.
- Melihat produk.
- Melihat detail produk.
- Menambahkan produk ke cart.
- Checkout.
- Melakukan simulated payment.
- Melihat riwayat order.
- Melihat status order.

### 4.2 Admin

Admin adalah user internal yang mengelola data e-commerce.

Akses:

- Manage produk.
- Manage kategori.
- Manage varian produk.
- Manage stok.
- Manage order.
- Manage shipment.
- Melihat inventory log.
- Melihat dashboard summary.

### 4.3 Warehouse Staff

Warehouse staff adalah user internal untuk fulfillment.

Akses:

- Melihat order yang sudah paid.
- Update order menjadi packed.
- Input tracking number.
- Update shipment status.
- Melihat low stock alert.

### 4.4 Super Admin

Super admin memiliki akses penuh.

Akses:

- Semua akses admin.
- Manage user internal.
- Manage role.
- Melihat audit data sistem.

---

## 5. User Roles

Role utama:

```txt
CUSTOMER
ADMIN
WAREHOUSE
SUPER_ADMIN
```

Permission awal:

| Module | Customer | Admin | Warehouse | Super Admin |
|---|---:|---:|---:|---:|
| Product public read | Yes | Yes | Yes | Yes |
| Product write | No | Yes | No | Yes |
| Cart | Yes | No | No | No |
| Checkout | Yes | No | No | No |
| Own orders | Yes | No | No | No |
| All orders | No | Yes | Yes | Yes |
| Payment simulation | Yes | Admin optional | No | Yes |
| Shipment update | No | Yes | Yes | Yes |
| Inventory update | No | Yes | Limited | Yes |
| User management | No | No | No | Yes |

---

## 6. Core Backend Modules

## 6.1 Auth Module

### Features

- Register customer.
- Login user.
- Get current authenticated user.
- JWT access token.
- Password hashing using bcrypt.
- Role-based middleware.

### API

```txt
POST /api/v1/auth/register
POST /api/v1/auth/login
GET  /api/v1/auth/me
POST /api/v1/auth/logout
```

### Register Request

```json
{
  "name": "Nanda Surya",
  "email": "nanda@example.com",
  "password": "password123",
  "phone": "08123456789"
}
```

### Login Request

```json
{
  "email": "nanda@example.com",
  "password": "password123"
}
```

### Rules

- Email harus unique.
- Password minimal 8 karakter.
- Password disimpan sebagai hash.
- Customer register default role `CUSTOMER`.
- Admin user dibuat via seed atau super admin.

---

## 6.2 Category Module

### Features

- List category.
- Create category.
- Update category.
- Delete category.
- Slug unique.

### API

```txt
GET    /api/v1/categories
POST   /api/v1/admin/categories
PATCH  /api/v1/admin/categories/:id
DELETE /api/v1/admin/categories/:id
```

### Category Fields

```txt
id
name
slug
description
is_active
created_at
updated_at
```

---

## 6.3 Product Module

### Features

- Product listing.
- Product detail by slug.
- Product create/update/delete.
- Product status draft/published/archived.
- Product image upload using local storage.
- Product relation to category.

### API

```txt
GET    /api/v1/products
GET    /api/v1/products/:slug
POST   /api/v1/admin/products
PATCH  /api/v1/admin/products/:id
DELETE /api/v1/admin/products/:id
```

### Product Fields

```txt
id
category_id
name
slug
description
base_price
status
created_at
updated_at
```

### Product Status

```txt
DRAFT
PUBLISHED
ARCHIVED
```

### Product Listing Query

```txt
/api/v1/products?category=running&size=42&color=black&min_price=300000&max_price=1000000&sort=latest&page=1&limit=12
```

### Rules

- Public hanya bisa melihat product dengan status `PUBLISHED`.
- Admin bisa melihat semua status.
- Product slug harus unique.
- Product delete sebaiknya soft delete atau ubah status menjadi `ARCHIVED`.

---

## 6.4 Product Image Module

### Features

- Upload product image.
- Set image sort order.
- Delete image.
- Serve image statically from backend.

### API

```txt
POST   /api/v1/admin/products/:id/images
PATCH  /api/v1/admin/product-images/:id
DELETE /api/v1/admin/product-images/:id
```

### Local Storage Path

```txt
/uploads/products/{product_id}/{filename}
```

### Rules

- Allowed file type: jpg, jpeg, png, webp.
- Max size: 2MB per image for MVP.
- Backend menyimpan relative URL di database.

---

## 6.5 Product Variant & Inventory Module

### Features

- Create variant by product.
- Update variant.
- Update stock.
- View inventory list.
- Low stock alert.
- Inventory movement log.

### API

```txt
POST  /api/v1/admin/products/:id/variants
PATCH /api/v1/admin/variants/:id
PATCH /api/v1/admin/variants/:id/stock
GET   /api/v1/admin/inventory
GET   /api/v1/admin/inventory/logs
```

### Variant Fields

```txt
id
product_id
sku
size
color
price
stock
weight_gram
status
created_at
updated_at
```

### Variant Status

```txt
ACTIVE
INACTIVE
OUT_OF_STOCK
```

### Rules

- SKU harus unique.
- Stock tidak boleh minus.
- Size dan color wajib untuk footwear.
- Variant dengan stock 0 tidak bisa checkout.
- Setiap perubahan stok wajib membuat `inventory_logs`.

---

## 6.6 Address Module

### Features

- Customer add address.
- Customer update address.
- Customer delete address.
- Set default address.

### API

```txt
GET    /api/v1/addresses
POST   /api/v1/addresses
PATCH  /api/v1/addresses/:id
DELETE /api/v1/addresses/:id
PATCH  /api/v1/addresses/:id/default
```

### Address Fields

```txt
id
user_id
receiver_name
phone
province
city
district
postal_code
full_address
is_default
created_at
updated_at
```

### Rules

- Customer hanya bisa melihat dan mengubah address miliknya sendiri.
- Hanya boleh ada satu default address per user.

---

## 6.7 Cart Module

### Features

- Get cart.
- Add item to cart.
- Update quantity.
- Remove item.
- Clear cart after checkout.

### API

```txt
GET    /api/v1/cart
POST   /api/v1/cart/items
PATCH  /api/v1/cart/items/:id
DELETE /api/v1/cart/items/:id
```

### Rules

- Cart hanya untuk authenticated customer.
- Cart item menggunakan `product_variant_id`.
- Quantity minimal 1.
- Quantity tidak boleh lebih besar dari current stock.
- Jika item variant sudah ada di cart, quantity ditambahkan.

---

## 6.8 Checkout & Order Module

### Features

- Create order from cart.
- Save price snapshot.
- Save variant snapshot.
- Create payment record.
- Clear cart after order created.
- Customer view own orders.
- Admin view all orders.
- Admin update order status.

### API

```txt
POST  /api/v1/orders
GET   /api/v1/orders
GET   /api/v1/orders/:id
GET   /api/v1/admin/orders
GET   /api/v1/admin/orders/:id
PATCH /api/v1/admin/orders/:id/status
```

### Order Status

```txt
PENDING_PAYMENT
PROCESSING
PACKED
SHIPPED
DELIVERED
CANCELLED
```

### Payment Status

```txt
PENDING
PAID
FAILED
EXPIRED
CANCELLED
```

### Checkout Rules

Saat `POST /api/v1/orders`:

1. Ambil cart milik customer.
2. Validasi cart tidak kosong.
3. Validasi semua variant masih aktif.
4. Validasi stock cukup.
5. Hitung subtotal dari current variant price.
6. Hitung shipping cost dummy.
7. Hitung total amount.
8. Buat order dengan status `PENDING_PAYMENT`.
9. Buat order items dengan snapshot.
10. Buat payment dengan status `PENDING`.
11. Kosongkan cart.
12. Semua proses dibungkus dalam database transaction.

### Important Rule

Stock **belum dikurangi** saat order dibuat. Stock dikurangi saat payment success.

---

## 6.9 Simulated Payment Module

### Features

- Payment record generated after checkout.
- Customer can simulate successful payment.
- Customer can simulate failed payment.
- Payment success triggers stock deduction.
- Payment success updates order status.
- Payment success creates inventory logs and order status logs.

### API

```txt
POST /api/v1/payments/:order_id/simulate-success
POST /api/v1/payments/:order_id/simulate-failed
POST /api/v1/payments/:order_id/expire
GET  /api/v1/admin/payments
```

### Payment Success Rules

Saat `simulate-success`:

1. Validasi order milik customer.
2. Validasi order status masih `PENDING_PAYMENT`.
3. Validasi payment status masih `PENDING`.
4. Validasi stock ulang.
5. Update payment menjadi `PAID`.
6. Update order menjadi `PROCESSING`.
7. Kurangi stock product variants.
8. Buat inventory logs bertipe `SALE`.
9. Buat order status log.
10. Semua proses wajib menggunakan transaction.

### Payment Failed Rules

Saat `simulate-failed`:

- Payment status menjadi `FAILED`.
- Order tetap bisa dibiarkan `PENDING_PAYMENT` atau diubah ke `CANCELLED` tergantung policy.
- Untuk MVP, gunakan policy: order menjadi `CANCELLED`.

### Payment Expired Rules

Saat `expire`:

- Payment status menjadi `EXPIRED`.
- Order status menjadi `CANCELLED`.
- Tidak ada perubahan stok.

---

## 6.10 Shipment Module

### Features

- Admin/warehouse create shipment.
- Input courier.
- Input tracking number.
- Update shipment status.
- Customer view shipment status.

### API

```txt
POST  /api/v1/admin/orders/:id/shipment
PATCH /api/v1/admin/shipments/:id/status
GET   /api/v1/orders/:id/shipment
```

### Shipment Status

```txt
WAITING_FOR_PICKUP
PICKED_UP
IN_TRANSIT
OUT_FOR_DELIVERY
DELIVERED
FAILED_DELIVERY
```

### Rules

- Shipment hanya bisa dibuat kalau order minimal `PROCESSING` atau `PACKED`.
- Tracking number wajib sebelum status `SHIPPED`.
- Saat shipment menjadi `DELIVERED`, order status ikut menjadi `DELIVERED`.

---

## 6.11 Inventory Log Module

### Features

- Track all stock movements.
- View inventory history.
- Filter by product, variant, type, date.

### Inventory Log Types

```txt
RESTOCK
SALE
MANUAL_ADJUSTMENT
RETURN
CANCEL_CORRECTION
```

### Fields

```txt
id
product_variant_id
type
quantity
previous_stock
current_stock
note
created_by
created_at
```

### Rules

- Semua perubahan stock harus membuat inventory log.
- Quantity bisa positif atau negatif tergantung type.
- Previous stock dan current stock wajib disimpan untuk audit.

---

## 6.12 Order Status Log Module

### Features

- Track order lifecycle.
- Show status timeline on frontend.
- Admin can view status history.

### Fields

```txt
id
order_id
old_status
new_status
note
changed_by
created_at
```

### Rules

- Setiap perubahan order status wajib membuat log.
- Jika status berubah otomatis oleh system, `changed_by` boleh null dan note berisi `SYSTEM`.

---

## 6.13 Dashboard Summary Module

### Features

- Admin dashboard cards.
- Sales summary.
- Order status count.
- Low stock count.
- Best selling product.

### API

```txt
GET /api/v1/admin/dashboard/summary
GET /api/v1/admin/dashboard/sales
GET /api/v1/admin/dashboard/orders-by-status
GET /api/v1/admin/dashboard/low-stock
```

### MVP Metrics

```txt
Total revenue
Total orders
Pending payment orders
Processing orders
Delivered orders
Low stock variants
Best selling products
Recent orders
```

---

## 7. Global API Response Format

### Success Response

```json
{
  "success": true,
  "message": "Product retrieved successfully",
  "data": {}
}
```

### Paginated Response

```json
{
  "success": true,
  "message": "Products retrieved successfully",
  "data": [],
  "meta": {
    "page": 1,
    "limit": 12,
    "total": 120,
    "total_pages": 10
  }
}
```

### Error Response

```json
{
  "success": false,
  "message": "Validation error",
  "errors": [
    {
      "field": "email",
      "message": "Email is required"
    }
  ]
}
```

---

## 8. Error Handling

### Common HTTP Status

```txt
200 OK
201 Created
400 Bad Request
401 Unauthorized
403 Forbidden
404 Not Found
409 Conflict
422 Unprocessable Entity
500 Internal Server Error
```

### Error Examples

| Case | Status | Message |
|---|---:|---|
| Invalid login | 401 | Invalid email or password |
| Duplicate email | 409 | Email already registered |
| Product not found | 404 | Product not found |
| Stock insufficient | 422 | Insufficient stock |
| Invalid order status | 422 | Order cannot be paid in current status |
| Unauthorized access | 403 | You do not have permission |

---

## 9. Security Requirements

- Password hashing menggunakan bcrypt.
- JWT secret dari environment variable.
- Token expiration wajib.
- Admin endpoint wajib dilindungi role middleware.
- User hanya bisa akses resource miliknya sendiri.
- Validasi file upload untuk type dan size.
- Jangan expose password hash di response.
- Jangan trust price dari frontend.
- Semua perhitungan harga dilakukan di backend.
- CORS hanya allow frontend origin.
- Request body size dibatasi.

---

## 10. Environment Variables

```env
APP_ENV=development
APP_PORT=8080
APP_BASE_URL=http://localhost:8080
FRONTEND_URL=http://localhost:3000

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=footwear_commerce
DB_SSLMODE=disable

JWT_SECRET=change-me
JWT_EXPIRES_IN=24h

UPLOAD_DIR=./uploads
MAX_UPLOAD_SIZE_MB=2
```

---

## 11. Acceptance Criteria

Backend MVP dianggap selesai jika:

- Customer bisa register dan login.
- Admin bisa login.
- Product bisa dibuat dengan variant size/color.
- Customer bisa melihat product list dan product detail.
- Customer bisa add to cart.
- Customer bisa checkout dari cart.
- Order dan payment record terbentuk.
- Customer bisa simulate payment success.
- Payment success mengubah order menjadi processing.
- Payment success mengurangi stock.
- Inventory log terbentuk saat stock berkurang.
- Admin bisa melihat order.
- Admin/warehouse bisa update shipment.
- Customer bisa melihat order history.
- Semua protected endpoint memvalidasi JWT.
- Role-based access berjalan.
- API response konsisten.

---

## 12. Future Enhancements

Fase lanjutan:

- Midtrans/Xendit payment gateway.
- S3/Supabase Storage.
- Stock reservation.
- Coupon system.
- Return/refund module.
- Product review.
- Invoice PDF.
- Email notification.
- Webhook payment provider.
- Real courier API.
- Multi-warehouse support.
- Audit log for admin actions.
