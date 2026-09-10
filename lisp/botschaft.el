;;; botschaft.el --- Read your ChatGPT web conversations from Emacs -*- lexical-binding: t; -*-

;; Author: botschaft
;; Version: 0.1.0
;; Package-Requires: ((emacs "28.1"))
;; Keywords: comm, convenience

;;; Commentary:

;; 내 하네스 밖에 사는 대화를 내 자리에서 읽는다.  가져오는 것은 대화가 아니라
;; 접근이다 — 이 파일은 로컬에 대화를 저장하지 않는다.  정본은 언제나 서버다.
;;
;; 세 명령이 전부다:
;;
;;   `botschaft-projects'  프로젝트 하나를 골라 스코프로 쥔다
;;   `botschaft-search'    질의 → 후보 (annotation 이 snippet) → 열기
;;   `botschaft-open'      스코프의 대화 목록 → 고른 하나를 버퍼로 렌더
;;
;; 새 UI 프레임워크가 필요 없다.  프로젝트와 대화가 같은 모양(id · title ·
;; update_time)이라 `completing-read' 하나가 둘 다 고르고, 검색 응답이 snippet 을
;; 이미 준다.  `tabulated-list-mode' 도 쓰지 않는다.
;;
;; 프로세스 경계는 둘이고, 다른 이유로 다르다:
;;
;;   조회 → `$CWA_PY bin/cwaq …'    상위에 동사가 없어 내려간 비공개 shim
;;   읽기 → `cwa messages <id>'      공개 · schema 버전이 붙은 CLI
;;
;; shim 을 bare 실행하지 않는다.  `#!/usr/bin/env python3' 이 가리키는 파이썬에
;; CWA 가 있다는 보장이 없다 — 인터프리터를 명시해 부른다.

;;; Code:

