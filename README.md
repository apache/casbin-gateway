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
| Backend  | RESTful API backend for Casbin Gateway | Golang + Beego + MySQL | https://github.com/apache/casbin-gateway                 |

## Installation

Casbin Gateway uses Casdoor to manage members. So you need to create an organization and an application for Casbin Gateway in a Casdoor instance.

### Deployment Options

- **Docker Compose (recommended for a first run)**: one command brings up the whole stack, including a bundled Casdoor — see [Quick start](#quick-start-docker-compose) below
- **[Kubernetes Deployment](k8s/README.md)**: Deploy Casbin Gateway on Kubernetes with complete manifests and guide
- **Manual Installation**: Build and run from source (see [Necessary configuration](#necessary-configuration))

### Quick start (Docker Compose)

Try Casbin Gateway end to end without installing MySQL or setting up Casdoor by
hand. From the repository root:

```shell
docker compose up --build
```

This starts three services:

| Service   | Port  | Description                                  |
|-----------|-------|----------------------------------------------|
| `caswaf`  | 17000 | Casbin Gateway (backend + built web UI)      |
| `casdoor` | 8000  | Bundled, self-hosted Casdoor for login       |
| `db`      | 3306  | MySQL 8 (databases `caswaf` and `casdoor`)   |

Once all three are up, open **http://localhost:17000** and sign in with the
seeded account:

- **username:** `admin`
- **password:** `123`

The Casdoor organization, application, and admin user are seeded automatically
from [`deploy/casdoor/init_data.json`](deploy/casdoor/init_data.json) on first
boot. No private keys are shipped in this repository: Casdoor generates its own
signing certificate on first boot, and a one-shot `cert-export` step hands that
certificate's public half to the gateway so it can verify login tokens.

> ⚠️ The bundled Casdoor and the `admin/123` account are for **local evaluation
> only**. For any real deployment, point `casdoorEndpoint` / `clientId` /
> `clientSecret` at your own Casdoor instance and remove these demo credentials.

### Necessary configuration

#### Get the code

```shell
go get github.com/casdoor/casdoor
go get github.com/apache/casbin-gateway
```

or

```shell
git clone https://github.com/casdoor/casdoor
git clone https://github.com/apache/casbin-gateway
```

#### Setup database

Casbin Gateway will store its users, nodes and topics information in a MySQL database named: `caswaf`, will create it if not existed. The DB connection string can be specified at: https://github.com/apache/casbin-gateway/blob/master/conf/app.conf

```ini
dataSourceName = root:123@tcp(localhost:3306)/
```

Casbin Gateway uses XORM to connect to DB, so all DBs supported by XORM can also be used.

#### Configure Casdoor

After creating an organization and an application for Casbin Gateway in a Casdoor, you need to update `clientID`, `clientSecret`, `casdoorOrganization` and `casdoorApplication` in app.conf.

#### Run Casbin Gateway

- Configure and run Casbin Gateway by yourself. If you want to learn more, see the [documentation](https://caswaf.org).
- Open browser: http://localhost:16001/

### Optional configuration

#### Setup your WAF to enable some third-party login platform

Casbin Gateway uses Casdoor to manage members. If you want to log in with oauth, you should see [casdoor oauth configuration](https://casdoor.org/docs/provider/oauth/overview).

#### OSS, Mail, and SMS services

Casbin Gateway uses Casdoor to upload files to cloud storage, send Emails and send SMSs. See Casdoor for more details.

## Contribute

For Casbin Gateway, if you have any questions, you can open Issues, or you can also directly start Pull Requests(but we recommend opening issues first to communicate with the community).

## License

[Apache-2.0](https://github.com/apache/casbin-gateway/blob/master/LICENSE)
