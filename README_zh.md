<h1 align="center" style="border-bottom: none;">📦⚡️ Casbin Gateway</h1>
<h3 align="center">一个开源网关，管理你机器上的 AI 编程 Agent，由 Go 和 React 开发。</h3>
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
  <a href="./README.md">English</a> | <b>中文</b>
</p>

<p align="center"><b>机器上的每一个编程 Agent</b></p>
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

<p align="center"><b>一个入口对接任意模型厂商</b></p>
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

## 每个 Agent 支持到什么程度

| Agent | 行为监控 | 换供应商 | MCP | 技能 | 提示词 | 会话 | 安装 |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
| <img src="https://www.google.com/s2/favicons?domain=claude.com&sz=64" width="16" height="16" alt=""> **Claude Code** | ✅ | ✅ Anthropic | ✅ | ✅ | ✅ | ✅ | npm · brew · winget |
| <img src="https://www.google.com/s2/favicons?domain=claude.ai&sz=64" width="16" height="16" alt=""> **Claude Desktop** | ✅ | — | ✅ | — | — | ✅ | — |
| <img src="https://www.google.com/s2/favicons?domain=openai.com&sz=64" width="16" height="16" alt=""> **Codex CLI** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | npm · brew |
| <img src="https://www.google.com/s2/favicons?domain=openai.com&sz=64" width="16" height="16" alt=""> **ChatGPT 桌面版（Codex）** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | — |
| <img src="https://www.google.com/s2/favicons?domain=gemini.google.com&sz=64" width="16" height="16" alt=""> **Gemini CLI** | ✅ | ✅ Gemini | ✅ | ✅ | ✅ | ✅ | npm |
| <img src="https://www.google.com/s2/favicons?domain=cursor.com&sz=64" width="16" height="16" alt=""> **Cursor** | ✅ | — | ✅ | ✅ | — | ✅ | brew |
| <img src="https://www.google.com/s2/favicons?domain=cursor.com&sz=64" width="16" height="16" alt=""> **Cursor Agent** | ✅ | — | ✅ | ✅ | — | — | — |
| <img src="https://www.google.com/s2/favicons?domain=windsurf.com&sz=64" width="16" height="16" alt=""> **Windsurf** | ✅ | — | ✅ | — | ✅ | — | brew |
| <img src="https://www.google.com/s2/favicons?domain=opencode.ai&sz=64" width="16" height="16" alt=""> **opencode** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | npm |
| <img src="https://www.google.com/s2/favicons?domain=opencode.ai&sz=64" width="16" height="16" alt=""> **opencode 桌面版** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | — |
| <img src="https://www.google.com/s2/favicons?domain=openagentai.org&sz=64" width="16" height="16" alt=""> **OpenAgent** | ✅ | — | — | — | — | — | — |
| <img src="https://www.google.com/s2/favicons?domain=openclaw.ai&sz=64" width="16" height="16" alt=""> **OpenClaw** | ✅ | ✅ OpenAI | ✅ | ✅ | ✅ | ✅ | npm |
| <img src="https://www.google.com/s2/favicons?domain=nousresearch.com&sz=64" width="16" height="16" alt=""> **Hermes Agent** | ✅ | ✅ OpenAI | — | — | — | — | — |
| <img src="https://www.google.com/s2/favicons?domain=deepseek.com&sz=64" width="16" height="16" alt=""> **DeepSeek Harness** | ✅ | ✅ OpenAI | ✅ | ✅ | — | ✅ | npm |

- **行为监控** —— 只读地记录 Agent 做了什么：提示词、工具调用、权限询问。Agent 的任何动作都不会等待 Gateway，开着监控也不会改变任何一次回答。
- **换供应商** —— Gateway 按该 Agent 客户端自己说的协议格式，改写它的配置文件，把它指向绑定的供应商。不支持的 Agent 仍可以用界面上给出的环境变量接进来。
- **MCP · 技能 · 提示词** —— 在 Agent 之间读取、对比和复制 MCP 服务器、技能和指令文件。
- **会话** —— 直接从 Agent 自己的会话记录里读提示词和 token 用量，包括没走 Gateway 的那部分。
- **安装** —— Gateway 能用来安装和升级它的包管理器。其余的都只能去厂商页面下载。

## 功能特性

