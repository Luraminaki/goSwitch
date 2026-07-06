# INSTALL

Platform-specific setup for building and running goSwitch. See [README.md](README.md) for what the project actually does.

## TABLE OF CONTENT

<!-- TOC -->

- [INSTALL](#install)
  - [TABLE OF CONTENT](#table-of-content)
  - [REQUIREMENTS](#requirements)
  - [WINDOWS](#windows)
  - [DEBIAN / UBUNTU](#debian--ubuntu)
  - [ARCH LINUX](#arch-linux)
  - [RUNNING THE APP](#running-the-app)
  - [CONFIGURATION FILE](#configuration-file)

<!-- /TOC -->

## REQUIREMENTS

- Go **1.25** or newer (pinned in [go.mod](go.mod))
- Git (to clone the repository)
- No JavaScript toolchain, no Node, no database -- the whole stack is the Go standard library, [Echo](https://echo.labstack.com/), and [htmx](https://htmx.org/) served as a static asset.

Check your Go version with:

```sh
go version
```

## WINDOWS

Pick one:

- **Winget** (Windows 10/11):

  ```powershell
  winget install --id GoLang.Go
  ```

- **Chocolatey**:

  ```powershell
  choco install golang
  ```

- **Manual installer**: download the `.msi` from [go.dev/dl](https://go.dev/dl/) and run it.

After installing, open a **new** terminal (so `PATH` picks up `go`) and confirm with `go version`.

Then clone the repository:

```powershell
git clone https://github.com/Luraminaki/goSwitch.git
cd goSwitch
```

## DEBIAN / UBUNTU

The version shipped in `apt` lags behind upstream, so prefer the official tarball:

```sh
curl -fsSLO https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
rm go1.25.0.linux-amd64.tar.gz
```

Add Go to your `PATH` (append to `~/.bashrc` or `~/.profile`, then restart your shell):

```sh
export PATH=$PATH:/usr/local/go/bin
```

Confirm with `go version`, then clone the repository:

```sh
sudo apt update && sudo apt install -y git
git clone https://github.com/Luraminaki/goSwitch.git
cd goSwitch
```

## ARCH LINUX

Arch's `pacman` repository tracks upstream Go closely enough to use directly:

```sh
sudo pacman -Syu go git
```

Confirm with `go version`, then clone the repository:

```sh
git clone https://github.com/Luraminaki/goSwitch.git
cd goSwitch
```

## RUNNING THE APP

From the `goSwitch` directory, on any platform:

```sh
go run .
```

Or build a standalone executable:

```sh
go build
```

This produces `goSwitch` (or `goSwitch.exe` on Windows) in the current directory, runnable directly. Either way, the app reads [config.json](config.json) from the current working directory at startup, so run it from the repository root (or ship `config.json` alongside the executable).

Once running, open [http://localhost:10000](http://localhost:10000) (or whatever `Port` you configured).

## CONFIGURATION FILE

All limits and defaults live in [config.json](config.json) -- see the [README's CONFIGURATION section](README.md#configuration) for what each key controls. Edit it and restart the app to apply changes (invalid values, e.g. a `Dim` outside `[2, 5]`, are rejected at startup with an explanatory error instead of failing silently).
