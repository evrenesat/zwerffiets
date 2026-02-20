# DEVLOG

Internal maintainer notes live here.

Public release highlights are tracked in `CHANGELOG.md`.

## 2026-02-21 - Simple Blogging Feature

### Summary

Implemented a light-weight blogging system including database models, admin CRUD interface with rich text editor, and public site listing/detail pages.

### What changed and why

- **Database**: Introduced migration `0014_blog.sql` adding `blog_posts` (slug, title, html content, status) and `blog_media` tables. Added `name` field to `operators` table for author attribution.
- **Backend (Go)**:
  - Created `store_blog.go` for all database interactions (public and admin).
  - Developed `admin_blog_handlers.go` for management operations: List, Create, Update, Delete (via status), and image uploads.
  - Added new view models in `admin_view_models.go` and Dutch/English translations.
- **Admin UI**:
  - Integrated "Blog" in global sidebar.
  - Implemented `blog_list.tmpl` and `blog_edit.tmpl`.
  - Integrated **Quill JS** editor with custom image upload handler targeting `/bikeadmin/blog/media`.
  - Added auto-slug generation from title via Javascript.
- **Public Site (SvelteKit)**:
  - Added the `BlogPost` type and a `blog.ts` API client.
  - Linked "Blog" in the topbar navigation.
  - Created `/blog` (listing) and `/blog/[slug]` (post detail) routes with basic styling in `blog.css`.
- **Infrastructure**: Added `api/v1/blog/media/:filename` endpoint to serve blog-specific assets.

### Gotchas / follow-up

- The editor uses a CDN version of Quill JS. For offline-first environments, this may need local hosting.
- Basic excerpt logic in the frontend uses regex to strip HTML; could be enhanced with a proper server-side summary field.

### Verification

- Backend unit tests passed (`go test ./...` in apps/api).
- Frontend Vitest suite passed.
- Manual verification of form submission, slug generation, and image persistence confirmed.

## 2026-02-20 - Private production context handover doc

### Summary

Added a private production context handover document to speed up future operational/debug sessions.

### What changed and why

- Added `infra/PRODUCTION_CONTEXT.md` (ignored by Git via `infra` rule).
- Captured stable production facts in one place:
  - host/domain/runtime topology
  - canonical paths, services, and routing contract
  - deploy flow (`infra/deploy/poor_mans_ci.sh`)
  - Postgres/TCP/password handling notes
  - media serving and permission troubleshooting runbooks

### Verification

- `bun run test`
- `bun run api:test`

## 2026-02-20 - Dynamic Content Management

### Summary

Implemented a dynamic content management feature allowing administrators to edit text content on the front page, About Us, and Privacy Policy pages.

### What changed and why

- **Database**: Introduced migration `0012_dynamic_content.sql` to add the `site_contents` table with predefined keys for frontend copy.
- **Backend API**:
  - Created a thread-safe in-memory cache loaded on backend startup (`InitContentCache`) that flushes updates synchronously (`SaveSiteContent`).
  - Implemented `GET /api/v1/content` as a public endpoint to deliver cached translations efficiently.
- **Admin UI**:
  - Wired routes `GET /bikeadmin/content` and `POST /bikeadmin/content`.
  - Added a new `content_manager.tmpl` handling parallel Dutch and English localized form inputs.
  - Linked in the global admin sidebar.
- **Frontend App**:
  - `loadDynamicContent` invoked on `+layout.svelte` initialization, fetching from the API and overwriting static fallback keys inside `$lib/i18n/translations.ts`.
  - Triggered reactivity on Svelte's `$uiLanguage` store explicitly so overrides manifest immediately.

### Gotchas / follow-up

- Resolved: Migrated Svelte server-side dummy mocks (`memory-repository.ts` etc.) to use integer IDs to match the backend refactor, eliminating all `npm run check` errors.

### Verification

- Backend API tests passed (`go test ./...` in apps/api)
- Dynamic content confirmed loaded and overlaid in `t()` usage (About, Privacy, etc).

## 2026-02-20 - Showcase Editor Refinements

### Summary

Refined the Showcase Editor feature to include scale controls, auto-filling capabilities, and more natural panning.

### What changed and why

