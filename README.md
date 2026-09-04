<h1 align="center" style="border-bottom: none;">📦⚡️ Casbin Gateway</h1>
<h3 align="center">An open-source gateway for the AI coding agents on your machine, developed by Go and React.</h3>
<p align="center">
  <a href="https://github.com/apache/casbin-gateway/actions/workflows/golangci-lint.yml"><img alt="Lint" src="https://img.shields.io/github/actions/workflow/status/apache/casbin-gateway/golangci-lint.yml?branch=master&style=flat-square&logo=github&logoColor=white&label=lint"></a>
  <a href="https://github.com/apache/casbin-gateway/actions/workflows/build.yml"><img alt="Build" src="https://img.shields.io/github/actions/workflow/status/apache/casbin-gateway/build.yml?branch=master&style=flat-square&logo=github&logoColor=white&label=build"></a>
  <a href="https://pkg.go.dev/github.com/apache/casbin-gateway"><img alt="Go Reference" src="https://img.shields.io/badge/go.dev-reference-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
  <a href="https://github.com/apache/casbin-gateway/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/apache/casbin-gateway?style=flat-square&logo=github&logoColor=white&color=3b82f6"></a>
  <a href="https://github.com/apache/casbin-gateway/blob/master/LICENSE"><img alt="License" src="https://img.shields.io/github/license/apache/casbin-gateway?style=flat-square&color=3b82f6"></a>
  <a href="https://github.com/apache/casbin-gateway/issues"><img alt="Issues" src="https://img.shields.io/github/issues/apache/casbin-gateway?style=flat-square&logo=github&logoColor=white&color=3b82f6"></a>
  <a href="https://github.com/apache/casbin-gateway/stargazers"><img alt="Stars" src="https://img.shields.io/github/stars/apache/casbin-gateway?style=flat-square&logo=github&logoColor=white&color=3b82f6"></a>
  <a href="https://github.com/apache/casbin-gateway/network/members"><img alt="Forks" src="https://img.shields.io/github/forks/apache/casbin-gateway?style=flat-square&logo=github&logoColor=white&color=3b82f6"></a>
  <a href="https://discord.gg/S5UjpzGZjN"><img alt="Discord" src="https://img.shields.io/discord/1022748306096537660?style=flat-square&logo=discord&logoColor=white&label=discord&color=5865F2"></a>
</p>

<p align="center">
  <b>English</b> | <a href="./README_zh.md">中文</a>
</p>

<p align="center"><b>Every coding agent on the machine</b></p>
<p align="center">
  <a href="https://claude.com" title="Claude Code"><img src="https://www.google.com/s2/favicons?domain=claude.com&sz=64" width="30" height="30" alt="Claude Code"></a>
  <a href="https://claude.ai" title="Claude Desktop"><img src="https://www.google.com/s2/favicons?domain=claude.ai&sz=64" width="30" height="30" alt="Claude Desktop"></a>
  <a href="https://openai.com" title="Codex / Codex CLI"><img src="https://www.google.com/s2/favicons?domain=openai.com&sz=64" width="30" height="30" alt="Codex"></a>
  <a href="https://gemini.google.com" title="Gemini CLI"><img src="https://www.google.com/s2/favicons?domain=gemini.google.com&sz=64" width="30" height="30" alt="Gemini CLI"></a>
  <a href="https://cursor.com" title="Cursor / Cursor Agent"><img src="https://www.google.com/s2/favicons?domain=cursor.com&sz=64" width="30" height="30" alt="Cursor"></a>
  <a href="https://windsurf.com" title="Windsurf"><img src="https://www.google.com/s2/favicons?domain=windsurf.com&sz=64" width="30" height="30" alt="Windsurf"></a>
  <a href="https://opencode.ai" title="opencode / opencode Desktop"><img src="https://www.google.com/s2/favicons?domain=opencode.ai&sz=64" width="30" height="30" alt="opencode"></a>
  <a href="https://openagentai.org" title="OpenAgent"><img src="https://www.google.com/s2/favicons?domain=openagentai.org&sz=64" width="30" height="30" alt="OpenAgent"></a>
  <a href="https://openclaw.ai" title="OpenClaw"><img src="https://www.google.com/s2/favicons?domain=openclaw.ai&sz=64" width="30" height="30" alt="OpenClaw"></a>
  <a href="https://nousresearch.com" title="Hermes Agent"><img src="https://www.google.com/s2/favicons?domain=nousresearch.com&sz=64" width="30" height="30" alt="Hermes Agent"></a>
  <a href="https://deepseek.com" title="DeepSeek Harness"><img src="https://www.google.com/s2/favicons?domain=deepseek.com&sz=64" width="30" height="30" alt="DeepSeek Harness"></a>
