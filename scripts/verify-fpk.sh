#!/usr/bin/env sh
set -eu
PACKAGE="${1:?usage: verify-fpk.sh PACKAGE ARCH}"
ARCH="${2:?usage: verify-fpk.sh PACKAGE ARCH}"
case "$ARCH" in amd64) MACHINE='X86-64';; arm64) MACHINE='AArch64';; *) exit 2;; esac
CHECK=$(mktemp -d "${TMPDIR:-/tmp}/fnos-fpk-check.XXXXXXXX")
trap 'rm -rf -- "$CHECK"' EXIT
gzip -t "$PACKAGE"
tar -xzf "$PACKAGE" -C "$CHECK"
for required in manifest ICON.PNG ICON_256.PNG app.tgz; do test -s "$CHECK/$required"; done
for script in main install_init uninstall_init upgrade_init; do test -x "$CHECK/cmd/$script"; done
mkdir "$CHECK/payload"
gzip -t "$CHECK/app.tgz"
tar -xzf "$CHECK/app.tgz" -C "$CHECK/payload"
test -x "$CHECK/payload/bin/fnos-media-throttle"
test -s "$CHECK/payload/ui/index.html"
test -s "$CHECK/payload/ui/config"
LC_ALL=C readelf -h "$CHECK/payload/bin/fnos-media-throttle" | grep "Machine:.*$MACHINE"
echo 'FPK compression, payload, executable permissions and architecture verified.'
