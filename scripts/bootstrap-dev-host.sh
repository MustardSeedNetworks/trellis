#!/usr/bin/env bash
# Bring a Linux build host up to the versions the build contract names, so that
# `make build`, `make test` and `make packages` mean the same thing there as in
# CI. Written because dev-srv-fedora had no Node, no checkout and no rpm tooling
# at all, and dev-srv-ubuntu ran a Node two patches behind the pin (#163) — a
# dev server that cannot run the documented build is not a second opinion on CI.
#
# Idempotent, needs sudo, and pins every version: a "latest" here is how the
# host silently drifts away from the pin it exists to check.
#
# Usage: ssh dev-srv-ubuntu 'bash -s' < scripts/bootstrap-dev-host.sh
set -euo pipefail

NODE_VER=26.8.1
NPM_VER=12.0.2
# The goreleaser inside goreleaser-cross v1.27.0, which release.yml runs.
GORELEASER_VER=2.17.1

case "$(uname -m)" in
  x86_64) node_arch=x64; gr_arch=x86_64 ;;
  aarch64) node_arch=arm64; gr_arch=arm64 ;;
  *) echo "unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"

if [ "$(node --version 2>/dev/null)" != "v${NODE_VER}" ]; then
  echo "installing Node ${NODE_VER}"
  tarball="node-v${NODE_VER}-linux-${node_arch}.tar.xz"
  curl -fsSLO "https://nodejs.org/dist/v${NODE_VER}/${tarball}"
  curl -fsSLO "https://nodejs.org/dist/v${NODE_VER}/SHASUMS256.txt"
  grep " ${tarball}\$" SHASUMS256.txt | sha256sum -c -
  sudo rm -rf /opt/node
  sudo mkdir -p /opt/node
  sudo tar -xJf "$tarball" -C /opt/node --strip-components=1
  for b in node npm npx; do sudo ln -sf "/opt/node/bin/$b" "/usr/local/bin/$b"; done
fi

# Fedora's minimal install has no libatomic, which the official Node build links.
if ! node --version >/dev/null 2>&1 && command -v dnf >/dev/null; then
  sudo dnf install -y libatomic
fi

if [ "$(npm --version 2>/dev/null)" != "$NPM_VER" ]; then
  sudo /usr/local/bin/npm install -g "npm@${NPM_VER}"
fi

if ! goreleaser --version 2>/dev/null | grep -q "GitVersion:    ${GORELEASER_VER}"; then
  echo "installing goreleaser ${GORELEASER_VER}"
  tarball="goreleaser_Linux_${gr_arch}.tar.gz"
  base="https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VER}"
  curl -fsSLO "${base}/${tarball}"
  curl -fsSLO "${base}/checksums.txt"
  grep " ${tarball}\$" checksums.txt | sha256sum -c -
  tar -xzf "$tarball" goreleaser
  sudo install -m0755 goreleaser /usr/local/bin/goreleaser
fi

# make drives every documented build; a minimal Fedora has neither it nor dpkg.
# nfpm writes both package formats itself, so building needs no packaging tools
# — but validating an install does, and each host should be able to at least
# inspect the other's format.
if command -v dnf >/dev/null; then
  sudo dnf install -y make dpkg >/dev/null
else
  sudo apt-get install -y make rpm >/dev/null
fi

echo "node $(node --version) · npm $(npm --version) · go $(go version | awk '{print $3}') · goreleaser $(goreleaser --version | awk '/GitVersion/{print $2}')"
