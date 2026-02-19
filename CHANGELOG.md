# Changelog

All notable changes to this repository are documented in this file.

## 2026-02-19

### Security and Hardening

- Restricted Gin trusted proxies to loopback (`127.0.0.1`, `::1`).
- Tightened municipality scope enforcement for non-admin operator access.
- Strengthened auth/session handling and report access checks across API and admin surfaces.

### Media and Uploads

- Finalized operator photo delivery flow:
  - authorization in Gin
  - production file delivery via `X-Accel-Redirect`
  - development fallback to direct streaming
  - compatibility fallback for legacy extensionless stored paths
- Switched new uploaded photo storage filenames to random names.

### Citizen Reporting UX

- Added client-side photo normalization before upload (JPEG conversion/compression).
- Improved offline queue behavior with malformed-entry pruning and retry policy hardening.
- Added explicit `rate_limited` error mapping in the UI.

### Admin Workflow

- Added server-side pagination for admin users.
- Added scoped city filter options in triage with caching.
- Added/expanded bulk status actions and operator/user management workflows.

### Documentation and OSS Readiness

- Removed public production deployment instructions from root docs.
- Kept production deployment automation and host-specific infra outside tracked repo docs.
- Rewrote `ARCHITECTURE.md` into a concise, current architecture map.
- Compacted internal engineering notes in `DEVLOG.md`.

### Verification

- `bun run test`
- `bun run api:test`

## 2026-02-18

### Citizen Accounts

- Added optional reporter email capture during submission.
- Added magic-link based citizen login/session flow.
- Added automatic claiming of pending anonymous reports after verification.
- Added My Reports UI and corresponding API/session wiring.

### Verification

- `bun run test`
- `bun run api:test`

## 2026-02-17

### Geocoding and Admin Foundations

- Added automated geocoding and municipality enrichment pipeline.
- Extended admin triage/map views with address and city visibility.
- Added core bulk triage mechanics and supporting admin behaviors.

### Verification

- `bun run api:test`