- **[查一查那个 Key 背后的 API 是不是真的](#杀手锏那个-key-背后的-api真是卖给你的那个吗)** —— 中转商可以偷偷换成便宜模型、伪造缓存命中，流量里根本看不出来。Authenticity 直接问上游，测出 A 到 F 的等级。
- **[一个地方切换所有 Agent 的 API 供应商](#让-agent-的流量走-gateway)** —— 改一次 Key 或 base URL，接进 Gateway 的每个 Agent 都跟着换。
- **[一条链接就能导入 Provider、MCP、提示词或技能](#从一条链接导入)** —— 在网页上点厂商的「添加到」按钮，Gateway 就带着链接里的东西打开，写进去之前先给你看。
- **[同一个 Agent 开多个实例](#接下来做什么)** —— 比如同时跑好几个 Claude Desktop，各自登录不同账号。
- **[看到完整的请求，而不只是一个数字](#记录提示词)** —— 每一条 prompt、消息和工具 schema，都留在这台机器上。
- **[规定每个 Agent 能做什么](#每个-agent-能做什么)** —— 每个 Agent 四十来个开关，分组管理，覆盖工具、模型和供应商，每个转发的请求都由 Casbin 判定。
- **[统计每个 Agent 花了多少，包括没走 Gateway 的部分](#每个-agent-花了多少包括没走-gateway-的那部分)** —— 直接读 Agent 自己写的会话记录。
- **[跨 Agent 对比、复制技能 / MCP / 提示词](#接下来做什么)** —— 一张表看到所有 Agent 装了什么。

## 界面预览

| 本机上的每一个 Agent | 这些 Agent 身上装的每一样东西 |
| :---: | :---: |
| [![Agents](https://cdn.casbin.org/img/casbin-gateway-home.png)](https://cdn.casbin.org/img/casbin-gateway-home.png) | [![Skills, MCP & Prompts](https://cdn.casbin.org/img/casbin-gateway-skills.png)](https://cdn.casbin.org/img/casbin-gateway-skills.png) |
| 每个 Agent 接在哪、登录的是哪个账号、在那里花了多少、此刻是不是在跑 | 所有 Agent 的技能、MCP 服务器和提示词文件并排对比，还能从一个 Agent 复制到另一个 |

| 每个 Agent 花了多少 | 每个模型厂商一个入口 |
| :---: | :---: |
| [![用量](https://cdn.casbin.org/img/casbin-gateway-usage.png)](https://cdn.casbin.org/img/casbin-gateway-usage.png) | [![新建 Provider](https://cdn.casbin.org/img/casbin-gateway-new-provider.png)](https://cdn.casbin.org/img/casbin-gateway-new-provider.png) |
| 从 Agent 自己写的会话记录里读出来，所以没走 Gateway 的请求也一样算得上 | 44 个厂商预设，或任何 OpenAI / Anthropic 兼容的 base URL |

| Agent 转发过的每一个请求 | 完整的请求，而不只是一个计数 |
| :---: | :---: |
| [![LLM 记录](https://cdn.casbin.org/img/casbin-gateway-llm-records.png)](https://cdn.casbin.org/img/casbin-gateway-llm-records.png) | [![单条记录](https://cdn.casbin.org/img/casbin-gateway-llm-record.png)](https://cdn.casbin.org/img/casbin-gateway-llm-record.png) |
| 按模型统计请求数、Token、缓存命中率和成本 | 系统提示词、每一条消息，以及模型拿到的每个工具的 schema |

## 运行

一条命令。不需要数据库，不需要 Go，不需要 Node，不需要配置。

Linux 和 macOS：

```bash
curl -fsSL https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.sh | bash
```

Windows，在 PowerShell 中：

```powershell
irm https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.ps1 | iex
```

两者都会下载适配本机的构建产物，解压到 `~/.local/share/casbin-gateway`（Windows 上是 `%LOCALAPPDATA%\casbin-gateway`），把 `casbin-gateway` 命令加入 PATH，启动它，并设置成开机（登录）自启。安装用的那个终端窗口立刻就还给你。

Gateway 会在自己的窗口里打开 —— 不用登录：它只服务本机，本机访问会直接以管理员身份进入。关掉窗口不会退出，它缩到托盘图标后台继续跑，重新打开窗口和真正退出都在托盘菜单里。桌面和开始菜单里会有一个 **Casbin Gateway** 快捷方式；macOS 上在 `~/Applications`，Linux 上在应用菜单里。手动解压的压缩包也一样，启动器第一次运行时会自己建好。

想用浏览器，或者机器上根本没有桌面环境，**http://localhost:17000** 一样能访问；`casbin-gateway start` 则只跑服务端，没有窗口也没有托盘。

安装到此为止。Gateway 把数据存在自己目录下的一个 SQLite 文件里，登录也走它自己的用户表。

### 保持最新

当前运行的版本号显示在右上角，后面跟着它的构建日期，有新版本时会标上**新**。点开它按**立即升级**：Gateway 会下载适配本机的构建产物，先确认它能跑起来，再替换掉自己并重启。新版本起来后页面会自动刷新，其他什么都不用动 —— 数据、设置和 `casbin-gateway` 命令都留在原处。

如果 Gateway 不是这样装的，或者所在目录不可写，页面会直接说明，并给出手动升级的命令。

### 接下来做什么

| 页面 | 你能得到什么 | 需要什么 |
| --- | --- | --- |
| **Agents** | 本机安装的每一个 AI 编程 Agent —— Claude Code、Codex CLI、Cursor、Gemini CLI、opencode 等等 —— 一行四张卡片，每张写明它登录的是哪个账号、接的是哪个 Provider、花了多少、此刻是不是在跑。可以在卡片上直接启动或停止，也可以给同一个 Agent 开多个**实例**，每个实例有自己的状态目录和自己的登录账号，同时运行。本机没装的 Agent 也会列出来，可以直接用宿主机已有的包管理器安装或升级。 | 无 |
| **Skills, MCP & Prompts** | 所有 Agent 的所有技能、MCP 服务器和提示词文件汇总在一张表里。可以从 GitHub 仓库、`.zip` / `.tar.gz` 压缩包，或本机的一个目录安装技能，一次装进一个或多个 Agent。MCP 服务器也一样，还能编辑某个 Agent 每次会话前读到的提示词，或者打开、删除、复制到另一个 Agent。 | 无 |
| **Sessions** | 这些 Agent 的每一次会话，从它们留在磁盘上的会话记录里读出来：完整的对话，一条一条。一共多少次、今天跑了多少次、其中多少条是从会话记录里读来而不是监控到的，都在页头；再按 Agent 或按这两种来源筛。 | 无 |
| **Activity** | 被监控的 Agent 正在做什么：每一次工具调用、调用的目标，以及耗时。 | 给某个 Agent 打开监控 |
| **Providers** | 挡在模型厂商前面的统一入口。API Key 由 Gateway 持有，Agent 拿不到；也可以转发 Agent 自己的登录，什么都不持有。 | 一个厂商的 API Key，或者什么都不用 |
| **Authenticity** | 每个 Provider 一个 100 分制的分数和一个等级，不用点按钮就会自动测出来 —— 见[下面这一节](#杀手锏那个-key-背后的-api真是卖给你的那个吗)。 | 一个带 API Key 的 Provider |
| **LLM Records** | Agent 转发的每一次请求：完整的 system prompt、每一条消息和工具调用、模型可用的每个工具的 schema，以及 token 数和费用。 | 一个 Provider，以及 `llmRecordMode` —— 见[记录提示词](#记录提示词) |
| **Usage** | 本机每个 Agent 花了多少：按时间、按模型、按 Agent 分别统计，数据来自 Agent 自己写的会话记录 —— 所以没走 Gateway 的请求也算得上。另一个页签是 Gateway 转发过的部分，那是唯一知道由哪个 Provider 作答、是否失败的账。 | 无 |
| **Model pricing** | 每百万 Token 多少钱，Usage 页面上的每一个金额都由它算出来。可以手工改某个价格，也可以让 Gateway 按计划从 [models.dev](https://models.dev) 的目录给本机跑过的模型重新定价；你手工改过的价格不会被覆盖。 | 无 |

带页签或分节的页面，在侧边栏里各有自己的名字，所以从导航栏可以直接落到测试用例、MCP 服务器或安全设置那一节，而不是只能落到它们所在页面的顶部。**⌘K**（Windows 和 Linux 上是 **Ctrl+K**）在当前页面上打开一个搜索框：按名字搜每一个页面，也搜本机的每一个 Agent 和 Provider —— 三十个 Provider 里找一个，不用一页页翻。

Agent 是通过读取 **Gateway 所在机器**的用户账户、home 目录和安装路径发现的，所以要在你想观察的那台机器上运行它。

### 杀手锏：那个 Key 背后的 API，真是卖给你的那个吗？

中转商可以卖着前沿模型、跑着便宜模型，可以把缓存过的前缀当新输入计费，也可以假装自己会说某套 API。这些在流量里都看不出来，所以 **Authenticity** 直接去问上游。每个 Provider 都会被单独探测——加入时、端点/类型/密钥变更时、从未探测过时，以及报告过期后——测出来的是一个 100 分制的分数和一个 A 到 F 的等级，显示在 Authenticity 页面上，也显示在首页 Agent 列表的正上方。不用点按钮，也不用配置什么。

[![Authenticity](https://cdn.casbin.org/img/casbin-gateway-authenticity.png)](https://cdn.casbin.org/img/casbin-gateway-authenticity.png)

分数只是背后那些测试用例的汇总，而用例全都摆在页面上。一半的用例问的是「这上游到底是什么」：作答的模型是不是点名的那个——在不是厂商自己域名的接口上只按半题算，因为那个字段是上游想写什么就写什么；模型自己说是谁训练的；请求前面有没有被塞进别的指令；同一个请求连发几次是不是同一个模型在答；以及 API 文档里写着的参数（`logprobs`、`n`、停止序列）到底是照做了、明确拒绝了，还是收下 200 之后悄悄丢掉了。另有一个题库，问的是只有一个正确答案的问题：从数一个单词里有几个字母，到《滕王阁序》的作者是谁。剩下的看的是信封本身：强制调用时两层嵌套的工具 schema 能不能扛住、事件流里该有的事件是不是都到齐了、提示词缓存是不是真的在计费上体现出来、两个完全相同的请求计费是否一致、厂商自己的响应头在不在。每条用例都写明它问上游什么、实际发出去的请求长什么样、答案怎么判、占多少权重。

[![测试用例](https://cdn.casbin.org/img/casbin-gateway-authenticity-cases.png)](https://cdn.casbin.org/img/casbin-gateway-authenticity-cases.png)

可以改权重、停用、重写问题，也可以自己加一条——该问中转商什么，各家情况并不一样，而评分方法不公开就算不上证据。**恢复默认**会把内置用例恢复原样，你自己写的不受影响。

报告还有不花钱、也不发任何请求的另一半：直接读 Gateway 已经留下的记录，看这个上游到底表现如何 —— 缓存实际被
计入了多少、有多少次请求失败、响应有多快、有多少跑过的模型还没有价格。

一次探测会花掉该 Provider 一点点额度，具体花了多少就写在报告上。`providerProbeIntervalHours` 决定报告多久算过期，`providerProbeMode = "manual"` 表示只在手动触发时探测，`"off"` 表示从不探测。

### 让 Agent 的流量走 Gateway

这是 **LLM Records** 的数据来源，也是让 Gateway 而不是 Agent 持有厂商 Key 的方式。

1. **Providers** → **Add**：选类型（OpenAI 兼容或 Anthropic 兼容），填厂商的 base URL 和 API Key，并列出它提供的模型。
2. **Agents** → 打开一个 Agent → 选中该 Provider。如果这个 Agent 的配置格式 Gateway 认识，点 **Write configuration** 就会写进它自己的配置文件 —— **Preview** 会先原样列出将要写入的内容，**Restore** 可以还原。写过一次之后再换 Provider，文件会立即被改写，所以之后切换只需要点一下，两个页面上都可以。
3. 其他 Agent 则复制页面显示的环境变量片段，在设置了这些变量的 shell 里启动：

```bash
export ANTHROPIC_BASE_URL="http://localhost:17000/v1/agents/claude-code"
export ANTHROPIC_AUTH_TOKEN="cg-..."
```

这个 token 是 Gateway 自己的中继令牌，不是厂商的 Key：Agent 没有它就拒绝启动，而 Gateway 会用 Provider 自己的 Key 去认证上游。页面上的片段里已经填好了真实的值。

同一个 base URL 会按 Agent 说的那套 API 应答：OpenAI 客户端走 `/chat/completions`，Anthropic 客户端走 `/v1/messages`，Codex 走 `/responses` —— 它去掉了 chat completions 这套线格式，只剩这一种，Gemini CLI 走 `/v1beta/models/<model>:generateContent` —— 它只会说 Google 自己那套 API。Provider 用的是哪套 API 不必与之相同：Gateway 会在这四套之间双向转换，流式回复也一样，所以 Codex 能跑在 DeepSeek、Kimi、Qwen 上，Claude Code 同样能跑在它们上面。如果 Provider 正好就说请求进来的那套 API，就原样转发，一个字节都不改。唯一由 Gateway 自己应答的是 Anthropic 和 Gemini 客户端每轮之前问的 token 数：绑定的 Provider 没有对应接口时，由 Gateway 估算。

### 没有 API Key：沿用 Agent 已有的登录

用 ChatGPT 或 Claude 订阅登录的 Agent 根本没有 API Key 可填。把 Provider 的**认证方式**设成**调用方自己的登录**，它就不需要 Key：base URL 指向厂商，每个请求都带着 Agent 自己发来的凭据转发上游，Agent 继续用它已有的登录。**Models** 留空，这个 Provider 就接受任何模型名。

这种 Provider 的环境变量片段只设 base URL，不设别的 —— 在这里再设一个 token 会覆盖 Agent 已有的登录。记录和路由和有 Key 的 Provider 完全一样，只是 Gateway 从头到尾没见过任何 Key。

Codex 是例外：它的 ChatGPT 登录走的是没有任何 Provider 能替代的端点，所以 Codex CLI 仍然需要一个带 API Key 的 Provider。

### 从一条链接导入

厂商会在自己的页面上放一个「添加到我的 Agent 管理器」按钮，点开的是一条 `ccswitch://` 链接，而不是一个网页。Gateway 直接读这个格式，而不是再造一个没人会给你按钮的格式。一条链接可以带四样东西之一：一个 Provider，连同它的 base URL、Key 和模型列表；一个 MCP 服务器，就是它平时写成的那段 JSON；一份 Agent 每次会话前要读的提示词；或者一个安装技能用的仓库。

Gateway 启动时会在 Windows 和 Linux 上接管这个协议，所以点这种按钮就会打开 Gateway 的 **Import** 页面，把链接里的东西摊开给你看 —— MCP 服务器会以什么参数启动、提示词的全文、技能从哪个仓库来。**在你按下下面那个按钮之前，什么都不会写进去**：链接来自别人的网站，所以先看清楚里面是什么。Provider 会转到 Providers 的表单里，在那里过目并向上游探测之后才存下来；另外三样从这里写入，走的是手工添加时的同一批接口，写进你勾选的那些 Agent。链接里写的是「应用」而不是本机的 Agent，其中 Gateway 不管的那些会明确告诉你，而不是悄悄丢掉。

链接是放在一次 API 调用的 body 里交给 Gateway 的，不是放在它打开的那个页面的地址里 —— Provider 链接带着 API Key，而地址会留在浏览器历史里，还会跟着页面之后的跳转一起发出去。

这个协议只在第一次从原来的持有者手里接管，Gateway 卸载时再还回去；已经是 Gateway 自己的注册则每次启动都重写一遍，因为它记的是一个路径，升级或移动之后会指向一个已经不在那里的 Gateway。macOS 上则什么都不接管 —— 那里的 URL 协议属于应用程序包，链接是以 Apple event 送达的，而这个启动器没有接收它的事件循环，声明了反而会把链接从能打开它的程序那里抢走。在 macOS 上把链接粘进 Import 页面的输入框即可，这个办法在所有平台都能用；Providers 页面同样可以直接粘一条 Provider 链接。

### 每个 Agent 能做什么

侧边栏的 **Permissions** 就是这个页面：一侧是这台机器上的每个 Agent，另一侧是选中那个能做什么；Agent 自己的详情页上也有同一张卡片。这个 Agent 经 Gateway 转发的所有请求，都按这里设的来判，不用改它自己的配置，就能把能力收窄。

- **工具** —— 四十来个开关，分成六组：终端、读取项目、修改项目、访问网络、规划与委派，再加上这个 Agent 装的每一个 MCP 服务各一个开关。点分组标题上的开关就能整组一起设；展开这一组，粒度想多细有多细 —— 执行命令、读取运行中命令的输出、停止命令，是三个独立开关。关掉的工具会在请求离开这台机器之前被删掉，模型压根看不到它，Agent 也就无从调用。各家 Agent 的工具名都不一样，`Bash`、`shell`、`run_shell_command` 都归同一个开关管。每组最后都有一个兜底项，管的是 Gateway 没见过的同类工具 —— 有了它，关掉一组才是真的关死，而不是只关掉设置那天恰好列出来的几个。
- **模型** —— 任意模型、只允许勾选的、或除勾选的以外都允许。名称可以用 `*` 结尾，`claude-opus-*` 就覆盖一整个系列。
- **供应商** —— 这个 Agent 的请求可以发给哪几个 Provider。

请求里出现被关掉的东西，返回的是这个 Agent 所用 API 格式的 `permission_error`，读起来是一次拒绝，而不是网关坏了。

底层上，这些开关会编译成一份 [Casbin](https://casbin.org) 策略，每个转发的请求都由 enforcer 判定，而不是靠手写的 if。点 **Advanced** 就能看到它们编译出的真实 `model.conf` 和 `policy.csv`，也可以自己往里加策略行：

```
p, claude-code, model:claude-opus-*, use, deny
p, claude-code, model:*, use, allow
p, claude-code, tool:shell/run, use, deny
p, claude-code, tool:mcp/github, use, allow
p, claude-code, tool:mcp/*, use, deny
p, claude-code, tool:*, use, allow
```

第一条匹配上的规则说了算，所以例外可以排在兜底规则前面：MCP 服务全关，只留下一个。你自己写的那几行，排在开关生成的规则之前。

规则管的是走代理的流量，所以直连供应商的 Agent（它自己的配置指向厂商而不是 Gateway）不受这些规则约束，页面上会直接提示。

### 每个 Agent 花了多少，包括没走 Gateway 的那部分

用自己订阅登录的 Agent 不会往 Gateway 转发任何东西，直接打到厂商的请求在这里也不会留下记录 —— 但 Agent 自己会
把这次会话写到磁盘上。**Usage** 读的就是这些会话记录，所以哪怕一个 Provider 都没配、什么都没接管，本机每个 Agent
花了多少也从第一次启动起就摆在一个页面上：Token、缓存命中率和费用，按时间、按模型、按 Agent 分别统计。另一个页签
**Gateway 转发的部分** 是更窄的一本账，但它是唯一知道由哪个 Provider 作答、是否失败的。

上面的每一个金额都由 **Model pricing** 算出来，那是一张“每百万 Token 多少钱”的表。厂商会调价、中转商也不跟着调，
所以价格可以手工改，也可以让 Gateway 按计划（`modelsDevSyncMode`、`modelsDevSyncIntervalHours`）从
[models.dev](https://models.dev) 的目录给本机跑过的模型重新定价，你手工改过的那些不受影响。

[![模型价格](https://cdn.casbin.org/img/casbin-gateway-pricing.png)](https://cdn.casbin.org/img/casbin-gateway-pricing.png)

### 停止、升级、卸载

- **停止**：`casbin-gateway stop`。**再次启动**：`casbin-gateway start`。**查看状态**：`casbin-gateway status`。三条命令在任意目录都能用 —— 这个命令是一个包装脚本，总是在安装目录（数据所在的地方）里启动 Gateway。
- **前台运行**（想盯着它跑的时候）：`casbin-gateway`，用 `Ctrl-C` 停止。后台运行时控制台输出写在 `logs/casbin-gateway.out`。
- **升级**：点右上角的版本号，再点**立即升级**，或者再跑一遍安装命令。两种方式都不影响数据库和设置。
- **卸载**：删除 `~/.local/share/casbin-gateway` 和 `~/.local/bin/casbin-gateway`（Windows 上是 `%LOCALAPPDATA%\casbin-gateway` 及其 PATH 条目），以及安装脚本结束时告诉你的那个自启动条目。

设置 `INSTALL_DIR` 可以装到别的位置，`NO_START=1` 只安装不启动，`NO_AUTOSTART=1` 则不设置登录自启。

### 让其他机器也能访问

Gateway 默认只监听 `127.0.0.1`，因为有两样东西对能连上这个端口的人是完全敞开的：Web UI 会直接把本机管理员登进去，`/v1` 会用这里存的 API Key 去转发。这两点正是本地工具该有的样子，但都不该暴露给网络。

如果确实要让其他机器访问，在 `conf/app.conf` 里把 `httpaddr` 设成 `0.0.0.0`，然后：

1. **改掉管理员密码**（**My Account** 里）。自动登录在遇到第一个非本机请求时就停止，从那以后密码是唯一的门槛。
2. **每个发往 `/v1` 的请求都要带上中继令牌**。Gateway 在首次启动时生成它，显示在 **Settings → Security** 里；Providers 和 Agents 页面上的环境变量片段已经填好了它，Gateway 为 Agent 写配置时写进去的也是它。来自本机的请求永远不需要它。

**这些是 nightly 构建**，每次推送都从 `master` 重新构建，并作为 [`nightly`](https://github.com/apache/casbin-gateway/releases/tag/nightly) 预发布版本发布。它们的用途是让人不装 Go 和 Node 工具链就能试用 Gateway；其他场景都应该从源码发布版构建。

### 在 Docker 或 Podman 里运行

**容器看不到你机器上的 Agent。** Agent 是靠读取 Gateway 所在机器的用户主目录和安装路径发现的，而在容器里那是容器自己的文件系统。所以 **Agents**、**Skills, MCP & Prompts** 和 Agent 监控在容器里一直是空的，页面会直接说明这一点，而不是让人以为什么都没装。不依赖宿主机的部分照常可用：**Providers**、**Authenticity** 和 **LLM Records**。

因此，想监控哪台机器上的 Agent，就在那台机器上跑上面的一键安装；只把 Gateway 当作别的机器的模型入口时，才用容器部署。

我们没有发布镜像，compose 文件会用本仓库的源码构建一个，所以先把仓库 clone 下来，在仓库根目录执行：

```bash
docker compose up -d
```

Podman 读同一个文件：

```bash
podman compose up -d
```

两种方式下管理界面都在 http://localhost:17000，SQLite 数据库存在一个命名卷里，`down` 之后依然保留；`conf/app.conf` 从仓库挂载进去，首次启动前可以先改它播下的那份初始配置，不用重新构建镜像。

## 配置

所有配置都是可选的。设置在 Web UI 的 **设置** 页面里修改，保存在数据库中，既不用手工改文件，也不用重启。可执行文件旁边的 `conf/app.conf` 只在第一次启动时播下这些值，每一项在文件里都有说明；一步安装装出来的单个可执行文件旁边没有这个文件，播的是编进二进制里的那一份。第一次启动之后再改这个文件不会有任何效果，只有在数据库打开之前就要读的那几项例外：`httpport`、`driverName`、`dataSourceName`、`dbName` 和 `redisEndpoint`。真正常被改动的是这些：

| 配置项 | 默认值 | 作用 |
| --- | --- | --- |
| `httpport` | `17000` | Web UI 和 REST API 的端口 |
| `httpaddr` | `127.0.0.1` | Web UI 监听的网卡 —— 见[让其他机器也能访问](#让其他机器也能访问) |
| `driverName` / `dataSourceName` | `sqlite` / `./data/casbin-gateway.db` | 数据存放位置 |
| `llmRecordMode` | `full` | 每次转发的 LLM 请求保留多少内容 —— 包含提示词正文，见[记录提示词](#记录提示词) |
| `providerProbeMode` | `auto` | Provider 是自动做真伪探测，还是只在手动触发时探测（`manual`），或者从不探测（`off`） |
| `apiKeyEncryptionKey` | 空 | 加密存储 Provider 的 API Key（AES-256-GCM） |
| `casdoorEndpoint` | 空 | 把登录切换到 [Casdoor](https://casdoor.org) SSO |

Gateway 启动时会打印它实际在做什么，所以可以直接看结果而不用去查配置文件：

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

还占着这个端口的上一个 Gateway 会先被停掉，所以重启不用等它。被别的程序占着的端口则留给它：Gateway 会指出是哪个进程占着并停止启动，而不是抢过端口，也不会半配置地跑起来。

### 记录提示词

**从第一个请求起，每次转发的请求都会被完整记录下来，包括提示词正文。** LLM Records 页面就是拿这些内容做出来的，它们也不会离开这台机器 —— 但提示词里粘进过什么它就带着什么，所以如果你不想要这样，在把 Agent 接进 Gateway 之前就该先改掉这个设置。LLM Records 页面顶部的选择框可以在 **不记录**、**只记录元数据** 和 **记录元数据和正文** 之间切换，从下一次请求起生效；设置页面里有同样的选项和相关的上限，它们的初始值来自：

```ini
; 默认是 "full"：它会存下请求体，这是 LLM Records 展示提示词、消息和
; 工具 schema 所需要的。"metadata" 只记录谁调用了哪个模型、结果如何，
; "off" 则什么都不留。
llmRecordMode = "full"
llmRecordRetentionDays = 30
llmRecordMaxRecords = 10000
llmRecordMaxPayloadBytes = 1048576
```

请求体在存储前会被脱敏：看起来像凭据的内容会被替换掉，替换次数会随记录一起显示。请求头（入站 API Key 就在里面）根本不会进入记录。超过 `llmRecordMaxPayloadBytes` 的请求体会保留结构、只截断其中最长的字符串，所以一段很长的对话仍然能逐条消息列出来。

每条记录旁边的费用用的是官方标价，而厂商会调价、中转商也不按它来。在 **Model pricing** 页面上改掉，或者把 `llmPricingFile` 指向你自己的费率 JSON 文件即可修正。

### 接入 Casdoor

[Casdoor](https://casdoor.org) 是可选的，接管成员管理。在一个 Casdoor 实例里为 Gateway 创建组织和应用，然后在 **设置 → 登录** 里填好那五项。只要设置了 `casdoorEndpoint`，登录就会跳转到 Casdoor，同时还会启用 [OAuth 登录](https://casdoor.org/docs/provider/oauth/overview)。

## 开发

### 环境要求

Go 1.20+，以及带 Yarn 的 Node.js。

### 从源码运行

后端从 `web/build` 提供编译好的前端，所以先构建一次：

```bash
cd web && yarn install && yarn build
```

```bash
go run main.go
```

然后打开 http://localhost:17000，本机访问会直接以管理员身份进入，和安装版一样。SQLite 数据库在首次启动时创建，不需要安装数据库服务。

### 前端开发

```bash
cd web && yarn dev
```

这会在 http://localhost:16002 上提供带热更新的 UI，并把 API 请求代理到 17000 端口的后端，所以两边都得跑着。

### 使用 MySQL 代替 SQLite

项目用的是 XORM，所以它支持的每种数据库都能用。把 Gateway 指向你的服务器，首次启动时如果 `dbName` 不存在它会自动创建：

```ini
driverName = mysql
dataSourceName = root:123@tcp(localhost:3306)/
dbName = casbin_gateway
```

### 构建单文件二进制

Gateway 平时会从磁盘读两样东西：`conf/app.conf` 和 `web/build` 里编译好的 UI。`embed` 构建标签会把这两样都打进可执行文件，安装脚本发布的就是这种产物：

```bash
cd web && yarn install && yarn build
```

```bash
go build -tags embed -o casbin-gateway .
```

要先构建前端 —— `web/build` 下的所有内容都会进入二进制，该目录不存在时 `go build -tags embed` 会编译失败。

磁盘上的文件始终优先于内嵌副本，所以单文件二进制照样可以被配置和用于开发，不必重新构建：

| 内嵌资源 | 被什么覆盖 |
| --- | --- |
| `conf/app.conf` | 工作目录下、或可执行文件旁边的 `conf/app.conf` |
| `web/build` | 工作目录下的 `web/build/index.html`，此后整个 UI 都从那里提供 |

启动摘要会报告每一项实际来自哪里。

### 数据存放位置

自包含说的是启动，不代表运行时只读。运行中的 Gateway 会相对于工作目录写入 `./data`（SQLite 数据库和 Agent 的 patch 状态）、`./logs` 和 `./tmp` —— 这也是为什么安装出来的 `casbin-gateway` 命令是一个总在安装目录里启动它的包装脚本。直接在别处运行可执行文件，会在那里得到第二个空的安装。

## 架构

Casbin Gateway 包含 2 个部分：

| 名称 | 说明 | 语言 | 源代码 |
|----------|----------------------------------------|------------------------|----------------------------------------------------------|
| 前端 | Casbin Gateway 的 Web 前端 UI | TypeScript + React + shadcn/ui | https://github.com/apache/casbin-gateway/tree/master/web |
| 后端 | Casbin Gateway 的 RESTful API 后端 | Golang + Beego + XORM | https://github.com/apache/casbin-gateway |

## 在线演示

https://ai.casbin.com

## 文档

https://caswaf.org

## 贡献

有任何问题欢迎提 issue，或者直接提 pull request —— 不过我们建议先开一个 issue 和社区讨论清楚。

## 许可证

[Apache-2.0](https://github.com/apache/casbin-gateway/blob/master/LICENSE)
