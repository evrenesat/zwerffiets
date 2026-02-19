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
