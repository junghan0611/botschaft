# botschaft — AGENTS.md

## What this house is

**A window that brings a conversation living outside my harness to my own seat.**
What it brings is **access**, not the conversation. That distinction generates
every rule in this repository.

The counterpart on the other side is deliberately **not my harness** — no
identity prompt was ever installed there. It has search wired in, so it pulls in
outside sources; it never has to worry about session size; and it cannot code.
You pull that session out when a topic comes up. This tool **moves that meeting
into the terminal and Emacs.**

Hence the name `Botschaft`: message, and embassy. A window kept in someone else's
country, not a colony.

## The absolute rule — the server is canonical

**Never build a local conversation store.** This is not a preference; it is why
the house exists.

- Never call `snapshot` or `export`. Never build a cache database
- The moment a local file becomes canonical, **continuing a conversation becomes
  impossible in principle.** That is exactly how `conversations.json` export
  readers fail
- The UX of TUIs that keep sessions in sqlite (oterm and friends) is worth
  studying, but **their storage model is not.** They keep conversations they
  created; we query conversations someone else holds

The one exception is **configuration**, and even that needs a decision from the
repository owner — something like "the project you looked at last".

## The contract is canonical; backends are swapped

```
projects · list · search · read  (+ send)
```

- The front ends (Emacs, TUI) know **only these verbs.** They never call the
  adapter directly
- Paging, route selection and query timestamp normalization are handled **at
  the contract layer.** The front ends consume that stable envelope rather than
  reimplementing those server rules. That is the evidence the boundary is right.
  If a query-route trap leaks into a front end, the boundary is wrong
- **A Claude web backend is a TODO. Do not generalise the interface now.**
  Fixing a boundary before using it is how `gptel-backend` and `LlmProvider`
  failed (see the table in `README.md`). Settle it when a second product is
  actually attached

## `bin/cwaq` exists in order to be deleted

CWA 0.3.0's public CLI has no `list`, `search` or `projects` verb. So the shim
reaches one level down and hits the endpoints directly — **it depends on CWA
internals.**

- This is a debt, and it is left as a debt. The day upstream exposes the verbs,
  delete `cwaq`
- Therefore **do not grow the shim.** It carries exactly three verbs —
  `projects`, `list`, `search` — and **`read` is not one of them.** The fourth
  verb, `read`, belongs to the public `cwa messages`, and that is the only
  contract here carrying a `schema` number. Pulling reads down into the shim
  would throw that version marker away
- Upstream CWA moving can break this. When it breaks, do not patch — **measure
  again**
- The adapter commit is pinned in `run.sh` (`CWA_COMMIT`). Two hosts on two
  architectures have to agree on which upstream the measurements were taken
  against, so moving the pin means re-measuring, not just bumping a string

## How Emacs calls the shim

**Never execute it bare.** A `#!/usr/bin/env python3` shebang is no guarantee
that the interpreter it finds has the adapter installed. The TUI does not do this
either — it runs `$CWA_PY bin/cwaq …` explicitly. What Emacs owns is not an HTTP
implementation but three things: **a pinned shim, interpreter discovery, and a
stable JSON contract.**

## How to handle facts

This house stands on **undocumented server behaviour**. So:

- **Record the query and the time alongside any search count.** The index is
  live and the numbers move. A search count without its query and timestamp is
  an impression, not a fact
- **Never write down what appeared on screen as a measurement.** This is
  especially dangerous with paged data (an actual mistake on day one of this
  repository: five rows visible in a capture were counted and recorded; the real
  answer was 12)
- When a server fact is measured anew, leave it in `docs/chatgpt-protocol.md`
  with the date

## Publishing and safety

- CWA is an **unofficial adapter**. The risk to the account is not zero, and this
  house inherits it
- `auth_data.json` holds an access token and cookies. **Never commit it.** Its
  home is `$XDG_STATE_HOME/botschaft/`, falling back to
  `~/.local/state/botschaft/`, mode 0600 in a 0700 directory. All components on
  one machine resolve that one path, so one login serves every front end there
- **One auth file per host. Never copy it to another machine** — each runs its
  own `./run.sh login`. Sharing one credential across two egress IPs preceded a
  browser logout on 2026-09-10; the measurement, including what was ruled out,
  is in `docs/chatgpt-protocol.md`
- Conversation titles, bodies and project names are **personal data**. They do
  not go into documents, issues or commit messages. When recording a
  measurement, keep the numbers and the structure and drop the names
- No AI attribution in commit logs (`Generated with`, `Co-Authored-By`)
- **Interface strings, documentation and comments are all English.** Both front
  ends carry this

## Boundaries

| | Owner |
|---|---|
| The contract (four verbs) and the shim | **here** |
| The Emacs package | **here** — `lisp/`. The main line |
| The TUI | **here** — `tui/`. A verification harness, not the deliverable |
| Facts about the ChatGPT web protocol | **here** — `docs/chatgpt-protocol.md` |
| Bugs and features of CWA itself | upstream. Report them; do not fix them here |
| Installation and dependency declarations | the environment repository, once decided |
| Deciding on commits and pushes | the repository owner |
