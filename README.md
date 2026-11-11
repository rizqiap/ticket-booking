# Ticket Booking

An event ticket booking API built with Go and Gin.

Currently includes environment configuration, PostgreSQL connectivity, and a health check endpoint.
Booking features are planned in [the requirements](docs/REQUIREMENT.MD).

## Setup

Requires Go 1.25 or newer.

1. Clone the repository:

   ```sh
   git clone https://github.com/rizqiap/ticket-booking.git
   cd ticket-booking
   ```

2. Copy `.env.example` to `.env`:

   ```sh
   cp .env.example .env
   ```

   PowerShell:

   ```powershell
   Copy-Item .env.example .env
   ```

3. Update the values in `.env`:

   | Variable | Description | Default |
   | --- | --- | --- |
   | `APP_PORT` | HTTP port, between 1 and 65535 | `8080` |
   | `DATABASE_URL` | PostgreSQL connection URL | Required |
   | `JWT_SECRET` | Secret for signing JWTs | Required |
   | `MIDTRANS_SERVER_KEY` | Sandbox Server Key for Midtrans API requests | Required |
   | `MIDTRANS_SIGNATURE_KEY` | Signature Secret Key for notification verification | Required |

   Replace the example password and JWT secret with your own values.
   System environment variables take priority over `.env`.

4. Start PostgreSQL with Docker:

   ```sh
   docker compose up -d db
   ```

   PostgreSQL uses host port `15432`. Use this port in `DATABASE_URL`.

5. Run from the project root:

   ```sh
   go run ./cmd/api
   ```

## CI

GitHub Actions runs on every push and pull request: formatting, `go vet`, build,
unit tests, and integration tests. Tests run with the race detector enabled.
CI uses the Go version from `go.mod` and a temporary PostgreSQL 17 service.
Integration tests apply their migrations inside isolated schemas; no `.env` or
repository secrets are required. Results appear in the repository's Actions tab.

Ticket type service unit tests live in `internal/tickettype/service_test.go`.
Database tests live in `tests/integration` and require the `integration` build
tag and `TEST_DATABASE_URL`, including the ticket type listing test.

## Health Check

```sh
curl http://localhost:8080/health
```

Response:

```json
{"status":"ok"}
```

Use your configured port if you change `APP_PORT`.
Authentication supports registration, login, and profiles.

## Swagger

Run `go run ./cmd/api`, then open <http://localhost:8080/swagger/index.html>.
Use your configured `APP_PORT` if different. Swagger UI loads its assets from a CDN,
so the browser needs internet access.

The contract is maintained in [docs/openapi.yaml](docs/openapi.yaml) and embedded
in the Go binary. Restart the API after changing it. All endpoints from
[the requirements](docs/REQUIREMENT.MD) are listed; those marked `[RENCANA]` are not
implemented and have no agreed request/response body yet. Active endpoints follow
the current handlers, including event publishing and ticket type listing, creation,
and updates.

To call a protected endpoint, login through Swagger, copy `data.access_token`,
click **Authorize**, and paste the token without the `Bearer` prefix.

## Create Booking

`POST /api/v1/bookings` requires a Bearer token and an `Idempotency-Key` header.
The key must be 8-128 bytes after trimming leading and trailing whitespace.

```http
POST /api/v1/bookings
Authorization: Bearer <access_token>
Idempotency-Key: booking-001
Content-Type: application/json

{"ticket_type_id":12,"quantity":2}
```

The event must be published and not started. Quantity is 1-4; the server calculates
the total price and reserves stock in the same transaction that creates the booking.
A new booking returns 201 with `message` and `data`, status `pending`, and an
`expires_at` 10 minutes in the future. Reusing a key with the same ticket type and
quantity for the same user returns 201 with the original booking, even after stock
runs out. Its price, status, and expiry remain unchanged. Different booking data
with the same key returns 409. Unavailable tickets for new bookings,
unpublished/started events, and insufficient stock also return 409.
Automatic expiration and stock release on expiry are not implemented yet.

Run booking unit tests with `go test ./internal/booking`. They cover service
validation, handler responses, and error handling using a repository stub,
without a database or HTTP server.

Concurrency tests use PostgreSQL and run separately from unit tests. The database
user must be able to create schemas; each scenario creates and removes an isolated
schema using the existing migrations.

```powershell
$env:TEST_DATABASE_URL = 'postgres://user:password@localhost:15432/test_db?sslmode=disable'
go test -tags integration ./tests/integration -run TestCreateBookingConcurrency -count=1 -v
```

`booking_test.go` groups booking integration tests by operation. Its concurrency
tests send 20 concurrent requests per scenario to verify
limited stock cannot be oversold, a shared user/key creates only one booking with
one stock reservation, and simultaneous cancellations restore stock only once.
Shared setup helpers at the bottom create an isolated database and service for each scenario.

## Booking History

`GET /api/v1/bookings?page=1&limit=10` requires a Bearer token. It returns only the
authenticated user's bookings, with all statuses, newest first (`created_at DESC`,
then `id DESC`). Pagination matches event listing: page 1-100000, limit 1-100.

```json
{"data":[],"pagination":{"page":1,"limit":10}}
```

Non-empty `data` contains the same booking fields as create. The endpoint does not
expire bookings or release stock; statuses reflect the stored database values.

## Booking Detail

`GET /api/v1/bookings/{id}` requires a Bearer token and returns 200 with
`{"data": <booking>}`, using the same booking fields as create/history.
Only the authenticated owner can read it. Both an unknown ID and another user's
booking return 404 with `{"error":"booking not found"}`. Invalid IDs return 400.
Reading a booking does not expire it or release reserved stock.

## Cancel Booking

Apply migration `000007_add_cancelled_booking_status.up.sql` before using cancellation.
`POST /api/v1/bookings/{id}/cancel` requires a Bearer token and no body. Only the owner
can cancel a pending booking. The response is 200 with `message` and the booking in
`data`; its status is `cancelled`. History is preserved and stock is restored once.
Repeating cancellation returns the same booking without restoring stock again.
Confirmed/expired bookings return 409; unknown IDs and other users' bookings return 404.
The down migration refuses to remove the cancelled status while cancelled bookings exist.

## Project Structure


```text
cmd/api/          API entry point
internal/config/  Environment loader
internal/database/ PostgreSQL connection
docs/             Project requirements
```