(require 'seq)
(require 'browse-url)

(defgroup botschaft nil
  "Read conversations that live outside this harness."
  :group 'comm
  :prefix "botschaft-")

;;; ── 배선 ─────────────────────────────────────────────────────────────────────

(defvar botschaft--source-file (or load-file-name buffer-file-name)
  "이 파일의 경로.  shim 을 리포 기준으로 찾기 위한 앵커다.")

(defun botschaft--default-shim ()
  "리포 안의 `bin/cwaq' 를 찾는다.
shim 은 TUI 와 공유하므로 `lisp/' 옆이 아니라 리포 루트 아래 산다."
  (let ((root (and botschaft--source-file
                   (file-name-directory
                    (directory-file-name
                     (file-name-directory botschaft--source-file))))))
    (or (and root (expand-file-name "bin/cwaq" root)) "cwaq")))

(defcustom botschaft-python
  (or (getenv "CWA_PY")
      (expand-file-name "~/tmp/cwa-phase1/src/.venv/bin/python"))
  "`chatgpt_web_adapter' 를 import 할 수 있는 파이썬.
shim 은 이 인터프리터로 실행된다.  bare 실행하지 않는 이유는 Commentary 참고."
  :type 'file)

(defcustom botschaft-cwa
  (or (getenv "CWA_BIN")
      (expand-file-name "~/tmp/cwa-phase1/src/.venv/bin/cwa"))
  "공개 CWA CLI.  읽기(`messages')만 여기를 통한다."
  :type 'file)

(defcustom botschaft-shim (or (getenv "CWAQ_BIN") (botschaft--default-shim))
  "조회 shim `bin/cwaq' 의 경로."
  :type 'file)

(defcustom botschaft-auth-file
  (or (getenv "CWA_AUTH")
      (let ((documented (expand-file-name "~/.local/state/botschaft/auth_data.json")))
        (if (file-exists-p documented)
            documented
          ;; AGENTS.md 는 `~/.local/state/' 를 집으로 정했지만 지금 살아 있는 파일은
          ;; 아직 CWA 의 작업 디렉터리에 있다.  문서를 따르되 없으면 실물로 떨어진다.
          (expand-file-name "~/tmp/cwa-phase1/auth_data.json"))))
  "인증 파일.  access token 과 cookie 가 들어 있다 — 비밀이다."
  :type 'file)

(defcustom botschaft-list-limit 200
  "`botschaft-open' 이 한 번에 끌어올 대화 수의 상한."
  :type 'integer)

(defcustom botschaft-search-limit 200
  "`botschaft-search' 가 커서를 따라가며 모을 hit 수의 상한.
첫 페이지 30 건은 총계가 아니다.  shim 이 `cursor == null' 까지 따라간다."
  :type 'integer)

(defcustom botschaft-read-limit 1000
  "한 대화에서 읽어올 turn 수의 상한."
  :type 'integer)

(defcustom botschaft-show-tool-turns nil
  "대화 버퍼를 열 때 도구 turn 을 처음부터 보일지 여부."
  :type 'boolean)

;;; ── 서버 문자열 ──────────────────────────────────────────────────────────────

(defun botschaft--oneline (value)
  "VALUE 를 한 줄짜리 표시 가능한 문자열로 만든다.

서버 제목은 한 줄이 아니다.  후행 개행과 미할당 코드포인트가 실제로 온다
\(실측 2026-09-10).  개행 하나가 minibuffer 후보를 두 줄로 밀고, 미할당
코드포인트는 폭을 못 재게 한다."
  (let ((text (if (stringp value) value (format "%s" (or value "")))))
    (mapconcat
     #'identity
     (split-string
      (apply #'string
             (seq-filter
              (lambda (char)
                (not (memq (get-char-code-property char 'general-category)
                           '(Cn Cc Cf Cs Co))))
              (string-to-list text)))
      nil t)
     " ")))

(defun botschaft--short-time (iso)
  "shim 이 준 offset 붙은 ISO-8601 을 목록 한 칸 크기로 줄인다.
offset 은 계약(JSON)에 남고 표시에서만 떨어진다."
  (let ((text (botschaft--oneline iso)))
    (if (>= (length text) 16) (substring text 0 16) text)))

(defun botschaft--stamp (epoch)
  "EPOCH float 을 turn 머리에 붙일 짧은 시각으로 만든다.
`cwa messages' 의 `create_time' 은 epoch 이고, shim 의 `update_time' 은 ISO 다.
route 마다 시간 타입이 다르므로 각각을 제 타입으로 읽는다."
  (if (and (numberp epoch) (> epoch 0))
      (format-time-string "%m-%d %H:%M" (seconds-to-time epoch))
    ""))

;;; ── 프로세스 ─────────────────────────────────────────────────────────────────

(defun botschaft--run-json (program args)
  "PROGRAM 을 ARGS 로 돌려 stdout 의 JSON 을 alist 로 돌려준다.
shim 은 실패도 stdout 에 JSON 으로 적고 종료코드만 0 이 아니다.  그래서
종료코드보다 먼저 stdout 을 파싱한다."
  (let ((stderr (make-temp-file "botschaft-err")))
    (unwind-protect
        (with-temp-buffer
          (let* ((code (apply #'call-process program nil (list t stderr) nil args))
                 (out (buffer-string))
                 (parsed (ignore-errors
                           (goto-char (point-min))
                           (json-parse-buffer :object-type 'alist
                                              :array-type 'list
                                              :null-object nil
                                              :false-object nil))))
            (cond
             (parsed parsed)
             (t (user-error
                 "botschaft: %s failed (exit %s): %s"
                 (file-name-nondirectory program) code
                 (botschaft--oneline
                  (let ((msg (with-temp-buffer
                               (insert-file-contents stderr)
                               (buffer-string))))
                    (car (last (split-string
                                (if (string-empty-p (string-trim msg)) out msg)
                                "\n" t))))))))))
      (delete-file stderr))))

(defun botschaft--query (verb &rest args)
  "shim 의 조회 동사 VERB 를 ARGS 와 함께 부르고 items 리스트를 돌려준다."
  (let* ((argv (append (list botschaft-shim "--auth-file" botschaft-auth-file verb)
                       (delq nil args)))
         (data (botschaft--run-json botschaft-python argv)))
    (unless (alist-get 'ok data)
      (user-error "botschaft: %s" (or (alist-get 'error data) "query failed")))
    (alist-get 'items data)))

(defvar botschaft-scope nil
  "지금 쥐고 있는 프로젝트.  `botschaft-projects' 가 정한다.  nil 이면 전역.")

(defun botschaft--scope-args ()
  "스코프가 잡혀 있으면 `--project <gizmo-id>' 를 돌려준다.
빈 `--project' 는 생략과 같지 않고, 애초에 목록 route 는 gizmo_id 를 무시한다.
스코프 라우팅은 argv 가 아니라 shim 이 한다."
  (when-let* ((id (alist-get 'id botschaft-scope)))
    (list "--project" id)))

(defun botschaft-projects-fetch ()
  "프로젝트 목록."
  (botschaft--query "projects" "--limit" "50"))

(defun botschaft-list-fetch (&optional global)
  "대화 목록.  스코프가 있으면 그 프로젝트, GLOBAL 이면 전역.

전역 목록에는 프로젝트 대화가 하나도 안 온다 — 두 집합의 교집합이 0 이다
\(실측 2026-09-10: 프로젝트 3 개 합 262, 전역 전량 70).  스코프가 있는데도
전역을 부르면 대화 대부분을 못 본다."
  (apply #'botschaft--query "list" "--limit" (number-to-string botschaft-list-limit)
         (unless global (botschaft--scope-args))))

(defun botschaft-search-fetch (query &optional global)
  "QUERY 로 서버측 검색.  스코프가 있으면 거기로 좁힌다.

목록 route 와 달리 검색 route 는 gizmo_id 를 존중한다."
  (apply #'botschaft--query "search" query
         "--limit" (number-to-string botschaft-search-limit)
         (unless global (botschaft--scope-args))))

(defconst botschaft-read-schema 1
  "우리가 읽을 줄 아는 `cwa messages' 의 schema 판.

읽기만이 버전이 붙은 계약이다 — 응답에 `schema' 가 온다.  상위가 이 수를 올리면
`role' `recipient' `text' 중 무엇이 이름을 바꿨는지 모르는 채로 렌더가 조용히
빈 turn 을 그릴 수 있다.  그래서 다르면 경고한다.  막지는 않는다 — 서버가 정본이고
읽기는 안전하다.")

(defvar botschaft--schema-warned nil
  "이번 세션에서 schema 경고를 이미 했는지.  한 대화마다 반복하지 않는다.")

(defun botschaft-read-fetch (id)
  "대화 ID 의 turn 전체.  공개 CLI 를 통한다."
  (let ((data (botschaft--run-json
               botschaft-cwa
               (list "messages" id "--json"
                     "--auth-file" botschaft-auth-file
                     "--limit" (number-to-string botschaft-read-limit)))))
    (unless (alist-get 'ok data)
      (user-error "botschaft: read failed (schema=%s)" (alist-get 'schema data)))
    (let ((schema (alist-get 'schema data)))
      (when (and schema (not (equal schema botschaft-read-schema))
                 (not botschaft--schema-warned))
        (setq botschaft--schema-warned t)
        (display-warning
         'botschaft
         (format "cwa messages schema %s (expected %s) -- turn 필드가 바뀌었을 수 있다"
                 schema botschaft-read-schema)
         :warning)))
    (alist-get 'messages data)))

;;; ── 고르기 ───────────────────────────────────────────────────────────────────

(defun botschaft--scope-label ()
  "스코프를 프롬프트에 붙일 짧은 꼬리로 만든다."
  (if botschaft-scope
      (format " [%s]" (botschaft--oneline (alist-get 'title botschaft-scope)))
    ""))

(defcustom botschaft-annotation-width 72
  "annotation 에 붙일 snippet 의 표시 폭.
서버 snippet 은 180 자까지 온다(실측 2026-09-10) — 그대로 두면 minibuffer 한 줄을
넘긴다.  잘리는 것은 표시일 뿐 계약의 snippet 은 온전하다."
  :type 'integer)

(defun botschaft--annotation (item)
  "ITEM 의 시각과 snippet 을 annotation 한 줄로 만든다."
  (let ((time (botschaft--short-time (alist-get 'update_time item)))
        (snippet (botschaft--oneline (alist-get 'snippet item))))
    (concat "  " time
            (unless (string-empty-p snippet)
              (concat "  " (truncate-string-to-width
                            snippet botschaft-annotation-width nil nil t))))))

(defun botschaft--read-item (prompt items)
  "ITEMS 중 하나를 PROMPT 로 고르게 하고 그 alist 를 돌려준다.

서버가 준 순서(update_time 내림차순)가 답이므로 정렬하지 않는다.
제목은 유일하지 않아서 겹치면 id 앞자리를 붙여 가른다."
  (unless items (user-error "botschaft: nothing to choose"))
  (let ((table (make-hash-table :test #'equal))
        (candidates nil))
    (dolist (item items)
      (let* ((title (botschaft--oneline (alist-get 'title item)))
             (title (if (string-empty-p title) "(untitled)" title))
             (id (or (alist-get 'id item) ""))
             (key (if (gethash title table)
                      (format "%s [%s]" title (substring id 0 (min 8 (length id))))
                    title)))
        ;; 앞자리까지 같으면 전체 id 로 간다 — 후보가 조용히 사라지면 안 된다.
        (when (gethash key table) (setq key (format "%s [%s]" title id)))
        (puthash key item table)
        (push key candidates)))
    (setq candidates (nreverse candidates))
    (let* ((annotate (lambda (key)
                       (let ((item (gethash key table)))
                         (and item (botschaft--annotation item)))))
           (collection
            (lambda (string predicate action)
              (if (eq action 'metadata)
                  `(metadata (category . botschaft-conversation)
                             (annotation-function . ,annotate)
                             (display-sort-function . identity)
                             (cycle-sort-function . identity))
                (complete-with-action action candidates string predicate)))))
      (gethash (completing-read prompt collection nil t) table))))

;;; ── 대화 버퍼 ────────────────────────────────────────────────────────────────

(defface botschaft-user '((t :inherit font-lock-keyword-face :weight bold))
  "사람 turn 의 머리.")

(defface botschaft-assistant '((t :inherit font-lock-function-name-face :weight bold))
  "상대 turn 의 머리.")

(defface botschaft-tool '((t :inherit shadow))
  "도구 turn 의 머리.")

(defface botschaft-meta '((t :inherit shadow))
  "시각·개수 같은 곁말.")

(defvar-local botschaft--id nil "이 버퍼가 보고 있는 대화 id.")
(defvar-local botschaft--title nil "이 버퍼가 보고 있는 대화 제목.")
(defvar-local botschaft--turns nil "마지막으로 받아온 turn 들.")
(defvar-local botschaft--show-tools nil "도구 turn 을 보이는 중인지.")

(defvar botschaft-conversation-mode-map
  (let ((map (make-sparse-keymap)))
    (define-key map (kbd "t") #'botschaft-toggle-tool-turns)
    (define-key map (kbd "y") #'botschaft-copy-url)
    (define-key map (kbd "o") #'botschaft-browse)
    map)
  "`botschaft-conversation-mode' 키맵.  `g' 는 special-mode 의 revert 를 쓴다.")

(define-derived-mode botschaft-conversation-mode special-mode "Botschaft"
  "한 대화를 읽는 모드.  읽기 전용이고, 아무것도 디스크에 남기지 않는다."
  (setq-local revert-buffer-function #'botschaft--revert)
  (visual-line-mode 1))

(defun botschaft--tool-turn-p (turn)
  "TURN 이 사람이 읽을 것이 아닌지.

축은 `recipient' 다 — `role' 로는 못 가른다.  assistant 가 도구에게 말하는
turn 도 role 은 assistant 이기 때문이다 (실측 2026-09-10: 42 turn 중
사람이 읽을 것은 17)."
  (or (not (equal (alist-get 'recipient turn) "all"))
      (equal (alist-get 'role turn) "tool")))

(defun botschaft--turn-heading (turn)
  "TURN 머리 한 줄."
  (let ((role (alist-get 'role turn))
        (recipient (alist-get 'recipient turn))
        (model (alist-get 'model turn)))
    (cond
     ((botschaft--tool-turn-p turn)
      (propertize (format "-- %s -> %s" role recipient) 'face 'botschaft-tool))
     ((equal role "user")
      (propertize "-- GLG" 'face 'botschaft-user))
     (t
      (propertize (if (and model (not (string-empty-p model)))
                      (format "-- ChatGPT (%s)" model)
                    "-- ChatGPT")
                  'face 'botschaft-assistant)))))

(defun botschaft--render ()
  "버퍼 안의 `botschaft--turns' 를 다시 그린다."
  (let ((inhibit-read-only t)
        (shown 0))
    (erase-buffer)
    (insert (propertize (botschaft--oneline botschaft--title) 'face 'bold) "\n")
    (dolist (turn botschaft--turns)
      (unless (and (botschaft--tool-turn-p turn) (not botschaft--show-tools))
        (setq shown (1+ shown))
        (insert "\n" (botschaft--turn-heading turn) "  "
                (propertize (botschaft--stamp (alist-get 'create_time turn))
                            'face 'botschaft-meta)
                "\n"
                (string-trim-right (or (alist-get 'text turn) "")) "\n")))
    (save-excursion
      (goto-char (point-min))
      (end-of-line)
      (insert (propertize
               (format "  |  %d/%d turns%s  |  %s"
                       shown (length botschaft--turns)
                       (if botschaft--show-tools " (with tools)" "")
                       botschaft--id)
               'face 'botschaft-meta)))
    (when (zerop shown)
      (insert "\n" (propertize "nothing to show -- press t to include tool turns"
                               'face 'botschaft-meta)
              "\n"))
    (goto-char (point-min))))

(defun botschaft--revert (&rest _)
  "서버에서 다시 읽는다.  캐시가 없으므로 revert 는 곧 재조회다."
  (message "botschaft: reading...")
  (setq botschaft--turns (botschaft-read-fetch botschaft--id))
  (botschaft--render)
  (message "botschaft: %d turns" (length botschaft--turns)))

(defun botschaft-toggle-tool-turns ()
  "도구 turn 을 보였다 감췄다 한다."
  (interactive)
  (unless (derived-mode-p 'botschaft-conversation-mode)
    (user-error "botschaft: not a conversation buffer"))
  (setq botschaft--show-tools (not botschaft--show-tools))
  (botschaft--render))

(defun botschaft-url (id)
  "대화 ID 의 웹 주소."
  (concat "https://chatgpt.com/c/" id))

(defun botschaft-copy-url ()
  "이 대화의 주소를 kill-ring 에 넣는다."
  (interactive)
  (unless botschaft--id (user-error "botschaft: not a conversation buffer"))
  (kill-new (botschaft-url botschaft--id))
  (message "botschaft: %s" (botschaft-url botschaft--id)))

(defun botschaft-browse ()
  "이 대화를 브라우저에서 연다."
  (interactive)
  (unless botschaft--id (user-error "botschaft: not a conversation buffer"))
  (browse-url (botschaft-url botschaft--id)))

(defun botschaft--open-item (item)
  "ITEM 이 가리키는 대화를 버퍼에 연다."
  (let* ((id (alist-get 'id item))
         (title (botschaft--oneline (alist-get 'title item)))
         (buffer (get-buffer-create
                  (format "*botschaft: %s*"
                          (truncate-string-to-width title 40 nil nil t)))))
    (with-current-buffer buffer
      (botschaft-conversation-mode)
      (setq botschaft--id id
            botschaft--title title
            botschaft--show-tools botschaft-show-tool-turns)
      (botschaft--revert))
    (pop-to-buffer buffer)))

;;; ── 명령 셋 ──────────────────────────────────────────────────────────────────

;;;###autoload
(defun botschaft-projects (&optional clear)
  "프로젝트 하나를 골라 스코프로 쥔다.  CLEAR (\\[universal-argument]) 면 스코프를 놓는다."
  (interactive "P")
  (if clear
      (progn (setq botschaft-scope nil) (message "botschaft: scope cleared"))
    (message "botschaft: fetching projects...")
    (let ((item (botschaft--read-item "Project: " (botschaft-projects-fetch))))
      (setq botschaft-scope item)
      (message "botschaft: scope%s" (botschaft--scope-label)))))

;;;###autoload
(defun botschaft-search (query &optional global)
  "QUERY 로 검색해 고른 대화를 연다.  GLOBAL (\\[universal-argument]) 면 스코프를 무시한다."
  (interactive
   (list (read-string (format "Search%s: " (botschaft--scope-label)))
         current-prefix-arg))
  (when (string-empty-p (string-trim query))
    (user-error "botschaft: empty query"))
  (message "botschaft: searching...")
  (let ((items (botschaft-search-fetch query global)))
    (message "botschaft: %d hits" (length items))
    (botschaft--open-item (botschaft--read-item "Hit: " items))))

;;;###autoload
(defun botschaft-open (&optional global)
  "대화 목록에서 하나를 골라 연다.  GLOBAL (\\[universal-argument]) 면 스코프를 무시한다."
  (interactive "P")
  (message "botschaft: fetching conversations%s..." (botschaft--scope-label))
  (let ((items (botschaft-list-fetch global)))
    (botschaft--open-item (botschaft--read-item "Conversation: " items))))

(provide 'botschaft)
;;; botschaft.el ends here
