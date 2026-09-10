# 선행 조사 — 이미 있는가, 배울 것이 있는가

2026-09-10, `sorge#18` 워크트리에서 형제 둘이 재고 남긴 것을 이 집으로 옮겼다.
워크트리는 정리됐고 **이 파일이 그 정본이다.**

> **다시 사지 마라.** 아래 표는 형제 턴 두 번의 값이다. 후보를 다시 조사하기 전에
> 여기 없는 이름인지부터 본다. 날짜가 붙어 있으므로 낡았는지도 여기서 판단한다.

두 라운드의 질문이 달랐다. 1라운드 = **"이미 있는가"**(전송 축), 2라운드 = **"배울 것이
있는가"**(프런트엔드 축). 결론은 각각 (c) 없다 / 한 물건도 T3+T4를 동시에 넘지 못한다.

---

조사 2026-09-10, thinkpad. `gh api` + exa-search. 판정만 보고, 설치·편집·PR 없음(read-only).

## 후보 × 게이트

| 후보 | G1 라이브세션 | G2 continue | G3 서버검색 | G4 projects | G5 서버canonical | G6 Emacs | 최종 |
|---|---|---|---|---|---|---|---|
| **Octo-Lex/ChatGPT-Web2API** | ✅ CDP, 로그인 브라우저 | ✅ `conversation_id` 로 재접속 | ❌ 검색 툴 없음(16툴 목록에 `search_*` 부재, 실측) | ❌ `list_projects` 는 있으나 `list_conversations` 가 `/backend-api/conversations?offset&limit&order` 만 침(`backend_client.py:460`). 프로젝트 대화를 못 본다 — 우리가 `README.md:21`에서 잡은 바로 그 함정 | ✅ 로컬 저장소 없음 | ❌ | **G3·G4 실패** |
| **shoyu-ramen/codex-chats-mcp** | ✅ 세션 쿠키/토큰 | ✅ `get_conversation` 있음 | ▲ `search_conversations` 있으나 클라이언트측 제목 substring(`codex_chats_mcp.py:306`), 서버 검색 아님 | ❌ 코드 전체에 `gizmo` 문자열 0건(실측 grep) — 프로젝트 개념 자체가 없음 | ✅ | ❌ | **G4 실패, G3 약함** |
| **kabxx/portal** | ✅ Chromium 구동, 로그인 지속 | ▲ "conversation history" 라 주장하나 목록/검색 verb 문서 없음 — `/thread agent chatgpt` 로 새 스레드 시작이 주 동작 | ❌ 언급 없음 | ❌ 언급 없음 | ✅(브라우저 그 자체가 세션) | ❌ | **다른 장르** — 히스토리 뷰어가 아니라 에이전트 작업 브릿지 |
| **jackwener/OpenCLI** | ✅ | ▲ `history` 동사가 **지금 깨져 있음** — issue #2435 open, PR #2436 unstable (GLG가 이미 앎) | ? | ? | ✅ | ❌ | **읽기 자체가 고장** |
| **imoonkey/openweb** | ✅ | ? | ? | ? | ✅ | ❌ | chatgpt 경로 마지막 손댄 게 2026-04-24 — 정체 |
| **kymuco/chatgpt-web-adapter**(우리가 씀) | ✅ | ✅ `attach_conversation` | ❌ SDK에 list/search 동사 자체가 없음(`cwatui/README.md:51` 실측) | ❌ 같은 이유 | ✅ | — | 우리가 `cwaq` shim 으로 덮은 이유가 이것 |
| **h-ohsaki/chatgpt-el**(fork: eval-exec) | ✅ CDP(Puppeteer), API키 불필요 | ▲ `C-u C-u C-c q` 로 연속 조회만, 대화 목록/id 재접속 아님 | ❌ | ❌ | ✅ | ✅ Emacs 자체 | **G3·G4 실패, 검색·목록 개념 없음. 순수 1턴 query/insert** |
| **xenodium/chatgpt-shell, karthink/gptel, joshcho/ChatGPT.el, emacs-openai/chatgpt** | ❌ 전부 **공식 API 키** 요구(`gptel.el` README: `OpenAI API key` 필수) | — | — | — | — | ✅ | **G1에서 전부 탈락** — Emacs LLM 클라이언트 생태계의 주류는 API-키 클라이언트지 웹세션 뷰어가 아니다 |
| **atondwal/chatgpt_broswer (CCSM)** | ❌ `conversations.json` export 를 읽음(README: "Export data → conversations.json") | ❌ 정의상 불가 — export 는 정지 스냅샷 | 로컬 grep 뿐 | 프로젝트 필드가 export 에 있으면 반영될 수 있으나 서버 아님 | ❌ **G5 실패 — 이게 바로 게이트가 경고한 "export reader"** | ❌ | **분류: export reader, G5로 탈락. 겉보기엔 예쁜 TUI지만 구조적으로 계속(continue)이 안 된다** |

