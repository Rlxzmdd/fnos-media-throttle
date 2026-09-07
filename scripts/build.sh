#!/usr/bin/env sh
set -eu
ARCH="${1:-amd64}"
case "$ARCH" in amd64) PLATFORM=x86;; arm64) PLATFORM=arm;; *) echo "architecture must be amd64 or arm64" >&2; exit 2;; esac
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
APP="$ROOT/app"
DIST="$ROOT/dist"
VERSION=$(awk -F= '/^version[[:space:]]*=/{gsub(/[[:space:]]/, "", $2); print $2; exit}' "$ROOT/packaging/manifest")
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/fnos-media-throttle.XXXXXXXX")
trap 'rm -rf -- "$STAGE"' EXIT
mkdir -p "$STAGE/app/bin" "$STAGE/wizard" "$DIST"
(cd "$APP/ui" && pnpm install --frozen-lockfile && pnpm build)
(cd "$APP" && CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -trimpath -ldflags="-s -w -X github.com/fnos-media-throttle/fnos-media-throttle/buildinfo.Version=$VERSION" -o "$STAGE/app/bin/fnos-media-throttle" .)
cp -R "$APP/ui/dist" "$STAGE/app/ui"
cp "$ROOT/packaging/manifest" "$STAGE/manifest"
cp -R "$ROOT/packaging/config" "$ROOT/packaging/cmd" "$STAGE/"
cp "$ROOT/packaging/ui/config" "$STAGE/app/ui/config"
mkdir -p "$STAGE/app/ui/images"
cp "$ROOT/packaging/assets/app-icon-64.png" "$STAGE/ICON.PNG"
cp "$ROOT/packaging/assets/app-icon-256.png" "$STAGE/ICON_256.PNG"
cp "$ROOT/packaging/assets/app-icon-64.png" "$STAGE/app/ui/images/icon_64.png"
cp "$ROOT/packaging/assets/app-icon-256.png" "$STAGE/app/ui/images/icon_256.png"
sed -i "s/^platform = .*/platform = $PLATFORM/" "$STAGE/manifest"
chmod +x "$STAGE/cmd"/* "$STAGE/app/bin/fnos-media-throttle"
(cd "$STAGE" && fnpack build)
set -- "$STAGE"/*.fpk
[ "$#" -eq 1 ] && [ -s "$1" ] || { echo "fnpack must produce exactly one non-empty fpk" >&2; exit 1; }
PACKAGE="$DIST/fnos-media-throttle-$VERSION-linux-$ARCH.fpk"
sh "$ROOT/scripts/verify-fpk.sh" "$1" "$ARCH"
mv "$1" "$PACKAGE"
(cd "$DIST" && sha256sum "$(basename "$PACKAGE")" > "$(basename "$PACKAGE").sha256")
echo "Built $PACKAGE"
