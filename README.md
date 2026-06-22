# Handy Notes

Notion-inspired personal notes app using Vue 3, Go, PostgreSQL, and the existing `handy-auth` service.

## Architecture

- `web/`: Vue 3 + Vite, block editor, search, favorites, archive, autosave.
- `server/`: Go + Gin + GORM API. Every protected request validates its bearer token against `handy-auth`.
- PostgreSQL: notes live in their own `handy_notion` database. Every note is owned by the UUID returned by `handy-auth`.

## Run locally

Requirements: Go 1.23+, Node 22+, PostgreSQL, and `handy-auth` running on port `8081`.

1. Create a dedicated PostgreSQL database:

   ```sql
   CREATE DATABASE handy_notion;
   ```

2. Configure and run the API:

   ```sh
   cd server
   cp .env.example .env
   # Edit DB_POSTGRES for your PostgreSQL server.
   go run .
   ```

3. Configure and run the web app:

   ```sh
   cd web
   cp .env.example .env
   npm install
   npm run dev
   ```

Open `http://localhost:5174`. Add that origin to `handy-auth`'s `CORS_ORIGINS` as well.

## Existing PostgreSQL server

You can reuse the server used by `go-passmanager`, but create a separate `handy_notion` database and database user. Put its DSN only in `server/.env` or your deployment secret manager—never commit it. The API creates the `notes` table automatically at startup.

## Production notes

- Serve both apps over HTTPS.
- Set exact `CORS_ORIGINS`, `AUTH_SERVICE_URL`, `VITE_API_URL`, and `VITE_AUTH_URL` values for your domains.
- The current frontend follows the existing Handy convention of bearer tokens in local storage. A future security hardening pass can move authentication to secure, HttpOnly, SameSite cookies, which requires a small coordinated change in `handy-auth`.

