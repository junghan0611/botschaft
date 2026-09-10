# NEXT — botschaft

```
RAIL  1 read contract ✅ → 2 TUI verification ✅ → 3 Emacs read ✅ → 4 write (#2) → 5 Claude backend (#3)
```

The issue tracker carries the shape of the work; this file carries the next
concrete move. Doors: **#2** write path · **#3** Claude backend · **#4** upstream
verbs · **#5** packaging. **#1** is the roadmap that ties them together.

## NOW — reading works; the TUI has to become a surface you can talk in (#2)

`lisp/botschaft.el` runs against a live account once `CWA_PY` and `CWA_BIN`
are set (use `./run.sh emacs` to print them); the auth path itself is zero-config:

- `botschaft-projects` — pick a project and hold it as the scope (`C-u` clears)
- `botschaft-search` — query → candidates (snippet as annotation) → open
- `botschaft-open` — list the scope's conversations → render into a `special-mode` buffer

One `completing-read` was enough. No `tabulated-list-mode`, no new UI framework,
built-ins only (`seq`, `subr-x`, `browse-url`, `json-parse-buffer`).
Byte-compile, `checkdoc`, `gofmt` and `go vet` are all clean.

**RAIL 4.1 is done** (#2): `n`/`p` with a turn index, `v` for a collapsed
human-turns overview, `e` to expand a turn folded at 20 lines, `/` with `]`/`[`
to find within a conversation, `Y` to copy the turn under the cursor, and fences
left unwrapped. 13 headless tests; `gofmt` and `go vet` silent.

**Five of the six were verified headlessly; `Y` was not** — the clipboard needs
`xclip` or `wl-copy` and a seated human. Live scroll on a ~315-line conversation
and the 43-fence conversation in a real terminal are also eye-only. **That is the
next thing to do, and it needs a keyboard, not a decision.**

**The next implementable step is RAIL 4.2 in #2: a compose surface** — a
multi-line composer with `$EDITOR` handoff, a draft that survives leaving the
conversation, and a dry run that prints the exact `cwa send` invocation and sends
nothing. Still zero writes.

One premise changed: there are **two** write transports, and `browserless-request`
is already `ready: true` on the same session token reading uses — no extension, no
`debugger` permission. What is still the owner's call is *which* transport, because
`web_search` is UNKNOWN on that one and AVAILABLE on the other. UNKNOWN means
unmeasured; one sent turn settles it.

## What the last session settled

- The four front-end-observable query-route traps in
  `docs/chatgpt-protocol.md` were measured again from the Emacs side. **Nothing
  out of line.** The Reproduction section is that table
- The repository went public. Every document, comment and docstring is English.
  Two comments in `tui/` had quoted live conversation titles as evidence; they
  now cite the code point alone
- The auth file resolves through XDG in all three components, so one login serves
  every front end and no personal path is baked into the source
- Three rules in `AGENTS.md` were wrong and are corrected: the shim carries three
  verbs and not four, the auth location was a decision stated as a fact, and the
  English-only rule lived only in `README.md`

## Loose ends (not blocking)

- **Queries are synchronous, so Emacs blocks for 2–3s.** Little is gained by going
  async, since candidates cannot be shown before the data arrives. Writes are
  where it first earns its keep — noted in #2
- **`botschaft-open` refetches the list every time.** Not caching is the house
  rule, so this is intended. Lower `botschaft-list-limit` if it drags
- The conversation buffer leaves markdown as raw text. Whether to render it with
  `markdown-mode` is undecided
- The shim is found relative to `botschaft.el`, which works from a checkout but
  not from an installed package. That has to be solved before a MELPA recipe
  makes sense — tracked in #5
- Nothing has been pushed since RAIL 3 began

## Where to read

- `README.md` — the contract, why existing abstractions cannot hold it, setup and keys
- `AGENTS.md` — the rules of this house. **The server is canonical** is the core one
- `docs/chatgpt-protocol.md` — five measured server facts plus the second front end's reproduction
- `docs/prior-art.md` — two survey rounds; do not buy this research twice
- `lisp/botschaft.el` — the Commentary explains the shape before the code does
