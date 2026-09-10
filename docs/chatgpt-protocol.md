# ChatGPT 웹 백엔드 — 서버 실측

한 계정에서 2026-09-10 측정. **이 문서가 이 리포의 실제 기여다** — 아래 다섯은
문서에 없고, 코드로 부딪혀야 알 수 있으며, 다른 팀들도 빠졌다.

## 엔드포인트

```
GET /backend-api/conversations?offset&limit&order=updated        전역 목록
GET /backend-api/conversations/search?query=[&gizmo_id=]          검색
GET /backend-api/gizmos/snorlax/sidebar?conversations_per_gizmo&limit   프로젝트 목록
GET /backend-api/gizmos/<gizmo_id>/conversations?limit&cursor     한 프로젝트의 대화
GET /backend-api/conversation/<id>                                한 대화 (CWA `messages`가 씀)
```

## 함정 다섯

### 1. 전역 목록에 프로젝트 대화가 하나도 없다

두 집합의 교집합이 **0**이다. 측정한 계정에서:

| | 대화 수 |
|---|---|
| 프로젝트 3개 합 | 262 |
| 프로젝트 밖 (전역 목록 전량) | 70 |

**79%가 전역 목록 밖에 있다.** 프로젝트 축이 없는 뷰어는 대화 대부분을 못 본다.

### 2. `gizmo_id`는 목록에서 무시되고 검색에서만 먹는다

- `/conversations?...&gizmo_id=<id>` → 파라미터를 **조용히 무시**하고 전역 목록을 돌려준다
- `/conversations/search?...&gizmo_id=<id>` → **존중한다** (한 질의에서 전역 4건 → 스코프 1건)

같은 이름의 파라미터가 route마다 다르게 취급된다.

### 3. 검색은 커서로 페이징하고, 첫 페이지 30건은 총계가 아니다

`?query=…` → 30 items + `cursor="30"`. `&cursor=30`을 붙이면 나머지가 온다.
한 질의에서 30 → **39**, 다른 질의에서는 30 → **101**이었다.
**끝은 `cursor == null`로 안다.** 커서를 버리면 오래된 hit가 조용히 사라지는데,
하필 "한 달 전 대화를 찾아 다시 연다"가 이런 도구의 존재 이유다.

### 4. 전역 목록의 `total` 필드는 총계가 아니다

남은 페이지가 있으면 **`개수+1`**을 돌려주고, 소진했을 때만 실제 개수와 같아진다:

```
limit=28  → items 28, total 29
limit=50  → items 50, total 51
limit=100 → items 70, total 70     ← 여기가 진짜 끝
limit=101 → HTTP 422 (상한 100)
```

**종료 조건은 `total`이 아니라 짧은 페이지다.** 프로젝트 route는 opaque cursor를 쓴다(50/page).

### 5. 시간 타입이 route마다 다르고, 제목은 한 줄이 아니다

- 전역 목록 `update_time` = `"2026-09-10T05:27:04.545482Z"` (UTC 문자열)
- 검색 `update_time` = epoch float

앞을 naive로 읽고 뒤를 local로 읽으면 **같은 대화가 같은 목록 안에서 9시간 어긋난다.**
각각을 제 타입으로 파싱해 한 축으로 옮기고 **offset을 문자열에 남겨야** 한다.

그리고 제목에 **후행 개행과 미할당 코드포인트**가 들어온다(실측 `"…\U0005FFFF\n"`).
개행 하나가 목록 렌더를 한 줄 밀어 헤더를 화면 밖으로 보냈다.
**서버 문자열이 한 줄이라고 가정하면 안 된다.**

## 재현 — Emacs 프런트에서 다시 잰 값 (2026-09-10 17:2x KST)

두 번째 프런트(`lisp/botschaft.el`)를 붙이며 같은 계정에 다시 물었다.
**다섯 함정 중 프런트에서 관측 가능한 넷이 전부 재현됐고, 어긋난 값은 없다.**

| 사실 | 최초 실측 | 재현 |
|---|---|---|
| 전역 목록 ∩ 프로젝트 대화 | 0 | **0** (전역 70, 프로젝트 한 개 171) |
| `gizmo_id` — 검색에서만 먹는다 | 전역 4 → 스코프 1 | 같은 질의 전역 **39** → 스코프 **1** |
| 검색 첫 페이지 30 ≠ 총계 | 30 → 39 | 커서 끝까지 **39** (17:26 KST) |
| 도구 turn 은 `recipient` 로 가른다 | 42 turn → 17 | **42 → 17** (`recipient != "all"` 25건) |
| route 마다 다른 시간 타입 | 9시간 어긋남 | 두 route 에 함께 나온 **9건, 불일치 0** |

전역 목록 수(70)는 최초 실측과 같고, 프로젝트 한 개의 171 은 그날 잰
3개 합 262 의 일부라 직접 비교 대상이 아니다.
검색 수치는 질의 `"이맥스"` · 2026-09-10 17:26–17:28 KST 기준이다 —
인덱스가 살아 있어 다음에 재면 달라진다.

넷은 계약(shim) 층에서 이미 잡혀 있었고 **elisp 은 한 줄도 이 사실을 다시 다루지
않는다.** 프런트가 두 개가 되어도 함정이 새지 않았다는 것이 경계가 옳다는 두 번째 증거다.

## 읽기와 쓰기는 다른 문을 쓴다

`cwa doctor` 실측: 인증만 마치면 목록·검색·읽기가 전부 성공하는데 같은 시점에
`bridge.*`는 FAIL이다. **읽기에 브라우저 확장은 필요 없다.** 남은 FAIL은 전부 쓰기 배선이다:

```
install.native_host_manifest · install.native_host_registration
bridge.available · bridge.extension_connected · runtime.health
```

`cwa browser-native install` + Chrome에 압축해제 확장 로드로 닫힌다.

**쓰기 착수 순서 주의:** CWA upstream #79(미해결)가 **새 대화** id를 `WEB:<uuid>`로 승격해
canonical read가 거부한다. **기존 대화 이어쓰기를 먼저, 새 대화를 나중에** 해야 한다.

## 참고 — 다른 구현도 빠진 자리

`Octo-Lex/ChatGPT-Web2API`(2026-09-01)는 `src/chatgpt_web2api/backend_client.py:427`에서
`sidebar?conversations_per_gizmo=5`를 부르면서 응답의 `conversations`를 매퍼에서 버린다.
`/gizmos/<id>/conversations`는 그 리포 어디에도, 그들의 `docs/protocol-reference.md`
엔드포인트 표에도 없다. 활발한 독립 리버스엔지니어링 팀도 이 자리를 놓쳤다.
