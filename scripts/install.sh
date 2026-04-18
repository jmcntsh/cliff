#!/usr/bin/env sh
# cliff installer.
#
# Usage:
#   curl -fsSL https://cliff.sh | sh
#
# What it does:
#   1. Detect OS and architecture.
#   2. Download the matching cliff binary from the latest GitHub release.
#   3. Verify the checksum.
#   4. Install to /usr/local/bin (or ~/.local/bin if the former isn't
#      writable, which is the common case on Linux without sudo).
#
# It does NOT:
#   - require sudo unless you explicitly want a system-wide install
#   - touch your shell rc files (the chosen install dir is added to a
#     hint at the end if it's not on PATH; we don't modify ~/.zshrc etc)
#   - phone home or send anything anywhere
#
# Override variables:
#   CLIFF_VERSION   pin a release tag (default: latest)
#   CLIFF_INSTALL_DIR  install destination (default: /usr/local/bin or ~/.local/bin)
#   CLIFF_REPO      override the GitHub repo (default: jmcntsh/cliff)

set -eu

REPO="${CLIFF_REPO:-jmcntsh/cliff}"
VERSION="${CLIFF_VERSION:-latest}"

main() {
  detect_platform
  resolve_version
  resolve_install_dir
  download_and_install
  print_success
}

detect_platform() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin|linux) ;;
    *) die "unsupported OS: $os (cliff supports darwin and linux)" ;;
  esac

  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) die "unsupported arch: $arch (cliff supports amd64 and arm64)" ;;
  esac
}

resolve_version() {
  if [ "$VERSION" = "latest" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
    if [ -z "$VERSION" ]; then
      die "could not resolve latest version (is the repo private or rate-limited?)"
    fi
  fi
  vnum="${VERSION#v}"
}

resolve_install_dir() {
  if [ -n "${CLIFF_INSTALL_DIR:-}" ]; then
    install_dir="$CLIFF_INSTALL_DIR"
    return
  fi
  if [ -w /usr/local/bin ] 2>/dev/null; then
    install_dir="/usr/local/bin"
  else
    install_dir="$HOME/.local/bin"
  fi
}

download_and_install() {
  archive="cliff_${vnum}_${os}_${arch}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${VERSION}/${archive}"
  sums_url="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  echo "Downloading cliff ${VERSION} for ${os}/${arch}..."
  curl -fsSL "$url" -o "$tmp/$archive" \
    || die "download failed: $url"
  curl -fsSL "$sums_url" -o "$tmp/checksums.txt" \
    || die "checksum download failed"

  expected="$(grep "  $archive$" "$tmp/checksums.txt" | awk '{print $1}')"
  if [ -z "$expected" ]; then
    die "could not find checksum for $archive in checksums.txt"
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$tmp/$archive" | awk '{print $1}')"
  else
    actual="$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')"
  fi
  if [ "$expected" != "$actual" ]; then
    die "checksum mismatch (expected $expected, got $actual)"
  fi

  tar -xzf "$tmp/$archive" -C "$tmp"
  mkdir -p "$install_dir"
  mv "$tmp/cliff" "$install_dir/cliff"
  chmod +x "$install_dir/cliff"
}

print_success() {
  echo
  echo "Installed cliff ${VERSION} to ${install_dir}/cliff"
  case ":$PATH:" in
    *":$install_dir:"*)
      echo "Run: cliff"
      ;;
    *)
      echo "Note: ${install_dir} is not on your PATH."
      echo "      Add it to your shell rc, or run: ${install_dir}/cliff"
      ;;
  esac
}

die() {
  echo "error: $*" >&2
  exit 1
}

main
