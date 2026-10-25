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

## Workspace and editor controls

- Type `/` at the start of an otherwise empty block to open the block menu, then type a label or alias (for example `/task`). Use ↑/↓ and Enter to select, or Escape to dismiss without changing your text. Click the block handle to change an existing block's type without losing its text.
- Supported blocks remain text, heading, to-do, and bulleted list; no content/schema migration is required. Enter splits a block at the cursor and continues lists, Enter in an empty list item returns to text, and Shift+Enter inserts a newline.
- Use Cmd/Ctrl+K to focus search and Cmd/Ctrl+Alt+N to create a note (subject to browser/OS shortcut interception). On mobile, open the notes drawer from the toolbar; Escape or the backdrop closes it. Save/recovery status remains visible on small screens.
- Before merging UI changes, smoke-test keyboard and pointer menu selection, Escape/no-match behavior, multiline content after reload, mobile drawer focus, and all reliability scenarios below. Helper tests are not a substitute for end-to-end authenticated tests.

## Reliability validation

Run `npm ci`, `npm test`, and `npm run build` in `web/`; run `go test -race ./...` and `go vet ./...` in `server/`. Frontend regression tests use Node's built-in test runner and Vue reactivity without a new test dependency. CI runs frontend and backend checks before publishing images; pull requests run checks without publishing.

Autosave writes are serialized. Switching pages and leaving the workspace flush pending edits; a failed save keeps the draft visible and blocks the transition. Search only refreshes the sidebar and ignores obsolete responses. Failed archive/delete requests keep the page visible. Use **Retry save** after a save failure.

Unsaved drafts are copied to user-scoped `sessionStorage` when available. Reloading the same tab and opening that page restores the draft for review; a successful save removes that recovery copy. Closing the tab clears session storage, and browsers may not display unload warnings in every situation. This is not an offline sync system or a backup. Recovered drafts retain their original base timestamp so that an older recovery copy cannot silently overwrite newer server changes.

## Public read-only sharing

### Immediate production deployment

Every push to `main` (including a merged pull request) runs tests, publishes both images, then immediately deploys that exact commit over SSH. There is no polling delay. The `deploy` job waits for the host script and health checks, then checks public web/API routing. A failed deployment fails GitHub Actions. Builds must still finish before production changes; existing browser tabs need a refresh.

Repository secrets `VPS_DEPLOY_KEY` and `VPS_KNOWN_HOSTS` contain a dedicated private deploy key and the verified host key. Its public key must have `restrict,command="/usr/local/sbin/handy-notion-ssh-deploy"` in root's authorized_keys. Install `deploy/ssh-deploy.sh` at that path and `deploy/auto-update.sh` at `/usr/local/sbin/handy-notion-auto-update`, both root-owned, mode 0755. The forced command accepts only `deploy <full commit SHA>` and does not permit an interactive shell or forwarding. This key still invokes a privileged deployment script: keep it dedicated and rotate it when needed.

The polling timer source has been removed. Remove any previously installed host timer with the commands below. Do not run polling alongside direct deployments: mutable `latest` tags could otherwise redeploy a superseded commit. Concurrent workflows are serialized and superseded commits are skipped before SSH deployment. The host deployment lock waits for an existing deployment instead of reporting a false success.

### Legacy VPS automatic image updates

Older installations used `/etc/systemd/system/handy-notion-update.timer` to check Docker Hub approximately every five minutes. Remove that timer when migrating to direct deployment. The retained service executes `/usr/local/sbin/handy-notion-auto-update` (source: `deploy/auto-update.sh`), targeting only `api` and `web` in `/myproject/handy-notion/docker-compose.prod.yml`. Other applications and the PostgreSQL container are not restarted.

Only `main` publishes production images. Both API and web images must carry the same 40-character `org.opencontainers.image.revision`; unlabelled or mismatched images are skipped. The deployment pins the exact downloaded image IDs rather than mutable tags. Production builds use same-origin API/auth URLs so both configured domains can work. Feature-branch pushes do not deploy.

