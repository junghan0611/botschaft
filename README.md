# botschaft

> [!WARNING]
> **Under construction.** Read paths are usable, but the continuation path has
> not completed its first controlled live-account turn. Expect sharp edges and
> do not treat this repository as a released client yet.

**Read and continue the conversations that live outside your harness, from your own seat.**

`Botschaft` is German for both **message** and **embassy**. This repository needs
both senses: the ChatGPT web app is not a harness you run, and the conversations
living there are not data you own. So this repository does not **fetch** those
conversations. It opens a window onto them.

## What it is, and what it is not

Find, pick, read, and continue on top of a logged-in ChatGPT web session.
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
send       continue an existing conversation
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
| `lisp/` | Read-only Emacs package: projects, search, list, read |
| `tui/` | Go + bubbletea. Projects, search, list and read are live-tested; compose, stream and canonical readback are implemented pending one controlled live turn |
| `bin/cwaq` | The query shim both front ends share |
| `run.sh` | Setup, build, check and run — the same on x86-64 and aarch64 |
| `docs/chatgpt-protocol.md` | Measured server behaviour — five traps |

Reading requires no browser. The TUI sends existing-conversation text through
CWA's experimental `browserless-request` transport. Its readiness sentinel reports
ready without Chrome while full preflight remains pending; web-search behavior is
unmeasured. Browser-owned features still require a logged-in Chrome and extension.

## Quick start

`./run.sh` is the front door. With no arguments it opens a menu; with a
subcommand it runs headlessly, so another host or an agent can call it directly.

```bash
./run.sh setup     # install the ChatGPT backend at the pinned commit, for this architecture
./run.sh login     # authenticate once -- the only step that opens a browser
./run.sh doctor    # what is missing, and the one command that fixes it
./run.sh tui       # build and run
```

| Subcommand | What it does |
|---|---|
| `doctor` | Report what is missing, the fix for each, and whether the adapter is at the pinned commit |
| `setup` | Clone and install the adapter at the pinned commit |
| `login` | Authenticate once into the XDG state directory, mode 0600 |
| `build` · `clean` | Build or remove `tui/cwatui` |
| `test` | `gofmt`, `go vet`, `go test`, byte-compile, `checkdoc` |
| `smoke` | One live read through the contract |
| `tui` | Build and run the TUI |
| `emacs` | Print the Emacs setup snippet resolved for this host |

All front ends on one host resolve the auth file to the same XDG location, so
one login serves every one of them. Never copy that file to another host; each
host must run its own `./run.sh login`. By hand, without the script, name the
adapter's interpreter explicitly — do not execute the shim bare:

```bash
CWA_PY=/path/to/venv/bin/python
CWA_BIN=/path/to/venv/bin/cwa
mkdir -p ~/.local/state/botschaft && chmod 700 ~/.local/state/botschaft
"$CWA_BIN" auth login --force --auth-file ~/.local/state/botschaft/auth_data.json
chmod 600 ~/.local/state/botschaft/auth_data.json
./run.sh smoke  # expiry metadata alone does not prove server acceptance

"$CWA_PY" bin/cwaq projects
"$CWA_PY" bin/cwaq list --limit 50
"$CWA_PY" bin/cwaq search 'query' --project <project-id>

cd tui && go build -o cwatui . && CWA_PY="$CWA_PY" CWA_BIN="$CWA_BIN" ./cwatui
```

### Emacs

Put `lisp/` on your `load-path`. The package finds `bin/cwaq` on its own,
relative to its own file. `./run.sh emacs` prints this snippet with the paths
already resolved for the host you are on.

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
| List | `p` projects · `/` search · `enter` open · `j/k` move · `g/G` first/last · `y` URL · `o` browser · `r` reload · `q` quit |
| Projects | `j/k` move · `enter` select · `esc` back · `q` quit |
| Reading | `c` compose · scroll · `n/p` next/previous turn · `v` overview · `e` expand/fold · `/` find · `]/[` next/previous hit · `Y` copy turn · `g/G` first/last turn · `t` tool turns · `y` URL · `o` browser · `r` reload · `esc` back · `q` quit |
| Composing | multi-line input · `ctrl+e` `$EDITOR` · `ctrl+d` exact dry run · `ctrl+s` send · `ctrl+c` cancel in flight / quit when idle / stay put when unresolved · `esc` preserve draft and return |

## Warning

CWA is an unofficial adapter. The risk to your account is not zero. This
repository stands on it and inherits that risk — including while it only reads.

The `$EDITOR` handoff uses a mode `0600` plaintext temporary file in the private
XDG runtime directory when one is available, falling back to the system temporary
directory, and removes it when the editor callback completes. `$EDITOR` must remain
in the foreground until the file is saved and closed (for example, do not add
`emacsclient --no-wait`). An uncatchable `SIGKILL` can prevent cleanup and leave
the draft there.

Send receipts also exist only in memory. If the TUI itself is killed during an
uncertain send, restart it and read the canonical server history before sending
that draft again; the restarted process cannot know the lost receipt.
