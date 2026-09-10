#!/usr/bin/env bash
set -euo pipefail

# botschaft — build, check and run the two front ends.
#
# Menu with no arguments; a subcommand runs headlessly so an agent or another
# host can call it directly:
#
#   ./run.sh doctor     what is missing, and the one command that fixes it
#   ./run.sh setup      install the ChatGPT backend at the pinned commit
#   ./run.sh build      build the TUI
#   ./run.sh test       gofmt, go vet, go test, byte-compile, checkdoc
#   ./run.sh tui        build and run

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/botschaft"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/botschaft"
AUTH_FILE="${CWA_AUTH:-$STATE_DIR/auth_data.json}"

# The ChatGPT backend, pinned. Two hosts on two architectures have to agree on
# which upstream they measured against; see docs/chatgpt-protocol.md.
CWA_REPO="https://github.com/kymuco/chatgpt-web-adapter.git"
CWA_COMMIT="e7f041a"
CWA_HOME="$DATA_DIR/cwa"

# Helper functions
info() { echo -e "${BLUE}ℹ ${NC}$1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
warn() { echo -e "${YELLOW}⚠${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; }

# find_python locates an interpreter that can actually import the adapter. The
# shebang on bin/cwaq says nothing about that, which is why the shim is never
# executed bare. Order: an explicit override, this script's own install, an
# install made before this script existed, then whatever is on PATH.
#
# The older location stays in the search on purpose. Dropping it does not make a
# host safer, it only makes a working host report that nothing is installed —
# which is worse, because the real risk is running against an adapter the
# measurements were not taken against. So the commit is checked (adapter_commit)
# rather than inferred from the path.
find_python() {
    local candidates=()
    [[ -n "${CWA_PY:-}" ]] && candidates+=("$CWA_PY")
    candidates+=("$CWA_HOME/src/.venv/bin/python")
    candidates+=("$HOME/tmp/cwa-phase1/src/.venv/bin/python")
    candidates+=("$(command -v python3 || true)")
    local py
    for py in "${candidates[@]}"; do
        [[ -x "$py" ]] || continue
        if "$py" -c 'import chatgpt_web_adapter' >/dev/null 2>&1; then
            echo "$py"
            return 0
        fi
    done
    return 1
}

find_cwa() {
    local candidates=()
    [[ -n "${CWA_BIN:-}" ]] && candidates+=("$CWA_BIN")
    candidates+=("$CWA_HOME/src/.venv/bin/cwa")
    candidates+=("$HOME/tmp/cwa-phase1/src/.venv/bin/cwa")
    candidates+=("$(command -v cwa || true)")
    local bin
    for bin in "${candidates[@]}"; do
        if [[ -x "$bin" ]]; then
            echo "$bin"
            return 0
        fi
    done
    return 1
}

# adapter_commit reports the commit of the adapter checkout backing an
# interpreter, or nothing when it is not a checkout at all. A venv lives at
# <src>/.venv, so the checkout is two levels above bin/.
adapter_commit() {
    local src
    src=$(cd "$(dirname "$1")/../.." 2>/dev/null && pwd) || return 0
    git -C "$src" rev-parse --short HEAD 2>/dev/null || true
}

# Check the environment
check_env() {
    local missing=0

    if command -v go >/dev/null 2>&1; then
        success "go: $(go version | awk '{print $3, $4}')"
    else
        error "go: not found — the TUI cannot be built"
        missing=1
    fi

    if command -v emacs >/dev/null 2>&1; then
        success "emacs: $(emacs --version 2>/dev/null | head -1 | awk '{print $3}')"
    else
        warn "emacs: not found — the Emacs package cannot be checked here"
    fi

    if PY=$(find_python); then
        success "adapter python: $PY"
        local at
        at=$(adapter_commit "$PY")
        if [[ -z "$at" ]]; then
            warn "  adapter commit unknown — not a git checkout, so it cannot be compared to the pin"
        elif [[ "$CWA_COMMIT" == "$at"* ]] || [[ "$at" == "$CWA_COMMIT"* ]]; then
            success "  adapter at the pinned commit ($at)"
        else
            warn "  adapter is at $at, not the pinned $CWA_COMMIT"
            info "  the server facts in docs/chatgpt-protocol.md were measured against the pin"
        fi
    else
        error "adapter python: not found — no interpreter can import chatgpt_web_adapter"
        info "  fix: ./run.sh setup"
        missing=1
    fi

    if CWA=$(find_cwa); then
        success "cwa: $CWA"
    else
        error "cwa: not found"
        info "  fix: ./run.sh setup"
        missing=1
    fi

    if [[ -f "$AUTH_FILE" ]]; then
        local mode
        mode=$(stat -c '%a' "$AUTH_FILE")
        if [[ "$mode" == "600" ]]; then
            success "auth: $AUTH_FILE (mode $mode)"
        else
            warn "auth: $AUTH_FILE is mode $mode, expected 600"
            info "  fix: chmod 600 '$AUTH_FILE'"
        fi
    else
        error "auth: $AUTH_FILE does not exist"
        info "  fix: ./run.sh login"
        missing=1
    fi

    if [[ -x "$REPO_DIR/bin/cwaq" ]]; then
        success "shim: bin/cwaq"
    else
        error "shim: bin/cwaq missing or not executable"
        missing=1
    fi

    return $missing
}

# Install the ChatGPT backend from source at the pinned commit. A virtualenv is
# built here rather than copied between hosts: the two machines are x86-64 and
# aarch64, and a venv does not survive that.
cmd_setup() {
    command -v git >/dev/null 2>&1 || { error "git is required"; return 1; }
    command -v python3 >/dev/null 2>&1 || { error "python3 is required"; return 1; }

    mkdir -p "$STATE_DIR" && chmod 700 "$STATE_DIR"
    success "state dir: $STATE_DIR"

    if [[ -z "${BOTSCHAFT_FORCE_SETUP:-}" ]] && PY=$(find_python); then
        if [[ "$(adapter_commit "$PY")" == "$CWA_COMMIT"* ]]; then
            success "an adapter at the pinned commit is already installed: $PY"
            info "set BOTSCHAFT_FORCE_SETUP=1 to install another one under $CWA_HOME"
            return 0
        fi
    fi

    mkdir -p "$CWA_HOME"
    if [[ ! -d "$CWA_HOME/src/.git" ]]; then
        info "cloning the adapter"
        git clone -q "$CWA_REPO" "$CWA_HOME/src"
    fi
    ( cd "$CWA_HOME/src" && git fetch -q origin && git checkout -q "$CWA_COMMIT" )
    success "adapter pinned at $CWA_COMMIT"

    info "building a virtualenv for $(uname -m)"
    python3 -m venv "$CWA_HOME/src/.venv"
    "$CWA_HOME/src/.venv/bin/pip" install -q --upgrade pip
    "$CWA_HOME/src/.venv/bin/pip" install -q -e "$CWA_HOME/src"
    success "installed: $CWA_HOME/src/.venv"
}

# Log in once. This is the only step that opens a browser.
cmd_login() {
    local cwa
    cwa=$(find_cwa) || { error "cwa not found — run ./run.sh setup first"; return 1; }
    mkdir -p "$STATE_DIR" && chmod 700 "$STATE_DIR"
    "$cwa" auth login --auth-file "$AUTH_FILE"
    chmod 600 "$AUTH_FILE"
    success "auth written to $AUTH_FILE (mode 600)"
}

cmd_build() {
    command -v go >/dev/null 2>&1 || { error "go is required"; return 1; }
    ( cd "$REPO_DIR/tui" && go build -o cwatui . )
    success "built: tui/cwatui ($(uname -m))"
}

cmd_test() {
    local failed=0

    if command -v go >/dev/null 2>&1; then
        info "gofmt"
        local unformatted
        unformatted=$(cd "$REPO_DIR/tui" && gofmt -l .)
        if [[ -n "$unformatted" ]]; then
            error "gofmt: $unformatted"
            failed=1
        else
            success "gofmt: clean"
        fi

        info "go vet"
        ( cd "$REPO_DIR/tui" && go vet ./... ) && success "go vet: clean" || failed=1

        info "go test"
        ( cd "$REPO_DIR/tui" && go test ./... ) && success "go test: pass" || failed=1
    else
        warn "go not found — the TUI was not checked"
    fi

    if command -v emacs >/dev/null 2>&1; then
        info "byte-compile"
        local out
        out=$(cd "$REPO_DIR" && emacs -Q --batch --eval '(byte-compile-file "lisp/botschaft.el")' 2>&1 || true)
        rm -f "$REPO_DIR/lisp/botschaft.elc"
        if [[ -n "$out" ]]; then
            error "byte-compile:"
            echo "$out"
            failed=1
        else
            success "byte-compile: clean"
        fi

        info "checkdoc"
        out=$(cd "$REPO_DIR" && emacs -Q --batch --eval '(progn (require (quote checkdoc)) (checkdoc-file "lisp/botschaft.el"))' 2>&1 || true)
        if [[ -n "$out" ]]; then
            error "checkdoc:"
            echo "$out"
            failed=1
        else
            success "checkdoc: clean"
        fi
    else
        warn "emacs not found — the package was not checked"
    fi

    return $failed
}

# A live read through the contract. Costs latency and nothing else; the server
# stays canonical and nothing is written locally.
cmd_smoke() {
    local py
    py=$(find_python) || { error "no interpreter can import the adapter — run ./run.sh setup"; return 1; }
    info "projects"
    "$py" "$REPO_DIR/bin/cwaq" --auth-file "$AUTH_FILE" projects --limit 5 \
        | python3 -c 'import json,sys; d=json.load(sys.stdin); print("  ok=%s count=%s" % (d["ok"], d["count"]))'
    info "list"
    "$py" "$REPO_DIR/bin/cwaq" --auth-file "$AUTH_FILE" list --limit 3 \
        | python3 -c 'import json,sys; d=json.load(sys.stdin); print("  ok=%s count=%s" % (d["ok"], d["count"]))'
    success "the read contract answers"
}

cmd_tui() {
    cmd_build
    local py cwa
    py=$(find_python) || { error "no interpreter can import the adapter — run ./run.sh setup"; return 1; }
    cwa=$(find_cwa) || { error "cwa not found — run ./run.sh setup"; return 1; }
    CWA_PY="$py" CWA_BIN="$cwa" CWA_AUTH="$AUTH_FILE" "$REPO_DIR/tui/cwatui"
}

# Print what to put in an init file, resolved for this host.
cmd_emacs() {
    local py cwa
    py=$(find_python) || py="/path/to/venv/bin/python"
    cwa=$(find_cwa) || cwa="/path/to/venv/bin/cwa"
    cat <<EOF
(add-to-list 'load-path "$REPO_DIR/lisp")
(require 'botschaft)
;; The auth file resolves itself through XDG. These two are only needed
;; because the adapter is in a virtualenv rather than on PATH.
(setq botschaft-python "$py"
      botschaft-cwa    "$cwa")
EOF
}

cmd_clean() {
    rm -f "$REPO_DIR/tui/cwatui" "$REPO_DIR/lisp/botschaft.elc"
    success "removed build artifacts"
}

# Show menu
show_menu() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${GREEN}botschaft${NC} - ${BLUE}$(uname -m) · $(hostname)${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo -e "  ${YELLOW}Run${NC}"
    echo "    1) TUI (build and run)"
    echo "    2) Emacs setup snippet for this host"
    echo ""
    echo -e "  ${YELLOW}Check${NC}"
    echo "    3) Doctor (what is missing, and the fix)"
    echo "    4) Test (gofmt, go vet, go test, byte-compile, checkdoc)"
    echo "    5) Smoke (one live read through the contract)"
    echo ""
    echo -e "  ${YELLOW}Install${NC}"
    echo "    6) Setup (adapter at the pinned commit, this architecture)"
    echo "    7) Login (opens a browser once)"
    echo ""
    echo -e "  ${YELLOW}Build${NC}"
    echo "    8) Build the TUI"
    echo "    9) Clean build artifacts"
    echo ""
    echo "    0) Exit"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# Execute command
execute_cmd() {
    local cmd="$1"
    echo ""
    info "running: $cmd"
    echo ""
    set +e
    eval "$cmd"
    local status=$?
    set -e
    echo ""
    if [[ $status -eq 0 ]]; then
        success "done"
    else
        error "failed (exit code: $status)"
    fi
    return 0
}

usage() {
    cat <<'EOF'
usage: ./run.sh [command]

  (no command)  interactive menu
  doctor        report what is missing and the command that fixes it
  setup         install the ChatGPT backend at the pinned commit
  login         authenticate once (opens a browser)
  build         build the TUI
  test          gofmt, go vet, go test, byte-compile, checkdoc
  smoke         one live read through the contract
  tui           build and run the TUI
  emacs         print the Emacs setup snippet for this host
  clean         remove build artifacts
EOF
}

# Main loop
main() {
    cd "$REPO_DIR"

    if [[ $# -gt 0 ]]; then
        case "$1" in
            doctor) check_env ;;
            setup) cmd_setup ;;
            login) cmd_login ;;
            build) cmd_build ;;
            test) cmd_test ;;
            smoke) cmd_smoke ;;
            tui) cmd_tui ;;
            emacs) cmd_emacs ;;
            clean) cmd_clean ;;
            -h | --help | help) usage ;;
            *)
                error "unknown command: $1"
                echo ""
                usage
                exit 1
                ;;
        esac
        exit $?
    fi

    while true; do
        show_menu
        read -r -p "choose (0-9): " choice

        case $choice in
            1) execute_cmd "cmd_tui" ;;
            2) execute_cmd "cmd_emacs" ;;
            3) execute_cmd "check_env" ;;
            4) execute_cmd "cmd_test" ;;
            5) execute_cmd "cmd_smoke" ;;
            6) execute_cmd "cmd_setup" ;;
            7)
                warn "This opens a browser and writes credentials. Continue? (y/N)"
                read -r -p "> " confirm
                if [[ "$confirm" =~ ^[Yy]$ ]]; then
                    execute_cmd "cmd_login"
                else
                    info "cancelled"
                fi
                ;;
            8) execute_cmd "cmd_build" ;;
            9) execute_cmd "cmd_clean" ;;
            0)
                info "bye"
                exit 0
                ;;
            *)
                error "no such choice"
                ;;
        esac

        echo ""
        read -r -p "press Enter to continue..."
    done
}

main "$@"
