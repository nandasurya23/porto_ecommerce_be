# Frontend API Guide - Footwear Backend

Dokumen ini dibuat untuk membantu frontend Next.js/React mengonsumsi API backend ini secara konsisten.

## 1. Base URL

### Local

```txt
API Base URL    : http://127.0.0.1:8080/api/v1
Asset Base URL  : http://127.0.0.1:8080
```

### Frontend env

Gunakan env berikut di frontend:

```env
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080/api/v1
NEXT_PUBLIC_ASSET_BASE_URL=http://127.0.0.1:8080
```

## 2. Response Format

### Success

```json
{
  "success": true,
  "message": "Success message",
  "data": {}
}
```

### Paginated

```json
{
  "success": true,
  "message": "Success message",
  "data": [],
  "meta": {
    "page": 1,
    "limit": 12,
    "total": 100,
    "total_pages": 9
  }
}
```

### Error

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

## 3. Authentication

Backend memakai JWT Bearer token.

### Header

```http
Authorization: Bearer <token>
Content-Type: application/json
```

### Token storage

- Simpan token di memory/store frontend yang aman.
- Jika pakai localStorage, pastikan hanya untuk MVP/dev.
- Semua protected request harus menyertakan `Authorization` header.

## 4. Public Endpoints

### Health

```txt
GET /health
```

### Auth

```txt
POST /auth/register
POST /auth/login
POST /auth/logout
GET  /auth/me
```

### Category

```txt
GET /categories
```

### Product

```txt
GET /products
GET /products/:slug
```

#### Product detail shape

Product detail mengembalikan nested data:

```json
{
  "id": "product-id",
  "name": "Adis Runner Pro",
  "slug": "adis-runner-pro",
  "base_price": 799000,
  "images": [
    {
      "image_url": "/uploads/products/.../image.jpg",
      "is_primary": true
    }
  ],
  "variants": [
    {
      "id": "variant-id",
      "sku": "ARP-BLK-42",
      "size": "42",
      "color": "Black",
      "price": 799000,
      "stock": 8
    }
  ]
}
```

### Image URL

`image_url` dari backend bersifat relative. Frontend harus prefix dengan `NEXT_PUBLIC_ASSET_BASE_URL`.

Contoh:

```ts
const fullImageUrl = `${process.env.NEXT_PUBLIC_ASSET_BASE_URL}${imageUrl}`;
```

## 5. Customer Endpoints

Semua endpoint ini butuh JWT dan role `CUSTOMER`.

### Address

```txt
GET    /addresses
POST   /addresses
PATCH  /addresses/:id
DELETE /addresses/:id
```

### Cart

```txt
GET    /cart
POST   /cart/items
PATCH  /cart/items/:id
DELETE /cart/items/:id
DELETE /cart/items
```

#### Add cart item payload

```json
{
  "product_variant_id": "uuid",
  "quantity": 2
}
```

#### Update cart item payload

```json
{
  "quantity": 1
}
```

### Order

```txt
POST /orders
GET  /orders
GET  /orders/:id
GET  /orders/:id/shipment
```

#### Checkout payload

```json
{
  "address_id": "uuid",
  "payment_method": "BANK_TRANSFER"
}
```

#### Payment method values

```txt
BANK_TRANSFER
VIRTUAL_ACCOUNT
QRIS_SIMULATION
COD
```

### Simulated payment

```txt
POST /payments/:order_id/simulate-success
POST /payments/:order_id/simulate-failed
POST /payments/:order_id/expire
```

## 6. Admin and Warehouse Endpoints

### Access

- `ADMIN`
- `SUPER_ADMIN`
- `WAREHOUSE` only for shipment-related actions

### Category

```txt
GET    /admin/categories
POST   /admin/categories
PATCH  /admin/categories/:id
DELETE /admin/categories/:id
```

### Product

```txt
GET    /admin/products
POST   /admin/products
PATCH  /admin/products/:id
DELETE /admin/products/:id
POST   /admin/products/:id/images
POST   /admin/products/:id/variants
PATCH  /admin/variants/:id
PATCH  /admin/variants/:id/stock
GET    /admin/inventory/logs
```

### Orders / Payments

```txt
GET   /admin/orders
PATCH /admin/orders/:id/status
GET   /admin/payments
```

### Shipment

```txt
POST  /admin/orders/:id/shipment
PATCH /admin/shipments/:id/status
```

### Dashboard

```txt
GET /admin/dashboard/summary
GET /admin/dashboard/sales
GET /admin/dashboard/orders-by-status
GET /admin/dashboard/low-stock
```

## 7. Suggested Frontend Flow

### Initial load

1. Fetch categories and public products.
2. Fetch product detail by slug for product page.
3. If token exists, call `GET /auth/me` to restore session.

### Customer checkout flow

1. Register or login.
2. Fetch addresses and select default address.
3. Add variant to cart.
4. Open cart and validate quantity/price display.
5. Call `POST /orders`.
6. Show payment code / order summary.
7. Call payment simulation endpoint only for demo.
8. Refresh order detail and shipment status.

### Admin flow

1. Login as admin.
2. Fetch dashboard summary.
3. Manage category/product/variant.
4. Review orders and payments.
5. Create shipment after payment is `PAID`.
6. Update shipment status until `DELIVERED`.

## 8. Practical Notes

- `logout` endpoint currently returns success response, but it does not revoke JWT server-side.
- Product/public listing is paginated.
- `POST /orders` calculates total from backend DB price, not from frontend.
- Payment success deducts stock and writes inventory logs.
- Shipment delivered will sync order status to `DELIVERED`.

## 9. Local Development With Colima

If you run this backend locally with Colima:

```bash
colima start
docker compose up -d
go run ./cmd/api
```

Important:

- This repo maps PostgreSQL to host port `5433` to avoid conflict with any local service already using `5432`.
- The backend reads `.env` automatically if present.
- If you need a different DB port, override `DB_PORT` in `.env`.

## 10. Minimal Axios Example

```ts
import axios from "axios";

export const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_BASE_URL,
});

api.interceptors.request.use((config) => {
  const token = typeof window !== "undefined" ? localStorage.getItem("token") : null;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});
```