</p>

<p align="center"><b>Any model vendor behind one endpoint</b></p>
<p align="center">
  <a href="https://openai.com" title="OpenAI"><img src="https://www.google.com/s2/favicons?domain=openai.com&sz=64" width="28" height="28" alt="OpenAI"></a>
  <a href="https://anthropic.com" title="Anthropic"><img src="https://www.google.com/s2/favicons?domain=anthropic.com&sz=64" width="28" height="28" alt="Anthropic"></a>
  <a href="https://gemini.google.com" title="Google Gemini"><img src="https://www.google.com/s2/favicons?domain=gemini.google.com&sz=64" width="28" height="28" alt="Google Gemini"></a>
  <a href="https://x.ai" title="xAI"><img src="https://www.google.com/s2/favicons?domain=x.ai&sz=64" width="28" height="28" alt="xAI"></a>
  <a href="https://mistral.ai" title="Mistral"><img src="https://www.google.com/s2/favicons?domain=mistral.ai&sz=64" width="28" height="28" alt="Mistral"></a>
  <a href="https://cohere.com" title="Cohere"><img src="https://www.google.com/s2/favicons?domain=cohere.com&sz=64" width="28" height="28" alt="Cohere"></a>
  <a href="https://perplexity.ai" title="Perplexity"><img src="https://www.google.com/s2/favicons?domain=perplexity.ai&sz=64" width="28" height="28" alt="Perplexity"></a>
  <a href="https://deepseek.com" title="DeepSeek"><img src="https://www.google.com/s2/favicons?domain=deepseek.com&sz=64" width="28" height="28" alt="DeepSeek"></a>
  <a href="https://moonshot.cn" title="Moonshot"><img src="https://www.google.com/s2/favicons?domain=moonshot.cn&sz=64" width="28" height="28" alt="Moonshot"></a>
  <a href="https://z.ai" title="Zhipu GLM"><img src="https://www.google.com/s2/favicons?domain=z.ai&sz=64" width="28" height="28" alt="Zhipu GLM"></a>
  <a href="https://dashscope.aliyuncs.com" title="Qwen"><img src="https://www.google.com/s2/favicons?domain=aliyun.com&sz=64" width="28" height="28" alt="Qwen"></a>
  <a href="https://minimaxi.com" title="MiniMax"><img src="https://www.google.com/s2/favicons?domain=minimaxi.com&sz=64" width="28" height="28" alt="MiniMax"></a>
  <a href="https://baichuan-ai.com" title="Baichuan"><img src="https://www.google.com/s2/favicons?domain=baichuan-ai.com&sz=64" width="28" height="28" alt="Baichuan"></a>
  <a href="https://stepfun.com" title="StepFun"><img src="https://www.google.com/s2/favicons?domain=stepfun.com&sz=64" width="28" height="28" alt="StepFun"></a>
  <a href="https://volcengine.com" title="Volcengine Ark"><img src="https://www.google.com/s2/favicons?domain=volcengine.com&sz=64" width="28" height="28" alt="Volcengine Ark"></a>
  <a href="https://cloud.tencent.com" title="Tencent Hunyuan"><img src="https://www.google.com/s2/favicons?domain=cloud.tencent.com&sz=64" width="28" height="28" alt="Tencent Hunyuan"></a>
  <a href="https://baidu.com" title="Baidu ERNIE"><img src="https://www.google.com/s2/favicons?domain=baidu.com&sz=64" width="28" height="28" alt="Baidu ERNIE"></a>
  <a href="https://lingyiwanwu.com" title="01.AI"><img src="https://www.google.com/s2/favicons?domain=lingyiwanwu.com&sz=64" width="28" height="28" alt="01.AI"></a>
  <a href="https://ai21.com" title="AI21 Labs"><img src="https://www.google.com/s2/favicons?domain=ai21.com&sz=64" width="28" height="28" alt="AI21 Labs"></a>
  <a href="https://reka.ai" title="Reka"><img src="https://www.google.com/s2/favicons?domain=reka.ai&sz=64" width="28" height="28" alt="Reka"></a>
  <a href="https://siliconflow.cn" title="SiliconFlow"><img src="https://www.google.com/s2/favicons?domain=siliconflow.cn&sz=64" width="28" height="28" alt="SiliconFlow"></a>
  <a href="https://groq.com" title="Groq"><img src="https://www.google.com/s2/favicons?domain=groq.com&sz=64" width="28" height="28" alt="Groq"></a>
  <a href="https://together.ai" title="Together AI"><img src="https://www.google.com/s2/favicons?domain=together.ai&sz=64" width="28" height="28" alt="Together AI"></a>
  <a href="https://fireworks.ai" title="Fireworks AI"><img src="https://www.google.com/s2/favicons?domain=fireworks.ai&sz=64" width="28" height="28" alt="Fireworks AI"></a>
  <a href="https://novita.ai" title="Novita AI"><img src="https://www.google.com/s2/favicons?domain=novita.ai&sz=64" width="28" height="28" alt="Novita AI"></a>
  <a href="https://deepinfra.com" title="DeepInfra"><img src="https://www.google.com/s2/favicons?domain=deepinfra.com&sz=64" width="28" height="28" alt="DeepInfra"></a>
  <a href="https://modelscope.cn" title="ModelScope"><img src="https://www.google.com/s2/favicons?domain=modelscope.cn&sz=64" width="28" height="28" alt="ModelScope"></a>
  <a href="https://ppinfra.com" title="PPIO"><img src="https://www.google.com/s2/favicons?domain=ppinfra.com&sz=64" width="28" height="28" alt="PPIO"></a>
  <a href="https://cerebras.ai" title="Cerebras"><img src="https://www.google.com/s2/favicons?domain=cerebras.ai&sz=64" width="28" height="28" alt="Cerebras"></a>
  <a href="https://sambanova.ai" title="SambaNova"><img src="https://www.google.com/s2/favicons?domain=sambanova.ai&sz=64" width="28" height="28" alt="SambaNova"></a>
  <a href="https://hyperbolic.xyz" title="Hyperbolic"><img src="https://www.google.com/s2/favicons?domain=hyperbolic.xyz&sz=64" width="28" height="28" alt="Hyperbolic"></a>
  <a href="https://nebius.ai" title="Nebius AI Studio"><img src="https://www.google.com/s2/favicons?domain=nebius.ai&sz=64" width="28" height="28" alt="Nebius AI Studio"></a>
  <a href="https://lambda.ai" title="Lambda"><img src="https://www.google.com/s2/favicons?domain=lambda.ai&sz=64" width="28" height="28" alt="Lambda"></a>
  <a href="https://baseten.co" title="Baseten"><img src="https://www.google.com/s2/favicons?domain=baseten.co&sz=64" width="28" height="28" alt="Baseten"></a>
  <a href="https://build.nvidia.com" title="NVIDIA NIM"><img src="https://www.google.com/s2/favicons?domain=nvidia.com&sz=64" width="28" height="28" alt="NVIDIA NIM"></a>
  <a href="https://ollama.com" title="Ollama"><img src="https://www.google.com/s2/favicons?domain=ollama.com&sz=64" width="28" height="28" alt="Ollama"></a>
  <a href="https://lmstudio.ai" title="LM Studio"><img src="https://www.google.com/s2/favicons?domain=lmstudio.ai&sz=64" width="28" height="28" alt="LM Studio"></a>
  <a href="https://docs.vllm.ai" title="vLLM"><img src="https://www.google.com/s2/favicons?domain=vllm.ai&sz=64" width="28" height="28" alt="vLLM"></a>
  <a href="https://github.com/ggml-org/llama.cpp" title="llama.cpp"><img src="https://www.google.com/s2/favicons?domain=github.com&sz=64" width="28" height="28" alt="llama.cpp"></a>
  <a href="https://openrouter.ai" title="OpenRouter"><img src="https://www.google.com/s2/favicons?domain=openrouter.ai&sz=64" width="28" height="28" alt="OpenRouter"></a>
  <a href="https://aihubmix.com" title="AiHubMix"><img src="https://www.google.com/s2/favicons?domain=aihubmix.com&sz=64" width="28" height="28" alt="AiHubMix"></a>
  <a href="https://302.ai" title="302.AI"><img src="https://www.google.com/s2/favicons?domain=302.ai&sz=64" width="28" height="28" alt="302.AI"></a>
  <a href="https://vercel.com/docs/ai-gateway" title="Vercel AI Gateway"><img src="https://www.google.com/s2/favicons?domain=vercel.com&sz=64" width="28" height="28" alt="Vercel AI Gateway"></a>
  <a href="https://litellm.ai" title="LiteLLM"><img src="https://www.google.com/s2/favicons?domain=litellm.ai&sz=64" width="28" height="28" alt="LiteLLM"></a>
