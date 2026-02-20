# ZwerfFiets Architecture

## Overview

ZwerfFiets is a web-first abandoned-bike reporting platform.

- Public UX: static SvelteKit app (`/`, `/report`, `/login`, `/my-reports`, `/blog`, `/report/status/*`)
- Admin UX: Gin-rendered HTML workspace (`/bikeadmin/*`)
- API: Gin JSON endpoints under `/api/v1/*`
- Storage: PostgreSQL + filesystem (`DATA_ROOT`) for photos/exports

## Repository Structure

- `apps/web`: SvelteKit frontend (adapter-static)
- `apps/api`: Gin backend, migrations, admin templates, business logic
- `libs/mailer`: shared mailer library for log/resend providers

## Runtime Components

### Frontend (SvelteKit static)

- Built as static files via `@sveltejs/adapter-static`
- Uses build-id app directory (`_app_<BUILD_ID>`) for cache-busting
- Registers a service worker in production
- Dynamic routes that depend on runtime API data (for example `/blog/[slug]`) opt out of prerendering.
- Uses `/api/v1/*` for API calls; `/bikeadmin/*` is server-rendered by API

### Backend (Gin API + SSR admin)

- Handles report intake, dedupe/signal logic, geocoding, exports, auth/session
- Serves operator admin pages via server-rendered templates at `/bikeadmin/*`
- Runs DB migrations on startup before serving traffic
- Supports maintenance commands:
  - `run-export [weekly|monthly]`
  - `backfill-addresses`
  - `seed-municipality-operators`
  - `send-municipality-reports`

### Data Stores

- PostgreSQL is source of truth for reports, photos metadata, events, operators, users, exports
- Filesystem under `DATA_ROOT` stores:
  - `uploads/reports/<report_id>/<random_filename>.<ext>`
  - `exports/*`
  - `uploads/blog/*` (for blog media)

## Reverse Proxy Contract (Production)

This repo does not ship production infra configs, but the runtime contract is:

1. `/api/v1/*` -> Gin (`127.0.0.1:8080`)
2. `/bikeadmin/*` -> Gin (`127.0.0.1:8080`)
3. `/_protected_media/*` -> internal-only alias to `DATA_ROOT` (for authorized media delivery)
4. `/operator/*` -> `404` decoy path
5. all other paths -> static frontend with SPA fallback to `/index.html`

## Core Flows

### Report Creation

1. Client collects geolocation + 1..3 photos + 1..10 tags + optional note.
2. Web app normalizes photos client-side (JPEG conversion + compression target).
3. API validates payload, rate limits, validates active tags, strips EXIF on JPEG re-encode.
4. API stores report row + photo metadata + files + event log in a transaction.
5. API recomputes bike-group signal state and dedupe candidates.
6. API asynchronously geocodes and updates address/city/postcode/municipality.

### Citizen Access

- Optional email-based magic-link login (`/api/v1/auth/request-magic-link`, `/api/v1/auth/verify`)
- User session cookie: `zwerffiets_user_session`
- `/api/v1/user/reports` returns reports linked to authenticated user
- Anonymous reports with matching email are claimed after magic-link verification

### Operator/Admin Access

- Operator session cookie: `zwerffiets_operator_session`
- API operator routes at `/api/v1/operator/*`
- SSR admin workspace at `/bikeadmin/*`
- Municipality scoping rules:
  - `admin`: unrestricted
  - `municipality_operator`: restricted to own municipality; nil municipality is denied

### Photo Delivery

- Operator photos are requested via `/api/v1/operator/reports/:id/photos/:photoID`
  - Handler always performs auth/scope checks first
  - Production: returns `X-Accel-Redirect` to `/_protected_media/...`
  - Development: Gin reads and streams file directly
- Public showcase photos are served via `/api/v1/showcase/:slot/photo`
  - Unauthenticated, but strictly limits access to explicitly configured showcase images.
  - Production delivery mechanism is identical to operator photos (`X-Accel-Redirect`).
- Public blog media are served via `/api/v1/blog/media/:filename`
  - Served directly from `DATA_ROOT/blog` (local) or via `X-Accel-Redirect` (production).
- Legacy compatibility: extensionless stored paths are resolved via on-disk `.*` fallback

## Business Rules

- Status lifecycle:
  - `new -> triaged -> forwarded -> resolved`
  - `new|triaged|forwarded -> invalid`
- Signal logic distinguishes:
  - same-day repeat (ignored for reconfirmation counters)
  - same-reporter reconfirmation
  - distinct-reporter reconfirmation
- Municipality field is derived from geocoded city using Dutch municipality mapping

## Security Controls

- Trusted proxies restricted to loopback (`127.0.0.1`, `::1`)
- CORS allowlist is explicit (`PUBLIC_BASE_URL` + local dev origins in development)
- HttpOnly auth cookies; `Secure` enabled in production
- Atomic magic-link consumption (`UPDATE ... WHERE used_at IS NULL AND expires_at > NOW() RETURNING`)
- Signed tracking tokens for report status links
- In-memory rate limiter + cleanup cycle
- Path-safe resolution for media files under `DATA_ROOT`

## Development Topology

- API server: `:8080`
- Vite dev server: `:5173`
- Vite proxies `/api/v1` and `/bikeadmin` to `DEV_API_PROXY_TARGET` (default `http://127.0.0.1:8080`)

## Operational Note

Production deployment automation and host-specific infrastructure are intentionally maintained outside this repository.
