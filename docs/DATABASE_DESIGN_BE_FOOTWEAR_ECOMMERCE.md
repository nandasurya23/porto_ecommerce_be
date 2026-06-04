# Database Design Backend — Footwear E-Commerce Operations Platform

## 1. Database Overview

Database untuk **Footwear E-Commerce Operations Platform** menggunakan **PostgreSQL**. Model data dirancang untuk mendukung e-commerce footwear dengan kebutuhan utama:

```txt
- User authentication
- Role-based access
- Product catalog
- Product variant by size and color
- Stock inventory
- Cart
- Checkout
- Order lifecycle
- Simulated payment
- Shipment
- Inventory log
- Order status log
```

Fokus utama desain database adalah menjaga data tetap konsisten, terutama pada flow checkout, payment success, dan stock deduction.

---

## 2. Database Principles

### 2.1 Use UUID Primary Key

Gunakan UUID untuk primary key agar aman untuk public-facing resource.

```sql
id UUID PRIMARY KEY DEFAULT gen_random_uuid()
```

### 2.2 Use Snapshot for Order Items

Order item harus menyimpan snapshot data product saat transaksi dibuat.

Alasan:

- Nama produk bisa berubah.
- Harga produk bisa berubah.
- Varian bisa diubah admin.
- Riwayat order customer harus tetap benar.

### 2.3 Do Not Trust Frontend Price

Frontend tidak boleh menjadi sumber final price. Backend mengambil harga dari `product_variants.price` saat checkout.

### 2.4 Stock Deducted After Payment Success

Untuk MVP, stock dikurangi saat payment success, bukan saat checkout.

### 2.5 Every Stock Change Must Have Inventory Log

Setiap perubahan stock harus tercatat di `inventory_logs`.

### 2.6 Every Order Status Change Should Have Status Log

Setiap perubahan order status sebaiknya tercatat di `order_status_logs`.

---

## 3. Entity Relationship Summary

```txt
users 1---N addresses
users 1---1 carts
users 1---N orders

categories 1---N products
products 1---N product_images
products 1---N product_variants

carts 1---N cart_items
product_variants 1---N cart_items

orders 1---N order_items
orders 1---1 payments
orders 1---1 shipments
orders 1---N order_status_logs

product_variants 1---N order_items
product_variants 1---N inventory_logs
```

---

## 4. Enum Definitions

PostgreSQL enum yang disarankan.

```sql
CREATE TYPE user_role AS ENUM (
  'CUSTOMER',
  'ADMIN',
  'WAREHOUSE',
  'SUPER_ADMIN'
);

CREATE TYPE product_status AS ENUM (
  'DRAFT',
  'PUBLISHED',
  'ARCHIVED'
);

CREATE TYPE variant_status AS ENUM (
  'ACTIVE',
  'INACTIVE',
  'OUT_OF_STOCK'
);

CREATE TYPE order_status AS ENUM (
  'PENDING_PAYMENT',
  'PROCESSING',
  'PACKED',
  'SHIPPED',
  'DELIVERED',
  'CANCELLED'
);

CREATE TYPE payment_status AS ENUM (
  'PENDING',
  'PAID',
  'FAILED',
  'EXPIRED',
  'CANCELLED'
);

CREATE TYPE payment_method AS ENUM (
  'BANK_TRANSFER',
  'VIRTUAL_ACCOUNT',
  'QRIS_SIMULATION',
  'COD'
);

CREATE TYPE shipment_status AS ENUM (
  'WAITING_FOR_PICKUP',
  'PICKED_UP',
  'IN_TRANSIT',
  'OUT_FOR_DELIVERY',
  'DELIVERED',
  'FAILED_DELIVERY'
);

CREATE TYPE inventory_log_type AS ENUM (
  'RESTOCK',
  'SALE',
  'MANUAL_ADJUSTMENT',
  'RETURN',
  'CANCEL_CORRECTION'
);
```

---

## 5. Tables

## 5.1 users

Menyimpan data user untuk customer dan internal team.

