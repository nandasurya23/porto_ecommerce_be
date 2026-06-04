# Footwear Backend

Go REST API backend for the footwear e-commerce operations platform.

## Local run

1. Start Colima:

```bash
colima start
```

2. Start PostgreSQL in Docker:

```bash
docker-compose up -d
```

3. Copy `.env.example` to `.env`.

4. Run the API:

```bash
go run ./cmd/api
```

If your machine already uses PostgreSQL on `5432`, this repo uses `5433` for the Docker container and `DB_PORT=5433` in `.env.example`.
If you upload images or post JSON bodies, the backend limits request body size through `MAX_BODY_SIZE_MB`.
If you want to wipe and recreate demo data on the next start, set `RESET_DEMO_DATA=true` once in `.env`.

## Seed data only

If you want to refresh demo data without starting the API:

```bash
go run ./cmd/seed
```

## Seed users

- `superadmin@demo.com` / `password123`
- `admin@demo.com` / `password123`
- `warehouse@demo.com` / `password123`
- `customer@demo.com` / `password123`

## Health check

- `GET /api/v1/health`

## Frontend API Guide

See [`docs/FRONTEND_API_GUIDE.md`](/Users/nandasuryamac/Desktop/PORTO/backend/docs/FRONTEND_API_GUIDE.md) for the endpoint map, auth flow, and frontend consumption examples.
