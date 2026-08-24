#!/bin/bash
#
# Build and sign the Trellis Capture application bundle.
#
# macOS hides Wi-Fi network names and BSSIDs from any process without Location
# Services authorization. That grant is given per user, inside a login session,
# to a signed application bundle carrying the location entitlement — so capture
# must be a signed, entitled .app, not a bare executable. Without it a survey
# records signal strength against nameless BSSIDs.
#
# Usage:
#   ./build-capture.sh [VERSION] [OUTPUT_DIR]
#
# Requirements:
#   - Go toolchain
#   - codesign, and a "Developer ID Application" identity in the keychain
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VERSION="${1:-0.0.0}"
VERSION="${VERSION#v}"
OUTPUT_DIR="${2:-$REPO_ROOT/dist/macos-capture}"

BUNDLE_ID="net.mustardseed.trellis.capture"
BUNDLE="$OUTPUT_DIR/Trellis Capture.app"
BINARY_NAME="trellis-capture"

SIGN_IDENTITY="${TRELLIS_SIGN_IDENTITY:-Developer ID Application}"
TEAM_ID="${TRELLIS_TEAM_ID:-X6JWYP43HG}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "Building Trellis Capture.app $VERSION"

rm -rf "$BUNDLE"
mkdir -p "$BUNDLE/Contents/MacOS"

# cgo is required: the capture backend links CoreWLAN. Apple Silicon only,
# matching the fleet's release targets.
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
    -trimpath \
    -o "$BUNDLE/Contents/MacOS/$BINARY_NAME" \
    "$REPO_ROOT/cmd/trellis-capture"

sed -e "s|<string>0\.0\.0</string>|<string>$VERSION</string>|g" \
    "$SCRIPT_DIR/capture/Info.plist" > "$BUNDLE/Contents/Info.plist"

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
    --entitlements "$SCRIPT_DIR/capture/Capture.entitlements" \
    --identifier "$BUNDLE_ID" \
    --sign "$SIGN_IDENTITY" \
    "$BUNDLE"

codesign --verify --strict --verbose=2 "$BUNDLE"

echo -e "${GREEN}Built and signed:${NC} $BUNDLE"
echo
echo -e "${YELLOW}Not done here — notarization.${NC}"
echo "Notarizing requires Apple credentials this script deliberately does not handle."
echo
echo "One-time setup, if you have no keychain profile yet:"
echo
echo "  xcrun notarytool store-credentials trellis-notary \\"
echo "      --key /path/to/AuthKey_XXXXXXXX.p8 --key-id KEYID --issuer ISSUER_UUID"
echo
echo "Then, remembering that notarytool takes an ARCHIVE and never a bare .app,"
echo "and that the staple goes back onto the .app:"
echo
echo "  ditto -c -k --keepParent \"$BUNDLE\" \"$OUTPUT_DIR/capture.zip\""
echo "  xcrun notarytool submit \"$OUTPUT_DIR/capture.zip\" --keychain-profile trellis-notary --wait"
echo "  xcrun stapler staple \"$BUNDLE\""
echo
echo "Notarization is for distribution. It is not what enables the Location"
echo "prompt — the entitlement above is (team $TEAM_ID)."
