#!/usr/bin/env bash
# ==============================================================================
# build.sh - Automated Build & Packaging Pipeline for 七喵记账 (com.xsguyu.accounting)
# 
# Features:
#   1. Toolchain discovery & verification (Node.js, pnpm/npm, Go 1.22+, zig, ugcli, Docker)
#   2. Intelligent Go path resolution (PATH, ~/.local/bin, /usr/local/go, /opt/homebrew, Homebrew)
#   3. Containerized Go cross-compilation fallback via Docker if Go is not installed
#   4. Frontend compilation (Vite + Vue 3) -> rootfs_common/www
#   5. Backend Go cross-compilation (CGO_ENABLED=0 Linux amd64/arm64) -> rootfs_*/bin
#   6. 3-tier launcher compilation/generation (zig cc -> Docker gcc -> POSIX #!/bin/sh fallback)
#   7. Package structure validation (ugcli check with internal structural fallback)
#   8. UPK packaging automation (ugcli pack --build <num> --arch <arch>)
#   9. Clean artifacts option (--clean, --clean-only)
#   10. Rich ANSI color logging and POSIX-compliant status codes
# ==============================================================================

set -euo pipefail
umask 0022

# ------------------------------------------------------------------------------
# Constants & Colors
# ------------------------------------------------------------------------------
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_ID="com.xsguyu.accounting"
APP_PORT=21010
REQUIRED_GO_MINOR=22

# ANSI Colors
COLOR_RESET=$'\033[0m'
COLOR_BOLD=$'\033[1m'
COLOR_CYAN=$'\033[36m'
COLOR_GREEN=$'\033[32m'
COLOR_YELLOW=$'\033[33m'
COLOR_RED=$'\033[31m'
COLOR_BLUE=$'\033[34m'
COLOR_GRAY=$'\033[90m'

log_info()    { echo "${COLOR_CYAN}[INFO]${COLOR_RESET} $*"; }
log_step()    { echo -e "\n${COLOR_BOLD}${COLOR_BLUE}=== $* ===${COLOR_RESET}"; }
log_success() { echo "${COLOR_GREEN}✓ [SUCCESS]${COLOR_RESET} $*"; }
log_warn()    { echo "${COLOR_YELLOW}⚠ [WARN]${COLOR_RESET} $*"; }
log_error()   { echo "${COLOR_RED}✗ [ERROR]${COLOR_RESET} $*" >&2; }

# ------------------------------------------------------------------------------
# Help & Usage
# ------------------------------------------------------------------------------
print_usage() {
  echo "${COLOR_BOLD}SevenCats Accounting (七喵记账) - Build & Packaging Script${COLOR_RESET}"
  echo ""
  echo "${COLOR_CYAN}Usage:${COLOR_RESET}"
  echo "  ./build.sh [OPTIONS]"
  echo ""
  echo "${COLOR_CYAN}Options:${COLOR_RESET}"
  echo "  -h, --help            Show this help message and exit"
  echo "  -r, --run             Compile host binary and run local dev server (port 21010)"
  echo "  -c, --clean           Clean previous build artifacts before building"
  echo "      --clean-only      Clean previous build artifacts and exit immediately"
  echo "  -p, --pack            Execute package generation (ugcli pack) after build & check"
  echo "  -b, --build <num>     Set package build number for ugcli pack (default: 1)"
  echo "  -a, --arch <arch>     Set target architecture: all, amd64, arm64 (default: all)"
  echo "      --skip-frontend   Skip building frontend (Vue 3 / Vite)"
  echo "      --skip-backend    Skip cross-compiling backend (Go)"
  echo "      --docker          Force compiling backend using Docker (golang:1.22-alpine)"
  echo ""
  echo "${COLOR_CYAN}Examples:${COLOR_RESET}"
  echo "  ./build.sh                     # Build frontend + backend and validate package structure"
  echo "  ./build.sh --run               # Build host binary and start local server immediately"
  echo "  ./build.sh --run --skip-frontend # Start local server skipping frontend build"
  echo "  ./build.sh --pack              # Build, validate, and generate .upk packages"
  echo "  ./build.sh --pack --build 2    # Build and generate .upk with build number 2"
  echo "  ./build.sh --clean             # Clean artifacts then perform fresh build"
  echo "  ./build.sh --clean-only        # Only clean build artifacts"
  echo "  ./build.sh --skip-frontend     # Only compile backend binaries"
}

