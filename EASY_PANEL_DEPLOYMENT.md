# Easypanel deployment — Our Lady of the Valley Parish

Same shape as `Wildflower-Go`: a single Go binary served from `debian:trixie-slim`, Postgres beside it, uploads on a persistent volume. First deploy needs one migration + one bootstrap command; after that normal deploys are just `git push`.

## 1. Create the Postgres service

Add Easypanel's **Postgres** service to the project. Pick Postgres 16+. Note the generated hostname, port, user, and password — they go into the app's env below.

Pick a service name you'll recognize (e.g. `olv-postgres`). The internal hostname is just that name.

## 2. Create the app service

Add a **New App**:

- **Source**: this GitHub repo, branch `main`.
- **Build method**: Dockerfile. Build context = repo root.
- **Service port**: 8080.
- **Health check**: `GET /health` (every 30 s, 5 s timeout). The endpoint returns `{"status":"healthy","database":true,...}` when happy, `"degraded"` with `database:false` if Postgres is unreachable.

## 3. Environment variables

Required (the server refuses to boot without these):

| Name | Example | Notes |
|------|---------|-------|
| `DB_HOST` | `olv-postgres` | Internal hostname of the Postgres service |
| `DB_PORT` | `5432` | |
| `DB_USER` | `olv_parish` | |
| `DB_PASSWORD` | *generated* | |
| `DB_NAME` | `olv_parish` | |
| `DB_SSLMODE` | `disable` or `require` | `require` if Postgres is TLS-terminated |
| `JWT_SECRET` | *see below* | 32+ random chars. Rotating this signs everyone out. |
| `SESSION_SECRET` | *see below* | 32+ random chars |
| `FRONTEND_URL` | `https://olv-parish.easypanel.app` | Used for the CORS allowlist |
| `PORT` | `8080` | |
| `GIN_MODE` | `release` | Enables Secure cookies, hides stack traces, cuts log noise |

Generate secrets:

```bash
openssl rand -base64 48
```

`config.Validate` rejects any secret shorter than 32 chars or starting with `your-`.

Optional:

| Name | Purpose |
|------|---------|
| `COURIER_FEED_URL` | Override the Catholic Courier RSS feed (default: `https://catholiccourier.com/feed/`) |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` | Email delivery for parish-registration notifications. When empty, the mailer logs instead of sending. |
| `EMAIL_FROM` | From-address on notification emails |
| `PARISH_NOTIFY_EMAIL` | Where registration notifications are sent. Defaults to `EMAIL_FROM`, then the parish contact in [site.go](internal/handlers/site.go) |
| `ADMIN_EMAIL` + `ADMIN_PASSWORD` | Used by `--create-admin` to bootstrap the first admin (see below) |

## 4. Persistent uploads volume

The admin uploads PDFs, images, and audio into `web/static/uploads/`. The container exposes that path as `VOLUME ["/app/web/static/uploads"]`, so attach a persistent Easypanel volume there — otherwise every container restart wipes the uploads tree.

Volume mount path: `/app/web/static/uploads`

## 5. First-deploy sequence

Open the app's **Shell** in Easypanel and run, in order:

```bash
# 1. Full migration — creates every table.
./main --migrate

# 2. Baseline content — Mass times, confession times, nav tree, starter pages.
./main --seed

# 3. First admin — ADMIN_EMAIL + ADMIN_PASSWORD must be set as app env vars.
./main --create-admin
```

After the first admin exists, invite other staff from `/admin` (users management, Phase 7+). Remove `ADMIN_EMAIL` and `ADMIN_PASSWORD` from the environment once the account is created so they don't linger in config.

## 6. Optional: import live content from olv14843.org

If the parish wants a copy of what's currently on olv14843.org as a starting point:

```bash
# Dry-run first to see what would be fetched (no DB writes, no downloads).
./main --help     # importer is a separate binary; see cmd/import/
# From the repo root at build time:
./import           # if you add it to the Dockerfile; otherwise run from a dev shell
```

The importer scrapes ~45 pages, extracts images and PDFs as assets, and upserts Pages in the DB. Idempotent: pages key on slug, assets key on their source URL (stored in `Asset.OriginalName`).

See [HANDOFF.md](HANDOFF.md) for how to run the importer against the production DB from a dev machine.

## 7. Follow-up deploys

Normal flow: push to the connected branch. Easypanel rebuilds the Docker image, swaps the container, and runs the health check. The additive `EnsureAdminSchema` + `EnsureContentSchema` run on every startup so new tables land automatically. If you add a column, add it via `AutoMigrate` in `internal/database/content.go` so it rolls forward without a manual migration.

## 8. Hardening checklist for prod

- `GIN_MODE=release` set
- `DB_SSLMODE=require` (or stricter)
- Secrets rotated: `JWT_SECRET`, `SESSION_SECRET`, `DB_PASSWORD`
- Admin password changed from the dev default (`admin1234!` was the dev admin — don't reuse)
- Volume mounted at `/app/web/static/uploads`
- Daily `pg_dump` of the Postgres service (Easypanel can schedule it)
- `FRONTEND_URL` matches the real public URL so CORS is tight

## 9. Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Boot fails: `JWT_SECRET is required` | Env var not set, or starts with `your-` | Set a real 32+ char value |
| `/health` returns `"status":"degraded"` with `database:false` | Postgres unreachable | Check `DB_HOST` and that the Postgres service is up |
| Login succeeds but `/admin` redirects to `/admin/login` | `GIN_MODE=release` and the browser isn't on HTTPS → `Secure` cookie refuses to send | Terminate TLS at Easypanel |
| Catholic Courier block is empty | Feed URL blocked by the firewall, or RSS format changed | Check logs for `CourierFetcher:` lines |
| Admin uploads disappear after deploy | No persistent volume mounted at `/app/web/static/uploads` | Attach a volume at that path |
