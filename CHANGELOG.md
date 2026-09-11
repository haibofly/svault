# Changelog

All notable changes to this project are documented here.

## v0.1.1 - 2026-09-11

### Changed

- Release builds now cache the vcpkg installed tree (warmed on the default branch), cutting release time from ~15 minutes to ~2 minutes.
- Upgraded GitHub Actions: `actions/checkout@v7`, `actions/setup-go@v7`, `actions/cache@v6`.

> No functional changes since v0.1.0; this is a build/release-pipeline maintenance release.

## v0.1.0 - 2026-09-11

### Added

- Local encrypted secret manager CLI: `init`, `put`, `get`, `list`, `rm`, `passwd`.
- JSON import/export and encrypted backups (`export --encrypt`, `import`).
- Session password cache backed by Windows DPAPI: `unlock`, `lock`, `status`, with `--ttl` / `--no-cache`.
- Dynamic linking against the official SQLCipher (via vcpkg) and a `make`-driven build.
