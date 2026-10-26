# Our Lady of the Valley Parish — Website

A modern redesigned website for [Our Lady of the Valley Parish](https://www.olv14843.org/) (St. Ann in Hornell + St. Mary in Rexville, Diocese of Rochester), built on the same Go stack as [Wildflower-Go](../Go/Wildflower-Go).

> **Status:** Phase 1 scaffold. Home page, contact form, admin dashboard shell, and admin login are live. Phase 2 adds the content model (pages / bulletins / homilies / events); Phase 4 ships the drag-and-drop block editor that lets parish staff manage pages without touching code. See [`swirling-jingling-whale.md`](../../.claude/plans/swirling-jingling-whale.md) for the full plan.

## Stack

- **Go 1.23+** / **Gin** / **GORM** / **PostgreSQL**
- Server-rendered `html/template`, Tailwind 2 (CDN) + bespoke `main.css` for the parish palette (Marian blue + warm cream + burgundy accent + liturgical gold)
- JWT auth in an `auth_token` cookie, bcrypt hashing
- Docker multi-stage build, Easypanel-ready

## Project layout

```
olv-parish/
├── main.go                      # Entry point
├── go.mod                       # Module: olv-parish
├── env.example                  # Required and optional env vars
├── Dockerfile                   # Multi-stage build
├── docker-compose.yml           # Dev-only (Postgres + app)
├── migrate.bat / migrate.ps1    # Convenience wrappers for `go run main.go --migrate`
├── internal/
│   ├── auth/        jwt.go               # bcrypt + JWT
│   ├── config/      config.go            # Env-driven config with Validate()
│   ├── database/    database.go          # Connection, Migrate, EnsureAdminSchema
│   ├── handlers/
│   │   ├── admin.go    # Dashboard + Messages + mark/delete
│   │   ├── auth.go     # Login / Logout / Register (admin-only)
│   │   ├── contact.go  # Public Contact form submit
│   │   ├── site.go     # Site funcMap: {site} / {photo} / {currentYear}
│   │   ├── user.go     # Admin user management
│   │   └── web.go      # Home / Contact / Admin login HTML handlers
│   ├── middleware/  auth.go, middleware.go
│   ├── models/      user.go, contact_message.go
│   └── routes/      routes.go            # Route table
└── web/
    ├── static/      css/main.css, js/main.js, images/
    └── templates/   base.html + per-page content partials
```

## Getting started (local)

### Prerequisites

- Go 1.23+
- PostgreSQL 15+ (or run via `docker-compose`)

### Run with Docker Compose

```bash
docker-compose up --build
# then (first run only) migrate:
docker-compose exec app ./main --migrate
```

Visit <http://localhost:8080>. The admin login lives at <http://localhost:8080/admin/login>.

### Run against your own Postgres

```bash
cp env.example .env
# edit .env: set DB_HOST, DB_PASSWORD, JWT_SECRET, SESSION_SECRET
# (secrets must be 32+ random chars; validate rejects "your-…" placeholders)

go mod tidy
go run main.go --migrate   # first time only
go run main.go
```

### Promoting the first admin

The site has no public sign-up; staff accounts are created from the admin dashboard. The first admin is promoted directly in the database:

```sql
-- after creating a user (via /api/v1/admin/auth/register, protected by an existing admin,
-- OR by inserting manually), flip the role:
UPDATE users SET role = 'admin' WHERE email = 'you@example.com';
```

Or for the very first user, insert a bcrypted row directly — matching the pattern in [`internal/handlers/auth.go`](internal/handlers/auth.go).

## Deployment (Easypanel)

Same shape as Wildflower-Go. Required env vars:

- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`
- `JWT_SECRET`, `SESSION_SECRET` (32+ chars each)
- `FRONTEND_URL`, `GIN_MODE=release`, `PORT=8080`

Health check: `GET /health` returns `{"status":"healthy","database":true,...}`.

See [`EASY_PANEL_DEPLOYMENT.md`](EASY_PANEL_DEPLOYMENT.md).

## Roadmap

Full plan lives in `.claude/plans/swirling-jingling-whale.md`. Short version:

| Phase | Scope | Status |
|---|---|---|
| 1 | Scaffold, strip commerce bits, parish branding, home + contact + admin shell | ✅ |
| 2 | Content model (Page / Block / Bulletin / Homily / Event / MassSchedule / etc.) | — |
| 3 | Public URL tree matching olv14843.org, block renderer, Catholic Courier feed | — |
| 4 | Drag-and-drop block editor (SortableJS), typed admin forms | — |
| 5 | One-shot content import from olv14843.org | — |
| 6 | Public forms: parish registration | — |
| 7 | Deploy to Easypanel + optional DNS cutover | — |
