# Backend Architecture — Footwear E-Commerce Operations Platform

## 1. Architecture Overview

Backend untuk **Footwear E-Commerce Operations Platform** dibangun sebagai REST API menggunakan **Golang**. Backend bertanggung jawab untuk authentication, product catalog, product variant inventory, cart, checkout, simulated payment, order lifecycle, shipment, inventory log, dan admin dashboard.

Arsitektur backend menggunakan pendekatan **modular layered architecture** agar kode mudah dibaca, mudah dites, dan mudah dikembangkan.

```txt
Next.js Frontend
        ↓ HTTP REST API
Golang Backend API
        ↓
PostgreSQL Database
        ↓
Local File Storage
```

Untuk MVP, backend tidak memakai external payment gateway dan tidak memakai external object storage. Payment dibuat simulated, sedangkan upload file disimpan di local storage.

---

## 2. Recommended Tech Stack

```txt
Language        : Golang
HTTP Framework  : Gin
Database        : PostgreSQL
Query Layer     : sqlc + pgx or pgx manual
Migration       : Goose / golang-migrate
Auth            : JWT + bcrypt
Validation      : go-playground/validator
Config          : godotenv / envconfig
Logger          : slog / zap
File Storage    : Local filesystem
Container       : Docker Compose for local PostgreSQL
```

### Recommended Choice for This Project

```txt
Gin + PostgreSQL + sqlc + pgx
```

Reason:

- Gin simple dan cepat untuk REST API.
- PostgreSQL cocok untuk relational e-commerce data.
- sqlc memberi type-safe query dari SQL file.
- pgx adalah PostgreSQL driver yang kuat untuk Go.
- Cocok untuk portfolio karena lebih production-minded dibanding CRUD ORM sederhana.

---

## 3. High-Level Backend Responsibilities

Backend bertanggung jawab untuk:

```txt
- Authentication and authorization
- Role-based access control
- Product and category management
- Product variant inventory by size and color
- Cart management
- Transaction-safe checkout
- Simulated payment workflow
- Stock deduction after payment success
- Inventory movement log
- Order status lifecycle
- Shipment management
- Dashboard summary API
- Local file upload and static file serving
```

Backend tidak bertanggung jawab untuk:

```txt
- UI rendering
- Client-side state management
- Payment gateway real integration in MVP
- Real courier API in MVP
- Cloud object storage in MVP
```

---

## 4. Project Folder Structure

```txt
backend/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── app.go
│   ├── config/
│   │   └── config.go
│   ├── database/
│   │   ├── db.go
│   │   └── transaction.go
│   ├── middleware/
│   │   ├── auth_middleware.go
│   │   ├── role_middleware.go
│   │   ├── cors_middleware.go
│   │   └── error_middleware.go
│   ├── modules/
│   │   ├── auth/
│   │   │   ├── handler.go
│   │   │   ├── service.go
│   │   │   ├── repository.go
│   │   │   ├── dto.go
│   │   │   └── routes.go
│   │   ├── users/
│   │   ├── categories/
│   │   ├── products/
│   │   ├── variants/
│   │   ├── cart/
│   │   ├── orders/
│   │   ├── payments/
│   │   ├── shipments/
│   │   ├── inventory/
│   │   └── dashboard/
│   ├── shared/
│   │   ├── response/
│   │   │   └── response.go
│   │   ├── errors/
│   │   │   └── app_error.go
│   │   ├── validator/
│   │   │   └── validator.go
│   │   ├── pagination/
│   │   │   └── pagination.go
│   │   ├── uploader/
│   │   │   └── local_uploader.go
│   │   └── utils/
│   └── sqlc/
│       ├── queries/
│       └── generated/
├── migrations/
├── uploads/
│   └── products/
├── docs/
├── docker-compose.yml
├── sqlc.yaml
├── go.mod
└── go.sum
```

---

## 5. Layered Architecture

Setiap module menggunakan 4 layer utama:

```txt
Routes → Handler → Service → Repository → Database
```

### 5.1 Routes

Routes bertugas mendaftarkan endpoint ke Gin router.

Responsibilities:

```txt
- Define endpoint path
- Attach middleware
- Group public/admin/customer routes
```

Example:

```txt
/api/v1/products
/api/v1/admin/products
```

---

### 5.2 Handler Layer

Handler adalah layer HTTP.

Responsibilities:

