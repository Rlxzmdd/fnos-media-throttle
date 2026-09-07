#!/usr/bin/env sh
set -eu
# Official fnOS SDK; pin both version and bytes rather than trusting a moving URL.
[ "$(uname -s)" = Linux ] && [ "$(uname -m)" = x86_64 ] || {
  echo 'This installer requires Linux x86_64 (cross-builds amd64 and arm64 packages).' >&2
  exit 1
}
SDK_DIR="${1:?usage: install-fnpack.sh OUTPUT_DIRECTORY}"
mkdir -p "$SDK_DIR"
curl --fail --location --retry 3 \
  'https://static2.fnnas.com/fnpack/fnpack-1.2.3-linux-amd64' -o "$SDK_DIR/fnpack"
printf '%s  %s\n' '54b97fa7b70968c4d05c79840f5daeff508957d0bb2062fdb0376d00d9615c93' "$SDK_DIR/fnpack" | sha256sum --check -
chmod +x "$SDK_DIR/fnpack"