Before a changed pair is deployed, the updater creates a root-only compressed `pg_dumpall` of the shared PostgreSQL cluster under `/var/backups/handy-notion/`. A failed dump blocks deployment. Backups contain sensitive data: arrange encrypted off-host copies, monitor disk capacity, and manage retention manually; this script does not delete backups or old images. A successful dump/gzip check is **not** proof of a successful restore; the restore release gate remains open.

Deployment recreates only the two Notes containers, causing a brief interruption. Failed HTTP checks trigger an attempt to restore the prior image IDs, **not** the database; incompatible schema migrations, process termination, or host failures can still require manual recovery. `/health` and web HTTP checks do not replace authenticated end-to-end checks. Monitor failed service runs; no external alerting is configured.

Host commands:

```sh
systemctl disable --now handy-notion-update.timer
rm -f /etc/systemd/system/handy-notion-update.timer
systemctl daemon-reload
systemctl list-timers --all handy-notion-update.timer
journalctl -u handy-notion-update.service -n 80 --no-pager
systemctl start handy-notion-update.service # Check now
```

Removing the timer does not interrupt an update already running. The service unit and scripts are versioned under `deploy/`; changes there must be installed on the host explicitly.

Open a note and choose **Share → Enable public link**, then **Copy link**. The page at `/s/<random-token>` is readable without login. Anyone who receives or forwards this link can read the note; it is not a private invitation, searchable public directory, or collaborative editor.

- Notes are private by default. Only the owner can enable, inspect, or revoke sharing. Enabling first saves pending edits; save errors/conflicts block enabling. Sharing status does not change the note concurrency version.
- Links display the latest saved title, icon and blocks, not an immutable snapshot. Future saved edits are public too. Viewers refresh to see updates; local unsaved drafts are not published.
- **Revoke link** immediately blocks subsequent API reads. Enabling again creates a different link. Already loaded/downloaded/copied content cannot be recalled.
- Archived/deleted notes are unavailable publicly. Restoring an archived note reactivates its existing link unless it was revoked. Revoke before archiving if access must stay disabled after restoration.
- Anonymous responses omit note IDs, owner IDs, favorites, sharing tokens, and private metadata. Text is rendered using Vue escaping, never as HTML. There are no public edit endpoints.
- The API uses `no-store` and `noindex` headers; the public page requests no indexing. These are best-effort instructions, not protection against screenshots, crawlers ignoring instructions, or forwarded links.

Deployment adds a `note_shares` table via the existing startup AutoMigrate flow. Back up and test restore before rollout; do not drop this table on rollback unless intentionally revoking every link. Deploy the backend before enabling the new UI. Configure the web server for SPA fallback on `/s/*`, HTTPS, and no caching of public API responses. Treat public URLs/tokens as secrets: redact `/s/*`, `/api/public/notes/*`, and share-management response bodies from access/analytics logs. Do not add third-party analytics to public pages. Apply normal public endpoint rate limits at the reverse proxy.

Before release, test owner enable/copy/revoke, opening in a logged-out/private browser, all four block types and multiline text, literal HTML displayed as text, updated saved content after refresh, old links after revocation/re-enable, archived/deleted notes, mobile layout, and keyboard access. API integration tests cover permissions, lifecycle, metadata allowlisting, and concurrent enable; they do not replace browser checks.

## Concurrent editing API details

Updates require `expected_updated_at` in the PUT JSON body; deletes require it as a URL-encoded query parameter. Use the exact `updated_at` returned by the last successful create, fetch, or save. The database checks ownership and timestamp in the same mutation and returns the exact saved row. Missing preconditions return HTTP 428, malformed timestamps return 400, and stale/missing/deleted/unowned rows return 409 with `code: "note_conflict"`. No schema migration or new dependency is required.

When another tab saves first, this tab keeps its local draft and pauses automatic saving. Download the local draft as JSON, then choose **Load server version** and confirm before replacing it. You can manually copy the downloaded text into the latest version; there is no automatic merge or force-overwrite. Navigation/logout stays blocked by an unresolved conflict. If the server note was deleted, loading fails without discarding the draft; download it before leaving or reloading the tab. A draft without a known base timestamp is also blocked rather than silently rebased.

