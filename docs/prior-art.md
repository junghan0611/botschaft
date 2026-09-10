# Prior art — does it already exist, and is there anything to learn from it

Surveyed 2026-09-10 with `gh api` and web search. Judgement only: nothing was
installed, edited or submitted upstream.

> **Do not buy this twice.** The tables below are the result of two survey
> rounds. Before researching a candidate again, check whether its name is
> already here. Every row is dated, so staleness is decided here too.

The two rounds asked different questions. Round 1: **"does it already exist?"**
(the transport axis). Round 2: **"is there anything to learn?"** (the front-end
axis). The answers were, respectively, (c) no, and: not one of them clears
T3 and T4 at the same time.

---

# Round 1 — does it already exist?

## Candidates × gates

| Candidate | G1 live session | G2 continue | G3 server-side search | G4 projects | G5 server canonical | G6 Emacs | Verdict |
|---|---|---|---|---|---|---|---|
| **Octo-Lex/ChatGPT-Web2API** | ✅ CDP, logged-in browser | ✅ reattaches by `conversation_id` | ❌ no search tool (no `search_*` in the 16-tool list; measured) | ❌ has `list_projects`, but `list_conversations` only hits `/backend-api/conversations?offset&limit&order` (`backend_client.py:460`). It cannot see project conversations — precisely trap 1 in `chatgpt-protocol.md` | ✅ no local store | ❌ | **fails G3 and G4** |
| **shoyu-ramen/codex-chats-mcp** | ✅ session cookie/token | ✅ has `get_conversation` | ▲ has `search_conversations`, but it is a client-side title substring match (`codex_chats_mcp.py:306`), not server search | ❌ zero occurrences of `gizmo` in the whole codebase (measured by grep) — no notion of projects at all | ✅ | ❌ | **fails G4, G3 weak** |
| **kabxx/portal** | ✅ drives Chromium, login persists | ▲ claims "conversation history" but documents no list/search verb — the main action is starting a new thread via `/thread agent chatgpt` | ❌ not mentioned | ❌ not mentioned | ✅ (the browser is the session) | ❌ | **different genre** — an agent work bridge, not a history viewer |
| **jackwener/OpenCLI** | ✅ | ▲ the `history` verb is **currently broken** — issue #2435 open, PR #2436 unstable | ? | ? | ✅ | ❌ | **reading itself is broken** |
| **imoonkey/openweb** | ✅ | ? | ? | ? | ✅ | ❌ | the ChatGPT path was last touched 2026-04-24 — stalled |
| **kymuco/chatgpt-web-adapter** (what we use) | ✅ | ✅ `attach_conversation` | ❌ the SDK has no list/search verb at all | ❌ same reason | ✅ | — | this is exactly why we cover it with the `cwaq` shim |
| **h-ohsaki/chatgpt-el** (fork: eval-exec) | ✅ CDP via Puppeteer, no API key | ▲ `C-u C-u C-c q` only queries repeatedly; no conversation list, no reattach by id | ❌ | ❌ | ✅ | ✅ Emacs itself | **fails G3 and G4; has no notion of search or listing. A pure one-turn query/insert tool** |
| **xenodium/chatgpt-shell, karthink/gptel, joshcho/ChatGPT.el, emacs-openai/chatgpt** | ❌ all require an **official API key** (`gptel` README: an OpenAI API key is required) | — | — | — | — | ✅ | **all eliminated at G1** — the mainstream of the Emacs LLM ecosystem is API-key clients, not web-session viewers |
| **atondwal/chatgpt_broswer (CCSM)** | ❌ reads a `conversations.json` export (README: "Export data → conversations.json") | ❌ impossible by definition — an export is a frozen snapshot | local grep only | may reflect a project field if the export has one, but not from the server | ❌ **fails G5 — this is exactly the "export reader" the gate warns about** | ❌ | **classification: export reader, eliminated at G5. A good-looking TUI that structurally cannot continue a conversation** |

## Conclusion — (c) nobody clears every gate

The closest is **Octo-Lex/ChatGPT-Web2API** (last commit 2026-09-01, ★47, an MCP
server with 16 tools, CDP-based — clears G1, G2 and G5). And yet it walks into
the same two traps measured here:

