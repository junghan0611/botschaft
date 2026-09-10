# The ChatGPT web backend — measured server behaviour

Measured against one account on 2026-09-10. **This document is this repository's
real contribution**: none of the five below is documented anywhere, each has to
be found by running into it, and other teams have missed them too.

## Endpoints

```
GET /backend-api/conversations?offset&limit&order=updated        every conversation
GET /backend-api/conversations/search?query=[&gizmo_id=]          search
GET /backend-api/gizmos/snorlax/sidebar?conversations_per_gizmo&limit   the project list
GET /backend-api/gizmos/<gizmo_id>/conversations?limit&cursor     one project's conversations
GET /backend-api/conversation/<id>                                one conversation (used by `cwa messages`)
```

## The five traps

### 1. The global list contains no project conversations at all

The two sets do not intersect. On the measured account:

| | Conversations |
|---|---|
| Three projects, combined | 262 |
| Outside any project (the entire global list) | 70 |

**79% lives outside the global list.** A viewer without a project axis cannot see
most of the account.

### 2. `gizmo_id` is ignored by the list route and honoured only by search

- `/conversations?...&gizmo_id=<id>` → **silently ignores** the parameter and
  returns the global list
- `/conversations/search?...&gizmo_id=<id>` → **honours it** (4 hits globally → 1
  when scoped, on one query)

The same parameter name is treated differently per route.

### 3. Search pages by cursor, and the first 30 hits are not the total

The reproduction query identified below returned 30 items plus `cursor="30"`.
Appending `&cursor=30` returned the remaining 9, for **39** results at
17:26 KST. **The end is `cursor == null`.** Dropping the cursor silently hides the
older matches — which is precisely the "find and reopen last month's conversation"
path this kind of tool exists for.

### 4. The `total` field on the global list is not a total

While pages remain it returns **`count + 1`**, and equals the real count only once
exhausted:

```
limit=28  → items 28, total 29
limit=50  → items 50, total 51
limit=100 → items 70, total 70     <- the real end
limit=101 → HTTP 422 (cap is 100)
```

**The termination condition is a short page, not `total`.** The project route uses
an opaque cursor instead (50 per page).

### 5. Time types differ per route, and titles are not one line

- Global list `update_time` = `"2026-09-10T05:27:04.545482Z"` (a UTC string)
- Search `update_time` = an epoch float

Reading the first as naive and the second as local puts **the same conversation
nine hours apart inside the same list.** Parse each as what it is, move both onto
one axis, and **keep the offset in the string.**

Titles also arrive with **trailing newlines and unassigned code points**
(measured: `"…\U0005FFFF\n"`). One newline pushed the list render down a row and
scrolled the header off screen. **Never assume a server string is one line.**

## Reproduction — measured again from the Emacs front end (2026-09-10, 17:2x KST)

The same account was asked again while the second front end (`lisp/botschaft.el`)
was attached. **Four query-route traps are observable from a front end, and all
four reproduced with no value out of line.**

| Fact | First measurement | Reproduction |
|---|---|---|
| Global list ∩ project conversations | 0 | **0** (global 70, one project 171) |
| `gizmo_id` honoured only by search | 4 global → 1 scoped | same query: **39** global → **1** scoped |
| First search page of 30 ≠ total | 30 → 39 | **39** following the cursor to the end (17:26 KST) |
| Tool turns split on `recipient` | 42 turns → 17 | **42 → 17** (25 with `recipient != "all"`) |
| Time types differ per route | nine hours apart | **9** conversations in both routes, **0** mismatches |

The global count (70) matches the first measurement. The 171 in one project is
part of the 262 measured across three that day, so it is not a direct comparison.
Search counts are for one Korean-language query (the word for "Emacs") between 17:26 and 17:28 KST — the index
is live, so measuring again will give different numbers.

The query shim already carries the paging, route-selection and timestamp-
normalization rules. The Emacs renderer applies separate display-only title
hygiene. That the query-route traps did not require a second paging or routing
implementation once a second front end existed is the second piece of evidence
that the boundary is drawn in the right place.

## One auth file, two hosts — do not (2026-09-10, unproven)

The auth file was copied to a second machine so reading would work from both. It
did work: the same token answered from a home connection and from a cloud VM.
About twenty-five minutes after the second host started reading, the account's
**browser** session was logged out.

