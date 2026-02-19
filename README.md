# ZwerfFiets

**ZwerfFiets** is an open-source, privacy-first platform designed to streamline the reporting and removal of abandoned bicycles in public spaces.

Built for municipalities and citizens, it bridges the gap between community reports and efficient enforcement workflows.

## Why ZwerfFiets?

Abandoned bicycles crowd public racks and obstruct sidewalks. ZwerfFiets solves this by providing:

- **Frictionless Reporting**: No app to install. Citizens can report a bike in seconds via a mobile-friendly web interface that captures precise GPS location and photos.
- **Operator Efficiency**: A dedicated dashboard for city enforcement teams to triage reports, deduplicate submissions, and track removal status.
- **Privacy by Design**: All photos are stripped of EXIF data on upload. Reporting is anonymous but protected by hashed tracking cookies to prevent abuse.
- **Smart Signals**: Distinguishes between duplicate reports (same reporter, same day) and valuable "double-confirmation" signals (different reporters confirming a bike is still abandoned weeks later).

## Technical Overview

ZwerfFiets is built as a modern, high-performance web application:

- **Frontend**: SvelteKit static SPA. Fast, responsive, and works offline.
- **Backend**: Go (Gin framework). Handles validation, image processing, JSON APIs, and server-rendered admin pages.
- **Database**: PostgreSQL for robust data integrity and geospatial queries.

---

## Local development

### 1. Prerequisites

- Bun
- Go (1.22+ recommended)
- Postgres

### 2. Configure environment

```bash
cp .env.example .env
```

Required values:

- `APP_SIGNING_SECRET` (16+ chars)
- Postgres via `DATABASE_URL` or `PG*`/`POSTGRES_*`
- Optional GPS cap override: `MAX_LOCATION_ACCURACY_M` (default `3000`)

### 3. Install deps

```bash
bun install
```

### 4. Start API

```bash
bun run api:dev
```

Default bind: `:8080` (LAN-accessible). Override with `GIN_ADDR`.

### 5. Start web dev server

```bash
bun run dev:lan
```

- Web: `http://<your-lan-ip>:5173`
- API proxy target: `DEV_API_PROXY_TARGET` (default `http://127.0.0.1:8080`)

## Checks and tests

```bash
bun run check
bun run test
bun run build
bun run api:test
```

## Key paths

- Frontend: `/Users/evren/code/zwerffiets/apps/web`
- Backend: `/Users/evren/code/zwerffiets/apps/api`
- Architecture: `/Users/evren/code/zwerffiets/ARCHITECTURE.md`
- Changelog (public): `/Users/evren/code/zwerffiets/CHANGELOG.md`
- Dev log (internal): `/Users/evren/code/zwerffiets/DEVLOG.md`