## 결론 — (c) 아무도 게이트를 다 못 넘는다

가장 가까운 것은 **Octo-Lex/ChatGPT-Web2API**(2026-09-01 마지막 커밋, ★47, MCP 16툴,
CDP 기반 — G1·G2·G5 전부 통과)다. 그런데 정확히 우리가 오늘 잡은 두 함정에
그대로 걸린다:

- **G3 없음** — 16개 툴 목록(README 실측)에 `search_*` 동사가 없다.
- **G4 반쪽** — `list_projects` 로 프로젝트를 나열할 수 있지만, 정작
  `list_conversations`(→ `get_conversations`)가 치는 엔드포인트는
  `backend_client.py:460`의 `GET /backend-api/conversations?offset&limit&order` 하나뿐이다.
  프로젝트 대화 조회는 `/backend-api/gizmos/{id}` (프로젝트 *상세*, `backend_client.py:696`)까지만
  가고, `/backend-api/gizmos/{id}/conversations` (프로젝트 *대화 목록*)를 치는 코드가 없다.
  즉 우리 `tui/README.md:21`이 실측한 것과 똑같이, **GLG 대화의 79%가 이 도구에서도 안 보인다.**
  둘 다 같은 서버 함정에 독립적으로 부딪혔다는 것 자체가 이 함정이 진짜라는 교차 증거다.

두 번째로 가까운 `shoyu-ramen/codex-chats-mcp`는 `search_conversations`를 갖고 있지만
서버 검색이 아니라 **가져온 목록 위의 클라이언트측 제목 substring**(`codex_chats_mcp.py:306`)이고,
코드 전체에 `gizmo` 문자열이 0건이라 프로젝트 개념 자체가 없다 — G3 조건(서버측 검색)도
G4도 만족 못 한다.

Emacs 쪽(G6)은 이분된다: 주류 패키지(`gptel`, `chatgpt-shell`, `ChatGPT.el`)는 전부 공식
API 키를 쓰므로 G1에서 탈락하고, 유일하게 API 키 없이 CDP로 도는 `h-ohsaki/chatgpt-el`은
1턴 질의/삽입 도구일 뿐 목록·검색·프로젝트 어느 것도 없다. **G1을 넘는 Emacs 패키지 자체가
이미 한 개뿐이고, 그마저 G2/G3/G4를 하나도 만족하지 않는다** — 우리가 만들려는 Emacs
패키지의 자리가 정말로 비어 있다.

## 권고

**(c) — 아무것도 게이트를 다 못 넘는다.** 만들 가치가 있다는 GLG의 직감은 근거가 있었다.
다만 (b)로 빌릴 것 하나: Octo-Lex 쪽 MCP 16-tool 설계(특히 `list_projects` /
`update_project_instructions` / `list_memories`)는 우리가 Phase 2(쓰기 문 3·4·5)를 열 때
참고할 표면 목록으로 쓸 만하다 — 단 그쪽도 프로젝트 *대화 목록* 엔드포인트를 놓쳤다는
점은 반면교사로 남긴다.

---

조사 2026-09-10 16:xx KST, thinkpad. round 1(`RESEARCH--prior-art-18.md`)은 재도출하지 않음 —
그건 "이미 있냐"였고 (c)로 끝났다. 이번은 "배울 게 있냐"다.

## 후보 × T1–T5

