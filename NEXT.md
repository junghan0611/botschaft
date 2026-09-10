# NEXT — botschaft

```
RAIL  1 읽기 계약 ✅ → 2 TUI 검수 ✅ → 3 Emacs 읽기 ✅ → 4 [지금] 쓰기(send) → 5 Claude 백엔드
```

## NOW — 읽기는 끝났다. 다음 문은 GLG가 연다

`lisp/botschaft.el` 이 라이브 계정에서 돈다. 세 명령 전부 동작:

- `botschaft-projects` — 프로젝트를 골라 스코프로 쥔다 (`C-u` 로 놓는다)
- `botschaft-search` — 질의 → 후보(annotation 이 snippet) → 열기
- `botschaft-open` — 스코프의 대화 목록 → `special-mode` 버퍼로 렌더

`completing-read` 하나로 끝났다. `tabulated-list-mode` 도, 새 UI 프레임워크도 쓰지 않았다.
`docs/chatgpt-protocol.md` 의 **재현 절**이 함정 넷을 프런트에서 다시 잰 결과다 — 어긋난 값 없음.

**RAIL 4(쓰기)는 GLG가 문을 열기 전에는 착수하지 않는다.**

### 쓰기를 열 때 (RAIL 4)

- `cwa send --conversation <id>` 가 이어쓰기다. 배선은 명령 하나 +
  Chrome 확장 로드 한 번(`cwa browser-native install` → `extension-dir`)
- 순서 주의: **이어쓰기 먼저, 새 대화 나중** (CWA #79 — 새 대화 id 가 `WEB:<uuid>` 로
  승격돼 canonical read 가 거부한다)
- 읽기 명령은 동기다(조회 2–3초, 읽기 3초). 쓰기는 응답을 기다리는 시간이 다른 자릿수라
  **거기서 비동기가 처음으로 필요해진다.** 읽기를 미리 비동기로 만들 이유는 없었다

### 그 뒤 (RAIL 5)

- Claude 웹. 대응물 후보 `cyber-wojtek/Claude-API`(2026-05-28, ★14)
- **인터페이스 일반화는 이때 한다. 지금 미리 하지 않는다**

## 열린 판정 (GLG)

- **쓰기 문을 열지** — RAIL 4 착수 여부
- 리포 공개 여부 — 지금은 private. 개인 대화 데이터를 걸러낸 뒤 판단
- `nixos-config` 에 의존성을 올릴지. Emacs 쪽은 새로 필요한 것이 없다 —
  built-in 만 쓴다(`seq` `browse-url` `json-parse-buffer`). 파이썬 ≥3.10 + system curl 뿐
- CWA 상위에 보낼 기능 요청 — `list`/`search`/`projects` 동사 노출.
  올라가면 `bin/cwaq` 와 양쪽 프런트의 문자열 몇 개를 지운다
- 커밋 6587295 이후 push 안 함

## 남은 거스러미 (막지는 않는다)

- **조회가 동기라 2–3초 Emacs 가 멈춘다.** 목록·검색을 시작하기 전에 사용자가
  `completing-read` 를 볼 수 없으니 비동기로 얻는 것이 적다. 쓰기에서 다시 본다
- **`botschaft-open` 은 목록을 매번 새로 받는다.** 캐시하지 않는 것이 이 집의 규칙이라
  의도한 값이다. 답답하면 `botschaft-list-limit` 을 낮춘다
- 대화 버퍼는 markdown 을 원문 그대로 둔다. `markdown-mode` 로 렌더할지는 미판정

## 읽을 곳

- `README.md` — 계약과 왜 기존 추상화로는 안 되는지, Emacs 설치·키
- `AGENTS.md` — 이 집의 규칙. **서버가 정본**이 핵심
- `docs/chatgpt-protocol.md` — 서버 실측 다섯 + 두 번째 프런트의 재현
- `lisp/botschaft.el` — Commentary 가 왜 이 모양인지 먼저 말한다
