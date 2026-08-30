<h1 align="center" style="border-bottom: none;">📦⚡️ Casbin Gateway</h1>
<h3 align="center">An open-source gateway for the AI coding agents on your machine, developed by Go and React.</h3>
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

<p align="center">
  <b>English</b> | <a href="./README_zh.md">中文</a>
</p>

## Screenshots

| Every agent on this machine | One endpoint per model vendor |
| :---: | :---: |
| [![Agents](https://cdn.casbin.org/img/casbin-gateway-agents.png)](https://cdn.casbin.org/img/casbin-gateway-agents.png) | [![New Provider](https://cdn.casbin.org/img/casbin-gateway-new-provider.png)](https://cdn.casbin.org/img/casbin-gateway-new-provider.png) |
| What each one runs on, what it has spent there, and whether it is running | 27 vendor presets, or any OpenAI- or Anthropic-compatible base URL |

| Everything the agents relayed | The whole request, not just a count |
| :---: | :---: |
| [![LLM Records](https://cdn.casbin.org/img/casbin-gateway-llm-records.png)](https://cdn.casbin.org/img/casbin-gateway-llm-records.png) | [![One record](https://cdn.casbin.org/img/casbin-gateway-llm-record.png)](https://cdn.casbin.org/img/casbin-gateway-llm-record.png) |
| Requests, tokens, cache hit rate and cost, broken down by model | The system prompt, every message, and the schema of every tool the model was offered |

## Run it

One command. No database, no Go, no Node, no configuration.

On Linux and macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.sh | bash
```

On Windows, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.ps1 | iex
```

Either one downloads the build for this machine, unpacks it into `~/.local/share/casbin-gateway` (`%LOCALAPPDATA%\casbin-gateway` on Windows), puts a `casbin-gateway` command on your PATH, starts it, and arranges for it to start again when you log in. The terminal you installed from is yours again straight away.

Gateway then opens in its own window — no sign-in: it serves this machine only and signs the local admin in on sight. Closing that window leaves Gateway running behind its tray icon, which is also where you reopen the window and quit for real. There is a **Casbin Gateway** entry on your desktop and in the Start menu, in `~/Applications`, or in the application menu, depending on the platform.

If you would rather use a browser, or you are on a machine with no desktop at all, everything is still at **http://localhost:17000**, and `casbin-gateway start` runs the server on its own with no window and no tray.

That is the whole installation. Gateway keeps its data in a SQLite file inside its own directory.

The password behind that account is `admin` / `123`, and it only matters if you open Gateway to the network — see [Serving other machines](#serving-other-machines).

### Keeping it up to date

The version Gateway is running sits in the top-right corner, with the date it was built, and it says **New** when the published build is a later one. Open it and press **Update now**: Gateway downloads the build for this machine, checks that it runs, puts it in place of itself and restarts into it. The page reloads on the new version when it comes back, and nothing else has to be touched — the data, the settings and the `casbin-gateway` command all stay where they are.

Gateway installed some other way, or in a directory it cannot write to, says so and shows the install command to run by hand instead.

### What to do next

| Page | What you get | What it needs |
| --- | --- | --- |
| **Agents** | Every AI coding agent installed on this machine — Claude Code, Codex CLI, Cursor and more. Click **Patch** on one and its activity streams into the page live. | Nothing |
| **Skills, MCP & Prompts** | Every skill, MCP server and instruction file of every agent in one table. Add an MCP server to one agent or to several at once, edit the instructions an agent reads before every session, open one, delete it, or copy it into another agent. | Nothing |
| **Providers** | One endpoint in front of your model vendors. Gateway holds the API key, so the agents never have it — or forwards the agent's own sign-in and holds nothing. | A vendor API key, or nothing at all |
| **LLM Records** | Every request an agent relayed: the full system prompt, every message and tool call, the schema of every tool the model was offered, plus tokens and cost. | A provider, and `llmRecordMode` — see [Recording prompts](#recording-prompts) |

Agents are found by reading the user accounts, home directories and install paths of **the machine Gateway runs on**, so run it on the machine whose agents you want to watch.

### Send an agent's traffic through Gateway

This is what fills **LLM Records**, and what lets Gateway keep the vendor key instead of the agent.

1. **Providers** → **Add**: pick the type (OpenAI- or Anthropic-compatible), paste the vendor base URL and API key, and list the models it serves.
2. **Agents** → open an agent → pick that provider. For an agent whose configuration format Gateway knows, **Write configuration** puts it in the agent's own file — **Preview** shows exactly what that will be first, and **Restore** undoes it. Picking a different provider afterwards rewrites the file on the spot, so switching from then on is one click, from either page.
3. For any other agent, copy the environment snippet the page shows and start the agent from a shell that has it:

```bash
export ANTHROPIC_BASE_URL="http://localhost:17000/v1/agents/claude-code"
export ANTHROPIC_AUTH_TOKEN="cg-..."
```

The token is Gateway's own relay token, not a vendor key: the agent refuses to start without something in that variable, and Gateway authenticates upstream with the provider's key instead. The snippet on the page already has the real value filled in.

One base URL answers whichever API the agent speaks: `/chat/completions` for an OpenAI client, `/v1/messages` for an Anthropic one, and `/responses` for Codex, which speaks nothing else since it dropped the chat completions wire format. The API the provider serves need not be the same one: Gateway translates between all three, in both directions and for streamed answers too, so Codex runs on DeepSeek, Kimi or Qwen, and Claude Code runs on any of them just as well. A provider serving the very API the request arrived in is relayed byte for byte, untouched, since there is nothing to translate. The one thing Gateway answers itself is the token count an Anthropic client asks for before each turn, which it estimates when the bound provider has no endpoint to ask.

### No API key: keep the sign-in the agent already has

An agent signed in with a ChatGPT or Claude subscription has no API key to paste. Set the provider's **Authentication** to **the caller's own login** and it needs none: the base URL points at the vendor, and every request is forwarded with the credentials the agent itself sent, so it keeps its own sign-in. Leave **Models** empty and the provider accepts any model name.

The environment snippet for such a provider sets the base URL and nothing else — a token there would replace the sign-in the agent already has. Gateway records and routes the traffic exactly as it does for a provider with a key; it just never sees one.

Codex is the exception: its ChatGPT sign-in talks to an endpoint no provider stands in for, so a Codex CLI still needs a provider with an API key.

### Stopping, upgrading, removing

- **Stop**: `casbin-gateway stop`. **Start again**: `casbin-gateway start`. **Check**: `casbin-gateway status`. All three work from any directory — the command is a wrapper that always starts Gateway in its install directory, where its data lives.
- **Run in the foreground** instead, to watch it: `casbin-gateway`, stopped with `Ctrl-C`. In the background its console output goes to `logs/casbin-gateway.out`.
- **Upgrade**: press the version in the top-right corner and then **Update now**, or run the install command again. Your database and settings are untouched either way.
- **Remove**: delete `~/.local/share/casbin-gateway` and `~/.local/bin/casbin-gateway` (on Windows, `%LOCALAPPDATA%\casbin-gateway` and its PATH entry), plus the startup entry the installer names when it finishes.

Set `INSTALL_DIR` to install somewhere else, `NO_START=1` to install without starting, or `NO_AUTOSTART=1` to install without starting at login.

### Serving other machines

Gateway binds `127.0.0.1` by default, because two things are wide open to whoever can reach the port: the UI signs the local admin in without asking, and `/v1` relays with the API keys stored here. Both are exactly what you want from a local tool and neither should be offered to a network.

To serve other machines anyway, set `httpaddr = 0.0.0.0` in `conf/app.conf`, and then:

1. **Change the admin password** from **My Account**. The auto sign-in stops at the first request that is not from this machine, so from then on the password is the only thing in the way.
2. **Send the relay token** with every request to `/v1`. Gateway generates one on first start and shows it under **Settings → Security**; the environment snippets on the Providers and Agents pages already carry it, and it is what Gateway writes into the configuration of an agent it switches. Requests from this machine never need it.

**These are nightly builds**, rebuilt from `master` on every push and published as the [`nightly`](https://github.com/apache/casbin-gateway/releases/tag/nightly) pre-release. They exist so that Gateway can be tried without a Go and Node toolchain; anything else should be built from a source release.

### Running in Docker or Podman

**A container cannot see the agents on your machine.** Agents are discovered by reading the home directories and install paths of the machine Gateway runs on, and inside a container that is the container's own filesystem. **Agents**, **Skills, MCP & Prompts** and agent monitoring therefore stay empty there, and the pages say so rather than pretending nothing is installed. Everything that does not depend on the host works normally: **Providers** and **LLM Records**.

So run the one-command install above on the machine whose agents you want to watch, and use a container when Gateway is only a model endpoint for other machines.

No image is published, so the compose file builds one from a checkout of this repository:

```bash
docker compose up -d
```

Podman reads the same file:

```bash
podman compose up -d
```

Either way the UI is on http://localhost:17000, the SQLite database lives in a named volume that survives `down`, and `conf/app.conf` is mounted from the repository, so the settings it seeds can be edited before the first start without rebuilding the image.

## Configuration

Everything is optional. Settings are changed on the **Settings** page of the web UI and stored in the database, so nothing has to be edited by hand and nothing has to be restarted. `conf/app.conf`, next to the executable, seeds them on the very first start and explains each one; the one-step install has no file beside it and seeds from the copy baked into the binary instead. Editing the file after that first start does nothing, except for the keys read before the database is open: `httpport`, `driverName`, `dataSourceName`, `dbName` and `redisEndpoint`. The settings people actually change:

| Setting | Default | What it does |
| --- | --- | --- |
| `httpport` | `17000` | Port of the web UI and the REST API |
| `httpaddr` | `127.0.0.1` | Interface the web UI binds to — see [Serving other machines](#serving-other-machines) |
| `driverName` / `dataSourceName` | `sqlite` / `./data/casbin-gateway.db` | Where data is stored |
| `llmRecordMode` | `full` | How much of each relayed LLM request is kept — including the prompt, see [Recording prompts](#recording-prompts) |
| `apiKeyEncryptionKey` | empty | Encrypts provider API keys at rest (AES-256-GCM) |
| `casdoorEndpoint` | empty | Switches sign-in over to [Casdoor](https://casdoor.org) SSO |

Gateway prints what it is actually doing when it starts, so the result can be checked instead of the file:

```
+---------------------------------------------------------------------+
| Casbin Gateway                                                      |
+---------------------------------------------------------------------+
| Management UI | http://localhost:17000 (this machine only)          |
| Settings      | Settings page, seeded from conf/app.conf            |
| Web UI files  | web/build                                           |
| Database      | sqlite, file "./data/casbin-gateway.db" (connected) |
| Sign-in       | built-in user table, Casdoor is not configured      |
| Relay auth    | this machine only, no token needed                  |
+---------------------------------------------------------------------+
```

A previous Gateway still holding this port is stopped first, so a restart never waits on it. A port held by anything else stays with that program: Gateway names the process holding it and stops, rather than taking the port or starting half-configured.

### Recording prompts

**Every relayed request is recorded in full from the first one, prompt included.** That is what the LLM Records page is made of, and none of it leaves this machine — but a prompt carries whatever was pasted into it, so if that is not what you want, this is the setting to change before you route an agent through Gateway. The picker at the top of the LLM Records page switches between **Recording off**, **Record metadata** and **Record metadata and bodies**, and takes effect from the next request; the Settings page holds the same choice and the limits around it, seeded from:

```ini
; The default is "full": it stores the request body, which is what LLM Records
; needs to show prompts, messages and tool schemas. "metadata" records only who
; called which model with which outcome, and "off" keeps nothing at all.
llmRecordMode = "full"
llmRecordRetentionDays = 30
llmRecordMaxRecords = 10000
llmRecordMaxPayloadBytes = 1048576
```

Bodies are sanitized before they are stored: anything that looks like a credential is replaced, and the number of replacements is shown with the record. Request headers, which is where the inbound API key is, never reach a record at all. A body over `llmRecordMaxPayloadBytes` keeps its structure and loses only its longest strings, so a large conversation is still listed message by message.

The cost next to each record uses built-in list prices, which vendors change and resellers do not follow. Point `llmPricingFile` at a JSON file of your own rates to correct them.

### Connecting Casdoor

[Casdoor](https://casdoor.org) is optional and takes over member management. Create an organization and an application for Gateway in a Casdoor instance, then fill in the five fields of **Settings → Sign-in**. Sign-in redirects to Casdoor as soon as `casdoorEndpoint` is set, which also enables [OAuth logins](https://casdoor.org/docs/provider/oauth/overview).

## Development

### Prerequisites

Go 1.20+, and Node.js with Yarn.

### Run from source

The backend serves the compiled frontend out of `web/build`, so build it once first:

```bash
cd web && yarn install && yarn build
```

```bash
go run main.go
```

Then open http://localhost:17000, where you are signed in as the local admin, same as an installed Gateway. The SQLite database is created on first start; there is no database server to install.

### Frontend development

```bash
cd web && yarn dev
```

That serves the UI on http://localhost:16002 with hot reload and proxies API calls to the backend on port 17000, so both have to be running.

### Using MySQL instead of SQLite

XORM is used, so every database it supports works. Point Gateway at your server and it creates `dbName` on first start if it does not exist:

```ini
driverName = mysql
dataSourceName = root:123@tcp(localhost:3306)/
dbName = casbin_gateway
```

### Building a single binary

Gateway normally reads two things from disk: `conf/app.conf` and the compiled UI in `web/build`. The `embed` build tag bakes both into the executable, which is what the install scripts ship:

```bash
cd web && yarn install && yarn build
```

```bash
go build -tags embed -o casbin-gateway .
```

Build the frontend first — everything under `web/build` goes into the binary, so `go build -tags embed` fails to compile while that directory is missing.

Files on disk always win over the embedded copies, so a single binary can still be configured and developed against without rebuilding it:

| Embedded asset | Overridden by |
| --- | --- |
| `conf/app.conf` | `conf/app.conf` in the working directory, or next to the executable |
| `web/build` | `web/build/index.html` in the working directory, which then serves the whole UI |

The startup summary reports which source each one came from.

### Where the data goes

Being self-contained is about startup, not about staying read-only. A running Gateway writes `./data` (the SQLite database and agent patch state), `./logs` and `./tmp` relative to its working directory — which is why the installed `casbin-gateway` command is a wrapper that always starts it in its install directory. Running the executable directly from somewhere else gives you a second, empty installation there.

## Architecture

Casbin Gateway contains 2 parts:

| Name     | Description                            | Language               | Source code                                              |
|----------|----------------------------------------|------------------------|----------------------------------------------------------|
| Frontend | Web frontend UI for Casbin Gateway     | TypeScript + React + shadcn/ui | https://github.com/apache/casbin-gateway/tree/master/web |
| Backend  | RESTful API backend for Casbin Gateway | Golang + Beego + XORM  | https://github.com/apache/casbin-gateway                 |

## Online demo

https://ai.casbin.com

## Documentation

https://caswaf.org

## Contribute

If you have any questions, open an issue, or start a pull request directly — though we recommend opening an issue first to talk it through with the community.

## License

[Apache-2.0](https://github.com/apache/casbin-gateway/blob/master/LICENSE)
