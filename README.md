<h1 align="center" style="border-bottom: none;">📦⚡️ casbin-gateway</h1>
<h3 align="center">An open-source Web Application Firewall (WAF) software developed by Go and React.</h3>
<p align="center">
  <a href="#badge">
    <img alt="semantic-release" src="https://img.shields.io/badge/%20%20%F0%9F%93%A6%F0%9F%9A%80-semantic--release-e10079.svg">
  </a>
  <a href="https://hub.docker.com/r/casbin/caswaf">
    <img alt="docker pull casbin/caswaf" src="https://img.shields.io/docker/pulls/casbin/caswaf.svg">
  </a>
  <a href="https://github.com/casbin/caswaf/releases/latest">
    <img alt="GitHub Release" src="https://img.shields.io/github/v/release/casbin/caswaf.svg">
  </a>
  <a href="https://hub.docker.com/r/casbin/caswaf">
    <img alt="Docker Image Version (latest semver)" src="https://img.shields.io/badge/Docker%20Hub-latest-brightgreen">
  </a>
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/casbin/caswaf">
    <img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/casbin/caswaf?style=flat-square">
  </a>
  <a href="https://github.com/casbin/caswaf/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/casbin/caswaf?style=flat-square" alt="license">
  </a>
  <a href="https://github.com/casbin/caswaf/issues">
    <img alt="GitHub issues" src="https://img.shields.io/github/issues/casbin/caswaf?style=flat-square">
  </a>
  <a href="#">
    <img alt="GitHub stars" src="https://img.shields.io/github/stars/casbin/caswaf?style=flat-square">
  </a>
  <a href="https://github.com/casbin/caswaf/network">
    <img alt="GitHub forks" src="https://img.shields.io/github/forks/casbin/caswaf?style=flat-square">
  </a>
</p>

## Online demo

- Read-only site: https://door.caswaf.com (any modification operation will fail)
- Writable site: https://demo.caswaf.com (original data will be restored for every 5 minutes)

## Documentation

https://caswaf.org

## Architecture

casbin-gateway contains 2 parts:

| Name     | Description                    | Language               | Source code                                      |
|----------|--------------------------------|------------------------|--------------------------------------------------|
| Frontend | Web frontend UI for casbin-gateway     | Javascript + React     | https://github.com/casbin/caswaf/tree/master/web |
| Backend  | RESTful API backend for casbin-gateway | Golang + Beego + MySQL | https://github.com/casbin/caswaf                 |

## Installation

casbin-gateway uses Casdoor to manage members. So you need to create an organization and an application for casbin-gateway in a Casdoor instance.

### Deployment Options

- **[Kubernetes Deployment](k8s/README.md)**: Deploy casbin-gateway on Kubernetes with complete manifests and guide
- **Docker Compose**: Use the provided `docker-compose.yml` for quick local setup
- **Manual Installation**: Build and run from source

### Necessary configuration

#### Get the code

```shell
go get github.com/casdoor/casdoor
go get github.com/casbin/caswaf
```

or

```shell
git clone https://github.com/casdoor/casdoor
git clone https://github.com/casbin/caswaf
```

#### Setup database

casbin-gateway will store its users, nodes and topics information in a MySQL database named: `caswaf`, will create it if not existed. The DB connection string can be specified at: https://github.com/casbin/caswaf/blob/master/conf/app.conf

```ini
dataSourceName = root:123@tcp(localhost:3306)/
```

casbin-gateway uses XORM to connect to DB, so all DBs supported by XORM can also be used.

#### Configure Casdoor

After creating an organization and an application for casbin-gateway in a Casdoor, you need to update `clientID`, `clientSecret`, `casdoorOrganization` and `casdoorApplication` in app.conf.

#### Run casbin-gateway

- Configure and run casbin-gateway by yourself. If you want to learn more about caswaf.
- Open browser: http://localhost:16001/

### Optional configuration

#### Setup your WAF to enable some third-party login platform

casbin-gateway uses Casdoor to manage members. If you want to log in with oauth, you should see [casdoor oauth configuration](https://casdoor.org/docs/provider/oauth/overview).

#### OSS, Mail, and SMS services

casbin-gateway uses Casdoor to upload files to cloud storage, send Emails and send SMSs. See Casdoor for more details.

## Contribute

For casbin-gateway, if you have any questions, you can open Issues, or you can also directly start Pull Requests(but we recommend opening issues first to communicate with the community).

## License

[Apache-2.0](https://github.com/caswaf/caswaf/blob/master/LICENSE)