- **No G3** — the 16-tool list in its README has no `search_*` verb.
- **Half of G4** — it can enumerate projects with `list_projects`, but the only
  endpoint `list_conversations` (→ `get_conversations`) hits is
  `GET /backend-api/conversations?offset&limit&order` at `backend_client.py:460`.
  Reading a project goes as far as `/backend-api/gizmos/{id}` (project *detail*,
  `backend_client.py:696`); no code path hits `/backend-api/gizmos/{id}/conversations`
  (the project's *conversation list*). So, exactly as measured in
  `chatgpt-protocol.md`, **79% of the account is invisible in that tool too.**
  Two implementations hitting the same server trap independently is cross-evidence
  that the trap is real.

The runner-up, `shoyu-ramen/codex-chats-mcp`, does have `search_conversations`,
but it is **a client-side title substring match over an already-fetched list**
(`codex_chats_mcp.py:306`), not server search, and the string `gizmo` appears
nowhere in its code, so it has no notion of projects — it satisfies neither G3
(server-side search) nor G4.

The Emacs side (G6) splits in two: the mainstream packages (`gptel`,
`chatgpt-shell`, `ChatGPT.el`) all use an official API key and are eliminated at
G1, while the one package that runs without a key, `h-ohsaki/chatgpt-el`, is a
one-turn query/insert tool with no listing, search or projects. **Only one Emacs
package clears G1 at all, and it satisfies none of G2, G3 or G4** — the slot for
the Emacs package being built here is genuinely empty.

## Recommendation

**(c) — nothing clears every gate.** The hunch that this is worth building had a
basis. One thing worth borrowing, though: the 16-tool MCP surface on the Octo-Lex
side (particularly `list_projects`, `update_project_instructions` and
`list_memories`) is a useful reference list of surfaces for when the write door
opens. Its miss of the project *conversation list* endpoint stays on the record
as the cautionary half.

---

# Round 2 — is there anything to learn?

Surveyed 2026-09-10, ~16:00 KST. Round 1 is not re-derived here — it asked
whether this already exists and ended at (c). This round asks what to learn.

## Candidates × T1–T5

| Candidate | T1 alive | T2 history browser | T3 own multi-provider boundary (file:line) | T4 multi-product web session | T5 what to borrow |
|---|---|---|---|---|---|
| **karthink/gptel** (Emacs) | ✅ 2026-09-09, ★3521, 181 open issues (active) | ▲ save/load to files (`gptel-mode`) — no list or search UI; `find-file` is the list | ✅ the `gptel-backend` base struct — **`gptel-request.el:919-926`**. Slots: `name host header protocol stream endpoint key models url request-params curl-args coding-system` | ❌ all API keys (the `key` slot plus a `header` lambda emitting `Authorization: Bearer`) | Less the struct than the design choice of a **transport-only contract** — conversation identity, search and projects are simply not in it. Not directly usable here (see conclusion) |
| **ggozad/oterm** | ✅ 2026-09-02, ★2430, 4 open issues (light) | ✅ many persistent sessions in sqlite, resumable | ▲ the boundary lives **in pydantic-ai, not in oterm** — oterm writes no provider trait of its own | ❌ every pydantic-ai provider is an API key or a local server (Ollama included) | The **UI layout and the session sqlite model** are worth borrowing. The provider abstraction belongs to someone else and is not a port target for a web-session backend |
| **ducks/llm-tui** (Rust) | ✅ 2026-07-28, ★10 (small), 31 open issues (early rather than active) | ✅ `db.rs` plus "project-based session management" (README) | ✅ **its own** `LlmProvider` trait — **`src/provider/mod.rs:83`**. Methods: `name` `is_available` `chat` `continue_with_tools` `list_models`. History sits outside the trait, in `db.rs` | ❌ `provider/{bedrock,claude,gemini,ollama,openai}.rs` are all key or local | The cleanest precedent for **where to cut**: the provider trait is one-turn request/response only, and storage and querying live entirely outside it. The same shape as our split between `cwaq` (three query verbs) and `cwa messages` (read) — independently arrived at |
| **darrenburns/elia** | ❌ **last push 2024-10-10.** Two years stalled. 25 open issues | it had one (sqlite, `EliaChatModel`, ChatGPT export **import**) | it had one (`EliaChatModel`) | ❌ | **eliminated at T1 — a pretty README and ★2480 on a dead repository.** Kept on the record as the concrete instance of "a pretty README on a dead repository is not a candidate" |
| **shoyu-ramen/codex-chats-mcp** / **Octo-Lex/ChatGPT-Web2API** (carried from round 1) | ✅ | ✅ (within a single product) | ❌ each handles one product only, so there is no "several backends" boundary to speak of | ✅ each for its own product (ChatGPT) — **not multi-product, so out of scope for T4** | Round 1's conclusion stands. Cited here only as the sample showing that single-product web sessions exist while multi-product ones do not |

## T3 — the real question: does that cut hold for a web-session backend?

All three boundaries (`gptel-backend`, pydantic-ai's `Model`, `LlmProvider`) cut
at **"a provider is a function that sends one prompt and receives one response."**
Every slot and method is of the `key` / `header` / `endpoint` / `chat()` family.
**`list_conversations`, `search` and `gizmo`/project appear in none of the three**
— and where something like them exists (`ducks/llm-tui`), it lives outside the
trait, in the DB layer.

**That cut does not fit this problem.** The read-only four verbs here
(`projects · list · search · read`, see `README.md`) are not "send a prompt" at
all; they are "query the conversation graph the server already holds." Fitting a
web-session backend into any of the three provider interfaces would fill the
`chat()` position (the continue/new door), but **list, search and projects are
verbs those interfaces never speak, so there is no position for them.** That
calls for an extension rather than a port, and the shape of that extension looks
closer to taking `ducks/llm-tui`'s "trait = one turn, history = separate layer"
split and going **one layer further** — putting `list_conversations` and `search`
into the provider trait itself.

## T4 — a multi-product logged-in-session abstraction: nobody has one

All six candidates fail T4, and they do not fail for six different reasons.
**There is only the dichotomy of API key versus single-product web session; no
attempt to treat a logged-in web session as a provider and bind several products
behind it was found at all.** All three of `gptel-backend`, pydantic-ai and
`LlmProvider` presuppose that a backend *is* a way of calling an API, so there is
no room in the type for the idea that a session cookie is the authentication —
the `key` slot already holds that position. That is the real finding: not an
empty slot, but **a slot the existing abstractions structurally closed off.**

## The next door (Claude side) — plausibility only, one paragraph

`cyber-wojtek/Claude-API` (pip `claude-webapi`, last push 2026-05-28, ★14) looks
like a plausible counterpart to CWA: it authenticates by reusing a `sessionKey`
cookie (copied from devtools) or by exchanging a Google login code, its
`ChatSession` threads multi-turn automatically, and it has **session
serialisation and restore** (the equivalent of G2). The older
`st1vms/unofficial-claude-api` (last push 2026-03-16, ★201, Selenium-based
session harvesting) is alive too. Neither README shows list, search or project
verbs — likely the same hole as CWA, but this was a plausibility check, not a
verification. Neither was examined closely.

## Conclusion

**No tool clears T3 and T4 at once.** Only `gptel` and `ducks/llm-tui` clear T3
(a multi-provider boundary of their own) — `oterm` delegates its boundary to
pydantic-ai and has none — and both of those exclude web sessions in the
definition of the boundary itself, with the key slot part of the contract. That
does not mean there is nothing to learn: **"the provider trait carries only
one-turn request/response, while history, search and projects live in a layer
outside it"** — the cut in `ducks/llm-tui` at `src/provider/mod.rs:83` — is
useful cross-evidence that this architecture (a transport layer of `cwaq`/`cwa`
versus four verbs) arrived independently at the same shape.

If one TUI is to be studied, it is **`ggozad/oterm`** (2026-09-02, ★2430): its
layout, and its model of keeping several persistent sessions and resuming them,
is the closest thing to the `completing-read` resume flow of the Emacs package
here. Its provider abstraction, however, belongs to someone else's library
(pydantic-ai) and is a reference, not a port target.
