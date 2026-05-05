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
  # Prefer a writable directory that's already on PATH so the user can
  # type `cliff` immediately. Order:
  #   1. /usr/local/bin            — on PATH on every Mac and most Linux
  #   2. /opt/homebrew/bin         — on PATH for Apple Silicon Homebrew
  #                                   users (where /usr/local/bin isn't
  #                                   writable without sudo)
  #   3. $HOME/.local/bin          — last resort; not on PATH by default
  for candidate in /usr/local/bin /opt/homebrew/bin "$HOME/.local/bin"; do
    if [ -d "$candidate" ] && [ -w "$candidate" ]; then
      install_dir="$candidate"
      return
    fi
  done
  # Nothing existed-and-writable; create ~/.local/bin and use that.
  install_dir="$HOME/.local/bin"
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

  write_install_state
}

# write_install_state drops a tiny breadcrumb at ~/.cliff/install.json
# so `cliff self-uninstall` knows where this script put the binary,
# without re-deriving (and possibly drifting from) the candidate-path
# list above. Best-effort: a write failure must not fail the install,
# self-uninstall falls back to os.Executable() when the file's missing.
write_install_state() {
  state_dir="$HOME/.cliff"
  state_file="$state_dir/install.json"
  mkdir -p "$state_dir" 2>/dev/null || return 0
  # Hand-rolled JSON keeps us free of jq; the three values are all
  # path/version strings under our control, no quoting hazards.
  printf '{"install_dir":"%s","install_method":"script","version":"%s"}\n' \
    "$install_dir" "$VERSION" >"$state_file" 2>/dev/null || return 0
}

print_success() {
  echo
  case ":$PATH:" in
    *":$install_dir:"*)
      echo "Installed cliff ${VERSION} to ${install_dir}/cliff"
      echo "Run: cliff"
      ;;
    *)
      # Lead with the warning so it isn't mistaken for plain success.
      # Suggest the exact rc-file line for the user's current shell so
      # the fix is copy-pasteable.
      shell_name="$(basename "${SHELL:-sh}")"
      case "$shell_name" in
        zsh)  rc_file="~/.zshrc" ;;
        bash) rc_file="~/.bashrc" ;;
        fish) rc_file="~/.config/fish/config.fish" ;;
        *)    rc_file="your shell's rc file" ;;
      esac
      echo "Installed cliff ${VERSION} to ${install_dir}/cliff"
      echo
      echo "WARNING: ${install_dir} is not on your PATH, so typing 'cliff' will not work yet."
      echo
      echo "To run cliff right now:"
      echo "  ${install_dir}/cliff"
      echo
      if [ "$shell_name" = "fish" ]; then
        echo "To make 'cliff' work in new shells, add this to ${rc_file}:"
        echo "  fish_add_path ${install_dir}"
      else
        echo "To make 'cliff' work in new shells, add this to ${rc_file}:"
        echo "  export PATH=\"${install_dir}:\$PATH\""
      fi
      ;;
  esac
}

die() {
  echo "error: $*" >&2
  exit 1
}

main
