# Deployment validation: the check that runs AFTER an install. Packages are
# built by goreleaser in CI, so there is no local deb/rpm target to duplicate
# that path; what the build contract needs locally is the assertion that the
# daemon answering is the release that was installed.
#
#   make deploy-validate EXPECT=v0.12.44
#   make deploy-validate EXPECT=v0.12.44 URL=http://10.44.40.40:8446
#
# EXPECT, not VERSION: the Makefile owns VERSION as this working tree's
# `git describe`, which is not a release. URL defaults to the loopback port
# trellisd serves plain HTTP on.

.PHONY: deploy-validate

deploy-validate:
ifndef EXPECT
	$(error EXPECT is required, e.g. make deploy-validate EXPECT=v0.12.44)
endif
	@./scripts/deploy-validate.py --expect "$(EXPECT)" $(if $(URL),--url "$(URL)")
