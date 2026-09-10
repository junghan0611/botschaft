;;; botschaft.el --- Read your ChatGPT web conversations from Emacs -*- lexical-binding: t; -*-

;; Author: Junghan Kim
;; URL: https://github.com/junghan0611/botschaft
;; Version: 0.1.0
;; Package-Requires: ((emacs "28.1"))
;; Keywords: comm, convenience

;;; Commentary:

;; Read the conversations that live outside this harness, from your own seat.
;; What this package brings home is ACCESS, not a copy: nothing is written to
;; disk, no cache database is built, and the server stays canonical.  Reverting
;; a conversation buffer is a re-read, because there is nothing to revert to.
;;
;; That is also why this is not another `conversations.json' viewer.  An export
;; is a frozen snapshot, and you cannot continue a frozen conversation.
;;
;; Three commands:
;;
;;   `botschaft-projects'  pick a project and hold it as the scope
;;   `botschaft-search'    query -> candidates (snippet as annotation) -> open
;;   `botschaft-open'      list the scope's conversations -> render one
;;
;; No new UI framework is needed.  Projects and conversations arrive in the same
;; shape (id, title, update_time), so one `completing-read' picks either, and the
;; search response already carries a snippet.  Not even `tabulated-list-mode'.
;;
;; There are two process boundaries, and they differ on purpose:
;;
;;   query -> $CWA_PY bin/cwaq ...   private shim; upstream exposes no such verb
;;   read  -> cwa messages <id>      public, schema-versioned CLI
;;
;; The shim is never executed bare.  Its `#!/usr/bin/env python3' line says
;; nothing about whether that interpreter can import the adapter, so the
;; interpreter is named explicitly.
;;
;; Setup:
;;
;;   (add-to-list 'load-path "/path/to/botschaft/lisp")
;;   (require 'botschaft)
;;   (setq botschaft-auth-file "~/.local/state/botschaft/auth_data.json")

;;; Code:

(require 'seq)
(require 'subr-x)
(require 'browse-url)

(defgroup botschaft nil
  "Read conversations that live outside this harness."
  :group 'comm
  :prefix "botschaft-")

;;; Wiring

(defvar botschaft--source-file (or load-file-name buffer-file-name)
  "Path of this file, used as the anchor for locating the shim.")

(defun botschaft--default-shim ()
  "Return the path to the repository's `bin/cwaq'.
The shim is shared with the terminal front end, so it lives at the
repository root rather than beside this file."
  (let ((root (and botschaft--source-file
                   (file-name-directory
                    (directory-file-name
                     (file-name-directory botschaft--source-file))))))
    (or (and root (expand-file-name "bin/cwaq" root))
        (executable-find "cwaq")
        "cwaq")))

