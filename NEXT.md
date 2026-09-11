# NEXT — botschaft

The issue tracker owns the roadmap; this file names the next executable move.

# RAIL — 현재 좌표

- [x] **1. Read contract and query shim** — projects, list, search, canonical read
- [x] **2. TUI reading core** — navigation, overview, folding, find and copy
- [x] **3. Emacs read commands** — projects, search, list and conversation buffers
- [ ] **4. TUI daily-use polish and Emacs packaging** ← CURRENT: reading is the stem again
- [ ] **5. Continuation write (#2)** ← PAUSED: open the ChatGPT URL in the browser; no live send
- [ ] **6. New conversations and Claude backend (#3)** ← PAUSED: write evidence first

Current position: 1–3 complete → 4 daily-use reading ← CURRENT → 5 write paused → 6 paused.

# NOW

- **Current:** Adapter pin is upstream `e7f041a` again. Do not run the soak fork as the live adapter. Compose/send code may stay in the TUI; do not use it.
- **Next:** (1) remaining TUI reading quality that still needs a seated terminal — long conversations, fence-heavy threads, `Y` clipboard. (2) Emacs package discovery so `lisp/` works as an installed package, not only from a checkout (issue **#5**).
- **Blocker:** none for reading. Write is parked on purpose: browserless hits Sentinel; browser-owned preflight 404s `CANONICAL_READ_NOT_VISIBLE` while `cwa messages` works. Same 404 on pin and fork.
- **Verify:** `./run.sh doctor` reports pin `e7f041a`; `./run.sh smoke` passes; `./run.sh test` clean; Emacs commands still open a live conversation.
- **Read:** `README.md`, `AGENTS.md`, `docs/chatgpt-protocol.md`, issue **#6** (write parking), **#5** (packaging).
- **Do not touch:** no live `cwa send`; no soak-fork pin; no kymuco PR; no new ChatGPT conversation; no `snapshot`/`export`/cache; no fourth `cwaq` verb; no Emacs write; no ChatGPT-web Andenken axis or background conversation sync — use the server search.

# RECENT

- **2026-09-11:** Keep recall server-side: `cwaq search` already pages ChatGPT's live search for the TUI and Emacs. The proposed Andenken derivative axis and async local corpus sync are rejected; no local conversation projection.
- **2026-09-11:** Forced login repaired reads. First send died on `--profile` (exit 2). Soak fork omitted profile and forwarded attached model; browserless then died on Sentinel (exit 3, no mutation).
- Browser-native host + Edge extension connected. `browser-owned` send preflight 404 on three conversations; `messages` ok. Fork did not cause it (`e7f041a` and `4374dcc` same 404).
- Writes continue in the ChatGPT web UI via conversation URL. Issue **#6** holds the write investigation.

# LEDGER

- **#6** — write-path investigation (profile, model slug, Sentinel, credential planes). Parked.
- **#2** — TUI continuation implementation; live turn postponed.
- **#5** — package discovery / installed Emacs package.
- **#3** — Claude backend, after ChatGPT continuation is useful.
- **#4** — delete `cwaq` when upstream grows list/search/projects.
- Soak fork `junghan0611/chatgpt-web-adapter` `cli/optional-send-profile` stays for a possible kymuco PR ~2026-09-25; it is not the live pin.