```txt
- Bind request body/query/path params
- Validate request format
- Get auth context from middleware
- Call service
- Return standardized response
```

Handler tidak boleh:

```txt
- Menulis SQL query
- Mengandung business logic besar
- Menghitung total order
- Mengurangi stock
- Mengakses database langsung
```

---

### 5.3 Service Layer

Service adalah tempat business logic.

Responsibilities:

```txt
- Business rule validation
- Checkout calculation
- Payment confirmation flow
- Stock deduction rules
- Order status transition rules
- Call repository
- Manage database transaction via transaction manager
```

Contoh business logic di service:

```txt
- Customer tidak bisa checkout jika cart kosong
- Quantity tidak boleh melebihi stock
- Stock dikurangi hanya saat payment success
- Shipment tidak bisa dibuat untuk unpaid order
```

---

### 5.4 Repository Layer

Repository adalah layer database access.

Responsibilities:

```txt
- Execute SQL query
- Insert/update/select data
- Return domain data to service
```

Repository tidak boleh:

```txt
- Mengatur HTTP response
- Mengandung logic role/permission
- Mengandung flow checkout penuh
```

Jika memakai sqlc, repository akan membungkus generated query agar service tidak terlalu bergantung pada detail query generated.

---

## 6. Module Boundaries

## 6.1 Auth Module

Responsibilities:

```txt
- Register customer
- Login user
- Hash password
- Verify password
- Generate JWT token
- Return current user
```

Dependencies:

```txt
- users repository
- jwt utility
- bcrypt utility
```

---

## 6.2 Products Module

Responsibilities:

```txt
- Product listing
- Product detail
- Product create/update/archive
- Product image relation
- Product category relation
```

Dependencies:

```txt
- categories repository
- product images repository
- uploader utility
```

---

## 6.3 Variants / Inventory Module

Responsibilities:

```txt
- Manage product variants
- Update variant stock
- Create inventory logs
- Provide low stock data
```

Rules:

```txt
- SKU unique
- Stock cannot be negative
- All stock changes must create inventory log
```

---

## 6.4 Cart Module

Responsibilities:

```txt
- Get active cart
- Add variant to cart
- Update cart item quantity
- Remove cart item
- Clear cart after checkout
```

Rules:

```txt
- Only customer can own cart
- Quantity cannot exceed current stock
- Cart item references product_variant_id
```

---

## 6.5 Orders Module

Responsibilities:

```txt
- Create order from cart
- Create order items snapshot
- Create payment record
- List customer orders
- List admin orders
- Update order status
- Create order status logs
```

Rules:

```txt
- Checkout must use database transaction
- Price must be calculated from database, not frontend
- Order items must store product snapshot
- Stock not deducted during checkout
```

---

## 6.6 Payments Module

Responsibilities:

```txt
- Simulate payment success
- Simulate payment failed
- Expire payment
- Update payment status
- Trigger stock deduction
- Trigger order status update
```

Rules:

```txt
- Payment success must use database transaction
- Payment success only valid for PENDING payment
- Payment success only valid for PENDING_PAYMENT order
- Stock deducted only once
```

---

## 6.7 Shipments Module

Responsibilities:

```txt
- Create shipment
- Update shipment status
- Store courier and tracking number
- Sync delivered shipment with delivered order
```

Rules:

```txt
- Shipment cannot be created for unpaid order
- Tracking number required for shipped status
```

---

## 7. Request Lifecycle

### 7.1 Authenticated Request Lifecycle

```txt
Client request
  ↓
CORS middleware
  ↓
JWT auth middleware
  ↓
Role middleware if needed
  ↓
Handler bind & validate
  ↓
Service business logic
  ↓
Repository query
  ↓
Database
  ↓
Standardized response
```

---

## 8. Checkout Flow Architecture

Checkout adalah salah satu flow paling penting.

```txt
POST /api/v1/orders
        ↓
Auth Middleware validates customer
        ↓
Order Handler validates address/payment method
        ↓
Order Service starts transaction
        ↓
Get user's cart items
        ↓
Validate cart not empty
        ↓
Validate variant active and stock enough
        ↓
Calculate subtotal from DB price
        ↓
Calculate dummy shipping cost
        ↓
Create order PENDING_PAYMENT
        ↓
Create order items with snapshot
        ↓
Create payment PENDING
        ↓
Clear cart
        ↓
Commit transaction
        ↓
Return order + payment info
```

### Important Principle

