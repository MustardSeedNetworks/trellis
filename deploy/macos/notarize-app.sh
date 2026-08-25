#!/bin/bash
#
# Notarize and staple Trellis.app.
#
# Notarization is for *distribution*: without it, a bundle copied to another Mac
# is quarantined and Gatekeeper refuses to open it. It is not what lets the app
# read Wi-Fi network names — the location entitlement is (see build-app.sh) —
# and a notarized bundle still has to be launched through LaunchServices to hold
# its Location Services grant.
#
# Usage:
#   ./notarize-app.sh [BUNDLE]
#
# Requirements:
#   - A bundle already built and signed by build-app.sh
#   - A notarytool keychain profile. The fleet's is 'seed-notary', which holds
#     the account-level App Store Connect credentials shared across products;
#     override with TRELLIS_NOTARY_PROFILE. Create one with:
#       xcrun notarytool store-credentials <name> \
#           --key AuthKey_XXXXXXXX.p8 --key-id KEYID --issuer ISSUER_UUID
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BUNDLE="${1:-$REPO_ROOT/dist/macos-app/Trellis.app}"
PROFILE="${TRELLIS_NOTARY_PROFILE:-seed-notary}"
ARCHIVE="${BUNDLE%.app}.zip"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

if [ ! -d "$BUNDLE" ]; then
    echo -e "${RED}No bundle at $BUNDLE${NC}"
    echo "Build one first:  ./deploy/macos/build-app.sh <version>"
    exit 1
fi

# Notarizing an unsigned or ad-hoc-signed bundle fails late, inside Apple's
# service, with a log you have to fetch separately. Checking here fails in a
# second with a message that says what is wrong.
if ! codesign --verify --strict "$BUNDLE" 2>/dev/null; then
    echo -e "${RED}$BUNDLE is not validly signed.${NC}"
    echo "Rebuild it with ./deploy/macos/build-app.sh, which signs with the"
    echo "hardened runtime and the location entitlement."
    exit 1
fi

if ! xcrun notarytool history --keychain-profile "$PROFILE" >/dev/null 2>&1; then
    echo -e "${RED}No usable notarytool profile '$PROFILE'.${NC}"
    echo "Create one with 'xcrun notarytool store-credentials $PROFILE …',"
    echo "or point TRELLIS_NOTARY_PROFILE at an existing one."
    exit 1
fi

# notarytool takes an archive and never a bare .app. ditto --keepParent is what
# preserves the bundle directory inside the zip; `zip -r` mangles symlinks in a
# framework and Apple rejects the result.
echo "Archiving $BUNDLE"
rm -f "$ARCHIVE"
ditto -c -k --keepParent "$BUNDLE" "$ARCHIVE"

echo "Submitting to Apple (this waits for the verdict)"
if ! xcrun notarytool submit "$ARCHIVE" --keychain-profile "$PROFILE" --wait; then
    echo -e "${RED}Notarization failed.${NC} Fetch the reasons with:"
    echo "  xcrun notarytool log <submission-id> --keychain-profile $PROFILE"
    exit 1
fi

# The staple goes onto the .app, not the archive: it writes the ticket into the
# bundle so Gatekeeper can verify it without network access — which is the whole
# point for an air-gapped survey laptop.
echo "Stapling the ticket to the bundle"
xcrun stapler staple "$BUNDLE"
xcrun stapler validate "$BUNDLE"

# The check that matters: what Gatekeeper says about launching it.
echo "Gatekeeper assessment:"
spctl --assess --type execute --verbose=2 "$BUNDLE"

rm -f "$ARCHIVE"
echo -e "${GREEN}Notarized and stapled:${NC} $BUNDLE"
