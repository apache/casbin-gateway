# CLAUDE.md

## Code style

- Don't add Go tests unless asked. Verify changes by building or running the code instead.
- Keep comments sparse and short. No paragraph-length doc blocks, no narration of what the code obviously does.

## Chinese translations

- Never put a space between Chinese and Latin letters, digits, or placeholders. Write `每个Agent`, `覆盖{count}个`, `写在~/.codex/config.toml里` — not `每个 Agent`.
- "Agent" stays English in Chinese text. The product term is `Agent客户端`; a spawned sub-agent is `子Agent`. Never `智能体`.
