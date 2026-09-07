# Markdown gate — fleet parity with seed, stem and niac-go.
#
# The version lives here rather than in the top-level Makefile because the org
# Renovate preset's regex manager for MARKDOWNLINT_CLI2_VERSION matches only
# `^mk/lint\.mk$` (MustardSeedNetworks/.github default.json). A pin in the
# Makefile would be invisible to it and would silently drift out of step with
# the CI action and the pre-commit hook, which is exactly the false clear this
# pin exists to prevent: MD060 landed in cli2 0.23 and a local copy at any
# earlier version reports a clean tree that CI rejects.
#
# Keep in lockstep with:
#   .github/workflows/ci.yml   markdownlint-cli2-action (bundles this version)
#   .pre-commit-config.yaml    markdownlint-cli2 rev
MARKDOWNLINT_CLI2_VERSION := 0.23.2

# --yes with no PATH fallback and no SKIP: an absent or differently-versioned
# local copy must fail loudly, not quietly lint with the wrong rule set.
# Scope is passed here, not in .markdownlint-cli2.jsonc — cli2 unions a config
# "globs" entry with the caller's rather than replacing it.
lint-md:
	@npx --yes markdownlint-cli2@$(MARKDOWNLINT_CLI2_VERSION) "**/*.md"