(defcustom botschaft-python
  (or (getenv "CWA_PY") (executable-find "python3") "python3")
  "Python interpreter that can import `chatgpt_web_adapter'.
The shim runs under this interpreter.  Point it at the virtualenv that
has the adapter installed; see the Commentary for why the shim is never
executed bare."
  :type 'file)

(defcustom botschaft-cwa
  (or (getenv "CWA_BIN") (executable-find "cwa") "cwa")
  "The public chatgpt-web-adapter CLI.
Only reading (\"messages\") goes through it."
  :type 'file)

(defcustom botschaft-shim (or (getenv "CWAQ_BIN") (botschaft--default-shim))
  "Path to the query shim `bin/cwaq'."
  :type 'file)

(defcustom botschaft-auth-file
  (or (getenv "CWA_AUTH")
      (expand-file-name "~/.local/state/botschaft/auth_data.json"))
  "File holding the web session credentials.
It contains an access token and cookies.  It is a secret: keep it out of
version control."
  :type 'file)

(defcustom botschaft-list-limit 200
  "Upper bound on how many conversations `botschaft-open' pulls at once."
  :type 'integer)

(defcustom botschaft-search-limit 200
  "Upper bound on how many hits `botschaft-search' collects.
The first search page holds 30 items and that is not the total; the shim
follows the cursor until it comes back null."
  :type 'integer)

(defcustom botschaft-read-limit 1000
  "Upper bound on how many turns are read from one conversation."
  :type 'integer)

(defcustom botschaft-show-tool-turns nil
  "Whether a conversation buffer starts with tool turns visible."
  :type 'boolean)

;;; Server strings

(defun botschaft--oneline (value)
  "Return VALUE as one displayable line.

Server titles are not one line.  Trailing newlines and unassigned code
points do arrive (measured 2026-09-10).  A single newline pushes a
minibuffer candidate onto a second row, and an unassigned code point
cannot be measured for width."
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
  "Trim the shim's offset-bearing ISO-8601 string ISO to list-column width.
The offset stays in the JSON contract; only the display drops it."
  (let ((text (botschaft--oneline iso)))
    (if (>= (length text) 16) (substring text 0 16) text)))

(defun botschaft--stamp (epoch)
  "Format EPOCH, a float, as the short time on a turn heading.
The routes disagree about the type: `create_time' from the read CLI is an
epoch, while `update_time' from the shim is ISO-8601.  Each is parsed as
what it is."
  (if (and (numberp epoch) (> epoch 0))
      (format-time-string "%m-%d %H:%M" (seconds-to-time epoch))
    ""))

;;; Process boundary

(defun botschaft--run-json (program args)
  "Run PROGRAM with ARGS and return the JSON on its stdout as an alist.
The shim reports failure as JSON on stdout and signals only through its
exit code, so stdout is parsed before the exit code is consulted."
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
            (or parsed
                (user-error
                 "Botschaft: %s failed (exit %s): %s"
                 (file-name-nondirectory program) code
                 (botschaft--oneline
                  (let ((msg (with-temp-buffer
                               (insert-file-contents stderr)
                               (buffer-string))))
                    (car (last (split-string
                                (if (string-empty-p (string-trim msg)) out msg)
                                "\n" t)))))))))
      (delete-file stderr))))

(defun botschaft--query (verb &rest args)
  "Call the shim's query VERB with ARGS and return its item list."
  (let* ((argv (append (list botschaft-shim "--auth-file" botschaft-auth-file verb)
                       (delq nil args)))
         (data (botschaft--run-json botschaft-python argv)))
    (unless (alist-get 'ok data)
      (user-error "Botschaft: %s" (or (alist-get 'error data) "query failed")))
    (alist-get 'items data)))

(defvar botschaft-scope nil
  "The project currently held as the scope, or nil for global.
Set by `botschaft-projects'.")

(defun botschaft--scope-args ()
  "Return the shim's project argument when a scope is held.
An empty \"--project\" is not the same as omitting it, and the plain list
route ignores a project id outright.  Scoping is routed by the shim, not
by this argument vector."
  (when-let* ((id (alist-get 'id botschaft-scope)))
    (list "--project" id)))

(defun botschaft-projects-fetch ()
  "Return the list of projects."
  (botschaft--query "projects" "--limit" "50"))

(defun botschaft-list-fetch (&optional global)
  "Return conversations for the held scope, or every one when GLOBAL.

The global list contains no project conversations at all -- the two sets
do not intersect (measured 2026-09-10: three projects held 262 between
them, the entire global list held 70).  Asking globally while a scope is
held therefore hides most of the account."
  (apply #'botschaft--query "list" "--limit" (number-to-string botschaft-list-limit)
         (unless global (botschaft--scope-args))))

(defun botschaft-search-fetch (query &optional global)
  "Search the server for QUERY, narrowed to the held scope unless GLOBAL.

Unlike the list route, the search route does honour a project id."
  (apply #'botschaft--query "search" query
         "--limit" (number-to-string botschaft-search-limit)
         (unless global (botschaft--scope-args))))

(defconst botschaft-read-schema 1
  "The `cwa messages' schema version this package knows how to read.

Reading is the only versioned contract here: the response carries a
`schema' field.  If upstream bumps it, one of `role', `recipient' or
`text' may have been renamed, and the renderer would quietly draw empty
turns.  A mismatch is therefore reported, but not treated as fatal --
the server is canonical and reading is safe.")

(defvar botschaft--schema-warned nil
  "Whether the schema mismatch has already been reported this session.")

(defun botschaft-read-fetch (id)
  "Return every turn of conversation ID, through the public CLI."
  (let ((data (botschaft--run-json
               botschaft-cwa
               (list "messages" id "--json"
                     "--auth-file" botschaft-auth-file
                     "--limit" (number-to-string botschaft-read-limit)))))
    (unless (alist-get 'ok data)
      (user-error "Botschaft: read failed (schema=%s)" (alist-get 'schema data)))
    (let ((schema (alist-get 'schema data)))
      (when (and schema (not (equal schema botschaft-read-schema))
                 (not botschaft--schema-warned))
        (setq botschaft--schema-warned t)
        (display-warning
         'botschaft
         (format "cwa messages schema %s (expected %s); turn fields may have moved"
                 schema botschaft-read-schema)
         :warning)))
    (alist-get 'messages data)))

;;; Choosing

(defcustom botschaft-annotation-width 72
  "Display width of the snippet shown as a completion annotation.
Server snippets run to 180 characters (measured 2026-09-10), which
overflows a minibuffer row.  Only the display is trimmed; the snippet in
the contract stays whole."
  :type 'integer)

(defun botschaft--scope-label ()
  "Return the held scope as a short prompt suffix."
  (if botschaft-scope
      (format " [%s]" (botschaft--oneline (alist-get 'title botschaft-scope)))
    ""))

(defun botschaft--annotation (item)
  "Return the time and snippet of ITEM as one annotation line."
  (let ((time (botschaft--short-time (alist-get 'update_time item)))
        (snippet (botschaft--oneline (alist-get 'snippet item))))
    (concat "  " time
            (unless (string-empty-p snippet)
              (concat "  " (truncate-string-to-width
                            snippet botschaft-annotation-width nil nil t))))))

(defun botschaft--read-item (prompt items)
  "Ask with PROMPT for one of ITEMS and return its alist.

The order the server gave (most recently updated first) is the answer, so
nothing is re-sorted.  Titles are not unique, so colliding ones are told
apart by a leading slice of the conversation id."
  (unless items (user-error "Botschaft: nothing to choose"))
  (let ((table (make-hash-table :test #'equal))
        (candidates nil))
    (dolist (item items)
      (let* ((title (botschaft--oneline (alist-get 'title item)))
             (title (if (string-empty-p title) "(untitled)" title))
             (id (or (alist-get 'id item) ""))
             (key (if (gethash title table)
                      (format "%s [%s]" title (substring id 0 (min 8 (length id))))
                    title)))
        ;; Even the slice can collide; fall back to the whole id rather than
        ;; letting a candidate disappear.
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

;;; The conversation buffer

(defface botschaft-user '((t :inherit font-lock-keyword-face :weight bold))
  "Face for the heading of a turn you wrote.")

(defface botschaft-assistant '((t :inherit font-lock-function-name-face :weight bold))
  "Face for the heading of a turn the model wrote.")

(defface botschaft-tool '((t :inherit shadow))
  "Face for the heading of a tool turn.")

(defface botschaft-meta '((t :inherit shadow))
  "Face for timestamps, counts and other marginalia.")

(defvar-local botschaft--id nil "Id of the conversation this buffer shows.")
(defvar-local botschaft--title nil "Title of the conversation this buffer shows.")
(defvar-local botschaft--turns nil "Turns as last fetched from the server.")
(defvar-local botschaft--show-tools nil "Whether tool turns are currently shown.")

(defvar botschaft-conversation-mode-map
  (let ((map (make-sparse-keymap)))
    (define-key map (kbd "t") #'botschaft-toggle-tool-turns)
    (define-key map (kbd "y") #'botschaft-copy-url)
    (define-key map (kbd "o") #'botschaft-browse)
    map)
  "Keymap for `botschaft-conversation-mode'.
Reverting is inherited from `special-mode', bound to \\`g'.")

(define-derived-mode botschaft-conversation-mode special-mode "Botschaft"
  "Major mode for reading one conversation.
The buffer is read-only and nothing it shows is written to disk."
  (setq-local revert-buffer-function #'botschaft--revert)
  (visual-line-mode 1))

(defun botschaft--tool-turn-p (turn)
  "Return non-nil when TURN is not meant for a human reader.

The axis is `recipient', not `role': a turn where the model addresses a
tool still has the assistant role.  Measured 2026-09-10, one conversation
of 42 turns held 17 that a person would read."
  (or (not (equal (alist-get 'recipient turn) "all"))
      (equal (alist-get 'role turn) "tool")))

(defun botschaft--turn-heading (turn)
  "Return the one-line heading for TURN."
  (let ((role (alist-get 'role turn))
        (recipient (alist-get 'recipient turn))
        (model (alist-get 'model turn)))
    (cond
     ((botschaft--tool-turn-p turn)
      (propertize (format "-- %s -> %s" role recipient) 'face 'botschaft-tool))
     ((equal role "user")
      (propertize "-- You" 'face 'botschaft-user))
     (t
      (propertize (if (and model (not (string-empty-p model)))
                      (format "-- ChatGPT (%s)" model)
                    "-- ChatGPT")
                  'face 'botschaft-assistant)))))

(defun botschaft--render ()
  "Redraw the current buffer from `botschaft--turns'."
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
  "Read this conversation from the server again.
There is no cache, so reverting is simply another read."
  (message "Botschaft: reading...")
  (setq botschaft--turns (botschaft-read-fetch botschaft--id))
  (botschaft--render)
  (message "Botschaft: %d turns" (length botschaft--turns)))

(defun botschaft-toggle-tool-turns ()
  "Show or hide the tool turns in this conversation."
  (interactive)
  (unless (derived-mode-p 'botschaft-conversation-mode)
    (user-error "Botschaft: not a conversation buffer"))
  (setq botschaft--show-tools (not botschaft--show-tools))
  (botschaft--render))

(defun botschaft-url (id)
  "Return the web address of conversation ID."
  (concat "https://chatgpt.com/c/" id))

(defun botschaft-copy-url ()
  "Put this conversation's address on the kill ring."
  (interactive)
  (unless botschaft--id (user-error "Botschaft: not a conversation buffer"))
  (kill-new (botschaft-url botschaft--id))
  (message "Botschaft: %s" (botschaft-url botschaft--id)))

(defun botschaft-browse ()
  "Open this conversation in a browser."
  (interactive)
  (unless botschaft--id (user-error "Botschaft: not a conversation buffer"))
  (browse-url (botschaft-url botschaft--id)))

(defun botschaft--open-item (item)
  "Open the conversation ITEM points at, and return its buffer."
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

;;; Commands

;;;###autoload
(defun botschaft-projects (&optional clear)
  "Pick a project and hold it as the scope.
With a prefix argument, CLEAR the scope instead."
  (interactive "P")
  (if clear
      (progn (setq botschaft-scope nil) (message "Botschaft: scope cleared"))
    (message "Botschaft: fetching projects...")
    (let ((item (botschaft--read-item "Project: " (botschaft-projects-fetch))))
      (setq botschaft-scope item)
      (message "Botschaft: scope%s" (botschaft--scope-label)))))

;;;###autoload
(defun botschaft-search (query &optional global)
  "Search for QUERY and open the conversation you pick.
With a prefix argument, GLOBAL is non-nil and the held scope is ignored."
  (interactive
   (list (read-string (format "Search%s: " (botschaft--scope-label)))
         current-prefix-arg))
  (when (string-empty-p (string-trim query))
    (user-error "Botschaft: empty query"))
  (message "Botschaft: searching...")
  (let ((items (botschaft-search-fetch query global)))
    (message "Botschaft: %d hits" (length items))
    (botschaft--open-item (botschaft--read-item "Hit: " items))))

;;;###autoload
(defun botschaft-open (&optional global)
  "List conversations and open the one you pick.
With a prefix argument, GLOBAL is non-nil and the held scope is ignored."
  (interactive "P")
  (message "Botschaft: fetching conversations%s..." (botschaft--scope-label))
  (let ((items (botschaft-list-fetch global)))
    (botschaft--open-item (botschaft--read-item "Conversation: " items))))

(provide 'botschaft)
;;; botschaft.el ends here