Roll out the backend and frontend together. Old frontend tabs cannot save against the new API until refreshed (428); preserve/download unsaved content before refreshing. Do not deploy the new frontend against an old backend, which cannot enforce preconditions. Before production, test two real authenticated browser tabs against PostgreSQL: simultaneous saves, archive/delete versus edit, recovery after reload, and download/load-server recovery. Unit tests simulate controller conflicts and SQL results; they do not replace this live concurrency check.

Before deployment, manually check rapid edit → page switch, search during editing, browser back/forward, offline save/archive failures and retry, same-tab reload recovery, and logout with unsaved edits against a local/test account. Automated tests exercise the state controller, not browser navigation or the DOM. Verify a database backup and a restore into an isolated database before any production rollout or schema change; these checks do not validate production backups.

### PostgreSQL integration checks (stage 4)

The API integration suite uses the real router, auth middleware, repository, and PostgreSQL with separate HTTP clients. Only the external auth service is stubbed. It checks timestamp precision and monotonicity (including a future stored timestamp), stale/missing/invalid preconditions, ownership, archive versus edit, update versus update, and delete versus update. Concurrent races run ten times each and require exactly one successful mutation and one 409, then verify the stored result.

Use a **disposable test database**, never a production DSN. The suite creates a randomly named schema and drops only that schema on cleanup; interruption can leave a `note_test_*` schema behind. The database user needs schema-creation permission. No new dependencies are required.

```sh
docker run --detach --rm --name handy-note-integration \
  -e POSTGRES_USER=note_test -e POSTGRES_PASSWORD=note_test \
  -e POSTGRES_DB=note_test -p 127.0.0.1:55439:5432 postgres:17-alpine
# Wait until this succeeds:
docker exec handy-note-integration pg_isready -U note_test -d note_test
cd server
NOTE_TEST_DATABASE_URL='postgres://note_test:note_test@127.0.0.1:55439/note_test?sslmode=disable' \
  go test -race -count=1 ./...
docker stop handy-note-integration
```

Without `NOTE_TEST_DATABASE_URL`, the integration test explicitly skips. CI provides a PostgreSQL 17 service and runs the suite with the race detector before image publication. This validates the API/database boundary, **not** two browser tabs or real `handy-auth` integration.

### Authenticated two-tab release gate

Run the app and real `handy-auth` against an isolated test database. Sign in as one test user and open the same note in two normal tabs, A and B. Use disposable notes and record the commit, browser, and pass/fail evidence for each step. Do not mark this gate complete based on API or controller tests alone.

- [ ] **Concurrent editing:** let A save, then edit B's older copy. B receives 409, retains its draft, pauses autosave, and cannot switch notes or log out. A's content remains on the server. Repeat with edits near-simultaneously.
- [ ] **Archive versus edit:** archive in A, then save the old copy in B. B conflicts without unarchiving or overwriting the server note.
- [ ] **Delete versus edit:** delete in A, then save in B. B retains its draft; loading the missing server version fails without discarding it. Download remains available.
- [ ] **Reload recovery:** before B's draft saves, take B offline and edit, save in A, then reload B in the same tab and reconnect. The recovered draft keeps its original base and conflicts rather than overwriting A.
- [ ] **Download:** download B's local JSON and inspect it for the expected unsaved title/content before any discard or refresh. Treat this file as private note data.
- [ ] **Server reload:** cancel the discard confirmation and verify the draft remains. Confirm it and verify the latest server version loads and conflict clears. Under network throttling, edit during the fetch and verify those edits are not discarded.
- [ ] **Compatibility:** preserve drafts in old frontend tabs before refreshing; verify old clients receive 428 from the new backend. Release backend/frontend together; never run the new frontend against an old backend.
- [ ] **Release operations:** verify backup/isolated restore, test offline failures and retry, browser navigation, and logout guards. Record explicit merge/deployment authorization separately.

Stage 4 is not production-ready until the unchecked browser and operational gates are completed. No automatic merge, deployment, or production backup validation is performed by this suite.