This is recorded as an observation, not a proven cause. What was measured:

| Question | Answer |
|---|---|
| Do reads rotate or rewrite the auth file? | **No** — same checksum and mtime before and after |
| Did the adapter's login take over an existing browser profile? | **No** — it keeps its own persistent profile |
| Were all sessions for the account invalidated? | **No** — the extracted token still answered from both hosts afterwards |
| Had the token expired? | **No** — the access token had ten days left and the session three months |

So nothing on this side rotated a credential, and the logout was selective: the
browser session died while the extracted token lived. The only new condition was
the second egress IP, in a different network and a different region.

**The operational rule that follows: one auth file per host.** Each machine runs
its own `./run.sh login`. Copying credentials between hosts is what created the
condition, it saves only one browser interaction, and the failure it invites is
one that logs *you* out while the tool keeps working — so you find out from the
browser, not from the tool.

## Reading and writing go through different doors

Measured with `cwa doctor`: once authentication is done, listing, searching and
reading all succeed while `bridge.*` is FAIL at the same moment. **Reading needs
no browser extension.** Every remaining FAIL belongs to the write path:

```
install.native_host_manifest · install.native_host_registration
bridge.available · bridge.extension_connected · runtime.health
```

They close with `cwa browser-native install` plus loading the unpacked extension
into Chrome.

### There are two write transports, and only one needs that extension

Measured 2026-09-10 with `cwa status` and `cwa capabilities`, both of which are
read-only and perform no write:

```
--transport browser-owned        ready: false   BROWSER_NATIVE_BRIDGE_UNAVAILABLE
--transport browserless-request  ready: true    BROWSERLESS_REQUEST_READY_SENTINEL_PREFLIGHT_PENDING
```

**The browserless transport is already ready on the same session token reading
uses.** No extension, no `debugger` permission, no resident Chrome. But the two
transports do not carry the same capabilities:

| Capability | browser-owned | browserless-request |
|---|---|---|
| `text_turns` · `continuation` · `streaming` · `new_chat` | AVAILABLE | AVAILABLE |
| `canonical_readback` · `conversation_attach` · `conversation_read` | AVAILABLE | AVAILABLE |
| `web_search` | AVAILABLE | **UNKNOWN** |
| `model_selection` · `reasoning_selection` | AVAILABLE | **UNKNOWN** |
| `files` | AVAILABLE | UNKNOWN |
| `images` · `multimodal_continuation` | AVAILABLE | **UNIMPLEMENTED** |
| `temporary_chat` | AVAILABLE | UNKNOWN |

So the transport choice is not only a question of risk. `web_search` is the
capability that makes the counterpart on the other side worth talking to at all,
and it is unverified on the transport that needs no extension. **UNKNOWN here
means unmeasured, not absent** — it can only be settled by sending one turn.

**Order matters when starting on writes:** CWA upstream #79 (open) promotes a
**new** conversation's id to `WEB:<uuid>`, which the canonical read then rejects.
**Continue an existing conversation first, create new ones later.**

## What a conversation actually weighs

Measured 2026-09-10 across the eight most recently updated conversations, human
turns only (`recipient == "all"` and not a tool role):

| | |
|---|---|
| Median conversation | ~5,100 characters ≈ 51 wrapped lines at 100 columns |
| Largest seen | 31,535 characters ≈ 315 lines |
| Median single turn | 104–539 characters |
| Largest single turn | 9,986 characters ≈ 99 lines — **twice a 50-row pane** |
| Worst tool ratio | 65 turns of which **5** are human-readable |
| Fenced code blocks in one conversation | 43 |

These numbers set the bar for any reading surface: plain viewport scrolling is
not enough to navigate a 315-line conversation, one turn can exceed the screen
twice over, and a conversation can be 92% tool traffic.

## For reference — where other implementations stopped

`Octo-Lex/ChatGPT-Web2API` (2026-09-01) calls
`sidebar?conversations_per_gizmo=5` at
`src/chatgpt_web2api/backend_client.py:427` and then discards the response's
`conversations` in its mapper. `/gizmos/<id>/conversations` appears nowhere in
that repository, nor in the endpoint table of their own
`docs/protocol-reference.md`. An active, independent reverse-engineering team
missed this same slot.
