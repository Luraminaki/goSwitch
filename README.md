# GOSWITCH

Couple of years ago (back in early 2022), a friend asked me to give him a hand for a `Kotlin` game he was trying to code: A 3x3 grid of 2 states switches that, when toggled, change the state of their neighbours in a + shaped area around the aforementioned toggled switches. (I later learned that the game is called [Lights out](https://en.wikipedia.org/wiki/Lights_Out_(game)))

Being a complete noob in `Kotlin`, I whipped up a [python draft](#python-draft) script in order to illustrate how I'd proceed...

Fast forward to 2024, after recieving some UIX advices from a former colleague and friend for a [side project](https://github.com/Luraminaki/pySET), he suggested me to give a try to `Go` and `HTMX`... And for some reason, this game from 2022 just popped in my brain... So... Here `Go`es nothing I guess... :D

_although some parts could have been a lot simpler, had I used some `JS`, I feel that it would have defeated the whole purpose of this project which was to stick to `Go` and `HTMX` as much as possible..._

## VERSION

The current version lives in [VERSION](VERSION) -- it's embedded into the binary at
build time and shown as a badge in the running app. See [CHANGELOG.md](CHANGELOG.md)
for the full version history.

## TABLE OF CONTENT

<!-- TOC -->

- [GOSWITCH](#goswitch)
  - [VERSION](#version)
  - [TABLE OF CONTENT](#table-of-content)
  - [TL;DR](#tldr)
  - [INSTALL AND RUN](#install-and-run)
  - [CONFIGURATION](#configuration)
  - [SESSIONS](#sessions)
  - [LOGGING](#logging)
  - [TESTING](#testing)
  - [DEVELOPMENT](#development)
  - [PYTHON DRAFT](#python-draft)

<!-- /TOC -->

## TL;DR

"I don't want to install anything or read anything, just make it quick and easy please." I hear you say? Sure, just click [here](https://goswitch.onrender.com/) and have fun. 

## INSTALL AND RUN

For detailed, platform-specific setup (Windows, Debian/Ubuntu, Arch), see [INSTALL.md](INSTALL.md).

Quick start, once `Go` is installed:

```sh
go run .
```

Then open your favorite web browser to [start the game](http://localhost:10000).

To compile an executable instead:

```sh
go build
```

## CONFIGURATION

Everything is driven by [config.json](config.json), read once at startup:

| Key                               | Meaning                                                                                   |
|------------------------------------|--------------------------------------------------------------------------------------------|
| `Port`                              | TCP port the server listens on                                                             |
| `Cheat`                             | Default: reveal the winning combination in a new session                                   |
| `Dim`                               | Default grid size (`N x N`), also the bound for the in-game grid-size field (`[2, 5]`)      |
| `ToggleSequence`                    | Default pattern selection, parallel to `AvailableToggleSequence`                            |
| `AvailableToggleSequence`           | The full set of selectable neighborhood patterns (`0`: self, `4`: plus-shaped, `8`: diagonals) |
| `MaxSessions`                       | Max number of concurrent per-client sessions                                               |
| `SessionTTLSeconds`                 | Absolute max lifetime of a session, from creation                                          |
| `SessionIdleTimeoutSeconds`         | Max inactivity a session can accrue once `MaxSessions` is reached (see [SESSIONS](#sessions)) |
| `SessionWaitCheckIntervalSeconds`   | How often a waiting client is silently re-checked for a freed-up slot                       |
| `LogFilePath`                       | Path to the rotating log file (see [LOGGING](#logging))                                    |
| `LogMaxSizeMB`                      | Max size (MB) a log file reaches before it's rotated                                       |
| `LogMaxBackups`                     | Max number of rotated log files kept around                                                |
| `LogLevel`                          | Minimum level logged: `DEBUG`, `INFO`, `WARN`, or `ERROR`                                   |
| `RateLimitRequestsPerSecond`        | Sustained requests/second allowed per client IP                                            |
| `RateLimitBurst`                    | Max requests a single client IP can burst above the sustained rate                          |
| `TrustProxyHeaders`                 | Whether to trust `X-Forwarded-For`/`X-Forwarded-Proto` (see below)                          |

`TrustProxyHeaders` should stay `false` for a bare `go run .`/direct-exposed deployment (the
default) -- otherwise a direct client could spoof those headers to dodge the per-IP rate limit
or force the session cookie's `Secure` flag off over an actual TLS connection. Deployments that
really do sit behind a reverse proxy that sets those headers (e.g. Render's edge) should set it
to `true` via the `GOSWITCH_TRUST_PROXY_HEADERS` environment variable rather than editing the
committed `config.json`, so the same file works correctly for both local dev and production.

## SESSIONS

Each client gets its own isolated grid, tracked via a cookie, capped at `MaxSessions` concurrent players.

Purging is lazy: nothing is evicted while there's a free slot. Only when a new client shows up and every slot is taken does the server look for something to reclaim, in order:

1. Any session past its `SessionTTLSeconds` (absolute lifetime).
2. Any session past its `SessionIdleTimeoutSeconds` (inactivity) -- this idle check only ever runs under this capacity pressure, never proactively.
3. If nothing is reclaimable, the new client waits.

A waiting client isn't polling: it opens a single [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) connection (via the vendored [htmx-ext-sse](https://github.com/bigskysoftware/htmx-extensions/tree/main/src/sse) extension, no hand-written JS) and is pushed straight into a live game the moment a slot frees up.

If a client comes back with a cookie for a session that's since been purged (evicted under capacity pressure while they were away), they're handed a fresh game along with a small on-screen notice explaining what happened, instead of a silently reset board.

## LOGGING

All server output goes through the standard `log/slog` package with a custom handler (in `utils.SetupLogging`), formatted as:

```
[2006-01-02 15:04:05,000] [pid] [name] [LEVEL]: funcName -- message
```

i.e. the Python `logging` module's classic `"[%(asctime)s] [%(process)s] [%(name)s] [%(levelname)s]: %(funcName)s -- %(message)s"`. Lines are written to both stdout and a rotating file at `LogFilePath`, via [lumberjack](https://github.com/natefinch/lumberjack). Once a log file reaches `LogMaxSizeMB`, it's rotated; once more than `LogMaxBackups` rotated files have piled up, the oldest is deleted. The log directory is created automatically if it doesn't exist. Only lines at or above `LogLevel` are emitted.

## TESTING

```sh
go test ./...
```

Covers unit tests per package (`grid`, `utils`, `session`, `template`) plus integration tests at the repo root (`main_test.go`) that spin up the real server and drive it over HTTP: full gameplay flow, per-client session isolation, the capacity/idle-timeout/SSE-waiting-room path, rate limiting, the session-expiry notice, and regression tests for two previously-fixed bugs (a crash on malformed requests, and a reflected-XSS in error messages).

## DEVELOPMENT

CI ([.github/workflows/ci.yml](.github/workflows/ci.yml)) runs on every push/PR: `gofmt` check, `go build`, `go vet`, `go test ./...`, and [golangci-lint](https://golangci-lint.run/) (config: [.golangci.yml](.golangci.yml)). [Dependabot](.github/dependabot.yml) keeps `go.mod` and the CI Actions themselves up to date automatically (the vendored, self-hosted JS in `webui/assets/` isn't Go-module-tracked, so that still needs an occasional manual check upstream).

The server shuts down gracefully on `SIGINT`/`SIGTERM` (or Ctrl+C): in-flight requests get up to 10 seconds to finish before the listener is forced closed.

## PYTHON DRAFT

<details>
<summary>CODE</summary>

```py
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Created on Mon Jan 31 22:59:51 2022

@author: Luraminaki
"""

#===================================================================================================
import random

#pylint: disable=wrong-import-order, wrong-import-position

#pylint: enable=wrong-import-order, wrong-import-position
#===================================================================================================


__version__ = '0.1.0'


class Grid():
    def __init__(self, dim: int, neighborhood: list[int]):
        self._rows = dim
        self._cols = dim
        self._neighborhood = neighborhood

        self._grid: list[int] = []
        self._solution: list[int] = [] # Not necessarily the fastest

        self._rand = random.Random()

        self.init_game()
        while self.check_win():
            self.init_game()


    def init_game(self) -> None:
        grid_size = self._rows*self._cols
        start = self._rand.choice(range(1))

        self._grid = [start] * grid_size
        self._solution = self._rand.sample(list(range(grid_size)),
                                           k=self._rand.choice(range(grid_size)) + 1)
        self._solution.sort()

        for hit in self._solution:
            self.switch(hit)


    def coord_flat_to_cart(self, dim: int) -> tuple[int]:
        if dim >= len(self._grid):
            return -1, -1
        return (dim % self._cols, dim // self._rows)


    def check_oob(self, x: int, y: int) -> bool:
        if (0 <= x < self._cols) and (0 <= y < self._rows):
            return True
        return False


    def switch_v4(self, x: int, y: int) -> list[list[int]] | list:
        coords_to_switch = []

        if self.check_oob(x + 1, y):
            coords_to_switch.append([x+1, y])
        if self.check_oob(x, y+1):
            coords_to_switch.append([x, y+1])
        if self.check_oob(x - 1, y):
            coords_to_switch.append([x-1, y])
        if self.check_oob(x, y - 1):
            coords_to_switch.append([x, y-1])

        return coords_to_switch


    def switch_v8(self, x: int, y: int) -> list[list[int]] | list:
        coords_to_switch = []

        if self.check_oob(x + 1, y + 1):
            coords_to_switch.append([x+1, y+1])
        if self.check_oob(x - 1, y - 1):
            coords_to_switch.append([x-1, y-1])
        if self.check_oob(x + 1, y - 1):
            coords_to_switch.append([x+1, y-1])
        if self.check_oob(x - 1, y + 1):
            coords_to_switch.append([x-1, y+1])

        return coords_to_switch


    def switch(self, pos: int) -> None:
        x, y = self.coord_flat_to_cart(pos)

        if not self.check_oob(x, y):
            return None

        coords_to_switch = []

        for val in self._neighborhood:
            if val == 0:
                coords_to_switch = coords_to_switch + [[x, y]]

            elif val == 4:
                coords_to_switch = coords_to_switch + self.switch_v4(x, y)

            elif val == 8:
                coords_to_switch = coords_to_switch + self.switch_v8(x, y)

            else:
                continue

        for cx, cy in coords_to_switch:
            self._grid[cx + self._cols*cy] = int(not self._grid[cx + self._cols*cy])


    def get_possible_solution(self) -> list[int]:
        return self._solution.copy()


    def get_grid(self) -> list[int]:
        return self._grid.copy()


    def check_win(self) -> bool:
        if sum(self._grid) in [0, self._rows*self._cols]:
            return True
        return False


    def pretty_print_grid(self) -> None:
        print("Game Layout:")
        line = ""
        r = 0
        while r < self._rows:
            c = 0
            while c < self._cols:
                line = line + str(self._grid[c + self._cols*r]) + " "
                c = c + 1
            print(line)
            line = ""
            r = r + 1
        print("")


def main() -> None:
    dim = 3
    switch_game = Grid(dim, [0, 4])

    print(f"Possible solution: {switch_game.get_possible_solution()}")

    switch_game.pretty_print_grid()

    while not switch_game.check_win():
        print(f"Input Switch Position (0 ~ {(dim*dim)-1}):")
        try:
            pos = int(input())
        except Exception as err:
            print(f"Error when reading input value: {repr(err)}")
            continue

        print(f"Switching ({pos})\n")
        switch_game.switch(pos)
        switch_game.pretty_print_grid()
        print(f"Did I Win: {'Yes' if switch_game.check_win() else 'No'}")


if __name__ == "__main__":
    main()
```
</details>
