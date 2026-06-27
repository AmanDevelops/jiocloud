#!/usr/bin/env sh
#
# jiocloud installer (Linux).
#
# Downloads the latest jiocloud release for your CPU architecture, verifies its
# checksum, and installs the binary into ~/.local/bin (override with
# JIOCLOUD_INSTALL_DIR). Pin a version with JIOCLOUD_VERSION=vX.Y.Z.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/AmanDevelops/jiocloud/master/install.sh | sh
#   sh install.sh
#
set -eu

REPO="AmanDevelops/jiocloud"
BIN="jiocloud"
INSTALL_DIR="${JIOCLOUD_INSTALL_DIR:-$HOME/.local/bin}"

err() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }

# --- platform checks --------------------------------------------------------
[ "$(uname -s)" = "Linux" ] || err "this installer supports Linux only (got $(uname -s))"

case "$(uname -m)" in
  x86_64 | amd64)  ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) err "unsupported architecture: $(uname -m) (released: amd64, arm64)" ;;
esac

command -v curl >/dev/null 2>&1 || err "curl is required"
command -v tar  >/dev/null 2>&1 || err "tar is required"

# --- resolve version --------------------------------------------------------
VERSION="${JIOCLOUD_VERSION:-}"
if [ -z "$VERSION" ]; then
  info "Resolving latest release..."
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -m1 '"tag_name"' | cut -d '"' -f4)
fi
[ -n "$VERSION" ] || err "could not determine the release version (is there a published release?)"

ASSET="jiocloud_${VERSION}_linux_${ARCH}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"

# --- download ---------------------------------------------------------------
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

info "Downloading $ASSET ($VERSION)..."
curl -fsSL "$BASE/$ASSET" -o "$TMP/$ASSET" \
  || err "download failed: $BASE/$ASSET"

# --- verify checksum (best effort) -----------------------------------------
if curl -fsSL "$BASE/checksums.txt" -o "$TMP/checksums.txt" 2>/dev/null; then
  if command -v sha256sum >/dev/null 2>&1; then
    expected=$(grep " $ASSET\$" "$TMP/checksums.txt" | awk '{print $1}')
    actual=$(sha256sum "$TMP/$ASSET" | awk '{print $1}')
    [ -n "$expected" ] || err "no checksum entry for $ASSET"
    [ "$expected" = "$actual" ] || err "checksum mismatch for $ASSET"
    info "Checksum verified."
  else
    info "sha256sum not found; skipping checksum verification."
  fi
else
  info "checksums.txt not published; skipping checksum verification."
fi

# --- install ----------------------------------------------------------------
tar -C "$TMP" -xzf "$TMP/$ASSET" || err "failed to extract $ASSET"
[ -f "$TMP/$BIN" ] || err "archive did not contain '$BIN'"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP/$BIN" "$INSTALL_DIR/$BIN"
info "Installed $BIN $VERSION -> $INSTALL_DIR/$BIN"

# --- PATH handling ----------------------------------------------------------
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    info "$INSTALL_DIR is already on your PATH. Run: $BIN version"
    exit 0
    ;;
esac

LINE="export PATH=\"$INSTALL_DIR:\$PATH\""

# Pick the most likely shell rc file.
case "$(basename "${SHELL:-sh}")" in
  zsh)  RC="$HOME/.zshrc" ;;
  bash) RC="$HOME/.bashrc" ;;
  *)    RC="$HOME/.profile" ;;
esac

add_to_rc() {
  if ! grep -qsF "$LINE" "$RC" 2>/dev/null; then
    printf '\n# Added by the jiocloud installer\n%s\n' "$LINE" >> "$RC"
  fi
  info "Added to $RC. Run:  . \"$RC\"   (or open a new terminal)"
}

info ""
info "$INSTALL_DIR is not on your PATH."

# Only prompt when attached to a terminal; a piped 'curl | sh' is non-interactive.
if [ -t 0 ]; then
  printf "Add it to %s now? [y/N] " "$RC"
  read -r ans || ans=""
  case "$ans" in
    y | Y | yes | YES) add_to_rc ;;
    *) info "Skipped. Add this line to your shell profile manually:"; info "  $LINE" ;;
  esac
else
  info "To use '$BIN', add this line to your shell profile:"
  info "  $LINE"
fi
