#!/bin/bash
#
# Build and sign Trellis.app.
#
# macOS hides Wi-Fi network names and BSSIDs from any process without Location
# Services authorization. That grant is given per user, inside a login session,
# to a signed application bundle carrying the location entitlement — and it is
# attributed by LaunchServices, so the process must be *launched* as the bundle
# too. trellisd links its capture backend directly (ADR-0006), which makes
# trellisd itself the thing that has to be bundled. Run outside the bundle it
# still serves imported surveys; it just cannot name what its radio sees.
#
# Usage:
#   ./build-app.sh [VERSION] [OUTPUT_DIR]
#
# Requirements:
#   - Go toolchain, and a built UI in internal/api/ui (npm --prefix ui run build)
#   - codesign, and a "Developer ID Application" identity in the keychain
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VERSION="${1:-0.0.0}"
VERSION="${VERSION#v}"
OUTPUT_DIR="${2:-$REPO_ROOT/dist/macos-app}"

BUNDLE_ID="net.mustardseed.trellis"
BUNDLE="$OUTPUT_DIR/Trellis.app"
BINARY_NAME="trellisd"
VERSION_PKG="github.com/MustardSeedNetworks/trellis/internal/version"

SIGN_IDENTITY="${TRELLIS_SIGN_IDENTITY:-Developer ID Application}"
TEAM_ID="${TRELLIS_TEAM_ID:-X6JWYP43HG}"
NOTARY_PROFILE="${TRELLIS_NOTARY_PROFILE:-seed-notary}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "Building Trellis.app $VERSION"

# The UI is embedded by go:embed from internal/api/ui. Building without it
# produces a bundle that signs and launches and then serves nothing, which is
# the same class of silent failure as an unentitled scan.
if [ ! -f "$REPO_ROOT/internal/api/ui/index.html" ]; then
    echo -e "${RED}internal/api/ui/index.html is missing — the UI has not been built.${NC}"
    echo "  npm --prefix ui ci && npm --prefix ui run build"
    exit 1
fi

UI_BUILD_HASH="$(find "$REPO_ROOT/internal/api/ui" -type f -exec md5 -q {} + | sort | md5 -q)"
COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

rm -rf "$BUNDLE"
mkdir -p "$BUNDLE/Contents/MacOS"

# cgo is required: the capture backend links CoreWLAN. Apple Silicon only,
# matching the fleet's release targets.
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
    -trimpath \
    -ldflags "-X $VERSION_PKG.Version=$VERSION \
              -X $VERSION_PKG.Commit=$COMMIT \
              -X $VERSION_PKG.BuildTime=$BUILD_TIME \
              -X $VERSION_PKG.UIBuildHash=$UI_BUILD_HASH" \
    -o "$BUNDLE/Contents/MacOS/$BINARY_NAME" \
    "$REPO_ROOT/cmd/trellisd"

sed -e "s|<string>0\.0\.0</string>|<string>$VERSION</string>|g" \
    "$SCRIPT_DIR/app/Info.plist" > "$BUNDLE/Contents/Info.plist"

plutil -lint "$BUNDLE/Contents/Info.plist" > /dev/null

if ! security find-identity -v -p codesigning | grep -q "$SIGN_IDENTITY"; then
    echo -e "${RED}No '$SIGN_IDENTITY' identity found in the keychain.${NC}"
    echo "An unsigned bundle cannot hold Location Services authorization, so every"
    echo "scan would omit network names. Set TRELLIS_SIGN_IDENTITY to override."
    exit 1
fi

# The hardened runtime is required for notarization. Under it, locationd will
# not show the authorization prompt without the location entitlement — it
# registers the client, then silently declines to ask. Signing without the
# entitlements file produces a bundle that looks correct and can never prompt.
codesign --force --timestamp --options runtime \
    --entitlements "$SCRIPT_DIR/app/Trellis.entitlements" \
    --identifier "$BUNDLE_ID" \
    --sign "$SIGN_IDENTITY" \
    "$BUNDLE"

codesign --verify --strict --verbose=2 "$BUNDLE"

echo -e "${GREEN}Built and signed:${NC} $BUNDLE"
echo
echo "Launch it through LaunchServices, never by executing the inner binary —"
echo "a direct exec registers a different Location Services client identity:"
echo
echo "  open -a \"$BUNDLE\""
echo "  python3 \"$SCRIPT_DIR/location-status.py\" $BUNDLE_ID"
echo
echo "Its log is at ~/Library/Logs/Trellis/trellisd.log; LaunchServices gives a"
echo "bundled process /dev/null for stdout, so that file is the only output."
echo
echo -e "${YELLOW}Not done here — notarization.${NC}"
echo "Notarizing needs Apple credentials this script deliberately does not handle."
echo "The fleet's keychain profile is '$NOTARY_PROFILE' (team $TEAM_ID); create one with"
echo "'xcrun notarytool store-credentials' if it is missing."
echo
echo "Remembering that notarytool takes an ARCHIVE and never a bare .app, and"
echo "that the staple goes back onto the .app:"
echo
echo "  ditto -c -k --keepParent \"$BUNDLE\" \"$OUTPUT_DIR/trellis.zip\""
echo "  xcrun notarytool submit \"$OUTPUT_DIR/trellis.zip\" --keychain-profile $NOTARY_PROFILE --wait"
echo "  xcrun stapler staple \"$BUNDLE\""
echo
echo "Notarization is for distribution. It is not what enables the Location"
echo "prompt — the entitlement above is."
