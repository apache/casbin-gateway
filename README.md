<h1 align="center" style="border-bottom: none;">📦⚡️ Casbin Gateway</h1>
<h3 align="center">An open-source Web Application Firewall (WAF) software developed by Go and React.</h3>
<p align="center">
  <a href="https://github.com/apache/casbin-gateway/actions/workflows/golangci-lint.yml">
    <img alt="Lint" src="https://github.com/apache/casbin-gateway/actions/workflows/golangci-lint.yml/badge.svg">
  </a>
  <a href="https://github.com/apache/casbin-gateway/actions/workflows/build.yml">
    <img alt="Build" src="https://github.com/apache/casbin-gateway/actions/workflows/build.yml/badge.svg">
  </a>
  <a href="https://pkg.go.dev/github.com/apache/casbin-gateway">
    <img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/apache/casbin-gateway.svg">
  </a>
  <a href="https://github.com/apache/casbin-gateway/releases/latest">
    <img alt="GitHub Release" src="https://img.shields.io/github/v/release/apache/casbin-gateway.svg">
  </a>
</p>

<p align="center">
  <a href="https://hub.docker.com/r/casbin/caswaf">
    <img alt="docker pull casbin/caswaf" src="https://img.shields.io/docker/pulls/casbin/caswaf.svg">
  </a>
  <a href="https://hub.docker.com/r/casbin/caswaf">
    <img alt="Docker Image Version (latest semver)" src="https://img.shields.io/badge/Docker%20Hub-latest-brightgreen">
  </a>
  <a href="https://github.com/apache/casbin-gateway/blob/master/LICENSE">
    <img alt="license" src="https://img.shields.io/github/license/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/issues">
    <img alt="GitHub issues" src="https://img.shields.io/github/issues/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/stargazers">
    <img alt="GitHub stars" src="https://img.shields.io/github/stars/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/network">
    <img alt="GitHub forks" src="https://img.shields.io/github/forks/apache/casbin-gateway">
  </a>
  <a href="https://discord.gg/S5UjpzGZjN">
    <img alt="Discord" src="https://img.shields.io/discord/1022748306096537660?logo=discord&label=discord&color=5865F2">
  </a>
</p>

## Online demo

- Read-only site: https://door.caswaf.com (any modification operation will fail)
- Writable site: https://demo.caswaf.com (original data will be restored for every 5 minutes)

## Documentation

https://caswaf.org

## Architecture

Casbin Gateway contains 2 parts:

| Name     | Description                            | Language               | Source code                                              |
|----------|----------------------------------------|------------------------|----------------------------------------------------------|
| Frontend | Web frontend UI for Casbin Gateway     | TypeScript + React + shadcn/ui | https://github.com/apache/casbin-gateway/tree/master/web |
| Backend  | RESTful API backend for Casbin Gateway | Golang + Beego + XORM  | https://github.com/apache/casbin-gateway                 |

## Installation

