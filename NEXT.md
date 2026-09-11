# NEXT — botschaft

The issue tracker owns the roadmap; this file names the next executable move.

# RAIL — current position

- [x] **1. Read contract and query shim** — projects, list, search, canonical read
- [x] **2. TUI reading surface** — navigation, overview, folding, find and copy
- [x] **3. Emacs read surface** — projects, search, list and conversation buffers
- [ ] **4. Existing-conversation continuation** ← CURRENT: repair auth, then run one controlled live turn
- [ ] **5. New conversations and Claude backend** ← PAUSED: continuation evidence comes first

Current position: 1–3 complete → 4 implemented and awaiting live authentication → 5 paused.

# NOW

- **Current:** The TUI compose/stream/readback loop is implemented and reviewed. No real account write has been made.
- **Next:** (1) run `./run.sh login` → (2) require `./run.sh smoke` to pass → (3) obtain one owner-designated existing conversation ID and exact prompt → (4) send once and inspect canonical readback → (5) record profile and web-search behavior in `docs/chatgpt-protocol.md`.
- **Blocker:** At 09:39 KST on 2026-09-11, both query routes returned HTTP 401 `token_expired` despite a future reported expiry; `cwa auth refresh` returned 200 but did not repair the reads.
- **Verify:** smoke passes; the canonical history contains the submitted user turn and a later assistant turn; no duplicate send occurs; `web_search` and profile behavior are recorded with query and time.
- **Read:** `README.md` for the surface, `AGENTS.md` for house rules, `docs/chatgpt-protocol.md` for measured traps, and issue **#2** for RAIL 4.
- **Do not touch:** no send before the owner names the conversation and prompt; no new-conversation path; no Emacs write work; no `snapshot`, `export`, cache, or fourth `cwaq` verb.

# RECENT

- **2026-09-11:** Added existing-conversation compose, exact dry-run argv, streaming, cancellation, canonical readback and in-memory draft/receipt handling.
- The review loop closed dash-prefixed argv false success, stale exit-0 reconciliation, canonical whitespace normalization, editor tempfile privacy/cleanup, pipe cleanup, and documentation overstatement.
- Automated verification passes: `./run.sh test`, `go test -race ./...`, `go test -count=100 ./...`, `git diff --check`, and `bash -n run.sh`.
- Auth metadata proved weaker than a live read: `./run.sh smoke` now reports the real error and explicit login uses CWA `--force`.

# LEDGER

- **#2** — current write path and first controlled continuation.
- **#3** — Claude backend, only after the ChatGPT continuation is useful in daily use.
- **#4** — delete `cwaq` verbs when upstream publishes equivalent list/search/project commands.
- **#5** — package discovery; the current relative shim lookup works from a checkout, not an installed package.
- Queries remain synchronous and conversation Markdown remains raw text; neither blocks the current rail.