| 후보 | T1 살아있음 | T2 히스토리 브라우저 | T3 멀티프로바이더 경계(file:line) | T4 웹세션 다제품 | T5 빌릴 것 |
|---|---|---|---|---|---|
| **karthink/gptel** (Emacs) | ✅ 2026-09-09, ★3521, issue 181(활발) | ▲ 파일로 save/load(`gptel-mode`) — 목록·검색 UI 없음, `find-file` 가 목록이다 | ✅ `gptel-backend` 기저 struct — **`gptel-request.el:919-926`**. 슬롯: `name host header protocol stream endpoint key models url request-params curl-args coding-system` | ❌ 전부 API 키(`key` 슬롯 + `header` 람다가 `Authorization: Bearer`) | 구조체 자체보다 **"transport-only 계약"** 이라는 설계 선택 — 대화 정체성·검색·프로젝트가 구조체 안에 없다. 우리 세션에 그대로 못 씀(아래 결론) |
| **ggozad/oterm** | ✅ 2026-09-02, ★2430, issue 4(가벼움) | ✅ 여러 영속 세션이 sqlite에, 재개 가능 | ▲ 경계가 **oterm 코드가 아니라 pydantic-ai**(외부 라이브러리)에 있다 — oterm 자신은 Provider 트레이트를 안 짠다 | ❌ pydantic-ai 프로바이더 전부 API키/로컬서버(Ollama 포함) | **UI 레이아웃과 세션 sqlite 모델**만 빌릴 만하다. 프로바이더 추상화는 남의 것이라 우리 웹세션 백엔드에 이식 대상이 아니다 |
| **ducks/llm-tui** (Rust) | ✅ 2026-07-28, ★10(작음), issue 31(열림 많음 — 활발이라기보다 초기) | ✅ `db.rs` + "project-based session management"(README) | ✅ **자체** `LlmProvider` 트레이트 — **`src/provider/mod.rs:83`**. 메서드: `name` `is_available` `chat` `continue_with_tools` `list_models`. 히스토리는 트레이트 밖(`db.rs`)에 분리 | ❌ `provider/{bedrock,claude,gemini,ollama,openai}.rs` 전부 키/로컬 | **경계를 어디서 끊었는가**가 가장 깨끗한 선례: 프로바이더 트레이트 = 1턴 요청/응답만, 저장·조회는 완전히 밖. 우리 `cwaq`(4동사) vs `cwa messages`(읽기)의 분리와 같은 모양 — 교차 검증됨 |
| **darrenburns/elia** | ❌ **마지막 push 2026-10-10 → 아니, 2024-10-10.** 2년 정체. issue 25 open | 있었음(sqlite, `EliaChatModel`, ChatGPT export **import**) | 있었음(`EliaChatModel`) | ❌ | **T1에서 탈락 — README가 예쁘고 ★2480 인데 죽은 리포다.** GLG가 경고한 "예쁜 README, 죽은 리포는 후보가 아니다"의 실물 사례로 남긴다 |
| **shoyu-ramen/codex-chats-mcp** / **Octo-Lex/ChatGPT-Web2API** (round 1 재사용) | ✅ | ✅(단일 제품 안에서만) | ❌ 애초에 한 제품만 다뤄 "여러 백엔드" 경계 자체가 없음 | ✅ 각자 자기 제품만(ChatGPT) — **다제품이 아니라서 T4 대상이 아님** | round 1 결론 그대로. 여기서는 "한 제품 웹세션은 있어도 다제품 웹세션은 없다"는 표본으로만 인용 |

## T3 — 진짜 물어야 할 질문: 그 컷이 웹세션 백엔드에서도 버티는가

세 경계(`gptel-backend`, pydantic-ai `Model`, `LlmProvider`) 모두 **"프로바이더 = 한 번의 프롬프트를
보내고 한 번의 응답을 받는 함수"**로 컷을 낸다. 슬롯/메서드를 보면 전부 `key`·`header`·`endpoint`·
`chat()` 류다. **`list_conversations`·`search`·`gizmo/project` 는 세 경계 어디에도 없다** — 있는
쪽(`ducks/llm-tui`)도 그건 트레이트 밖, DB 레이어의 일이다.