Frontend tidak boleh mengirim final price sebagai sumber kebenaran. Frontend hanya mengirim address/payment method. Backend mengambil harga dari database dan menghitung total sendiri.

---

## 9. Simulated Payment Flow Architecture

```txt
POST /api/v1/payments/:order_id/simulate-success
        ↓
Auth middleware validates customer
        ↓
Payment Handler calls service
        ↓
Payment Service starts transaction
        ↓
Find order and payment
        ↓
Validate order is PENDING_PAYMENT
        ↓
Validate payment is PENDING
        ↓
Validate stock again
        ↓
Update payment to PAID
        ↓
Update order to PROCESSING
        ↓
Deduct variant stock
        ↓
Create inventory log for each item
        ↓
Create order status log
        ↓
Commit transaction
        ↓
Return payment success response
```

### Why Stock Deducted After Payment Success?

Karena untuk MVP, sistem tidak memakai stock reservation. Jika stock langsung dikurangi saat checkout, stok bisa terkunci oleh customer yang tidak membayar.

---

## 10. Database Transaction Strategy

Flow yang wajib pakai transaction:

```txt
- Checkout / create order
- Simulated payment success
- Manual stock adjustment
- Shipment delivered status sync
- Cancel order with stock correction in future phase
```

Transaction helper:

```txt
WithTx(ctx, func(q *Queries) error {
    // transactional queries here
})
```

Rules:

```txt
- Jangan melakukan partial update pada checkout.
- Jika salah satu order item gagal dibuat, seluruh order rollback.
- Jika stock deduction gagal, payment tidak boleh menjadi PAID.
- Jika inventory log gagal dibuat, stock update harus rollback.
```

---

## 11. Authentication Architecture

### Token Type

MVP menggunakan access token JWT.

```txt
Authorization: Bearer <token>
```

JWT payload minimal:

```json
{
  "user_id": "uuid",
  "email": "user@example.com",
  "role": "CUSTOMER",
  "exp": 1710000000
}
```

### Middleware Responsibilities

Auth middleware:

```txt
- Read Authorization header
- Validate Bearer token
- Parse claims
- Attach user context to Gin context
```

Role middleware:

```txt
- Read role from context
- Check allowed roles
- Return 403 if role not allowed
```

---

## 12. Authorization Rules

### Customer Resource Ownership

Customer hanya boleh akses:

```txt
- Own cart
- Own addresses
- Own orders
- Own payment simulation for own order
- Own shipment detail from own order
```

### Admin Access

Admin boleh akses:

```txt
- Product management
- Inventory management
- All orders
- All payments
- Shipment management
- Dashboard summary
```

### Warehouse Access

Warehouse boleh akses:

```txt
- Paid orders
- Shipment update
- Packing status
- Low stock view
```

---

## 13. API Versioning

Gunakan prefix:

```txt
/api/v1
```

Tujuannya agar di masa depan bisa membuat breaking change:

```txt
/api/v2
```

---

## 14. Route Grouping

```txt
Public Routes:
GET  /api/v1/products
GET  /api/v1/products/:slug
GET  /api/v1/categories
POST /api/v1/auth/register
POST /api/v1/auth/login

Customer Routes:
GET    /api/v1/cart
POST   /api/v1/cart/items
POST   /api/v1/orders
GET    /api/v1/orders
POST   /api/v1/payments/:order_id/simulate-success

Admin Routes:
POST   /api/v1/admin/products
PATCH  /api/v1/admin/products/:id
GET    /api/v1/admin/orders
PATCH  /api/v1/admin/orders/:id/status
GET    /api/v1/admin/dashboard/summary

Warehouse Routes:
GET   /api/v1/warehouse/orders
PATCH /api/v1/admin/shipments/:id/status
```

---

## 15. Response Architecture

Semua response menggunakan wrapper konsisten.

### Success

```json
{
  "success": true,
  "message": "Order created successfully",
  "data": {}
}
```

### Error

```json
{
  "success": false,
  "message": "Insufficient stock",
  "errors": []
}
```

### Pagination

```json
{
  "success": true,
  "message": "Products retrieved successfully",
  "data": [],
  "meta": {
    "page": 1,
    "limit": 12,
    "total": 100,
    "total_pages": 9
  }
}
```

---

## 16. Validation Strategy

Validation dilakukan di 2 level:

### 16.1 DTO Validation

Contoh:

