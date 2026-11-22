#!/bin/bash
#
# Remove the Trellis.app that the .pkg installed.
#
# Surveys live in the operator's own data directory, so this never touches
# them (ADR-0007: no package operation deletes a survey). Delete
# "~/Library/Application Support/Trellis" by hand to remove them too. If you
# added Trellis to Login Items, remove it in System Settings > General.
#
# Usage:
#   sudo ./uninstall.sh
#   sudo /Applications/Trellis.app/Contents/Resources/uninstall.sh
#
set -euo pipefail

APP="/Applications/Trellis.app"
PKG_ID="net.mustardseed.trellis.pkg"

if [ "$(id -u)" -ne 0 ]; then
    echo "uninstall.sh must run as root:  sudo $0" >&2
    exit 1
fi

# trellisd has no Dock icon or window to quit it by (LSUIElement), and a
# running copy would keep serving from the deleted bundle.
killall -TERM trellisd 2>/dev/null || true

rm -rf "$APP"
pkgutil --forget "$PKG_ID" >/dev/null 2>&1 || true

echo "Trellis removed. Surveys in ~/Library/Application Support/Trellis were left in place."