**그 컷은 우리 문제엔 안 맞는다.** 우리 쓰기 없는 읽기 4동사(`projects · list · search · read`,
`tui/README.md:165`)는 애초에 "프롬프트를 보낸다"가 아니라 "서버가 이미 가진 대화 그래프를
조회한다"이다. 세 선례의 프로바이더 인터페이스에 웹세션 백엔드를 끼워 넣으면 `chat()` 자리는
채울 수 있어도(우리가 문 3에서 열 continue/new), **list/search/projects 는 그 인터페이스가 애초에
말한 적이 없는 동사라 자리가 없다.** 이식이 아니라 확장이 필요하다는 뜻이고, 그 확장 모양은
오히려 `ducks/llm-tui` 가 보여준 "트레이트=1턴, 히스토리=별도 레이어" 분리를 **우리 쪽에서
한 겹 더**(프로바이더 트레이트 자체에 `list_conversations`/`search` 동사를 추가) 하는 쪽에 가깝다.

## T4 — 다제품 로그인 세션 추상화: 아무도 없다(근거 있는 음성)

여섯 후보 전원이 T4에서 탈락했고, 탈락 사유가 전부 다르지 않다 — **API 키냐 단일제품 웹세션이냐의
이분법만 있고, "로그인된 웹세션을 프로바이더로 삼아 여러 제품을 묶는" 시도 자체가 안 보였다.**
`gptel-backend`/pydantic-ai/`LlmProvider` 셋 다 "backend = API 호출 방식"이라는 전제를 깔고
있어서, 애초에 "세션 쿠키가 곧 인증"이라는 발상이 그 타입 안에 들어갈 자리가 없다(`key` 슬롯이
그 자리를 이미 차지하고 있다). 이게 진짜 발견이다 — 빈 자리가 아니라 **기존 추상화들이 구조적으로
막아놓은 자리**다.

## 다음 문(Claude 쪽) — 플로서블 여부만, 한 단락

`cyber-wojtek/Claude-API`(pip `claude-webapi`, 2026-05-28 마지막 push, ★14)가 CWA 대응물로 그럴듯하다
— `sessionKey` 쿠키 재사용(devtools로 복사) 또는 구글 로그인 코드 교환으로 인증하고, `ChatSession`이
멀티턴을 자동 스레딩하며 **세션 직렬화/복원**(우리 G2에 해당)까지 있다. 더 오래된
`st1vms/unofficial-claude-api`(마지막 push 2026-03-16, ★201, selenium 기반 세션 채집)도 살아있다.
둘 다 list/search/project 동사는 README에 안 보인다 — CWA와 같은 구멍일 가능성이 높지만, 이건
검증이 아니라 **플로서블 여부**만 본 것이다. 깊이 보지 않았다.

## 결론

**T3+T4를 동시에 넘는 도구는 없다.** T3(자체 멀티프로바이더 경계)를 넘는 건 `gptel`과
`ducks/llm-tui` 둘뿐이고 — `oterm`은 경계를 pydantic-ai에 위임해 자체 경계가 없다 — 그 둘도
경계 정의 자체에서 웹세션을 배제한다(키 슬롯이 계약의 일부). 배울 게 없다는 뜻은 아니다:
**"프로바이더 트레이트는 1턴 요청/응답만 지고, 히스토리·검색·프로젝트는 트레이트 밖 레이어에
둔다"는 `ducks/llm-tui`(`src/provider/mod.rs:83`)의 컷은 우리 아키텍처(전송 계층 `cwaq`/`cwa`
vs 네 동사)와 독립적으로 같은 모양에 도달했다는 교차 증거로 쓸 만하다.

배울 TUI로 하나만 고르면 **`ggozad/oterm`**(2026-09-02, ★2430) — 레이아웃과 "여러 영속 세션을
sqlite에 두고 재개한다"는 모델이 우리 Emacs 패키지의 `completing-read` 재개 흐름과 가장 가깝다.
단, 그 프로바이더 추상화는 남의 라이브러리(pydantic-ai)의 것이라 **이식 대상이 아니라 참고용**이다.
