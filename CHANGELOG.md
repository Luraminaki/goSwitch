# Changelog

All notable changes to this project are documented here. Versioning loosely
follows [Semantic Versioning](https://semver.org/); the project is still
`-alpha`, so breaking changes can happen between minor versions.

The current version lives in [VERSION](VERSION) and is embedded into the
binary at build time -- it's also shown as a badge in the running app.

## 0.5.0-alpha

- Single source of truth for the version: the root `VERSION` file is now
  embedded into the binary and shown as a badge in the app itself, instead of
  being hand-tracked only in the README.
- Added this CHANGELOG, split out of the README's growing version list.
- Fixed the session cookie's `Secure` attribute being hardcoded `true`, which
  broke session-cookie round-tripping in CI: Go 1.25's `net/http/cookiejar`
  doesn't yet special-case loopback addresses as secure the way newer
  toolchains do. `Secure` is now set conditionally on the actual request
  scheme instead.
- Fixed CI breaking after `golangci-lint-action`'s `latest` silently resolved
  to golangci-lint v2 (a breaking config-schema change). Migrated
  `.golangci.yml` to the v2 schema and pinned the action/lint versions
  instead of tracking `latest`.
- Centered the win banner more robustly on narrow/mobile screens.

## 0.4.2-alpha

- Mobile fix (title clipping on narrow screens).
- Expanded linting (`revive`, `gocritic`, `prealloc`, `nestif`, `errorlint`,
  `wastedassign`, `contextcheck`).
- Idiomatic Go naming cleanups, package doc comments.
- Dependency/vulnerability audit (all clean).

## 0.4.1-alpha

- Structured `log/slog`-based logging (Python `logging`-style formatted
  lines, configurable level).
- Flashing retro "YOU WIN" banner.

## 0.4.0-alpha

- Graceful shutdown.
- Per-IP rate limiting.
- CI (build/vet/format/test/lint) + Dependabot.
- Session-expiry UX notice.
- Win/loading visual feedback.

## 0.3.0-alpha

- Rotating log files.
- Unit/integration test suite.
- Retro synthwave/arcade CSS reskin.

## 0.2.0-alpha

- Bug fixes, dependency upgrades.
- Per-client sessions (capacity limit, TTL, idle timeout) with an
  SSE-based waiting room.

## 0.1.0-alpha

- First release.
