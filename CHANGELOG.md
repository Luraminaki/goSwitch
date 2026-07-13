# Changelog

All notable changes to this project are documented here. Versioning loosely
follows [Semantic Versioning](https://semver.org/); the project is still
`-alpha`, so breaking changes can happen between minor versions.

The current version lives in [VERSION](VERSION) and is embedded into the
binary at build time -- it's also shown as a badge in the running app.

## 0.6.0-alpha

Full-codebase review (89 findings, triaged Critical -> High -> Medium -> Low/Info)
and a CI toolchain fix, in that order:

- **Security/concurrency (critical & high):** fixed a grid-solution-corruption bug;
  added row/col and neighborhood bounds validation; fixed a session-eviction race
  (now uses `TryLock`); fixed the session cookie's `MaxAge` not matching its actual
  TTL; added `TrustProxyHeaders` config + a real `IPExtractor` for rate limiting;
  demoted session-ID logging to `Debug`; capped concurrent `/wait` connections
  (unbounded-connection DoS); un-pinned `go.mod`'s exact patch version and added a
  `govulncheck` CI step.
- **Correctness & architecture (medium):** deduped `AvailableToggleSequence`; added a
  request body-size limit; `NewID` now returns an error instead of swallowing one;
  fixed a false "session expired" notice; unified `switchV4`/`switchV8`; made
  `SetPreviousMoves` copy defensively; routed `main.go`'s shutdown errors through
  `slog`; unlocked sessions before `Render` in `RevertMove`/`Switch` instead of
  across it; gated debug-only logging work behind a level check; replaced the
  `map[string]interface{}` template-data pattern with typed `pageState`/
  `pageResponse` structs; various accessibility fixes (checkbox/`aria-live`/
  `aria-label`) and CSS focus-visible/contrast fixes; fixed an unanchored
  `golangci-lint` `revive` exclude regex that silently suppressed an unrelated
  check.
- **Low/info cleanup:** `/revert` is now `POST`-only (was reachable via cross-site
  `GET` navigation); `Grid` now seeds `math/rand` from `crypto/rand` instead of the
  clock (avoids identical boards from concurrent session creation); CSS specificity,
  `prefers-reduced-motion`, mobile-width, and color-token fixes; bumped
  minor-behind `x/*` dependencies and added `go mod verify` to CI; narrowed an
  overly broad `.gitignore` pattern; `logging.go`'s `WithAttrs`/`WithGroup` now
  actually bind/qualify attributes instead of silently dropping them; fixed a
  `t.Cleanup` teardown ordering bug in the test suite; substantially expanded test
  coverage (real-template rendering, session-capacity/waiting-room branches, SSE
  client-disconnect, `SetupLogging`, `trimmedVersion`'s fallback, grid edge cases,
  defensive `CHANGELOG`/`VERSION` parsing).
- **CI:** fixed `govulncheck` failing on an unpatched `crypto/tls` stdlib
  (`GO-2026-5856`): `actions/setup-go@v6` pins `GOTOOLCHAIN=local` via its own
  `$GITHUB_ENV` write, silently overriding `go.mod`'s `toolchain go1.26.5` line and
  leaving CI on an older cataloged patch; now re-exported to `auto` right after
  `setup-go` so `go.mod` is the real source of truth again.

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