# ------------------------------------------------------------------------------
# Argument Parsing
# ------------------------------------------------------------------------------
DO_RUN=false
DO_CLEAN=false
CLEAN_ONLY=false
DO_PACK=false
BUILD_NUM=""
BUILD_NUM_EXPLICIT=false
TARGET_ARCH="all"
SKIP_FRONTEND=false
SKIP_BACKEND=false
FORCE_DOCKER=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      print_usage
      exit 0
      ;;
    -r|--run)
      DO_RUN=true
      shift
      ;;
    -c|--clean)
      DO_CLEAN=true
      shift
      ;;
    --clean-only)
      DO_CLEAN=true
      CLEAN_ONLY=true
      shift
      ;;
    -p|--pack)
      DO_PACK=true
      shift
      ;;
    -b|--build)
      if [[ -z "${2:-}" ]]; then
        log_error "--build requires a build number argument"
        exit 2
      fi
      BUILD_NUM="$2"
      BUILD_NUM_EXPLICIT=true
      shift 2
      ;;
    -a|--arch)
      if [[ -z "${2:-}" ]]; then
        log_error "--arch requires an architecture argument (all|amd64|arm64)"
        exit 2
      fi
      TARGET_ARCH="$2"
      shift 2
      ;;
    --skip-frontend)
      SKIP_FRONTEND=true
      shift
      ;;
    --skip-backend)
      SKIP_BACKEND=true
      shift
      ;;
    --docker)
      FORCE_DOCKER=true
      shift
      ;;
    *)
      log_error "Unknown option: $1"
      print_usage
      exit 2
      ;;
  esac
done

