<h1 align="center" style="border-bottom: none;">📦⚡️ Casbin Gateway</h1>
<h3 align="center">An open-source Web Application Firewall (WAF) software developed by Go and React.</h3>
<p align="center">
  <a href="#badge">
    <img alt="semantic-release" src="https://img.shields.io/badge/%20%20%F0%9F%93%A6%F0%9F%9A%80-semantic--release-e10079.svg">
  </a>
  <a href="https://hub.docker.com/r/casbin/caswaf">
    <img alt="docker pull casbin/caswaf" src="https://img.shields.io/docker/pulls/casbin/caswaf.svg">
  </a>
  <a href="https://github.com/apache/casbin-gateway/releases/latest">
    <img alt="GitHub Release" src="https://img.shields.io/github/v/release/apache/casbin-gateway.svg">
  </a>
  <a href="https://hub.docker.com/r/casbin/caswaf">
    <img alt="Docker Image Version (latest semver)" src="https://img.shields.io/badge/Docker%20Hub-latest-brightgreen">
  </a>
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/apache/casbin-gateway">
    <img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/apache/casbin-gateway?style=flat-square">
  </a>
  <a href="https://github.com/apache/casbin-gateway/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/apache/casbin-gateway?style=flat-square" alt="license">
  </a>
  <a href="https://github.com/apache/casbin-gateway/issues">
    <img alt="GitHub issues" src="https://img.shields.io/github/issues/apache/casbin-gateway?style=flat-square">
  </a>
  <a href="#">
    <img alt="GitHub stars" src="https://img.shields.io/github/stars/apache/casbin-gateway?style=flat-square">
  </a>
  <a href="https://github.com/apache/casbin-gateway/network">
    <img alt="GitHub forks" src="https://img.shields.io/github/forks/apache/casbin-gateway?style=flat-square">
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
| Frontend | Web frontend UI for Casbin Gateway     | Javascript + React     | https://github.com/apache/casbin-gateway/tree/master/web |
| Backend  | RESTful API backend for Casbin Gateway | Golang + Beego + SQLite | https://github.com/apache/casbin-gateway                |

## Installation

Casbin Gateway runs standalone out of the box: it signs users in against its own user table, seeding an `admin` account with the password `123` on first start. Connecting it to a [Casdoor](https://casdoor.org) instance is optional, and enables single sign-on plus the Casdoor-backed features listed under [Optional configuration](#optional-configuration).

### Deployment Options

- **[Kubernetes Deployment](k8s/README.md)**: Deploy Casbin Gateway on Kubernetes with complete manifests and guide
- **Docker Compose**: Use the provided `docker-compose.yml` for quick local setup
- **Manual Installation**: Build and run from source

The reverse-proxy gateway on ports 80 and 443 is disabled by default, so starting the management application does not take over those ports. Set `gatewayEnabled = true` in `conf/app.conf` when you are ready to use the WAF proxy. On Linux and macOS those ports also need root, so for a first try it is easier to point `gatewayHttpPort` at a high port such as `8080`.

### Quick start

From nothing to a request flowing through the gateway, in five steps.

#### 1. Use the default SQLite database

No database setup is required. Gateway creates `data/casbin-gateway.db` automatically on first start. MySQL and PostgreSQL remain available as optional external databases through `conf/app.conf`.

#### 2. Build the web UI

The backend serves the compiled frontend from `web/build`, so build it once before starting the backend:

```bash
cd web && yarn install && yarn build
```

For frontend development, run `yarn start` instead. That serves the UI on http://localhost:16001 with hot reload and proxies API calls to the backend on port 17000, so both have to be running.

#### 3. Run the backend

```bash
go run main.go
```

It prints a summary of what it is actually doing — ports, whether the reverse proxy is on, whether the database answered, and which sign-in it will use:

```
+----------------------------------------------------------------------------+
| Casbin Gateway                                                              |
+----------------+-----------------------------------------------------------+
| Management UI  | http://localhost:17000                                     |
| Web UI files   | web/build                                                  |
| Reverse proxy  | enabled                                                    |
| Gateway HTTP   | :8080                                                      |
| Gateway HTTPS  | :8443                                                      |
| Database       | sqlite, file "data/casbin-gateway.db" (connected)           |
| Sign-in        | built-in user table, Casdoor is not configured             |
| App dir        | ./data/apps                                                |
+----------------+-----------------------------------------------------------+
```

If a port is taken, Gateway says which process holds it and stops, rather than starting half-configured.

#### 4. Sign in and add a site

Open http://localhost:17000, sign in as `admin` with the password `123`, and change it from the "My Account" page.

Then go to **Sites** → **Add**, and set:

- **Domain**: the hostname clients will use, e.g. `test.example.com`
- **Host** and **Port**: where the traffic goes, e.g. `127.0.0.1` and `8000`
- **Mode**: `HTTP` (`HTTPS Only`, the default, redirects plain HTTP away before it reaches the backend)

Save. Then set `gatewayEnabled = true` in `conf/app.conf` and restart the backend — the Sites page shows a warning while the reverse proxy is off, because site configurations do nothing until it is on.

#### 5. Verify the forwarding

Start anything on the backend port you configured, for example:

```bash
python -m http.server 8000
```

The gateway routes on the `Host` header, so no DNS or `hosts` entry is needed to test it:

```bash
curl -H "Host: test.example.com" http://127.0.0.1:8080/
```

You should get your backend's response. A `site not found for host` reply means the request reached Gateway but no site matches that `Host` value.

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

Casbin Gateway uses SQLite by default and creates `data/casbin-gateway.db` automatically. No separate database service is required for native, Docker Compose, or Kubernetes deployments.

```ini
driverName = sqlite
dataSourceName = data/casbin-gateway.db
dbName =
```

Docker Compose stores the database in the `gateway-data` volume. Kubernetes stores it in the `caswaf-data` persistent volume claim.

MySQL and PostgreSQL remain available when an external database is needed. Set `driverName`, `dataSourceName`, and `dbName` in `conf/app.conf` for the selected database.

#### Run Casbin Gateway

- Build the web UI once with `cd web && yarn install && yarn build`, then run the backend with `go run main.go`. See [Quick start](#quick-start) for the whole path, and the [documentation](https://caswaf.org) for everything else.
- Open browser: http://localhost:17000/ (the backend serves the compiled UI). During frontend development, `yarn start` serves it on http://localhost:16001/ instead and proxies API calls to port 17000.
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
