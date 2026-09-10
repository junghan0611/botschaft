# botschaft

**Read the conversations that live outside your harness, from your own seat.**

`Botschaft` is German for both **message** and **embassy**. This repository needs
both senses: the ChatGPT web app is not a harness you run, and the conversations
living there are not data you own. So this repository does not **fetch** those
conversations. It opens a window onto them.

## What it is, and what it is not

Find, pick, read — and later continue — on top of a logged-in ChatGPT web session.
The server is always canonical. No conversation store is kept locally.

- **Not** a reimplementation of ChatGPT inside Emacs
- **Not** a `conversations.json` export viewer. An export is a frozen snapshot,
  so **you cannot continue the conversation**. A good number of nice-looking TUIs
  are exactly this
- **Not** another API-key client that starts fresh conversations. Those already
  exist in quantity (`gptel`, `chatgpt-shell`, and more)

**It is** a window onto the conversation graph you already have on the web.

## Why it has to exist — the slot existing abstractions closed off

Three multi-provider abstractions from the Emacs and TUI worlds, read on 2026-09-10:

| Abstraction | Boundary | Slot for querying conversations |
|---|---|---|
| `gptel-backend` (Emacs) | `gptel-request.el:919-926` — slots `name host header protocol stream endpoint key models url request-params curl-args` | **none** |
| `LlmProvider` (Rust, ducks/llm-tui) | `src/provider/mod.rs:83` — `name` `is_available` `chat` `continue_with_tools` `list_models` | **none** (`list_models` is a model list) |
| pydantic-ai `Model` (used by oterm) | owned by an external library | **none** |

All three cut at **"a provider is a function that sends one prompt and receives
one response."** The `key` slot already occupies the authentication position, so
**there is no room in that type for the idea that a session cookie is the
authentication.** This is not an empty slot; it is a slot the existing
abstractions structurally closed off.

Hence a different boundary here.

## The contract — four verbs (plus one)

```
projects   list the projects
list       list conversations (globally, or within one project)
search     server-side search (globally, or scoped to one project)
read       every turn of one conversation
send       continue / start a conversation     <- once the write path is wired
```

```
        Emacs package ─┐            ┌─ TUI (verification harness)
                       └── contract ┘
                   ┌────────┴─────────┐
             chatgpt backend      claude backend (TODO)
```

**The contract is canonical; backends are swapped underneath it.** Today the
ChatGPT backend stands on
[chatgpt-web-adapter](https://github.com/kymuco/chatgpt-web-adapter) (CWA).
Claude web joins behind the same contract when a counterpart exists — and
**the boundary is settled when a second product is actually attached**, not
before. Fixing the boundary before using it is how the three above failed.

## Current state

| | |
|---|---|
| `lisp/` | **The main line.** `botschaft.el` — three read-only commands, working |
| `tui/` | Go + bubbletea. List, projects, search, read — working |
| `bin/cwaq` | The query shim both front ends share (Python) |
| `docs/chatgpt-protocol.md` | Measured server behaviour — five traps |

Reading runs **without a browser** (system `curl`). Only writing needs a
logged-in Chrome.

## Quick start

All three front ends resolve the auth file to the same XDG location, so one
login serves every one of them.

```bash
# 1. Set up the ChatGPT backend (CWA) and log in once
mkdir -p ~/.local/state/botschaft && chmod 700 ~/.local/state/botschaft
cwa auth login --auth-file ~/.local/state/botschaft/auth_data.json
chmod 600 ~/.local/state/botschaft/auth_data.json

# 2. The query shim -- no environment variable needed
bin/cwaq projects
bin/cwaq list --limit 50
bin/cwaq search 'query' --project <project-id>

# 3. The TUI
cd tui && go build -o cwatui . && ./cwatui
```

### Emacs

Put `lisp/` on your `load-path`. The package finds `bin/cwaq` on its own,
relative to its own file.

```elisp
(add-to-list 'load-path "/path/to/botschaft/lisp")
(require 'botschaft)
;; The auth file is found on its own. Set these two only if the interpreter and
;; CLI that can reach the adapter are not on PATH.
(setq botschaft-python "/path/to/venv/bin/python"
      botschaft-cwa    "/path/to/venv/bin/cwa")
```

| Command | What it does |
|---|---|
| `botschaft-projects` | Pick a project and hold it as the scope. `C-u` clears it |
| `botschaft-search` | Query -> candidates (snippet as the annotation) -> open. `C-u` searches globally |
| `botschaft-open` | List the scope's conversations -> open. `C-u` lists globally |

In a conversation buffer (`special-mode`): `t` toggles tool turns, `g` re-reads,
`y` copies the URL, `o` opens a browser, `q` buries the buffer.
`g` does not drop a cache — **it reads the server again.** There is no cache.

| Environment variable | Use |
|---|---|
| `CWA_BIN` `CWA_PY` | The CWA CLI and a Python that can import the adapter |
| `CWAQ_BIN` | Shim path (default: `bin/cwaq`, found automatically) |
| `CWA_AUTH` | Auth file. Defaults to `$XDG_STATE_HOME/botschaft/auth_data.json`, falling back to `~/.local/state/botschaft/auth_data.json`. **It is a secret** — never commit it |

## Keys (TUI)

| Screen | Keys |
|---|---|
| List | `p` projects · `/` search · `enter` open · `j/k` · `y` URL · `o` browser · `r` reload · `q` |
| Projects | `j/k` · `enter` select · `esc` |
| Reading | scroll · `t` toggle tool turns · `y` · `o` · `r` reload · `esc` |

## Warning

CWA is an unofficial adapter. The risk to your account is not zero. This
repository stands on it and inherits that risk — including while it only reads.