# ------------------------------------------------------------------------------
# Step 0: Clean Artifacts (if requested)
# ------------------------------------------------------------------------------
if [ "$DO_CLEAN" = true ]; then
  log_step "[0/4] Cleaning Build Artifacts"
  if [ -d "$ROOT_DIR/rootfs_common/www" ]; then
    log_info "Cleaning rootfs_common/www/..."
    rm -rf "$ROOT_DIR/rootfs_common/www"/*
  fi
  if [ -d "$ROOT_DIR/rootfs_amd64/bin" ]; then
    log_info "Cleaning rootfs_amd64/bin/..."
    rm -rf "$ROOT_DIR/rootfs_amd64/bin"/*
  fi
  if [ -d "$ROOT_DIR/rootfs_arm64/bin" ]; then
    log_info "Cleaning rootfs_arm64/bin/..."
    rm -rf "$ROOT_DIR/rootfs_arm64/bin"/*
  fi
  if [ -d "$ROOT_DIR/frontend/dist" ]; then
    log_info "Cleaning frontend/dist/..."
    rm -rf "$ROOT_DIR/frontend/dist"
  fi
  if [ -d "$ROOT_DIR/build_dir" ]; then
    log_info "Cleaning build_dir/..."
    rm -rf "$ROOT_DIR/build_dir"
  fi
  if [ -d "$ROOT_DIR/.check-app" ]; then
    log_info "Cleaning .check-app/..."
    rm -rf "$ROOT_DIR/.check-app"
  fi
  log_success "Clean completed successfully."
  if [ "$CLEAN_ONLY" = true ]; then
    exit 0
  fi
fi

# Ensure icon.png is synchronized from cat7-logo.png
if [ -f "$ROOT_DIR/cat7-logo.png" ]; then
  if [ ! -f "$ROOT_DIR/rootfs_common/icon.png" ] || [ "$ROOT_DIR/cat7-logo.png" -nt "$ROOT_DIR/rootfs_common/icon.png" ]; then
    log_info "Synchronizing rootfs_common/icon.png from cat7-logo.png..."
    python3 "$ROOT_DIR/scripts/generate_icon.py"
  fi
elif [ ! -f "$ROOT_DIR/rootfs_common/icon.png" ]; then
  log_info "Generating rootfs_common/icon.png..."
  if [ -f "$ROOT_DIR/scripts/generate_icon.py" ] && command -v python3 &>/dev/null; then
    python3 "$ROOT_DIR/scripts/generate_icon.py"
  else
    log_warn "scripts/generate_icon.py or python3 not available. Please ensure rootfs_common/icon.png exists."
  fi
fi

# Sync logo to frontend assets and public
mkdir -p "$ROOT_DIR/frontend/public" "$ROOT_DIR/frontend/src/assets"
if [ -f "$ROOT_DIR/rootfs_common/icon.png" ]; then
  cp "$ROOT_DIR/rootfs_common/icon.png" "$ROOT_DIR/frontend/public/icon.png"
  cp "$ROOT_DIR/rootfs_common/icon.png" "$ROOT_DIR/frontend/src/assets/logo.png"
fi

# ------------------------------------------------------------------------------
# Step 1: Toolchain Discovery & Verification
# ------------------------------------------------------------------------------
log_step "[1/4] Discovering & Verifying Toolchains"

# 1.1 Node.js & Package Manager (Frontend)
NODE_BIN=""
PKG_MGR=""
if [ "$SKIP_FRONTEND" = false ]; then
  if command -v node &>/dev/null; then
    NODE_BIN="$(command -v node)"
    log_info "Found Node.js: $("$NODE_BIN" -v) (${NODE_BIN})"
  else
    log_error "Node.js not found in PATH."
    log_info "Please install Node.js (v18+) via https://nodejs.org or 'brew install node'."
    exit 1
  fi

  if command -v pnpm &>/dev/null; then
    PKG_MGR="pnpm"
    log_info "Found package manager: pnpm ($(pnpm -v))"
  elif command -v npm &>/dev/null; then
    PKG_MGR="npm"
    log_warn "pnpm not found, falling back to npm ($(npm -v))"
  else
    log_error "Neither pnpm nor npm found in PATH."
    log_info "Please install pnpm ('npm install -g pnpm' or 'brew install pnpm')."
    exit 1
  fi
fi

# 1.2 Go Compiler (Backend)
GO_BIN=""
USE_DOCKER=false
if [ "$SKIP_BACKEND" = false ]; then
  if [ "$FORCE_DOCKER" = true ]; then
    if command -v docker &>/dev/null && docker info &>/dev/null; then
      USE_DOCKER=true
      log_info "Forcing Docker Go compilation container (golang:1.26-alpine)"
    else
      log_error "--docker flag requested, but Docker is not installed or daemon is not running."
      exit 1
    fi
  else
    # Check PATH first
    if command -v go &>/dev/null; then
      GO_BIN="$(command -v go)"
    else
      # Probe common Go install paths
      COMMON_GO_PATHS=(
        "$HOME/.local/bin/go"
        "$HOME/.local/go/bin/go"
        "/usr/local/go/bin/go"
        "/opt/homebrew/bin/go"
        "/usr/local/bin/go"
        "$HOME/go/bin/go"
        "$HOME/.go/bin/go"
        "/usr/bin/go"
      )
      for candidate in "${COMMON_GO_PATHS[@]}"; do
        if [ -x "$candidate" ]; then
          GO_BIN="$candidate"
          export PATH="$(dirname "$candidate"):$PATH"
          break
        fi
      done
    fi

    if [ -n "$GO_BIN" ]; then
      GO_RAW_VER="$("$GO_BIN" version | awk '{print $3}')"
      log_info "Found Go compiler: ${GO_BIN} (${GO_RAW_VER})"

      # Version validation (>= 1.22)
      GO_NUM_VER="${GO_RAW_VER#go}"
      GO_MAJOR="$(echo "$GO_NUM_VER" | cut -d. -f1)"
      GO_MINOR="$(echo "$GO_NUM_VER" | cut -d. -f2)"
      if [ -n "$GO_MAJOR" ] && [ -n "$GO_MINOR" ]; then
        if [ "$GO_MAJOR" -lt 1 ] || { [ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt "$REQUIRED_GO_MINOR" ]; }; then
          log_warn "Detected Go version ${GO_RAW_VER} is below Go 1.${REQUIRED_GO_MINOR}+. Modern pure-Go SQLite and routing features may require Go 1.22+."
        fi
      fi
    else
      # Check if Docker is available as fallback
      if command -v docker &>/dev/null && docker info &>/dev/null; then
        USE_DOCKER=true
        log_warn "Go compiler not found on host, but Docker daemon is running."
        log_info "Falling back to containerized cross-compilation with golang:1.22-alpine."
      else
        log_error "Go compiler (Go 1.22+) not found in PATH or standard locations."
        log_info "Please install Go 1.22+ via one of the following:"
        log_info "  - macOS: brew install go"
        log_info "  - Official installer: https://go.dev/dl/"
        log_info "  - Linux: sudo apt install golang / sudo dnf install golang"
        log_info "Alternatively, start Docker to enable automatic containerized cross-compilation."
        exit 1
      fi
    fi
  fi
fi

# 1.3 UGOS CLI (ugcli)
UGCLI_BIN=""
if command -v ugcli &>/dev/null; then
  UGCLI_BIN="$(command -v ugcli)"
  log_info "Found ugcli: ${UGCLI_BIN} ($("$UGCLI_BIN" -v 2>/dev/null || echo 'installed'))"
elif [ -x "$HOME/.local/bin/ugcli" ]; then
  UGCLI_BIN="$HOME/.local/bin/ugcli"
  log_info "Found ugcli: ${UGCLI_BIN} ($("$UGCLI_BIN" -v 2>/dev/null || echo 'installed'))"
elif [ -x "/usr/local/bin/ugcli" ]; then
  UGCLI_BIN="/usr/local/bin/ugcli"
  log_info "Found ugcli: /usr/local/bin/ugcli"
else
  log_warn "ugcli not found. ugcli check and packaging (.upk) will require ugcli."
  log_info "See official documentation: https://developer.ugnas.com/doc/tools/ugcli.html"
fi

# ------------------------------------------------------------------------------
# Step 2: Frontend Build
# ------------------------------------------------------------------------------
if [ "$SKIP_FRONTEND" = true ]; then
  log_step "[2/4] Skipping Frontend Build (--skip-frontend)"
else
  log_step "[2/4] Building Frontend (Vue 3 + Vite + Tailwind CSS)"
  if [ ! -d "$ROOT_DIR/frontend" ]; then
    log_error "Frontend directory not found: $ROOT_DIR/frontend"
    exit 1
  fi

  cd "$ROOT_DIR/frontend"
  log_info "Installing dependencies with ${PKG_MGR}..."
  if [ "$PKG_MGR" = "pnpm" ]; then
    pnpm install --prefer-offline || pnpm install
    log_info "Compiling frontend assets with pnpm run build..."
    pnpm run build
  else
    npm install
    log_info "Compiling frontend assets with npm run build..."
    npm run build
  fi

  # Ensure rootfs_common/www exists
  mkdir -p "$ROOT_DIR/rootfs_common/www"

  # If build outputs to frontend/dist, copy to rootfs_common/www
  if [ -d "$ROOT_DIR/frontend/dist" ]; then
    log_info "Copying compiled dist files to rootfs_common/www/..."
    rm -rf "$ROOT_DIR/rootfs_common/www"/*
    cp -r "$ROOT_DIR/frontend/dist"/* "$ROOT_DIR/rootfs_common/www/"
  fi

  # Verify www/index.html
  if [ ! -f "$ROOT_DIR/rootfs_common/www/index.html" ]; then
    log_error "Frontend build failed to produce rootfs_common/www/index.html!"
    exit 1
  fi

  ASSET_COUNT="$(find "$ROOT_DIR/rootfs_common/www" -type f | wc -l | tr -d ' ')"
  TOTAL_SIZE="$(du -sh "$ROOT_DIR/rootfs_common/www" | awk '{print $1}')"
  log_success "Frontend built successfully: ${ASSET_COUNT} files (${TOTAL_SIZE}) in rootfs_common/www"
  cd "$ROOT_DIR"
fi

# ------------------------------------------------------------------------------
# Step 3: Backend Compilation (Host & Cross-Compilation)
# ------------------------------------------------------------------------------
if [ "$SKIP_BACKEND" = true ]; then
  log_step "[3/4] Skipping Backend Compilation (--skip-backend)"
else
  log_step "[3/4] Compiling Backend Service (pure-Go, CGO_ENABLED=0)"
  if [ ! -d "$ROOT_DIR/backend" ]; then
    log_error "Backend directory not found: $ROOT_DIR/backend"
    exit 1
  fi

  write_posix_launcher() {
    local target="$1"
    cat > "$target" <<'EOF'
#!/bin/sh
unset GODEBUG
BIN_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
APP_ROOT="$(dirname "$BIN_DIR")"
cd "$APP_ROOT" || exit 1
[ -x "$BIN_DIR/accounting_core" ] || chmod +x "$BIN_DIR/accounting_core" 2>/dev/null || true
exec "$BIN_DIR/accounting_core" "$@"
EOF
    chmod +x "$target"
  }

  compile_native_launchers() {
    local zig_bin=""
    if command -v zig &>/dev/null; then
      zig_bin="$(command -v zig)"
    else
      for candidate in "$HOME/.local/bin/zig" "/opt/homebrew/bin/zig" "/usr/local/bin/zig"; do
        if [ -x "$candidate" ]; then
          zig_bin="$candidate"
          break
        fi
      done
    fi

    if [ -n "$zig_bin" ]; then
      log_info "Compiling native static ELF launchers (bin/accounting_serv) via zig (${zig_bin})..."
      "$zig_bin" cc -target x86_64-linux-musl -static -O2 -s "$ROOT_DIR/backend/cmd/launcher/main.c" -o "$ROOT_DIR/rootfs_amd64/bin/accounting_serv"
      "$zig_bin" cc -target aarch64-linux-musl -static -O2 -s "$ROOT_DIR/backend/cmd/launcher/main.c" -o "$ROOT_DIR/rootfs_arm64/bin/accounting_serv"
      chmod 0755 "$ROOT_DIR/rootfs_amd64/bin/accounting_serv" "$ROOT_DIR/rootfs_arm64/bin/accounting_serv"
    elif command -v docker &>/dev/null && docker info &>/dev/null; then
      log_info "Compiling native static ELF launchers (bin/accounting_serv) via Docker..."
      docker run --rm -v "$ROOT_DIR":/work -w /work debian:bookworm-slim sh -c "
        apt-get update -qq && apt-get install -qq -y gcc gcc-aarch64-linux-gnu libc6-dev-arm64-cross >/dev/null 2>&1 &&
        gcc -static -O2 -s backend/cmd/launcher/main.c -o rootfs_amd64/bin/accounting_serv &&
        aarch64-linux-gnu-gcc -static -O2 -s backend/cmd/launcher/main.c -o rootfs_arm64/bin/accounting_serv
      "
      chmod 0755 "$ROOT_DIR/rootfs_amd64/bin/accounting_serv" "$ROOT_DIR/rootfs_arm64/bin/accounting_serv"
    elif [ "$DO_PACK" = true ] && [ "${ALLOW_POSIX_LAUNCHER:-0}" != "1" ]; then
      log_error "Neither zig nor Docker daemon available; refusing to pack .upk with POSIX shell fallback launcher."
      log_info "Install zig (e.g. 'brew install zig' or ~/.local/bin/zig) or start Docker so bin/accounting_serv is compiled as a static ELF binary."
      exit 1
    else
      log_warn "Neither zig nor Docker daemon available; generating zero-dependency POSIX #!/bin/sh launchers..."
      write_posix_launcher "$ROOT_DIR/rootfs_amd64/bin/accounting_serv"
      write_posix_launcher "$ROOT_DIR/rootfs_arm64/bin/accounting_serv"
    fi
  }

  # Always ensure local host binary is built when running locally or missing
  mkdir -p "$ROOT_DIR/bin"
  if [ "$DO_RUN" = true ] || [ ! -f "$ROOT_DIR/bin/accounting_core" ]; then
    HOST_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    HOST_ARCH="$(uname -m)"
    case "$HOST_ARCH" in
      x86_64) HOST_ARCH="amd64" ;;
      arm64|aarch64) HOST_ARCH="arm64" ;;
    esac
    log_info "Compiling host binary for ${HOST_OS}/${HOST_ARCH} ($ROOT_DIR/bin/accounting_core)..."
    cd "$ROOT_DIR/backend"
    "$GO_BIN" build -trimpath -ldflags="-s -w" -o "$ROOT_DIR/bin/accounting_core" ./cmd/server
    cd "$ROOT_DIR"
    chmod +x "$ROOT_DIR/bin/accounting_core"
    if command -v clang &>/dev/null; then
      clang -O2 "$ROOT_DIR/backend/cmd/launcher/main.c" -o "$ROOT_DIR/bin/accounting_serv"
    elif command -v gcc &>/dev/null; then
      gcc -O2 "$ROOT_DIR/backend/cmd/launcher/main.c" -o "$ROOT_DIR/bin/accounting_serv"
    else
      log_warn "Neither clang nor gcc found; generating POSIX #!/bin/sh launcher for host..."
      write_posix_launcher "$ROOT_DIR/bin/accounting_serv"
    fi
    chmod +x "$ROOT_DIR/bin/accounting_serv"
    HOST_SIZE="$(du -sh "$ROOT_DIR/bin/accounting_core" | awk '{print $1}')"
    log_success "Host binary & launcher built: bin/accounting_serv (${HOST_SIZE})"
  fi

  # Cross-compile for Linux amd64 and arm64
  mkdir -p "$ROOT_DIR/rootfs_amd64/bin"
  mkdir -p "$ROOT_DIR/rootfs_arm64/bin"

  AMD64_BIN="$ROOT_DIR/rootfs_amd64/bin/accounting_core"
  ARM64_BIN="$ROOT_DIR/rootfs_arm64/bin/accounting_core"
  AMD64_OUT="$ROOT_DIR/rootfs_amd64/bin/accounting_serv"
  ARM64_OUT="$ROOT_DIR/rootfs_arm64/bin/accounting_serv"

  # Fast path for --run: skip linux cross-compilation if already present and not packing
  if [ "$DO_RUN" = true ] && [ "$DO_PACK" = false ] && [ -x "$AMD64_OUT" ] && [ -x "$ARM64_OUT" ]; then
    log_info "Existing Linux target binaries found. Skipping Linux cross-compilation for local run."
  else
    if [ "$USE_DOCKER" = true ]; then
      log_info "Compiling Linux amd64 and arm64 binaries using Docker..."
      mkdir -p "$HOME/go/pkg/mod" 2>/dev/null || true
      docker run --rm \
        -v "$ROOT_DIR/backend":/src \
        -v "$ROOT_DIR/rootfs_amd64/bin":/out/amd64 \
        -v "$ROOT_DIR/rootfs_arm64/bin":/out/arm64 \
        -v "$HOME/go/pkg/mod":/go/pkg/mod \
        -w /src \
        golang:1.26-alpine \
        sh -c '
          set -e
          echo "Building Linux amd64..."
          CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/amd64/accounting_core ./cmd/server
          echo "Building Linux arm64..."
          CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o /out/arm64/accounting_core ./cmd/server
        '
    else
      cd "$ROOT_DIR/backend"
      log_info "Compiling Linux amd64 binary..."
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$GO_BIN" build \
        -trimpath \
        -ldflags="-s -w" \
        -o "$AMD64_BIN" \
        ./cmd/server

      log_info "Compiling Linux arm64 binary..."
      CGO_ENABLED=0 GOOS=linux GOARCH=arm64 "$GO_BIN" build \
        -trimpath \
        -ldflags="-s -w" \
        -o "$ARM64_BIN" \
        ./cmd/server
      cd "$ROOT_DIR"
    fi

    chmod 0755 "$AMD64_BIN" "$ARM64_BIN"
    compile_native_launchers

    # Validate binaries and launchers
    if [ ! -f "$AMD64_BIN" ] || [ ! -f "$ARM64_BIN" ] || [ ! -x "$AMD64_OUT" ] || [ ! -x "$ARM64_OUT" ]; then
      log_error "Cross-compilation failed to produce target binaries or launchers!"
      exit 1
    fi

    AMD64_SIZE="$(du -sh "$AMD64_BIN" | awk '{print $1}')"
    ARM64_SIZE="$(du -sh "$ARM64_BIN" | awk '{print $1}')"
    log_success "Backend amd64 built: ${AMD64_OUT} -> ${AMD64_BIN} (${AMD64_SIZE})"
    log_success "Backend arm64 built: ${ARM64_OUT} -> ${ARM64_BIN} (${ARM64_SIZE})"
  fi
fi

normalize_rootfs_permissions() {
  log_info "Normalizing rootfs permissions (dirs 0755, files 0644, bins 0755)..."
  for dir in "$ROOT_DIR/rootfs_common" "$ROOT_DIR/rootfs_amd64" "$ROOT_DIR/rootfs_arm64"; do
    if [ -d "$dir" ]; then
      find "$dir" -type d -exec chmod 0755 {} +
      find "$dir" -type f -exec chmod 0644 {} +
    fi
  done
  for bin in "$ROOT_DIR/rootfs_amd64/bin/accounting_core" \
             "$ROOT_DIR/rootfs_amd64/bin/accounting_serv" \
             "$ROOT_DIR/rootfs_amd64/bin/ppu-paddle-ocr" \
             "$ROOT_DIR/rootfs_arm64/bin/accounting_core" \
             "$ROOT_DIR/rootfs_arm64/bin/accounting_serv"; do
    if [ -f "$bin" ]; then
      chmod 0755 "$bin"
    fi
  done
}

# 本地 OCR（RapidOCR）：把 third_party/rapidocr 下的程序和模型拷进 rootfs。
# 程序是 amd64 的，只进 rootfs_amd64；模型是架构无关的，进 rootfs_common。
# 没下载时跳过（本地 OCR 不可用，不影响打包）；CI 里先跑 scripts/download-rapidocr.sh。
stage_rapidocr_files() {
  local src_bin="$ROOT_DIR/third_party/rapidocr/ppu-paddle-ocr"
  local src_models="$ROOT_DIR/third_party/rapidocr/models"
  if [ ! -f "$src_bin" ]; then
    log_warn "RapidOCR 程序未找到（third_party/rapidocr/ppu-paddle-ocr），跳过本地 OCR 打包；先跑 scripts/download-rapidocr.sh 下载。"
    return 0
  fi
  if ! is_elf_binary "$src_bin"; then
    log_error "RapidOCR 程序 $src_bin 不是原生 ELF 文件，拒绝打包。"
    exit 1
  fi
  for m in PP-OCRv6_small_det.ort PP-OCRv6_small_rec.ort ppocrv6_dict.txt; do
    if [ ! -f "$src_models/$m" ]; then
      log_error "RapidOCR 模型缺失：$src_models/$m（先跑 scripts/download-rapidocr.sh）"
      exit 1
    fi
  done
  mkdir -p "$ROOT_DIR/rootfs_amd64/bin" "$ROOT_DIR/rootfs_common/ocr/models"
  cp "$src_bin" "$ROOT_DIR/rootfs_amd64/bin/ppu-paddle-ocr"
  cp "$src_models/PP-OCRv6_small_det.ort" "$src_models/PP-OCRv6_small_rec.ort" "$src_models/ppocrv6_dict.txt" "$ROOT_DIR/rootfs_common/ocr/models/"
  log_success "RapidOCR 已打进安装包：bin/ppu-paddle-ocr + ocr/models/（PP-OCRv6 small，约 31MB 模型）"
}

is_elf_binary() {
  local f="$1"
  [ -f "$f" ] && [ "$(od -An -tx1 -N4 "$f" 2>/dev/null | tr -d ' \n')" = "7f454c46" ]
}

# ------------------------------------------------------------------------------
# Step 4: UGOS Pro Package Validation & Packaging
# ------------------------------------------------------------------------------
if [ "$DO_RUN" = true ] && [ "$DO_PACK" = false ]; then
  log_info "Skipping package validation for direct local run."
else
  log_step "[4/4] Package Structure Validation & Packaging"
  stage_rapidocr_files
  normalize_rootfs_permissions

  if [ "$DO_PACK" = true ] && [ "${ALLOW_POSIX_LAUNCHER:-0}" != "1" ]; then
    CHECK_ARCHS=()
    if [ "$TARGET_ARCH" = "all" ] || [ "$TARGET_ARCH" = "amd64" ]; then
      CHECK_ARCHS+=("amd64")
    fi
    if [ "$TARGET_ARCH" = "all" ] || [ "$TARGET_ARCH" = "arm64" ]; then
      CHECK_ARCHS+=("arm64")
    fi
    for arch in "${CHECK_ARCHS[@]}"; do
      launcher="$ROOT_DIR/rootfs_${arch}/bin/accounting_serv"
      if ! is_elf_binary "$launcher"; then
        log_error "Launcher $launcher is not a native ELF binary! Refusing to pack broken .upk."
        exit 1
      fi
    done
  fi

  # 4.1 Check package structure
  if [ -n "$UGCLI_BIN" ]; then
    log_info "Running ugcli check..."
    cd "$ROOT_DIR"
    if "$UGCLI_BIN" check; then
      log_success "ugcli check passed: package layout and project.yaml are valid."
    else
      log_error "ugcli check failed! Review errors above."
      exit 1
    fi
  else
    log_info "Running internal layout verification..."
    ERRORS=0
    if [ ! -f "$ROOT_DIR/project.yaml" ]; then
      log_error "Missing project.yaml in repository root"
      ERRORS=$((ERRORS + 1))
    fi
    if [ ! -f "$ROOT_DIR/rootfs_common/icon.png" ]; then
      log_error "Missing rootfs_common/icon.png"
      ERRORS=$((ERRORS + 1))
    fi
    if [ ! -f "$ROOT_DIR/rootfs_common/www/index.html" ]; then
      log_error "Missing rootfs_common/www/index.html"
      ERRORS=$((ERRORS + 1))
    fi
    if [ ! -x "$ROOT_DIR/rootfs_amd64/bin/accounting_serv" ]; then
      log_error "Missing or non-executable rootfs_amd64/bin/accounting_serv"
      ERRORS=$((ERRORS + 1))
    fi
    if [ ! -x "$ROOT_DIR/rootfs_arm64/bin/accounting_serv" ]; then
      log_error "Missing or non-executable rootfs_arm64/bin/accounting_serv"
      ERRORS=$((ERRORS + 1))
    fi

    if [ "$ERRORS" -gt 0 ]; then
      log_error "Internal package layout check failed with ${ERRORS} errors."
      exit 1
    fi
    log_success "Internal package layout check passed."
  fi

  # 4.2 Packaging (if --pack flag provided)
  if [ "$DO_PACK" = true ]; then
    if [ -z "$UGCLI_BIN" ]; then
      log_error "Packaging requested (--pack) but ugcli is not installed."
      exit 1
    fi

    # Read current version from project.yaml
    APP_VERSION="$(grep '^version:' "$ROOT_DIR/project.yaml" | head -n1 | awk '{print $2}' | tr -d '"' | tr -d "'")"
    if [ -z "$APP_VERSION" ]; then
      APP_VERSION="0.2.0"
    fi

    # Auto-increment build number if not explicitly specified via -b / --build
    if [ "$BUILD_NUM_EXPLICIT" = false ] || [ -z "$BUILD_NUM" ]; then
      HIGHEST_BUILD=0
      if [ -d "$ROOT_DIR/build_dir/pkgs/upk" ]; then
        for f in "$ROOT_DIR/build_dir/pkgs/upk"/*_${APP_ID}_${APP_VERSION}.*.upk; do
          if [ -f "$f" ]; then
            base_name="$(basename "$f" .upk)"
            raw_bnum="${base_name##*.}"
            clean_bnum="$((10#$raw_bnum))"
            if [ "$clean_bnum" -gt "$HIGHEST_BUILD" ]; then
              HIGHEST_BUILD="$clean_bnum"
            fi
          fi
        done
      fi
      BUILD_NUM=$((HIGHEST_BUILD + 1))
      log_info "Auto-detected/incremented build number for v${APP_VERSION}: ${BUILD_NUM} (version -> ${APP_VERSION}.$(printf "%04d" "$BUILD_NUM"))"
    fi

    log_info "Executing ugcli pack (version: ${APP_VERSION}, build: ${BUILD_NUM}, arch: ${TARGET_ARCH})..."
    cd "$ROOT_DIR"
    "$UGCLI_BIN" pack --build "$BUILD_NUM" --arch "$TARGET_ARCH"

    log_step "Packaging Summary"
    if [ -d "$ROOT_DIR/build_dir/pkgs/upk" ]; then
      PKG_PATTERN="*.upk"
      if [ "$TARGET_ARCH" = "amd64" ]; then
        PKG_PATTERN="amd64_*.upk"
      elif [ "$TARGET_ARCH" = "arm64" ]; then
        PKG_PATTERN="arm64_*.upk"
      fi
      for upk in "$ROOT_DIR/build_dir/pkgs/upk"/$PKG_PATTERN; do
        if [ -f "$upk" ]; then
          log_success "Package: $(basename "$upk") ($(du -sh "$upk" | awk '{print $1}'))"
        fi
      done
      log_info "Output directory: $ROOT_DIR/build_dir/pkgs/upk/"
    else
      log_warn "Pack command finished, but build_dir/pkgs/upk was not found."
    fi
  else
    echo ""
    log_info "Build and validation complete! UPK packaging was skipped."
    log_info "To generate UGOS Pro .upk packages, run:"
    echo -e "    ${COLOR_BOLD}./build.sh --pack [--build <num>] [--arch <all|amd64|arm64>]${COLOR_RESET}\n"
  fi
fi

log_success "Build pipeline finished successfully!"

# ------------------------------------------------------------------------------
# Step 5: Local Execution (if --run flag provided)
# ------------------------------------------------------------------------------
if [ "$DO_RUN" = true ]; then
  log_step "Starting QiMiao Accounting (七喵记账) Local Server"
  echo -e "${COLOR_GREEN}✓ Serving at http://localhost:${APP_PORT}${COLOR_RESET}"
  echo -e "${COLOR_CYAN}  • Web App:     http://localhost:${APP_PORT}${COLOR_RESET}"
  echo -e "${COLOR_CYAN}  • API Health:  http://localhost:${APP_PORT}/api/v1/health${COLOR_RESET}"
  echo -e "${COLOR_GRAY}  • Environment: development (Mock Guest Mode)${COLOR_RESET}"
  echo -e "${COLOR_GRAY}  • Static Dir:  $ROOT_DIR/rootfs_common/www${COLOR_RESET}"
  echo -e "${COLOR_YELLOW}Press Ctrl+C to terminate the server.${COLOR_RESET}\n"

  export APP_ENV=development
  export PORT="${APP_PORT}"
  export STATIC_DIR="$ROOT_DIR/rootfs_common/www"
  exec "$ROOT_DIR/bin/accounting_serv"
fi