Casbin Gateway runs standalone out of the box: it stores its data in a local SQLite file, so there is no database server to install, and it signs users in against its own user table, seeding an `admin` account with the password `123` on first start. Connecting it to a [Casdoor](https://casdoor.org) instance is optional, and enables single sign-on plus the Casdoor-backed features listed under [Optional configuration](#optional-configuration).

### Deployment Options

- **Docker Compose**: Use the provided `docker-compose.yml` for quick local setup
- **Manual Installation**: Build and run from source
- **Single binary**: Build one self-contained executable that runs with no files next to it, see [Single binary](#single-binary)

The Agent features are not supported by the default Docker Compose deployment because the container cannot access agents installed on the host.

The reverse-proxy gateway on ports 80 and 443 is disabled by default, so starting the management application does not take over those ports. Set `gatewayEnabled = true` in `conf/app.conf` when you are ready to use the WAF proxy. On Linux and macOS those ports also need root, so for a first try it is easier to point `gatewayHttpPort` at a high port such as `8080`.

### Quick start

From nothing to a request flowing through the gateway, in four steps. No database server is needed: Gateway creates `./data/casbin-gateway.db` on first start.

#### 1. Build the web UI

The backend serves the compiled frontend from `web/build`, so build it once before starting the backend:

```bash
cd web && yarn install && yarn build
```

For frontend development, run `yarn dev` instead. That serves the UI on http://localhost:16002 with hot reload and proxies API calls to the backend on port 17000, so both have to be running.

#### 2. Run the backend

```bash
go run main.go
```

It prints a summary of what it is actually doing — ports, whether the reverse proxy is on, whether the database answered, and which sign-in it will use:

```
+----------------------------------------------------------------------------+
| Casbin Gateway                                                              |
+----------------+-----------------------------------------------------------+
| Management UI  | http://localhost:17000                                     |
| Settings       | conf/app.conf                                              |
| Web UI files   | web/build                                                  |
| Reverse proxy  | enabled                                                    |
| Gateway HTTP   | :8080                                                      |
| Gateway HTTPS  | :8443                                                      |
| Database       | sqlite, file "./data/casbin-gateway.db" (connected)        |
| Sign-in        | built-in user table, Casdoor is not configured             |
| App dir        | ./data/apps                                                |
+----------------+-----------------------------------------------------------+
```

If a port is taken, Gateway says which process holds it and stops, rather than starting half-configured.

#### 3. Sign in and add a site

Open http://localhost:17000, sign in as `admin` with the password `123`, and change it from the "My Account" page.

Then go to **Sites** → **Add**, and set:

- **Domain**: the hostname clients will use, e.g. `test.example.com`
- **Host** and **Port**: where the traffic goes, e.g. `127.0.0.1` and `8000`
- **Mode**: `HTTP` (`HTTPS Only`, the default, redirects plain HTTP away before it reaches the backend)

Save. Then set `gatewayEnabled = true` in `conf/app.conf` and restart the backend — the Sites page shows a warning while the reverse proxy is off, because site configurations do nothing until it is on.

#### 4. Verify the forwarding

Start anything on the backend port you configured, for example:

```bash
python -m http.server 8000
```

The gateway routes on the `Host` header, so no DNS or `hosts` entry is needed to test it:

```bash
curl -H "Host: test.example.com" http://127.0.0.1:8080/
```

You should get your backend's response. A `site not found for host` reply means the request reached Gateway but no site matches that `Host` value.

### Single binary

Normally Gateway reads three things from disk when it starts: `conf/app.conf`, the compiled web UI in `web/build`, and the IP location database `ip/17monipdb.dat`. Building with the `embed` tag bakes all three into the executable, so it can be copied somewhere on its own and started from anywhere:

```bash
cd web && yarn install && yarn build
```

```bash
go build -tags embed -o casbin-gateway .
```

Build the frontend first: everything under `web/build` goes into the binary, so `go build -tags embed` fails to compile while that directory is missing. The Vite config leaves source maps out, which keeps them out of the binary too, where they would cost several times what the code they map costs.

Files on disk always win over the embedded copies, so a single binary can still be configured and developed against without rebuilding it:

| Embedded asset | Overridden by |
| --- | --- |
| `conf/app.conf` | `conf/app.conf` in the working directory, or next to the executable |
| `web/build` | `web/build/index.html` in the working directory, which then serves the whole UI |
| `ip/17monipdb.dat` | `ip/17monipdb.dat` in the working directory |

The startup summary reports which source each one came from:

```
| Settings       | embedded in the binary (put your own conf/app.conf next to it to override) |
| Web UI files   | embedded in the binary                                                     |
```

Being self-contained is about startup, not about staying read-only: a running Gateway still writes `./data` (the SQLite database, deployed apps, agent patch state), `./logs` and `./tmp` relative to its working directory. Start it from the directory where that state belongs.

#### Nightly builds

Every push to `master` rebuilds the [`nightly`](https://github.com/apache/casbin-gateway/releases/tag/nightly) pre-release, with one such archive per platform — Linux, macOS and Windows, `x86_64` — each carrying `LICENSE`, `NOTICE` and `DISCLAIMER`.

**Nightly builds are not official releases.** They are automated builds of whatever is on `master`, and exist so that a change can be tried without a Go and Node toolchain. Anything else should be built from a source release.

On Linux and macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.sh | bash
```

On Windows, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.ps1 | iex
```

Either one downloads the archive for this machine and unpacks it into `~/.local/share/casbin-gateway` (`%LOCALAPPDATA%\casbin-gateway` on Windows) and starts Gateway there. Set `INSTALL_DIR` to install somewhere else, or `NO_START=1` to install without starting.

The `casbin-gateway` command they put on your PATH is a small wrapper rather than the executable itself: Gateway keeps its database, logs and temporary files in the working directory, so the wrapper always starts it in the install directory. Running the executable directly from somewhere else gives you a second, empty installation there.

### Necessary configuration

#### Get the code

```shell
go get github.com/apache/casbin-gateway
```

or

```shell
git clone https://github.com/apache/casbin-gateway
```

#### Setup database

Casbin Gateway stores its users, nodes and topics information in a SQLite file, created on first start. Nothing has to be installed or configured for this; the defaults in https://github.com/apache/casbin-gateway/blob/master/conf/app.conf are:

```ini
driverName = sqlite
dataSourceName =
```

An empty `dataSourceName` means `./data/casbin-gateway.db`, relative to the working directory. Set it to another path to move the file.

Casbin Gateway uses XORM to connect to DB, so all DBs supported by XORM can also be used. To use MySQL instead, point it at your server and Gateway creates the database named by `dbName` if it does not exist:

```ini
driverName = mysql
dataSourceName = root:123@tcp(localhost:3306)/
dbName = casbin_gateway
```

#### Run Casbin Gateway

- Build the web UI once with `cd web && yarn install && yarn build`, then run the backend with `go run main.go`. See [Quick start](#quick-start) for the whole path, and the [documentation](https://caswaf.org) for everything else.
- Open browser: http://localhost:17000/ (the backend serves the compiled UI). During frontend development, `yarn dev` serves it on http://localhost:16002/ instead and proxies API calls to port 17000.
- Sign in as `admin` with the password `123`, then change it from the "My Account" page.

### Optional configuration

#### Connect to Casdoor

Casdoor takes over member management and single sign-on. Create an organization and an application for Casbin Gateway in a [Casdoor](https://casdoor.org) instance, then fill in `casdoorEndpoint`, `clientId`, `clientSecret`, `casdoorOrganization` and `casdoorApplication` in app.conf. The built-in user table is bypassed as soon as `casdoorEndpoint` is set, and sign-in redirects to Casdoor instead.

#### Setup your WAF to enable some third-party login platform

With Casdoor connected, you can log in with oauth: see the [casdoor oauth configuration](https://casdoor.org/docs/provider/oauth/overview).

#### OSS, Mail, and SMS services

Casbin Gateway uses Casdoor to upload files to cloud storage, send Emails and send SMSs. Health-check alerts, the `CAPTCHA` rule action, per-site Casdoor SSO and the resource storage provider are all inactive until Casdoor is configured. See Casdoor for more details.

## Contribute

For Casbin Gateway, if you have any questions, you can open Issues, or you can also directly start Pull Requests(but we recommend opening issues first to communicate with the community).

## License

[Apache-2.0](https://github.com/apache/casbin-gateway/blob/master/LICENSE)
