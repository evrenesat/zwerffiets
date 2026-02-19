# DEVLOG

Internal maintainer notes live here.

Public release highlights are tracked in `CHANGELOG.md`.

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
