<h1 align="center" style="border-bottom: none;">📦⚡️ Casbin Gateway</h1>
<h3 align="center">An open-source WAF, LLM gateway, and AI agent monitoring platform built with Go and React.</h3>
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

Casbin Gateway (formerly CasWAF) provides:

- a reverse proxy with Coraza WAF rules and automatic TLS certificates;
- an OpenAI-compatible LLM gateway at `/v1/chat/completions`, with channel management and failover;
- local AI agent discovery, patching, live activity records, and session inspection.

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

## Quick start

Casbin Gateway uses [Casdoor](https://casdoor.org/) for login. Create an organization and application in an existing Casdoor instance, then start the gateway with their values:

```shell
export CASDOOR_ENDPOINT=https://door.example.com
export CASDOOR_CLIENT_ID=your-client-id
export CASDOOR_CLIENT_SECRET=your-client-secret
export CASDOOR_ORGANIZATION=your-organization
export CASDOOR_APPLICATION=your-application
docker compose up --build
```

Open http://localhost:17000/ after the containers are healthy. MySQL data is stored in the Compose-managed `mysql-data` volume.

The reverse-proxy gateway on ports 80 and 443 is disabled by default, so starting the management application does not take over those ports. Set `gatewayEnabled = true` in `conf/app.conf` when you are ready to use the WAF proxy.

## Installation

### Deployment Options

- **[Kubernetes Deployment](k8s/README.md)**: Deploy Casbin Gateway on Kubernetes with complete manifests and guide
- **Docker Compose**: Use the quick start above
- **Manual Installation**: Build and run from source

### Necessary configuration

#### Get the code

```shell
git clone https://github.com/apache/casbin-gateway
cd casbin-gateway
```

#### Setup database

Casbin Gateway will store its users, nodes and topics information in a MySQL database named: `caswaf`, will create it if not existed. The DB connection string can be specified at: https://github.com/apache/casbin-gateway/blob/master/conf/app.conf

```ini
dataSourceName = root:123@tcp(localhost:3306)/
```

Casbin Gateway uses XORM to connect to DB, so all DBs supported by XORM can also be used.

#### Configure Casdoor

After creating an organization and application for Casbin Gateway in Casdoor, update `clientId`, `clientSecret`, `casdoorEndpoint`, `casdoorOrganization`, and `casdoorApplication` in `conf/app.conf`.

These settings, along with `httpport`, `gatewayEnabled`, `gatewayHttpPort`, `gatewayHttpsPort`, and `dataSourceName`, can also be supplied as environment variables with the same names. Environment variables take precedence over `conf/app.conf`.

#### Run Casbin Gateway

Build the frontend, then run the backend:

```shell
cd web
yarn install
yarn build
cd ..
go run .
```

Open http://localhost:17000/.

For frontend development, run `yarn start` in `web/` and open http://localhost:16001/. The development frontend sends API requests to the backend on port 17000, so keep `go run .` running in another terminal.

### Optional configuration

#### Setup your WAF to enable some third-party login platform

Casbin Gateway uses Casdoor to manage members. If you want to log in with oauth, you should see [casdoor oauth configuration](https://casdoor.org/docs/provider/oauth/overview).

#### OSS, Mail, and SMS services

Casbin Gateway uses Casdoor to upload files to cloud storage, send Emails and send SMSs. See Casdoor for more details.

## Contribute

For Casbin Gateway, if you have any questions, you can open Issues, or you can also directly start Pull Requests(but we recommend opening issues first to communicate with the community).

## License

[Apache-2.0](https://github.com/apache/casbin-gateway/blob/master/LICENSE)
