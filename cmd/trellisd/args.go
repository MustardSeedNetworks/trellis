// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/MustardSeedNetworks/trellis/internal/version"
)

const (
	exitOK    = 0
	exitUsage = 2
)

const usageText = `Usage: trellisd [--version]

Serves the Trellis survey API. trellisd takes no other arguments; it is
configured through the environment: TRELLIS_ADDR, TRELLIS_DATA_DIR,
TRELLIS_CAPTURE_MODE, TRELLIS_AUTH_USERNAME, TRELLIS_AUTH_PASSWORD,
TRELLIS_TLS_CERT and TRELLIS_TLS_KEY.

Flags:
`

// handleArgs settles the command line before the daemon starts. done reports
// that the process must exit with code instead of serving: --version and
// --help answer and leave, and anything else is refused, because a mistyped
// flag that started a second daemon would sit beside the running service.
func handleArgs(args []string, stdout, stderr io.Writer) (code int, done bool) {
	fs := flag.NewFlagSet("trellisd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), usageText)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return exitOK, true
		}
		return exitUsage, true
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "trellisd: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return exitUsage, true
	}
	if *showVersion {
		info := version.Info()
		_, _ = fmt.Fprintf(stdout, "trellisd %s (commit %s, built %s)\n",
			info["version"], info["commit"], info["buildTime"])
		return exitOK, true
	}
	return exitOK, false
}
