#!/usr/bin/env bash
#
# infra-shelf installer — zero to a running stack in one command.
#
#   curl -fsSL https://raw.githubusercontent.com/IvanMicai/infra-shelf/main/scripts/install.sh | bash
#
# What it does (idempotent — safe to re-run):
#   1. Checks prerequisites (git, docker, docker compose).
#   2. Clones infra-shelf (or fast-forwards an existing clone).
#   3. Creates .env from .env.example (never overwrites an existing .env).
#   4. Builds the `shelf` and `shelf-web` binaries — using host Go if available,
#      otherwise inside a throwaway golang:1.25 container (no Go required).
#   5. Starts the core stack (postgres, redis, rabbitmq, mongodb).
#   6. Prints the next step to provision your first app.
#
# Configuration (environment variables or flags):
#   INFRA_SHELF_DIR     install directory      (default: ./infra-shelf or $HOME/infra-shelf)
#   INFRA_SHELF_REPO    git remote to clone    (default: https://github.com/IvanMicai/infra-shelf.git)
#   INFRA_SHELF_BRANCH  branch to check out    (default: main)
#
# Flags:
#   --dir <path>     install directory
#   --app <name>     also provision a first app (-s postgres,redis,rabbitmq,mongodb)
#   --no-up          clone + build only; do not start containers
#   -h, --help       show this help
#
set -euo pipefail

REPO_URL="${INFRA_SHELF_REPO:-https://github.com/IvanMicai/infra-shelf.git}"
BRANCH="${INFRA_SHELF_BRANCH:-main}"
GO_IMAGE="golang:1.25-alpine"
MIN_GO_MINOR=25 # require Go 1.25+

TARGET_DIR="${INFRA_SHELF_DIR:-}"
FIRST_APP=""
DO_UP=1

# ---- pretty output -----------------------------------------------------------
if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GREEN=$'\033[32m'
  YELLOW=$'\033[33m'; CYAN=$'\033[36m'; RESET=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; CYAN=""; RESET=""
fi
info()  { printf '%s==>%s %s\n' "$CYAN" "$RESET" "$*"; }
ok()    { printf '%s ✔%s %s\n' "$GREEN" "$RESET" "$*"; }
warn()  { printf '%s ⚠%s %s\n' "$YELLOW" "$RESET" "$*" >&2; }
die()   { printf '%s �’%s %s\n' "$RED" "$RESET" "$*" >&2; exit 1; }
have()  { command -v "$1" >/dev/null 2>&1; }

usage() { sed -n '2,33p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

# ---- args --------------------------------------------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --dir)   TARGET_DIR="${2:-}"; shift 2 ;;
    --app)   FIRST_APP="${2:-}"; shift 2 ;;
    --no-up) DO_UP=0; shift ;;
    -h|--help) usage ;;
    *) die "unknown argument: $1 (try --help)" ;;
  esac
done

# ---- prerequisites -----------------------------------------------------------
info "Checking prerequisites"
have git || die "git is required — https://git-scm.com/downloads"
have docker || die "docker is required — https://docs.docker.com/get-docker/"
if ! docker compose version >/dev/null 2>&1; then
  die "Docker Compose v2 is required (the 'docker compose' subcommand) — https://docs.docker.com/compose/install/"
fi
docker info >/dev/null 2>&1 || die "the Docker daemon is not running — start Docker and re-run"
ok "git, docker, and docker compose are available"

# ---- resolve target dir ------------------------------------------------------
if [ -z "$TARGET_DIR" ]; then
  # If we are already inside a checkout, install in place; else default location.
  if [ -f "docker-compose.yml" ] && [ -f "go.mod" ] && grep -q "infra-shelf" go.mod 2>/dev/null; then
    TARGET_DIR="$(pwd)"
  elif [ -w "." ]; then
    TARGET_DIR="$(pwd)/infra-shelf"
  else
    TARGET_DIR="$HOME/infra-shelf"
  fi
fi

# ---- clone or update ---------------------------------------------------------
if [ -d "$TARGET_DIR/.git" ]; then
  info "Updating existing clone at $TARGET_DIR"
  git -C "$TARGET_DIR" pull --ff-only || warn "could not fast-forward — leaving the existing checkout as-is"
elif [ -f "$TARGET_DIR/docker-compose.yml" ] && [ -f "$TARGET_DIR/go.mod" ]; then
  info "Using existing infra-shelf checkout at $TARGET_DIR"
else
  info "Cloning $REPO_URL → $TARGET_DIR"
  git clone --branch "$BRANCH" "$REPO_URL" "$TARGET_DIR"
fi
cd "$TARGET_DIR"
ok "Repository ready at $TARGET_DIR"

# ---- .env --------------------------------------------------------------------
if [ -f ".env" ]; then
  ok ".env already present — leaving it untouched"
else
  cp .env.example .env
  mkdir -p data backups
  ok "Created .env from .env.example (remember to change the default passwords)"
fi

# ---- build -------------------------------------------------------------------
host_go_ok() {
  have go || return 1
  local minor
  minor="$(go env GOVERSION 2>/dev/null | sed -E 's/^go[0-9]+\.([0-9]+).*/\1/')"
  [ -n "$minor" ] && [ "$minor" -ge "$MIN_GO_MINOR" ] 2>/dev/null
}

if host_go_ok; then
  info "Building binaries with host Go ($(go env GOVERSION))"
  CGO_ENABLED=0 go build -trimpath -o shelf ./cmd/shelf
  CGO_ENABLED=0 go build -trimpath -o shelf-web ./cmd/shelf-web
else
  info "Host Go 1.${MIN_GO_MINOR}+ not found — building inside $GO_IMAGE (no Go required on host)"
  docker run --rm \
    -v "$PWD":/src -w /src \
    -e CGO_ENABLED=0 -e HOME=/tmp -e GOCACHE=/tmp/.gocache -e GOMODCACHE=/tmp/.gomodcache \
    --user "$(id -u):$(id -g)" \
    "$GO_IMAGE" \
    sh -c 'go build -trimpath -o shelf ./cmd/shelf && go build -trimpath -o shelf-web ./cmd/shelf-web'
fi
[ -x ./shelf ] && [ -x ./shelf-web ] || die "build did not produce ./shelf and ./shelf-web"
ok "Built ./shelf and ./shelf-web"

# ---- start -------------------------------------------------------------------
if [ "$DO_UP" -eq 1 ]; then
  info "Starting the core stack (postgres, redis, rabbitmq, mongodb)"
  docker compose --env-file .env up -d
  ok "Core stack is up"
else
  warn "Skipping 'up' (--no-up). Start it later with: make up"
fi

# ---- first app (optional) ----------------------------------------------------
if [ -n "$FIRST_APP" ] && [ "$DO_UP" -eq 1 ]; then
  info "Provisioning first app: $FIRST_APP"
  ./shelf setup "$FIRST_APP" -s postgres,redis,rabbitmq,mongodb
fi

# ---- done --------------------------------------------------------------------
printf '\n%s%s infra-shelf is ready in %s%s\n' "$BOLD" "$GREEN" "$TARGET_DIR" "$RESET"
cat <<EOF

${BOLD}Next steps${RESET}
  cd ${TARGET_DIR}
  ${DIM}# 1.${RESET} Change the default passwords in .env (see the Security Notes in the README)
  ${DIM}# 2.${RESET} Provision an app:
       ./shelf setup myapp -s postgres,redis,rabbitmq,mongodb
  ${DIM}# 3.${RESET} (optional) start the web UI:  make app   →  http://127.0.0.1:8080

Docs: https://github.com/IvanMicai/infra-shelf/tree/main/docs
EOF
