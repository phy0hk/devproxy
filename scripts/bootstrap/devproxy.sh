#!/usr/bin/env sh
set -eu

BOOTSTRAP_CHANNEL="v1"
BASE_URL="${DEVPROXY_DOWNLOAD_BASE_URL:-https://github.com/phy0hk/devproxy/releases/download/$BOOTSTRAP_CHANNEL}"
CACHE_DIR=".devproxy"
BIN_DIR="$CACHE_DIR/$BOOTSTRAP_CHANNEL/bin"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  linux) os="linux" ;;
  darwin) os="darwin" ;;
  *) echo "unsupported os: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

asset="devproxy-$os-$arch"
bin="$BIN_DIR/$asset"

mkdir -p "$BIN_DIR"

if [ ! -f .gitignore ]; then
  printf '.devproxy/\n' > .gitignore
elif ! grep -qxF '.devproxy/' .gitignore && ! grep -qxF '.devproxy' .gitignore; then
  printf '\n.devproxy/\n' >> .gitignore
fi

fetch() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
  else
    echo "curl or wget is required to download devproxy" >&2
    exit 1
  fi
}

should_download=0
expected=""
checksums="$CACHE_DIR/$BOOTSTRAP_CHANNEL/checksums.txt"

if [ "${DEVPROXY_SKIP_CHECKSUM:-}" != "1" ]; then
  fetch "$BASE_URL/checksums.txt" "$checksums"
  expected="$(awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print $1; exit }' "$checksums")"
  if [ -z "$expected" ]; then
    echo "no checksum found for $asset" >&2
    exit 1
  fi
fi

if [ ! -x "$bin" ]; then
  should_download=1
elif [ "${DEVPROXY_SKIP_CHECKSUM:-}" != "1" ]; then
  actual="$(sha256sum "$bin" | awk '{ print $1 }')"
  if [ "$actual" != "$expected" ]; then
    echo "devproxy $BOOTSTRAP_CHANNEL binary is outdated, updating" >&2
    should_download=1
  fi
fi

if [ "$should_download" = "1" ]; then
  tmp="$bin.tmp"
  echo "downloading $BASE_URL/$asset" >&2
  fetch "$BASE_URL/$asset" "$tmp"

  if [ "${DEVPROXY_SKIP_CHECKSUM:-}" != "1" ]; then
    actual="$(sha256sum "$tmp" | awk '{ print $1 }')"
    if [ "$actual" != "$expected" ]; then
      echo "checksum mismatch for $asset" >&2
      rm -f "$tmp"
      exit 1
    fi
  else
    echo "warning: checksum verification skipped" >&2
  fi

  mv "$tmp" "$bin"
  chmod +x "$bin"
fi

exec "$bin" "$@"
