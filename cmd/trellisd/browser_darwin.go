// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"os/exec"
)

// openBrowser hands rawURL to LaunchServices, which opens it in the
// operator's default browser.
func openBrowser(ctx context.Context, rawURL string) error {
	return exec.CommandContext(ctx, "/usr/bin/open", rawURL).Run()
}
