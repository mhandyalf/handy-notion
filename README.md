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

## Reliability checks

Run `npm ci`, `npm test`, and `npm run build` in `web/`; run `go test -race ./...` and `go vet ./...` in `server/`. Frontend regression tests use Node's built-in test runner and Vue reactivity without a new test dependency. CI runs frontend and backend checks before publishing images; pull requests run checks without publishing.

Autosave writes are serialized. Switching pages and leaving the workspace flush pending edits; a failed save keeps the draft visible and blocks the transition. Search only refreshes the sidebar and ignores obsolete responses. Failed archive/delete requests keep the page visible. Use **Retry save** after a save failure.

Unsaved drafts are copied to user-scoped `sessionStorage` when available. Reloading the same tab and opening that page restores the draft for review; a successful save removes that recovery copy. Closing the tab clears session storage, and browsers may not display unload warnings in every situation. This is not an offline sync system or a backup. Recovered drafts can be older than changes from another tab: inspect them before saving. The API still has no revision checking, so concurrent tabs can overwrite each other.

Before deployment, manually check rapid edit → page switch, search during editing, browser back/forward, offline save/archive failures and retry, same-tab reload recovery, and logout with unsaved edits against a local/test account. Automated tests exercise the state controller, not browser navigation or the DOM. Verify a database backup and a restore into an isolated database before any production rollout or schema change; these checks do not validate production backups.

