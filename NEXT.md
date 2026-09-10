# NEXT — botschaft

```
RAIL  1 읽기 계약 ✅ → 2 TUI 검수 ✅ → 3 [지금] Emacs 패키지 → 4 쓰기(send) → 5 Claude 백엔드
```

## NOW — `lisp/` 가 비어 있다. 그게 다음 한 걸음이다

읽기 경로는 검수를 한 바퀴 돌았고(교차검수 HIGH 1·MEDIUM 3·LOW 1 전부 수정),
TUI가 계약이 실제로 도는 것을 증명했다. **이제 본선은 Emacs다.**

### 첫 목표 — 읽기 전용 최소 뷰어

1. `botschaft-projects` → `completing-read`로 프로젝트 선택. 스코프를 하나 들고 있는다
2. `botschaft-search` → 질의를 받아 `completing-read`. **annotation에 snippet을 붙인다**
3. `botschaft-open` → 고른 대화를 `special-mode` 버퍼에 렌더

**새 UI 프레임워크가 필요 없다.** TUI를 짜면서 확인한 것:
프로젝트와 대화가 같은 모양(`id · title · update_time`)이라 `completing-read` 하나로
둘 다 고를 수 있고, 검색 응답이 `snippet`을 이미 준다. `tabulated-list-mode`도 필요 없다.

### 반드시 지고 갈 것 넷 (TUI가 먼저 밟았다)

- **도구 turn을 걸러라.** `recipient != "all"`이 축이다. `role`로는 못 가른다
  (실측: 42턴 → 사람이 읽을 17턴. 나머지는 검색 질의 원문 같은 것들)
- **서버 제목은 한 줄이 아니다.** 후행 개행과 미할당 코드포인트가 온다
- **검색은 커서로 끝까지 따라가라.** 첫 30건은 총계가 아니다
- **shim을 bare 실행하지 마라.** `$CWA_PY bin/cwaq …`

`docs/chatgpt-protocol.md`가 다섯 함정의 정본이다. 시작 전에 읽는다.

## 다음 (순서대로)

- **4. 쓰기.** `cwa send --conversation <id>`가 이어쓰기다. 배선은 명령 하나 + Chrome 확장
  로드 한 번(`cwa browser-native install` → `extension-dir`). **문은 GLG가 연다.**
  순서 주의: **이어쓰기 먼저, 새 대화 나중** (CWA #79)
- **5. Claude 웹.** 대응물 후보 `cyber-wojtek/Claude-API`(2026-05-28, ★14) 정도가 보인다.
  **인터페이스 일반화는 이때 한다. 지금 미리 하지 않는다**

## 열린 판정 (GLG)

- 리포 공개 여부 — 지금은 private. 개인 대화 데이터를 문서에서 걸러낸 뒤 공개 판단
- `nixos-config`에 의존성을 올릴지 (Python ≥3.10 + system curl이면 읽기는 충분)
- CWA 상위에 보낼 기능 요청 — `list`/`search`/`projects` 동사 노출

## 읽을 곳

- `README.md` — 계약과 왜 기존 추상화로는 안 되는지
- `AGENTS.md` — 이 집의 규칙. **서버가 정본**이 핵심
- `docs/chatgpt-protocol.md` — 서버 실측 다섯
