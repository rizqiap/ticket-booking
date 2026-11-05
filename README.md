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

## Project Structure

```text
cmd/api/          API entry point
internal/config/  Environment loader
internal/database/ PostgreSQL connection
docs/             Project requirements
```
