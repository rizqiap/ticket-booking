# ticket-booking-system

Membangun sistem pemesanan tiket event yang dapat menangani banyak request secara bersamaan, mencegah overselling dan duplicate booking, serta mengelola stok tiket secara konsisten.

# Tech Stack
- Gin
- PostgreSQL
- JWT
- Docker
- golang-migrate

# Ticket Booking System

## Problem Statement

Membangun sistem pemesanan tiket event yang dapat menangani banyak request secara bersamaan, mencegah overselling dan duplicate booking, serta mengelola stok tiket secara konsisten.

## Features

- Authentication & Authorization
- Event Management
- Ticket Management
- Ticket Booking
- Payment Simulation
- Booking Expiration
- E-Ticket
- Booking History

## Tech Stack

- **Language:** Golang
- **Framework:** Gin
- **Database:** PostgreSQL
- **Database Driver:** pgx
- **Authentication:** JWT
- **Migration:** golang-migrate
- **Containerization:** Docker

## Database (ERD)

### 1. users
- id (PK)
- name
- email
- password_hash
- role
- created_at
- updated_at

### 2. events
- id (PK)
- title
- description
- location
- starts_at
- status
- created_at
- updated_at

### 3. ticket_types
- id (PK)
- event_id (FK)
- name
- price
- total_stock
- available_stock
- created_at
- updated_at

### 4. bookings
- id (PK)
- user_id (FK)
- ticket_type_id (FK)
- quantity
- total_price
- status
- idempotency_key
- expires_at
- created_at
- updated_at

### 5. payments
- id (PK)
- booking_id (FK)
- amount
- status
- paid_at
- created_at

### 6. tickets
- id (PK)
- booking_id (FK)
- ticket_code
- created_at

## Relationships

- users (1) → (N) bookings
- events (1) → (N) ticket_types
- ticket_types (1) → (N) bookings
- bookings (1) → (N) payments
- bookings (1) → (N) tickets

## Database Relationships

- `users` (1) → (N) `bookings`
- `events` (1) → (N) `ticket_types`
- `ticket_types` (1) → (N) `bookings`
- `bookings` (1) → (N) `payments`
- `bookings` (1) → (N) `tickets`


## API Contract

**Base URL:** `/api/v1`

### 1. Authentication
- `POST /auth/register`
- `POST /auth/login`
- `GET /auth/me`

### 2. Event Management
- `GET /events`
- `GET /events/{id}`
- `POST /events`
- `PATCH /events/{id}`
- `PATCH /events/{id}/publish` — admin mempublikasikan event draft yang belum dimulai.

### 3. Ticket Management
- `GET /events/{id}/ticket-types`
- `POST /events/{id}/ticket-types`
- `PATCH /ticket-types/{id}`

### 4. Ticket Booking
- `POST /bookings`
- `GET /bookings`
- `GET /bookings/{id}`
- `POST /bookings/{id}/cancel`

### 5. Payment Simulation
- `POST /bookings/{id}/payments`
- `GET /bookings/{id}/payments`

### 6. E-Ticket
- `GET /bookings/{id}/tickets`


### Project Structure

```text
ticket-booking/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── worker/
│       └── main.go
├── internal/
│   ├── auth/
│   ├── event/
│   ├── booking/
│   ├── payment/
│   ├── ticket/
│   └── database/
├── migrations/
├── tests/
│   └── integration/
├── docker-compose.yml
├── go.mod
└── README.md
```

## Core Concepts

- REST API
- Database Transaction
- Concurrency Control
- Idempotency
- Background Worker
- Dependency Injection
- Unit & Integration Testing
- Error Handling & Logging