```txt
- email required and valid
- password min 8
- quantity min 1
- product name required
- price must be greater than 0
```

### 16.2 Business Rule Validation

Contoh:

```txt
- SKU must be unique
- Stock must be enough
- Order must be PENDING_PAYMENT before payment success
- Shipment cannot be created for unpaid order
```

DTO validation dilakukan dekat handler. Business validation dilakukan di service.

---

## 17. File Upload Architecture

MVP menggunakan local storage.

```txt
uploads/
├── products/
│   └── {product_id}/
├── returns/
└── custom-orders/
```

Gin static route:

```txt
/uploads/*filepath
```

Database hanya menyimpan URL/path:

```txt
/uploads/products/{product_id}/image.webp
```

Rules:

```txt
- Validate MIME type
- Validate extension
- Limit file size
- Generate safe filename
- Never trust original filename fully
```

---

## 18. Logging Strategy

Gunakan structured logging untuk:

```txt
- Server start
- Database connection error
- Auth failure suspicious cases
- Checkout failure
- Payment simulation result
- Internal server error
```

Jangan log:

```txt
- Plain password
- Full JWT token
- Sensitive credential
```

---

## 19. Configuration Architecture

Config dibaca dari environment variables.

```txt
internal/config/config.go
```

Config groups:

```txt
AppConfig
DatabaseConfig
JWTConfig
UploadConfig
CORSConfig
```

Rules:

```txt
- Jangan hardcode credential.
- Gunakan .env hanya untuk local development.
- Production pakai environment variable deployment platform.
```

---

## 20. Migration Strategy

Gunakan folder:

```txt
migrations/
├── 001_create_users.sql
├── 002_create_products.sql
├── 003_create_orders.sql
└── ...
```

Rules:

```txt
- Migration harus bisa dijalankan dari kosong.
- Jangan edit migration lama setelah sudah dipakai.
- Tambahkan migration baru untuk perubahan schema.
- Seed data dipisah dari migration utama.
```

---

## 21. Seed Data Strategy

Seed awal:

```txt
- Super admin user
- Admin user
- Warehouse user
- Sample categories
- Sample products
- Sample variants
```

Tujuan seed:

```txt
- Memudahkan demo portfolio
- Memudahkan testing frontend
- Memudahkan reviewer mencoba sistem
```

---

## 22. Testing Strategy

MVP minimal:

```txt
- Unit test service checkout
- Unit test payment simulation
- Unit test stock validation
- Unit test role guard
- Integration test auth login
- Integration test create order
```

Test case penting:

```txt
- Cannot checkout empty cart
- Cannot checkout quantity greater than stock
- Payment success deducts stock once
- Payment success cannot be repeated
- Customer cannot access another user's order
- Admin can access all orders
```

---

## 23. Docker Local Development

Docker Compose digunakan untuk PostgreSQL.

```txt
services:
  postgres:
    image: postgres:16
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: footwear_commerce
```

Backend bisa dijalankan lokal:

```txt
go run ./cmd/api
```

---

## 24. Performance Considerations

Untuk MVP:

```txt
- Pagination wajib untuk product dan order list.
- Index untuk slug, SKU, order_number, user_id.
- Jangan load semua order tanpa limit.
- Gunakan query aggregate untuk dashboard.
- Hindari N+1 query pada product listing.
```

---

## 25. Backend Best Practices

- Gunakan context pada database query.
- Gunakan transaction untuk flow atomic.
- Jangan trust data harga dari frontend.
- Jangan letakkan business logic di handler.
- Jangan expose internal DB error mentah ke client.
- Gunakan UUID untuk primary key public-facing.
- Gunakan order number yang readable untuk customer.
- Simpan snapshot order item agar histori order tidak berubah saat product berubah.
- Buat inventory log untuk setiap perubahan stok.
- Buat order status log untuk setiap perubahan lifecycle order.

---

## 26. Future Architecture Upgrade

Setelah MVP stabil, backend bisa dikembangkan dengan:

```txt
- Payment provider adapter interface
- S3 storage adapter
- Courier provider adapter
- Background worker for email and invoice
- Redis for caching/session/rate limit
- Stock reservation system
- Webhook handler for real gateway
- Audit log module
```

Recommended future abstraction:

```txt
PaymentProvider interface
StorageProvider interface
CourierProvider interface
NotificationProvider interface
```

Dengan begitu simulated payment bisa diganti Midtrans/Xendit tanpa merusak order service utama.