- **Database**: Introduced migration `0011_showcase_scale.sql` adding `scale_percent` to `showcase_items`.
- **Backend API**:
  - Expose `scalePercent` via models on `store_showcase.go` and `api_showcase.go`.
  - Pass the current database configurations injected as JSON to `showcase_editor.tmpl` in `admin_handlers.go`.
  - Added "Showcase" to the global navigation bar in `layout.tmpl` for easier access without first finding a report.
- **Admin UI**:
  - Built out slot `change` event listeners to deserialize and auto-fill previous slot settings (image URL, scale limits, and offsets).
  - Inverted the drag logic so translating a mouse right dynamically moves the image visually right instead of the focal center (making panning more intuitive and physical).
  - Added an HTML slider for scaling.
  - Fix: Fixed a Javascript overwrite bug that prematurely replaced the "Add to Showcase" active photograph with an empty/saved db state upon opening the dropdown.
  - Fix: Wrote an exact linear percentage scale constraint ratio calculation that accounts for intrinsic aspect ratios to make mouse dragging visually 1:1 on both horizontal and vertical bounds when zooming.
- **Frontend App**: Patched `+page.svelte` iterator to inline `transform: scale(n)` natively masking using its generic wrapper block styling, resolving vertical pan clamping by adding dynamic `transform-origin`.
- **Photo Serving**: Implemented public endpoint `/api/v1/showcase/:slot/photo` to serve photos securely via `X-Accel-Redirect` (production) or direct file serving (local), fixing the broken photo preview for existing slots.
- **Fix (Editor Image)**: Removed a Go `{{if .PhotoURL}}` conditional in `showcase_editor.tmpl` that was preventing the `#preview-wrapper` and `#pan-image` elements from being rendered when entering the editor via main navigation, which blocked JavaScript from updating the image source.

### Verification

- Database successfully reflects integer scaled dimensions
- UI correctly calculates reverse offsets and parses integers from native JS dom changes
- Component compiles properly matching new `ShowcaseItem` typescript shape
- Native Photo URI proxy now actively falls back to direct `c.File()` routing on non-production `api_showcase` paths.

## 2026-02-20 - Implement Showcase Editor

### Summary

Added a Showcase Editor feature for site administrators to feature up to four chosen report photos on the public landing page, complete with custom subtitles and focal point cropping.

### What changed and why

- **Database**: Added `showcase_items` table in `0010_showcase.sql` to persist slots (1-4), report photo associations, subtitles, and focal coordinates.
- **Backend API**:
  - `store_showcase.go` for data access logic.
  - `api_showcase.go` for public unauthenticated endpoints to fetch items and serve photos via `X-Accel-Redirect` safely.
  - Added new admin models and integrated "Add to showcase" to `report_detail.tmpl`.
  - Added `showcase_editor.tmpl` with interactive drag UI for focal coordinate selection.
- **Frontend App**: Modified `apps/web/src/routes/+page.svelte` to dynamically fetch and interpolate backend showcase data via `object-fit: cover` and `object-position`. Fallback remains strict to original static site images if not configured.

### Gotchas / follow-up

- Frontend Svelte component contains multiple preexisting linting errors for typings which are suppressed during runtime but may need a broader TS refactor.
- Public images are queried through an individualized endpoint `/api/v1/showcase/:slot/photo` to securely proxy internal uploads without exposing other protected `report_photos`.

### Verification

- `go test ./...` in `apps/api`
- `npm run test` and `npm run build` in `apps/web`

## 2026-02-19 - Changelog split

### Summary

Split documentation roles:

- `CHANGELOG.md`: public, release-facing highlights.
- `DEVLOG.md`: internal maintainer notes only.
- `infra/PRIVATE_NOTES_TEMPLATE.md` (ignored): private operational note template.

### Verification

- `bun run test`
- `bun run api:test`

## 2026-02-19 - Remove machine-specific identifiers

### Summary

Removed user/machine-specific references from tracked source and docs.

### What changed and why

- Replaced absolute local repository paths in `README.md` with repository-relative paths.
- Replaced GitHub links containing personal username in:
  - `apps/web/src/routes/+layout.svelte`
  - `apps/web/src/routes/about/+page.svelte`

### Gotchas / follow-up

- No behavior change expected. This is a privacy hardening/doc cleanup only.

### Verification

- `git grep -n "<private-name>\\|/Users/<username>/code/<repo>"`
- `bun run test`
- `bun run api:test`
