# botschaft

**내 하네스 밖에 사는 대화를, 내 자리에서 읽는다.**

`Botschaft`는 독일어로 **메시지**이자 **대사관**이다. 두 뜻이 이 리포에 다 필요하다 —
ChatGPT 웹은 내가 운영하는 하네스가 아니고, 거기 사는 대화는 내가 소유한 데이터가 아니다.
그래서 이 리포는 그 대화를 **가져오지 않는다.** 창구를 하나 낼 뿐이다.

## 무엇이고, 무엇이 아닌가

로그인된 ChatGPT 웹 세션 위에서 **찾고 · 고르고 · 읽고 · 이어 쓴다.**
정본은 언제나 서버다. 로컬에 대화 저장소를 만들지 않는다.

- **아니다** — ChatGPT를 Emacs에 다시 구현하는 일
- **아니다** — `conversations.json` export를 읽는 뷰어. export는 정지 스냅샷이라
  **대화를 이어 쓸 수 없다**. 겉보기 좋은 TUI 다수가 여기에 속한다
- **아니다** — API 키로 새 대화를 시작하는 클라이언트. 그건 이미 많다
  (Emacs만 해도 `gptel`, `chatgpt-shell` 등)

**맞다** — 이미 웹에 쌓인 대화 그래프를 조회하고 이어가는 창구.

## 왜 필요한가 — 기존 추상화가 막아놓은 자리

Emacs·TUI 진영의 멀티프로바이더 추상화 셋을 읽어봤다(2026-09-10 실측):

| 추상화 | 경계 | 대화 조회 슬롯 |
|---|---|---|
| `gptel-backend` (Emacs) | `gptel-request.el:919-926` — 슬롯 `name host header protocol stream endpoint key models url request-params curl-args` | **없음** |
| `LlmProvider` (Rust, ducks/llm-tui) | `src/provider/mod.rs:83` — `name` `is_available` `chat` `continue_with_tools` `list_models` | **없음** (`list_models`는 모델 목록) |
| pydantic-ai `Model` (oterm이 씀) | 외부 라이브러리 소유 | **없음** |

셋 다 **"프로바이더 = 프롬프트 하나 보내고 응답 하나 받는 함수"**로 컷을 냈다.
`key` 슬롯이 인증 자리를 이미 차지해서, **"세션 쿠키가 곧 인증"이라는 발상이 그 타입 안에
들어갈 자리가 없다.** 빈 자리가 아니라 기존 추상화가 구조적으로 막아놓은 자리다.

그래서 이 리포의 경계는 다르다.

## 계약 — 네 동사 (+ 하나)

```
projects   프로젝트 목록
list       대화 목록 (전역 또는 한 프로젝트)
search     서버측 검색 (전역 또는 한 프로젝트 스코프)
read       한 대화의 turn 전체
send       이어 쓰기 / 새 대화        ← 쓰기 배선이 선 뒤
```

```
        Emacs 패키지 ─┐            ┌─ TUI (검수 표본)
                      └── 계약 ────┘
                  ┌────────┴─────────┐
            chatgpt 백엔드       claude 백엔드 (TODO)
```

**정본은 이 계약이고 백엔드는 갈아끼운다.** ChatGPT 백엔드는 오늘
[chatgpt-web-adapter](https://github.com/kymuco/chatgpt-web-adapter)(CWA) 위에 선다.
Claude 웹은 대응물이 생기면 같은 계약 뒤에 붙는다 — **경계는 두 번째 제품을 실제로
붙일 때 확정한다.** 써보기 전에 경계를 정하는 것이 위 셋이 실패한 방식이다.

## 지금 상태

| | |
|---|---|
| `tui/` | Go + bubbletea. 목록 · 프로젝트 · 검색 · 읽기 동작 |
| `bin/cwaq` | 두 프런트가 공유하는 조회 shim (Python) |
| `lisp/` | **본선.** `botschaft.el` — 읽기 전용 세 명령 동작 |
| `docs/chatgpt-protocol.md` | 서버 실측 — 함정 다섯 |

읽기는 **브라우저 없이** 돈다(system `curl`). 쓰기만 로그인된 Chrome을 요구한다.

## 빠른 시작

```bash
# 1. ChatGPT 백엔드(CWA)를 준비하고 한 번 로그인한다
cwa auth login --auth-file ~/.local/state/botschaft/auth_data.json

# 2. 조회 shim
export CWA_AUTH=~/.local/state/botschaft/auth_data.json
bin/cwaq projects
bin/cwaq list --limit 50
bin/cwaq search '검색어' --project <project-id>

# 3. TUI
cd tui && go build -o cwatui . && ./cwatui
```

Emacs — `lisp/` 를 `load-path` 에 넣고 `botschaft` 를 부른다.
`bin/cwaq` 는 파일 위치를 기준으로 알아서 찾는다.

```elisp
(add-to-list 'load-path "~/repos/gh/botschaft/lisp")
(require 'botschaft)
(setq botschaft-auth-file "~/.local/state/botschaft/auth_data.json")
```

| 명령 | 하는 일 |
|---|---|
| `botschaft-projects` | 프로젝트를 골라 스코프로 쥔다. `C-u` 면 스코프를 놓는다 |
| `botschaft-search` | 질의 → 후보(annotation 이 snippet) → 열기. `C-u` 면 전역 |
| `botschaft-open` | 스코프의 대화 목록 → 열기. `C-u` 면 전역 |

대화 버퍼(`special-mode`)에서 `t` 도구 turn 토글 · `g` 다시 읽기 ·
`y` URL 복사 · `o` 브라우저 · `q` 닫기.
`g` 는 캐시를 버리는 게 아니라 **서버를 다시 읽는다** — 캐시가 없다.

| 환경변수 | 쓰임 |
|---|---|
| `CWA_BIN` `CWA_PY` | CWA CLI와 그것을 import 할 수 있는 파이썬 |
| `CWAQ_BIN` | shim 경로 (기본: `bin/cwaq` 자동 탐색) |
| `CWA_AUTH` | 인증 파일. **비밀이다** — 커밋 금지 |

## 키 (TUI)

| 화면 | 키 |
|---|---|
| 목록 | `p` 프로젝트 · `/` 검색 · `enter` 열기 · `j/k` · `y` URL · `o` 브라우저 · `r` 새로고침 · `q` |
| 프로젝트 | `j/k` · `enter` 선택 · `esc` |
| 읽기 | 스크롤 · `t` 도구 turn 토글 · `y` · `o` · `r` 다시 · `esc` |

화면 문자열은 영어, 문서와 주석은 한국어다.

## 경고

CWA는 비공식 어댑터다. 계정 리스크가 0이 아니다. 이 리포는 그 위에 서고,
같은 리스크를 물려받는다. 읽기만 쓰는 동안에도 그렇다.
