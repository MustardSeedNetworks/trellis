#!/bin/bash
#
# Build the macOS .pkg installer for Trellis.
#
# The package installs /Applications/Trellis.app and nothing else. It runs no
# postinstall, loads no launchd job and opens no port, mirroring the Linux
# packages (ADR-0007): trellisd serves one operator on loopback, so it runs as
# them, started by them. It is a bundle rather than a bare binary because
# macOS gives Wi-Fi network names only to a signed, entitled application
# launched through LaunchServices (see build-app.sh); a LaunchDaemon would run
# outside the user's login session and could never hold that grant.
#
# Usage:
#   ./build-pkg.sh BINARY VERSION [OUTPUT_DIR]
#
# BINARY is the arm64 trellisd from the goreleaser darwin archive, so the
# installer carries the same executable the archive does.
#
# Environment:
#   TRELLIS_SIGN_IDENTITY       codesign identity for the .app ("-" = ad hoc)
#   TRELLIS_INSTALLER_IDENTITY  "Developer ID Installer" identity for the .pkg;
#                               unset leaves the package unsigned (dry runs)
#
set -euo pipefail

# cp and pkgbuild otherwise carry extended attributes into the payload as
# AppleDouble ._ files, which the install then drops next to the bundle.
export COPYFILE_DISABLE=1

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BINARY="${1:?usage: build-pkg.sh BINARY VERSION [OUTPUT_DIR]}"
VERSION="${2:?usage: build-pkg.sh BINARY VERSION [OUTPUT_DIR]}"
VERSION="${VERSION#v}"
OUTPUT_DIR="${3:-$REPO_ROOT/dist}"

PKG_ID="net.mustardseed.trellis.pkg"
PKG="$OUTPUT_DIR/trellis-${VERSION}-darwin-arm64.pkg"

if [ "$(lipo -archs "$BINARY")" != "arm64" ]; then
    echo "$BINARY is not an arm64 binary; Trellis ships for Apple silicon only." >&2
    exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

TRELLIS_PREBUILT_BINARY="$BINARY" "$SCRIPT_DIR/build-app.sh" "$VERSION" "$WORK/app" >/dev/null

mkdir -p "$WORK/root/Applications" "$WORK/scripts" "$OUTPUT_DIR"
ditto --norsrc --noextattr "$WORK/app/Trellis.app" "$WORK/root/Applications/Trellis.app"

# Without this Installer treats the bundle as relocatable and, finding another
# copy of net.mustardseed.trellis on disk (a dist/ build, a Downloads copy),
# "upgrades" that one and leaves /Applications untouched. The version check is
# off too, so installing an older release over a newer one actually installs.
pkgbuild --analyze --root "$WORK/root" "$WORK/component.plist" >/dev/null
# Whether --analyze writes BundleIsRelocatable depends on the macOS release, so
# remove any entry before adding ours.
/usr/libexec/PlistBuddy -c "Delete :0:BundleIsRelocatable" "$WORK/component.plist" 2>/dev/null || true
/usr/libexec/PlistBuddy \
    -c "Add :0:BundleIsRelocatable bool false" \
    -c "Set :0:BundleIsVersionChecked false" \
    "$WORK/component.plist"

# An upgrade replaces the bundle under a running daemon, which would keep the
# loopback port and serve the old build until the operator noticed.
cat > "$WORK/scripts/preinstall" <<'SCRIPT'
#!/bin/bash
killall -TERM trellisd 2>/dev/null || true
exit 0
SCRIPT
chmod 755 "$WORK/scripts/preinstall"

sign=()
if [ -n "${TRELLIS_INSTALLER_IDENTITY:-}" ]; then
    sign=(--sign "$TRELLIS_INSTALLER_IDENTITY" --timestamp)
fi

pkgbuild \
    --root "$WORK/root" \
    --component-plist "$WORK/component.plist" \
    --identifier "$PKG_ID" \
    --version "$VERSION" \
    --scripts "$WORK/scripts" \
    --install-location "/" \
    ${sign[@]+"${sign[@]}"} \
    "$PKG"

if pkgutil --payload-files "$PKG" | grep '/\._'; then
    echo "Payload contains AppleDouble ._ files (listed above)" >&2
    exit 1
fi

echo "Built $PKG"
