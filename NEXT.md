# NEXT — botschaft

```
RAIL  1 read contract ✅ → 2 TUI verification ✅ → 3 Emacs read ✅ → 4 [now] write (send) → 5 Claude backend
```

## NOW — reading is done. The next door is the owner's to open

`lisp/botschaft.el` runs against a live account. All three commands work:

- `botschaft-projects` — pick a project and hold it as the scope (`C-u` clears)
- `botschaft-search` — query → candidates (snippet as annotation) → open
- `botschaft-open` — list the scope's conversations → render into a `special-mode` buffer

One `completing-read` was enough. No `tabulated-list-mode`, no new UI framework,
built-ins only (`seq`, `subr-x`, `browse-url`, `json-parse-buffer`).
Byte-compile and `checkdoc` are both clean.
The **Reproduction** section of `docs/chatgpt-protocol.md` is those four traps
measured again from this second front end — nothing out of line.

**Do not start RAIL 4 (writes) before the owner opens that door.**

### When writes open (RAIL 4)

- `cwa send --conversation <id>` is the continue path. Wiring is one command plus
  loading the Chrome extension once (`cwa browser-native install` → `extension-dir`)
- Order matters: **continue an existing conversation first, create new ones later**
  (CWA #79 — a new conversation's id is promoted to `WEB:<uuid>`, which the
  canonical read then rejects)
- The read commands are synchronous (2–3s per query, ~3s per read). Writing waits
  on a different order of magnitude, so **that is where async first earns its
  keep.** There was no reason to make reads async ahead of it

### After that (RAIL 5)

- Claude web. Candidate counterpart: `cyber-wojtek/Claude-API` (2026-05-28, ★14)
- **Generalise the interface then. Not before**

## Open decisions (owner)

- **Whether to open the write door** — start RAIL 4 or not
- The repository is public as of 2026-09-10. Every document is English; keep any
  new one that way, and keep conversation titles and project names out
- Whether to declare dependencies in the environment repository. The Emacs side
  needs nothing new — built-ins only. Just Python ≥3.10 and system `curl`
- A feature request to send upstream to CWA: expose `list` / `search` /
  `projects`. When they land, delete `bin/cwaq` and a few strings in both front ends
- Packaging: MELPA recipe, or leave it as a `load-path` package
- Nothing has been pushed since the RAIL 3 work

## Loose ends (not blocking)

- **Queries are synchronous, so Emacs blocks for 2–3s.** Not much is gained by
  going async, since the candidates cannot be shown before the data arrives.
  Revisit under writes
- **`botschaft-open` refetches the list every time.** Not caching is the house
  rule, so this is intended. Lower `botschaft-list-limit` if it drags
- The conversation buffer leaves markdown as raw text. Whether to render it with
  `markdown-mode` is undecided

## Where to read

- `README.md` — the contract, why existing abstractions cannot hold it, Emacs setup and keys
- `AGENTS.md` — the rules of this house. **The server is canonical** is the core one
- `docs/chatgpt-protocol.md` — five measured server facts plus the second front end's reproduction
- `lisp/botschaft.el` — the Commentary explains the shape before the code does
