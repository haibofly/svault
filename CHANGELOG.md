# Changelog

All notable changes to this project are documented here.

## v0.1.0 - 2026-09-11

### Added

- Local encrypted secret manager CLI: `init`, `put`, `get`, `list`, `rm`, `passwd`.
- JSON import/export and encrypted backups (`export --encrypt`, `import`).
- Session password cache backed by Windows DPAPI: `unlock`, `lock`, `status`, with `--ttl` / `--no-cache`.
- Dynamic linking against the official SQLCipher (via vcpkg) and a `make`-driven build.