</p>

<p align="center">
  <a href="https://cdn.casbin.org/img/casbin-gateway.gif"><img alt="Casbin Gateway" src="https://cdn.casbin.org/img/casbin-gateway.gif" width="900"></a>
</p>

## What Gateway does for each agent

| Agent | Monitoring | Provider | MCP | Skills | Prompt | Sessions | Install |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
| <img src="https://www.google.com/s2/favicons?domain=claude.com&sz=64" width="16" height="16" alt=""> **Claude Code** | ✅ | ✅ Anthropic | ✅ | ✅ | ✅ | ✅ | npm · brew · winget |
| <img src="https://www.google.com/s2/favicons?domain=claude.ai&sz=64" width="16" height="16" alt=""> **Claude Desktop** | ✅ | — | ✅ | — | — | ✅ | — |
| <img src="https://www.google.com/s2/favicons?domain=openai.com&sz=64" width="16" height="16" alt=""> **Codex CLI** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | npm · brew |
| <img src="https://www.google.com/s2/favicons?domain=openai.com&sz=64" width="16" height="16" alt=""> **ChatGPT Desktop (Codex)** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | — |
| <img src="https://www.google.com/s2/favicons?domain=gemini.google.com&sz=64" width="16" height="16" alt=""> **Gemini CLI** | ✅ | ✅ Gemini | ✅ | ✅ | ✅ | ✅ | npm |
| <img src="https://www.google.com/s2/favicons?domain=cursor.com&sz=64" width="16" height="16" alt=""> **Cursor** | ✅ | — | ✅ | ✅ | — | — | brew |
| <img src="https://www.google.com/s2/favicons?domain=cursor.com&sz=64" width="16" height="16" alt=""> **Cursor Agent** | ✅ | — | ✅ | ✅ | — | — | — |
| <img src="https://www.google.com/s2/favicons?domain=windsurf.com&sz=64" width="16" height="16" alt=""> **Windsurf** | ✅ | — | ✅ | — | ✅ | — | brew |
| <img src="https://www.google.com/s2/favicons?domain=opencode.ai&sz=64" width="16" height="16" alt=""> **opencode** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | npm |
| <img src="https://www.google.com/s2/favicons?domain=opencode.ai&sz=64" width="16" height="16" alt=""> **opencode Desktop** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | — |
| <img src="https://www.google.com/s2/favicons?domain=openagentai.org&sz=64" width="16" height="16" alt=""> **OpenAgent** | ✅ | — | — | — | — | — | — |
| <img src="https://www.google.com/s2/favicons?domain=openclaw.ai&sz=64" width="16" height="16" alt=""> **OpenClaw** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | npm |
| <img src="https://www.google.com/s2/favicons?domain=nousresearch.com&sz=64" width="16" height="16" alt=""> **Hermes Agent** | ✅ | ✅ OpenAI | — | — | — | — | — |
| <img src="https://www.google.com/s2/favicons?domain=deepseek.com&sz=64" width="16" height="16" alt=""> **DeepSeek Harness** | ✅ | ✅ OpenAI | ✅ | ✅ | — | ✅ | npm |