```sql
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(120) NOT NULL,
  email VARCHAR(160) NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  phone VARCHAR(30),
  role user_role NOT NULL DEFAULT 'CUSTOMER',
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Notes

- Password tidak pernah disimpan plain text.
- Admin/warehouse/super admin bisa dibuat lewat seed.
- `is_active` untuk disable user tanpa delete data.

### Indexes

```sql
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_is_active ON users(is_active);
```

---

## 5.2 addresses

Menyimpan alamat customer.

```sql
CREATE TABLE addresses (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  receiver_name VARCHAR(120) NOT NULL,
  phone VARCHAR(30) NOT NULL,
  province VARCHAR(100) NOT NULL,
  city VARCHAR(100) NOT NULL,
  district VARCHAR(100),
  postal_code VARCHAR(20),
  full_address TEXT NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_addresses_user_id ON addresses(user_id);
```

### Rule

Hanya satu address default per user. Bisa enforced di service layer atau partial unique index.

```sql
CREATE UNIQUE INDEX unique_default_address_per_user
ON addresses(user_id)
WHERE is_default = TRUE;
```

---

## 5.3 categories

Menyimpan kategori produk.

```sql
CREATE TABLE categories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(120) NOT NULL,
  slug VARCHAR(160) NOT NULL UNIQUE,
  description TEXT,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Sample Categories

```txt
Running Shoes
Lifestyle Shoes
Training Shoes
Football Shoes
Sandals
Limited Edition
Custom Footwear
```

---

## 5.4 products

Menyimpan produk utama. Satu produk bisa memiliki banyak varian ukuran dan warna.

```sql
CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
  name VARCHAR(180) NOT NULL,
  slug VARCHAR(220) NOT NULL UNIQUE,
  description TEXT,
  base_price NUMERIC(12,2) NOT NULL CHECK (base_price >= 0),
  status product_status NOT NULL DEFAULT 'DRAFT',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_status ON products(status);
CREATE INDEX idx_products_created_at ON products(created_at DESC);
```

---

## 5.5 product_images

Menyimpan gambar produk.

```sql
CREATE TABLE product_images (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  image_url TEXT NOT NULL,
  alt_text VARCHAR(180),
  sort_order INT NOT NULL DEFAULT 0,
  is_primary BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_product_images_product_id ON product_images(product_id);
CREATE INDEX idx_product_images_sort_order ON product_images(product_id, sort_order);
```

### Rule

Hanya satu primary image per product. Bisa enforced di service atau partial unique index.

```sql
CREATE UNIQUE INDEX unique_primary_image_per_product
ON product_images(product_id)
WHERE is_primary = TRUE;
```

---

## 5.6 product_variants

Menyimpan varian produk berdasarkan size dan color.

```sql
CREATE TABLE product_variants (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  sku VARCHAR(80) NOT NULL UNIQUE,
  size VARCHAR(20) NOT NULL,
  color VARCHAR(80) NOT NULL,
  price NUMERIC(12,2) NOT NULL CHECK (price >= 0),
  stock INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
  weight_gram INT NOT NULL DEFAULT 0 CHECK (weight_gram >= 0),
  status variant_status NOT NULL DEFAULT 'ACTIVE',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_product_variants_product_id ON product_variants(product_id);
CREATE INDEX idx_product_variants_size ON product_variants(size);
CREATE INDEX idx_product_variants_color ON product_variants(color);
CREATE INDEX idx_product_variants_stock ON product_variants(stock);
CREATE INDEX idx_product_variants_status ON product_variants(status);
```

### Rules

- SKU unique.
- Stock tidak boleh minus.
- Size dan color wajib.
- Jika stock 0, service bisa mengubah status menjadi `OUT_OF_STOCK`.

---

## 5.7 carts

Menyimpan cart aktif user.

```sql
CREATE TABLE carts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Notes

- Satu user hanya punya satu active cart.
- Cart item dihapus setelah order dibuat.

---

## 5.8 cart_items

Menyimpan item dalam cart.

```sql
CREATE TABLE cart_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cart_id UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
  product_variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
  quantity INT NOT NULL CHECK (quantity > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(cart_id, product_variant_id)
);
```

### Indexes

```sql
CREATE INDEX idx_cart_items_cart_id ON cart_items(cart_id);
CREATE INDEX idx_cart_items_product_variant_id ON cart_items(product_variant_id);
```

### Notes

- Price tidak wajib disimpan di cart karena final price dihitung saat checkout.
- Quantity divalidasi terhadap stock di service.

---

## 5.9 orders

Menyimpan order utama.

```sql
CREATE TABLE orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  address_id UUID REFERENCES addresses(id) ON DELETE SET NULL,
  order_number VARCHAR(40) NOT NULL UNIQUE,
  subtotal NUMERIC(12,2) NOT NULL CHECK (subtotal >= 0),
  shipping_cost NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (shipping_cost >= 0),
  discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
  total_amount NUMERIC(12,2) NOT NULL CHECK (total_amount >= 0),
  payment_status payment_status NOT NULL DEFAULT 'PENDING',
  order_status order_status NOT NULL DEFAULT 'PENDING_PAYMENT',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_order_status ON orders(order_status);
CREATE INDEX idx_orders_payment_status ON orders(payment_status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);
CREATE INDEX idx_orders_order_number ON orders(order_number);
```

### Notes

- `order_number` readable untuk customer, contoh `ORD-2026-000001`.
- `address_id` boleh null jika address dihapus, tapi lebih baik address snapshot ditambahkan di masa depan.

### Recommended Future Improvement

Tambahkan address snapshot:

```txt
shipping_receiver_name
shipping_phone
shipping_full_address
shipping_city
shipping_province
shipping_postal_code
```

Ini menjaga order history tetap akurat meskipun alamat user berubah.

---

## 5.10 order_items

Menyimpan item order dengan snapshot.

```sql
CREATE TABLE order_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  product_variant_id UUID REFERENCES product_variants(id) ON DELETE SET NULL,
  product_name_snapshot VARCHAR(180) NOT NULL,
  product_slug_snapshot VARCHAR(220),
  sku_snapshot VARCHAR(80) NOT NULL,
  size_snapshot VARCHAR(20) NOT NULL,
  color_snapshot VARCHAR(80) NOT NULL,
  price_snapshot NUMERIC(12,2) NOT NULL CHECK (price_snapshot >= 0),
  quantity INT NOT NULL CHECK (quantity > 0),
  subtotal NUMERIC(12,2) NOT NULL CHECK (subtotal >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_variant_id ON order_items(product_variant_id);
```

### Rules

- Snapshot wajib diisi saat checkout.
- `product_variant_id` boleh null jika variant dihapus di masa depan.
- `subtotal = price_snapshot * quantity` dihitung backend.

---

## 5.11 payments

Menyimpan data pembayaran simulated.

```sql
CREATE TABLE payments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
  payment_code VARCHAR(60) NOT NULL UNIQUE,
  method payment_method NOT NULL,
  amount NUMERIC(12,2) NOT NULL CHECK (amount >= 0),
  status payment_status NOT NULL DEFAULT 'PENDING',
  paid_at TIMESTAMPTZ,
  expired_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_payments_order_id ON payments(order_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_payment_code ON payments(payment_code);
```

### Notes

- Untuk MVP, payment gateway belum ada.
- `payment_code` bisa seperti `PAY-2026-000001`.
- Di masa depan bisa tambah:

```txt
provider
provider_reference_id
provider_payload
webhook_received_at
```

---

## 5.12 shipments

Menyimpan pengiriman order.

```sql
CREATE TABLE shipments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
  courier VARCHAR(80) NOT NULL,
  service_name VARCHAR(80),
  tracking_number VARCHAR(120),
  status shipment_status NOT NULL DEFAULT 'WAITING_FOR_PICKUP',
  shipped_at TIMESTAMPTZ,
  delivered_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_shipments_order_id ON shipments(order_id);
CREATE INDEX idx_shipments_status ON shipments(status);
CREATE INDEX idx_shipments_tracking_number ON shipments(tracking_number);
```

### Rules

- Satu order punya satu shipment untuk MVP.
- Tracking number wajib sebelum order dianggap shipped.
- Saat shipment delivered, order status ikut delivered.

---

## 5.13 inventory_logs

Mencatat semua perubahan stock.

```sql
CREATE TABLE inventory_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
  type inventory_log_type NOT NULL,
  quantity INT NOT NULL,
  previous_stock INT NOT NULL CHECK (previous_stock >= 0),
  current_stock INT NOT NULL CHECK (current_stock >= 0),
  note TEXT,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_inventory_logs_variant_id ON inventory_logs(product_variant_id);
CREATE INDEX idx_inventory_logs_type ON inventory_logs(type);
CREATE INDEX idx_inventory_logs_created_at ON inventory_logs(created_at DESC);
```

### Quantity Convention

```txt
RESTOCK           : positive quantity
SALE              : negative quantity
MANUAL_ADJUSTMENT : positive or negative
RETURN            : positive quantity
CANCEL_CORRECTION : positive quantity
```

Example sale:

```txt
previous_stock = 10
quantity = -2
current_stock = 8
```

---

## 5.14 order_status_logs

Mencatat perubahan status order.

```sql
CREATE TABLE order_status_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  old_status order_status,
  new_status order_status NOT NULL,
  note TEXT,
  changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Indexes

```sql
CREATE INDEX idx_order_status_logs_order_id ON order_status_logs(order_id);
CREATE INDEX idx_order_status_logs_created_at ON order_status_logs(created_at DESC);
```

### Notes

- `old_status` boleh null untuk initial status log.
- Jika perubahan dilakukan sistem, `changed_by` boleh null.

---

## 6. Optional Phase 2 Tables

Table ini belum wajib untuk MVP, tapi disiapkan untuk fase lanjut.

---

## 6.1 returns

```sql
CREATE TYPE return_status AS ENUM (
  'REQUESTED',
  'APPROVED',
  'REJECTED',
  'WAITING_FOR_ITEM',
  'ITEM_RECEIVED',
  'REFUNDED',
  'EXCHANGED',
  'CLOSED'
);

CREATE TABLE returns (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  reason TEXT NOT NULL,
  note TEXT,
  status return_status NOT NULL DEFAULT 'REQUESTED',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 6.2 return_items

```sql
CREATE TABLE return_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  return_id UUID NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
  order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE RESTRICT,
  quantity INT NOT NULL CHECK (quantity > 0),
  photo_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 6.3 coupons

```sql
CREATE TYPE coupon_type AS ENUM ('PERCENTAGE', 'FIXED_AMOUNT');

CREATE TABLE coupons (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code VARCHAR(50) NOT NULL UNIQUE,
  type coupon_type NOT NULL,
  value NUMERIC(12,2) NOT NULL CHECK (value >= 0),
  max_discount NUMERIC(12,2),
  min_purchase NUMERIC(12,2) DEFAULT 0,
  start_date TIMESTAMPTZ,
  end_date TIMESTAMPTZ,
  usage_limit INT,
  used_count INT NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 6.4 reviews

```sql
CREATE TABLE reviews (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  order_item_id UUID REFERENCES order_items(id) ON DELETE SET NULL,
  rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
  comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 6.5 custom_orders

```sql
CREATE TYPE custom_order_status AS ENUM (
  'REQUESTED',
  'REVIEWED',
  'QUOTED',
  'APPROVED_BY_CUSTOMER',
  'IN_PRODUCTION',
  'QUALITY_CHECK',
  'READY_TO_SHIP',
  'SHIPPED',
  'COMPLETED',
  'CANCELLED'
);

CREATE TABLE custom_orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  base_product_id UUID REFERENCES products(id) ON DELETE SET NULL,
  request_code VARCHAR(60) NOT NULL UNIQUE,
  upper_color VARCHAR(80),
  sole_color VARCHAR(80),
  material VARCHAR(120),
  size VARCHAR(20),
  reference_image_url TEXT,
  note TEXT,
  quoted_price NUMERIC(12,2),
  status custom_order_status NOT NULL DEFAULT 'REQUESTED',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 7. Main Flow Data Changes

## 7.1 Add to Cart

Affected tables:

```txt
carts
cart_items
product_variants read only
```

Rules:

```txt
- Create cart if not exists.
- Check variant exists and active.
- Check requested quantity <= stock.
- Upsert cart item.
```

---

## 7.2 Checkout

Affected tables:

```txt
orders
order_items
payments
cart_items
```

Transaction required.

Steps:

```txt
1. Read cart_items.
2. Read product_variants with product data.
3. Validate stock.
4. Calculate subtotal.
5. Create orders.
6. Create order_items snapshot.
7. Create payments.
8. Delete cart_items.
9. Commit.
```

Stock is not deducted here.

---

## 7.3 Simulated Payment Success

Affected tables:

```txt
payments
orders
product_variants
inventory_logs
order_status_logs
```

Transaction required.

Steps:

```txt
1. Read order and payment.
2. Validate payment pending.
3. Validate order pending payment.
4. Read order items.
5. Validate stock again.
6. Update payment to PAID.
7. Update order to PROCESSING.
8. Deduct product variant stock.
9. Insert inventory logs.
10. Insert order status log.
11. Commit.
```

---

## 7.4 Shipment Delivered

Affected tables:

```txt
shipments
orders
order_status_logs
```

Rules:

```txt
- Shipment status becomes DELIVERED.
- Order status becomes DELIVERED.
- Insert order status log.
```

---

## 8. Index Strategy

Important indexes:

```sql
CREATE INDEX idx_products_status ON products(status);
CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_product_variants_product_id ON product_variants(product_id);
CREATE INDEX idx_product_variants_sku ON product_variants(sku);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_order_status ON orders(order_status);
CREATE INDEX idx_orders_payment_status ON orders(payment_status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_inventory_logs_variant_id ON inventory_logs(product_variant_id);
CREATE INDEX idx_order_status_logs_order_id ON order_status_logs(order_id);
```

Purpose:

```txt
- Product listing filter.
- Product detail by slug.
- Admin order table.
- Customer order history.
- Dashboard query.
- Inventory log history.
```

---

## 9. Data Integrity Rules

### 9.1 Product Delete

Product sebaiknya tidak hard delete jika sudah pernah dibeli.

Recommended:

```txt
Set product status = ARCHIVED
```

### 9.2 Variant Delete

Variant sebaiknya tidak hard delete jika sudah ada order item.

Recommended:

```txt
Set variant status = INACTIVE
```

### 9.3 Order Item Snapshot

Order history tidak boleh rusak saat product/variant berubah.

Karena itu `order_items` menyimpan:

```txt
product_name_snapshot
sku_snapshot
size_snapshot
color_snapshot
price_snapshot
```

### 9.4 Payment Idempotency

Payment success tidak boleh mengurangi stock dua kali.

Rule:

```txt
Only process payment success if payment.status = PENDING and order.status = PENDING_PAYMENT
```

Jika payment sudah `PAID`, return error atau return existing success response tanpa mengulang stock deduction.

---

## 10. Suggested Migration Order

```txt
001_enable_extensions.sql
002_create_enums.sql
003_create_users.sql
004_create_addresses.sql
005_create_categories.sql
006_create_products.sql
007_create_product_images.sql
008_create_product_variants.sql
009_create_carts.sql
010_create_cart_items.sql
011_create_orders.sql
012_create_order_items.sql
013_create_payments.sql
014_create_shipments.sql
015_create_inventory_logs.sql
016_create_order_status_logs.sql
017_create_indexes.sql
018_seed_initial_data.sql
```

---

## 11. Required Extension

Untuk UUID generation:

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
```

---

## 12. Seed Data Recommendation

Seed user:

```txt
superadmin@demo.com / password123 / SUPER_ADMIN
admin@demo.com / password123 / ADMIN
warehouse@demo.com / password123 / WAREHOUSE
customer@demo.com / password123 / CUSTOMER
```

Seed categories:

```txt
Running Shoes
Lifestyle Shoes
Training Shoes
Football Shoes
Sandals
Limited Edition
```

Seed product example:

```txt
Product: Adis Runner Pro
Category: Running Shoes
Variants:
- ARP-BLK-40 / Black / Size 40 / Stock 10
- ARP-BLK-41 / Black / Size 41 / Stock 8
- ARP-WHT-40 / White / Size 40 / Stock 5
```

---

## 13. Dashboard Queries Needed

### Total Revenue

```sql
SELECT COALESCE(SUM(total_amount), 0) AS total_revenue
FROM orders
WHERE payment_status = 'PAID';
```

### Orders by Status

```sql
SELECT order_status, COUNT(*) AS total
FROM orders
GROUP BY order_status;
```

### Low Stock Variants

```sql
SELECT pv.*, p.name AS product_name
FROM product_variants pv
JOIN products p ON p.id = pv.product_id
WHERE pv.stock <= 5
ORDER BY pv.stock ASC;
```

### Best Selling Products

```sql
SELECT
  product_name_snapshot,
  SUM(quantity) AS total_sold,
  SUM(subtotal) AS total_sales
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
WHERE o.payment_status = 'PAID'
GROUP BY product_name_snapshot
ORDER BY total_sold DESC
LIMIT 10;
```

---

## 14. Data Model Notes for Frontend

Frontend akan membutuhkan nested data seperti:

```json
{
  "id": "product-id",
  "name": "Adis Runner Pro",
  "slug": "adis-runner-pro",
  "base_price": 799000,
  "images": [],
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

Backend boleh mengambil data dari beberapa table dan membentuk response DTO khusus agar frontend tidak perlu melakukan banyak request.

---

## 15. Final MVP Tables

Untuk MVP awal, table wajib:

```txt
users
addresses
categories
products
product_images
product_variants
carts
cart_items
orders
order_items
payments
shipments
inventory_logs
order_status_logs
```

Table fase 2:

```txt
returns
return_items
coupons
reviews
custom_orders
```

Dengan desain ini, backend sudah cukup kuat untuk portfolio karena mencakup:

```txt
- Auth
- RBAC
- Product variant inventory
- Cart
- Checkout
- Simulated payment
- Order lifecycle
- Shipment
- Inventory audit trail
- Dashboard-ready data
```
