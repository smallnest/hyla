# Agent marks

The icon the sidebar puts before a session whose agent is this one: a 14×14
mark in the agent's colour. The files are [Simple Icons](https://simpleicons.org)'s
SVGs (CC0, the brands their owners' trademarks), as
[Iconify](https://iconify.design) serves them, and each file's own `fill` is
its brand's colour. The exception is `antigravity.svg`, Google Antigravity's
icon in its own colours, from its [press page](https://antigravity.google/press),
which Simple Icons does not have: a mark of several colours is drawn as
its makers drew it (`marks.MarkFull`), the others in the row's text colour.
`Tint` (marks.go) adjusts a one-colour mark to the window: a near-black
one is drawn in the row's text colour, so that it reads on a dark window
too, and the palest brands are darkened on a light one.

The agent names are hylad's (`daemon.agents`), and `agentMarks` in
`icons.go` is the list itself — this table is the same list, to read.

| Agent | File | Colour |
| --- | --- | --- |
| `claude` | `claude.svg` | Claude's orange |
| `codex` | `openai.svg` | near-black (the text's) |
| `opencode` | `opencode.svg` | near-black (the text's) |
| `pi` | `pi.svg` | near-black (the text's) |
| `dsh` (DeepSeek Harness) | `deepseek.svg` | DeepSeek's blue |
| `gemini` | `googlegemini.svg` | Google's purple |
| `antigravity`, `agy` | `antigravity.svg` | its own, full colour |
| `amp` | `amp.svg` | its own |
| `cursor-agent` | `cursor.svg` | near-black |
| `copilot` | `githubcopilot.svg` | near-black |
| `cline` | `cline.svg` | near-black |
| `qwen` | `qwen.svg` | its own |
| `kimi` | `moonshotai.svg` | near-black |
| `windsurf` | `windsurf.svg` | its own |
| `zed` | `zedindustries.svg` | its own |
| `jetbrains` | `jetbrains.svg` | near-black |
| `trae` | `trae.svg` | its own |
| `mistral` | `mistralai.svg` | its own |
| `ollama`, `llama` | `ollama.svg` | near-black |
| `huggingface` | `huggingface.svg` | its yellow, darkened on white |
| `minimax` | `minimax.svg` | its own |
| `doubao` | `bytedance.svg` | its own |
| `alibabacloud` | `alibabacloud.svg` | its own |
| `anthropic` | `anthropic.svg` | its own |
| `vercel`, `warp`, `replit` | `vercel.svg`, `warp.svg`, `replit.svg` | their own |
| `perplexity`, `nvidia`, `meta`, `ray` | their own files | their own |
| `langchain`, `langgraph`, `crewai` | their own files | their own |
| `n8n`, `dify`, `vllm`, `lmstudio`, `modal` | their own files | their own |

A session with no agent shows no mark; an agent Hyla has no mark for shows
none either, rather than a wrong one. `aider`, `goose`, `crush`, `kiro-cli`,
`droid` and `auggie` are those: their makers drew no logo to use, and no
other agent's stands in for it.

`icons/README.md` covers the rest of Hyla's icons, including the pane
header's marks of programs (`si-*.svg`), which are drawn in the muted text
colour rather than the brand's.
