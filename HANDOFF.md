# Operator's handoff — Our Lady of the Valley Parish website

This document is for whoever runs the parish website day-to-day — whether that's the parish staff themselves, a diocesan IT person, or a developer handed the repo. It assumes nothing about the website's internals, only that you can read a terminal.

If you're a parishioner looking for Mass times, you want <https://www.olv14843.org/>. This file is for operators.

## What this is

A redesigned website for Our Lady of the Valley Parish, built as a gift to the parish by [Zach Smith](mailto:smith.zach119@gmail.com). It mirrors the structure of the current olv14843.org (same URL slugs, same content tree, same bulletins and Mass times), but:

- The front end is reverent and mobile-first (Marian blue, Cormorant Garamond, Playfair-style typography).
- Staff edit pages by **dragging content blocks around in a browser** rather than editing HTML.
- Mass times, confessions, bulletins, homilies, events, and parish registrations each have a dedicated admin screen.
- Content imports and exports are plain Postgres rows — no proprietary file formats.
- It runs anywhere a Docker container runs: a $5/month Hetzner VPS, Easypanel, DigitalOcean App Platform, the diocese's own hosting.

## Stack at a glance

- **Backend**: Go 1.23+ (one binary, no runtime dependencies)
- **Framework**: Gin (HTTP), GORM (DB)
- **Database**: PostgreSQL 15+
- **Front end**: server-rendered HTML templates + Tailwind 2 CDN + vanilla JS + SortableJS (for drag-and-drop)
- **Deploy**: Dockerfile → Easypanel is the primary target, but anything that runs containers will do

## Repo layout, at the level that matters

```
main.go                          Server + command-line flags
cmd/import/                      One-shot scraper for olv14843.org
internal/
  auth/            JWT + bcrypt
  config/          Env-var loading + Validate()
  database/        Connection pool, migrations, seed script
  email/           SMTP mailer (optional)
  handlers/        One file per surface (page.go, admin_pages.go, …)
  middleware/      Auth + session + CORS + cache
  models/          GORM models — User, Page, MassSchedule, …
  routes/          The single route table
web/
  static/          CSS, JS, images, uploads/
  templates/       base.html + per-page content partials
```

## How to run it

### Everything, in one place

```bash
# 1. Install Go 1.23+ and PostgreSQL 15+, or just use Docker.
# 2. Clone the repo.
git clone <this-repo>
cd olv-parish

# 3. Point at a Postgres database.
cp env.example .env
# Edit .env — set DB_PASSWORD, JWT_SECRET, SESSION_SECRET (32+ chars each).

# 4. Create the schema + seed baseline content + make the first admin.
go run main.go --migrate
go run main.go --seed
ADMIN_EMAIL=you@example.org ADMIN_PASSWORD=change-me-now go run main.go --create-admin

# 5. Start the server.
go run main.go
# Visit http://localhost:8080 — public site.
# Visit http://localhost:8080/admin/login — admin sign-in.
```

### With Docker Compose (local dev)

```bash
docker compose up
docker compose exec app ./main --migrate
docker compose exec app ./main --seed
docker compose exec -e ADMIN_EMAIL=you@example.org -e ADMIN_PASSWORD=change-me-now app ./main --create-admin
```

### In production (Easypanel)

See [EASY_PANEL_DEPLOYMENT.md](EASY_PANEL_DEPLOYMENT.md).

## Day-to-day: parish staff

Everything the parish staff needs lives under `/admin` after they sign in. The sidebar is organised by rhythm:

- **Dashboard** — overview, recent form submissions
- **Messages** — anyone who used the public Contact form
- **Registrations** — anyone who used the Parish Registration form; email the household head directly from the inbox
- **Content**: Pages / Homilies / Bulletins / Events — click any entry to edit it
- **Schedule**: Mass Times / Confessions — drag rows to reorder, inline-edit any field, click Save

### Editing a page

`/admin/pages` → click a page → the block editor opens.

- Blocks are stacked vertically. Grab the ⋮⋮ handle on any block and drag to reorder.
- "+ Add block" opens the palette: Heading, Text, Image, PDF link, Video embed, CTA button, Divider, plus "live" blocks (Mass times card, Confession times card, Event list, Bulletin list, Staff grid) that pull from their respective admin screens.
- Image and PDF blocks open an asset picker — upload from disk or pick an existing asset.
- Click **Save**; blocks save in whatever order you left them. Click **Preview** to see the public page.

### Posting the weekly bulletin

`/admin/bulletins` → "Upload bulletin" → date + label + the PDF → Save. It shows up on any page containing a Bulletin list block, and on the dedicated `/about/bulletins/` page.

### Posting a homily

`/admin/homilies` → "New homily" → preached-on date, title, optional author, optional audio/video URL, optional text body.

### Updating Mass times

`/admin/mass-times` → inline-edit any field, drag rows to reorder, click **Save**. The home page and `/liturgy-and-sacraments/mass-times/` both pull live from the same table, so there's one source of truth.

### Adding staff accounts

Phase 7+ ships a staff-accounts admin UI. Until then, direct SQL:

```sql
-- From the Postgres shell. Password here is bcrypted; generate with `go run main.go --create-admin`.
INSERT INTO users (email, password, first_name, last_name, is_active, role, created_at, updated_at)
VALUES ('staffer@example.org', '<bcrypt>', 'First', 'Last', true, 'staff', now(), now());
```

Then flip to admin if needed: `UPDATE users SET role = 'admin' WHERE email = 'staffer@example.org';`.

## Day-to-day: operator (you)

### Backups

The entire site state fits in two things:
1. The Postgres database (users, pages, blocks, Mass times, assets metadata, …).
2. The uploads directory on disk (`web/static/uploads/`).

Both must be backed up. In Easypanel, schedule a daily `pg_dump` of the Postgres service, and snapshot the uploads volume. Everything else — templates, Go code, CSS — lives in Git.

Restore test (do this once, so you know it works):

```bash
pg_dump -h <prod-host> -U olv_parish olv_parish > backup.sql
createdb olv_parish_restored
psql olv_parish_restored < backup.sql
# Also copy the uploads directory.
```

### Rotating secrets

- `JWT_SECRET`: rotating signs every staff member out. Do it when a device is lost or you suspect theft.
- `SESSION_SECRET`: same effect for Gorilla sessions.
- `DB_PASSWORD`: rotate yearly or on staff turnover.
- Admin passwords: parish staff should set their own on first sign-in.

### Checking that it's up

```
GET /health
→ {"status":"healthy","database":true,"time":<unix>}
```

- `status: healthy` + `database: true` — all good.
- `status: degraded` + `database: false` — the DB is unreachable; the public home + contact pages still serve, but admin is offline.

Point Easypanel's / your monitoring at `/health`.

### Content import from olv14843.org

If you want a fresh snapshot of what's currently on the live site:

```bash
# From a dev machine, pointed at your production DB (set DB_* env vars).
# Dry-run first — no DB writes, no downloads.
MSYS_NO_PATHCONV=1 go run ./cmd/import --dry-run --verbose

# When happy, drop --dry-run.
MSYS_NO_PATHCONV=1 go run ./cmd/import
```

The importer upserts pages by slug and dedups assets by source URL, so re-running is safe.

## Day-to-day: developer (you, or whoever)

### Building and testing

```bash
go build ./...
go vet  ./...
# No test suite yet — add one in Phase 7+.
```

### Hot-reloading in dev

```bash
# One terminal.
docker compose up postgres
# Another.
go run main.go
```

The server picks up template changes without restart (loads from disk each request in debug mode). Go code changes need a restart.

### Adding a new block type

1. Add the constant to [`internal/models/content.go`](internal/models/content.go) (e.g. `BlockQuote = "quote"`).
2. Add a renderer function in [`internal/handlers/blocks.go`](internal/handlers/blocks.go) — mirror the shape of `renderHeading` / `renderImage`.
3. Add the editor UI in [`web/static/js/admin.js`](web/static/js/admin.js) — add a key to `BLOCK_TYPES` with `label`, `icon`, `default`, `render(d)`, `collect(el)`.
4. Add any CSS classes in [`web/static/css/main.css`](web/static/css/main.css) under the "Block-level classes" section.

No DB migration needed — blocks are JSONB.

### Adding a new admin screen

Pattern: see [`internal/handlers/admin_structured.go`](internal/handlers/admin_structured.go) and the five tables under it (Mass times, confessions, homilies, bulletins, events). Each has a page handler + a short CRUD API. Register the routes in [`internal/routes/routes.go`](internal/routes/routes.go) under both `adminPages` (HTML) and `adminAPI` (JSON).

### What the admin API accepts

- Cookie auth (the admin UI uses this) — `auth_token` cookie set by `/api/v1/auth/login`.
- Bearer header (scripts, cron jobs) — `Authorization: Bearer <JWT>`.

`OptionalAuthMiddleware` tries both.

## Known rough edges, in priority order

- **Importer leaves inline WordPress attributes** on rich-text blocks (`class="size-16 m-size-16"`). Harmless but noisy. A sanitizer pass is a Phase 7+ polish item.
- **No test suite yet.** All verification so far is end-to-end against a live DB.
- **No user-management UI.** First admin is bootstrapped via `--create-admin`; subsequent staff accounts need SQL or Phase 7+.
- **CORS allowlist is static.** `FRONTEND_URL` env var controls it. Multiple domains need code change.
- **Rate limiting isn't wired.** Contact and registration forms rely on the honeypot. Add rate limiting if you see spam.

## When something breaks

Server logs (via `docker logs` or the Easypanel log stream) carry everything:
- `[GIN]` lines: every request, with status code and timing.
- `log.Printf` lines: application events (DB warnings, failed admin queries, mailer attempts, Courier fetches).

If `/health` is `degraded`, the DB is unreachable. If the public site serves but admin login spins, the JWT cookie's `Secure` flag probably refuses to send over HTTP — make sure the public URL is HTTPS and `GIN_MODE=release`.

For anything stranger, email <smith.zach119@gmail.com>.

## License and attribution

All original work (code, templates, design) is MIT-licensed. Content imported from olv14843.org belongs to the parish. The Catholic Courier headlines belong to the Diocese of Rochester.