- **Monitoring** — audit-only records of what the agent did: prompts, tool calls, permission prompts. Nothing an agent does waits on Gateway, and no answer changes because monitoring is on.
- **Provider** — Gateway writes the agent's own configuration to point it at a bound provider, in the wire format that agent's client speaks. An agent without it still reaches Gateway through the environment variables the UI shows.
- **MCP · Skills · Prompt** — read, compare and copy MCP servers, skills and the instruction file between agents.
- **Sessions** — prompts and token usage read straight from the agent's own transcripts, including what never went through Gateway.
- **Install** — the package managers Gateway installs and upgrades it with. Everything else is a download from its vendor's page.

## Features

- **[Is the API behind that key what it was sold as?](#the-killer-feature-is-the-api-behind-that-key-what-it-was-sold-as)** — a reseller can quietly swap in a cheaper model or fake a cache hit, and none of it shows up in the traffic. Authenticity asks the upstream directly and grades it A–F.
- **[Switch every agent's provider from one place](#send-an-agents-traffic-through-gateway)** — change an API key or base URL once, and every agent pointed at Gateway picks it up.
- **[Run several instances of one agent side by side](#what-to-do-next)** — e.g. multiple Claude Desktop instances, each signed in to a different account.
- **[See the whole request, not just a count](#recording-prompts)** — every prompt, message and tool schema an agent sent, kept on this machine.
- **[Say what each agent may do](#what-each-agent-is-allowed-to-do)** — around forty switches per agent, in groups, over its tools, models and providers, enforced by Casbin on every request it relays.
- **[Know what every agent spent, even off Gateway](#what-the-agents-spend-including-what-never-went-through-gateway)** — read straight from the agents' own transcripts.
- **[Compare and copy skills, MCP servers and prompts across agents](#what-to-do-next)** — every agent's install list in one table.

## Screenshots

| Every agent on this machine | Everything those agents carry |
| :---: | :---: |
| [![Agents](https://cdn.casbin.org/img/casbin-gateway-home.png)](https://cdn.casbin.org/img/casbin-gateway-home.png) | [![Skills, MCP & Prompts](https://cdn.casbin.org/img/casbin-gateway-skills.png)](https://cdn.casbin.org/img/casbin-gateway-skills.png) |
| What each one runs on, which account it is signed in to, what it has spent there, and whether it is running | Every skill, MCP server and instruction file of every agent, side by side, copied from one agent to another |

| What the agents spent | One endpoint per model vendor |
| :---: | :---: |
| [![Usage](https://cdn.casbin.org/img/casbin-gateway-usage.png)](https://cdn.casbin.org/img/casbin-gateway-usage.png) | [![New Provider](https://cdn.casbin.org/img/casbin-gateway-new-provider.png)](https://cdn.casbin.org/img/casbin-gateway-new-provider.png) |
| Read from the transcripts the agents write themselves, so it counts what never went through Gateway too | 44 vendor presets, or any OpenAI- or Anthropic-compatible base URL |

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

Gateway then opens in its own window — no sign-in: it serves this machine only and signs the local admin in on sight. Closing that window leaves Gateway running behind its tray icon, which is also where you reopen the window and quit for real. There is a **Casbin Gateway** entry on your desktop and in the Start menu, in `~/Applications`, or in the application menu, depending on the platform. An archive unpacked by hand gets the same entry the first time the launcher runs.

If you would rather use a browser, or you are on a machine with no desktop at all, everything is still at **http://localhost:17000**, and `casbin-gateway start` runs the server on its own with no window and no tray.

That is the whole installation. Gateway keeps its data in a SQLite file inside its own directory.

The password behind that account is `admin` / `123`, and it only matters if you open Gateway to the network — see [Serving other machines](#serving-other-machines).

### Keeping it up to date

The version Gateway is running sits in the top-right corner, with the date it was built, and it says **New** when the published build is a later one. Open it and press **Update now**: Gateway downloads the build for this machine, checks that it runs, puts it in place of itself and restarts into it. The page reloads on the new version when it comes back, and nothing else has to be touched — the data, the settings and the `casbin-gateway` command all stay where they are.

Gateway installed some other way, or in a directory it cannot write to, says so and shows the install command to run by hand instead.

### What to do next

| Page | What you get | What it needs |
| --- | --- | --- |
| **Agents** | Every AI coding agent installed on this machine — Claude Code, Codex CLI, Cursor, the Gemini CLI, opencode and more — four cards to a row, each naming the account it is signed in to, the provider it answers to, what it has spent, and whether it is running right now. Start or stop one from its card, or run several **instances** of the same agent at once, each with a state directory and an account of its own. An agent this machine does not have is listed too, and installed or upgraded from the page through the package manager the host already has. | Nothing |
| **Skills, MCP & Prompts** | Every skill, MCP server and instruction file of every agent in one table. Install skills from a GitHub repository, a `.zip` or `.tar.gz`, or a folder on this machine, into one agent or several at once. Add an MCP server the same way, edit the instructions an agent reads before every session, open one, delete it, or copy it into another agent. | Nothing |
| **Sessions** | Every session those agents have had, read from the transcripts they leave on disk: the whole conversation, message by message. | Nothing |
| **Activity** | What a monitored agent is doing as it does it — each tool call, its target and how long it took. | Monitoring on for an agent |
| **Providers** | One endpoint in front of your model vendors. Gateway holds the API key, so the agents never have it — or forwards the agent's own sign-in and holds nothing. | A vendor API key, or nothing at all |
| **Authenticity** | A score out of 100 and a grade for every provider, measured without being asked — see [the section below](#the-killer-feature-is-the-api-behind-that-key-what-it-was-sold-as). | A provider with an API key |
| **LLM Records** | Every request an agent relayed: the full system prompt, every message and tool call, the schema of every tool the model was offered, plus tokens and cost. | A provider, and `llmRecordMode` — see [Recording prompts](#recording-prompts) |
| **Usage** | What every agent on this machine spent, over time and broken down by model and by agent, read from the agents' own transcripts — so it counts the requests that never went through Gateway. A second tab shows what Gateway relayed, which is the only account that knows which provider answered and whether it failed. | Nothing |
| **Model pricing** | What a million tokens costs, which is what every figure on the Usage page is worked out from. Edit a price by hand, or let Gateway reprice the models this machine has run from the [models.dev](https://models.dev) catalogue on a schedule; a price you edited yourself is left alone. | Nothing |

Agents are found by reading the user accounts, home directories and install paths of **the machine Gateway runs on**, so run it on the machine whose agents you want to watch.

### The killer feature: is the API behind that key what it was sold as?

A reseller can sell a frontier model and serve a cheaper one, count a cached prefix as fresh input, or answer in an API it only pretends to speak. None of that shows up in the traffic, so **Authenticity** asks the upstream directly. Every provider is probed on its own — when it is added, when its endpoint, type or key changes, when it has never been probed, and again whenever its report goes stale — and comes back with a score out of 100 and a grade from A to F, on the Authenticity page and above the agents on the home page. No button to press, and nothing to configure.

[![Authenticity](https://cdn.casbin.org/img/casbin-gateway-authenticity.png)](https://cdn.casbin.org/img/casbin-gateway-authenticity.png)

The score is only a summary of the test cases behind it, and every one of them is on the page. Half of them ask what the upstream is: whether the model that answers is the one that was asked for — worth half credit off a vendor's own endpoint, where that field is whatever the upstream typed there — which vendor the model says trained it, whether anything was injected in front of the request, whether several identical requests come back from the same model at all, and whether a parameter the API documents (`logprobs`, `n`, a stop sequence) is honoured, refused, or accepted with a 200 and quietly dropped. A test bank asks questions with one right answer, from counting the letters in a word to who wrote the Preface to the Pavilion of Prince Teng. The rest read the envelope: whether a two-level tool schema survives a forced call, whether the event stream carries everything the API documents, whether the prompt cache is really accounted for, whether two identical requests are billed the same, whether the vendor's own headers are there. Each case names the question it puts to the upstream, the exact request it sends, how the answer is judged, and what it is worth.

[![Test cases](https://cdn.casbin.org/img/casbin-gateway-authenticity-cases.png)](https://cdn.casbin.org/img/casbin-gateway-authenticity-cases.png)

Reweight a case, turn it off, rewrite its question, or add one of your own — the questions worth asking of a reseller are not the same everywhere, and a score whose method is not published is not evidence. **Restore defaults** puts the shipped suite back and leaves your own cases alone.

The report has a second half that costs nothing and sends no request: what the records Gateway already kept say
about that upstream — how much of the cache it really accounted for, how many attempts failed, how long it took
to answer, and how much of what it served has no price.

A probe spends a few cents of that provider's own credit, which is on the report next to the finding. `providerProbeIntervalHours` sets how often a report goes stale, `providerProbeMode = "manual"` probes only when asked, and `"off"` never probes.

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

One base URL answers whichever API the agent speaks: `/chat/completions` for an OpenAI client, `/v1/messages` for an Anthropic one, `/responses` for Codex, which speaks nothing else since it dropped the chat completions wire format, and `/v1beta/models/<model>:generateContent` for the Gemini CLI, which speaks only Google's own API. The API the provider serves need not be the same one: Gateway translates between all four, in both directions and for streamed answers too, so Codex runs on DeepSeek, Kimi or Qwen, and Claude Code runs on any of them just as well. A provider serving the very API the request arrived in is relayed byte for byte, untouched, since there is nothing to translate. The one thing Gateway answers itself is the token count an Anthropic or Gemini client asks for before each turn, which it estimates when the bound provider has no endpoint to ask.

### No API key: keep the sign-in the agent already has

An agent signed in with a ChatGPT or Claude subscription has no API key to paste. Set the provider's **Authentication** to **the caller's own login** and it needs none: the base URL points at the vendor, and every request is forwarded with the credentials the agent itself sent, so it keeps its own sign-in. Leave **Models** empty and the provider accepts any model name.

The environment snippet for such a provider sets the base URL and nothing else — a token there would replace the sign-in the agent already has. Gateway records and routes the traffic exactly as it does for a provider with a key; it just never sees one.

Codex is the exception: its ChatGPT sign-in talks to an endpoint no provider stands in for, so a Codex CLI still needs a provider with an API key.

### What each agent is allowed to do

**Permissions** in the sidebar is the page for it: every agent on this machine down one side, what the one you picked may do beside it. The same card is on the agent's own page. Everything that agent relays through Gateway is held to what is set there, so it can be given less than it came with without editing its own configuration.

- **Tools** — around forty switches, in six groups: the terminal, reading the project, changing the project, the internet, planning and delegation, and one switch per MCP server that agent has installed. The switch on a group's header sets the whole group at once; open the group and the answer gets as fine as you like, since running a command, reading a running command's output and stopping one are three switches. A tool whose switch is off is taken out of the request before it leaves this machine, so the model is never offered it and the agent never gets to call it. Every agent names its tools differently, and `Bash`, `shell` and `run_shell_command` are all the same switch. Each group ends in a catch-all for the tools Gateway has never seen, which is what closes a group for good rather than for the tools that happened to be listed the day it was set.
- **Models** — any model, only the ones you pick, or all but them. A name may end in `*`, so `claude-opus-*` covers a whole family.
- **Providers** — which of the providers this agent's requests may be sent to.

A request that asks for something switched off comes back as a `permission_error` in the API the agent speaks, so it reads as a refusal rather than as a broken gateway.

Underneath, the switches compile to a [Casbin](https://casbin.org) policy, and every relayed request is decided by an enforcer rather than by a hand-written check. **Advanced** shows the `model.conf` and `policy.csv` they compile to, and takes extra policy lines of your own:

```
p, claude-code, model:claude-opus-*, use, deny
p, claude-code, model:*, use, allow
p, claude-code, tool:shell/run, use, deny
p, claude-code, tool:mcp/github, use, allow
p, claude-code, tool:mcp/*, use, deny
p, claude-code, tool:*, use, allow
```

The first rule that matches decides, which is what lets one exception stand in front of the rule behind it: every MCP server taken away except the one that stays. The lines you write yourself are checked before the ones the switches wrote.

The rules apply to what goes through the proxy, so an agent bound directly to a provider — its own configuration pointing at the vendor rather than at Gateway — is not held to them. The page says so where that is the case.

### What the agents spend, including what never went through Gateway

An agent on its own subscription relays nothing through Gateway, and a request that goes straight to the vendor
leaves no record here — but the agent writes a transcript of it on disk anyway. **Usage** reads those
transcripts, so the spend of every agent on this machine is on one page from the first start, with no provider
configured and nothing routed: tokens, cache hit rate and cost, over time and broken down by model and by
agent. The second tab, **What Gateway relayed**, is the narrower account of the traffic that did come through,
and the only one that knows which provider answered and whether it failed.

Every figure there is worked out from **Model pricing**, which is a table of what a million tokens costs.
Vendors change their prices and resellers do not follow, so a price can be edited by hand, and Gateway can
reprice the models this machine has run from the [models.dev](https://models.dev) catalogue on a schedule
(`modelsDevSyncMode`, `modelsDevSyncIntervalHours`), leaving anything you edited yourself alone.

[![Model pricing](https://cdn.casbin.org/img/casbin-gateway-pricing.png)](https://cdn.casbin.org/img/casbin-gateway-pricing.png)

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

**A container cannot see the agents on your machine.** Agents are discovered by reading the home directories and install paths of the machine Gateway runs on, and inside a container that is the container's own filesystem. **Agents**, **Skills, MCP & Prompts** and agent monitoring therefore stay empty there, and the pages say so rather than pretending nothing is installed. Everything that does not depend on the host works normally: **Providers**, **Authenticity** and **LLM Records**.

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
| `providerProbeMode` | `auto` | Whether providers are probed for authenticity on their own, only when asked (`manual`), or never (`off`) |
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

The cost next to each record uses list prices, which vendors change and resellers do not follow. Correct them on the **Model pricing** page, or point `llmPricingFile` at a JSON file of your own rates.

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
