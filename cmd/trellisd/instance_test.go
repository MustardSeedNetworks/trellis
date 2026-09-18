// SPDX-License-Identifier: BUSL-1.1

package main

// instance_test.go pins D-TRL-6's second half: a second trellisd on a data
// directory another instance already holds must refuse to start and say who
// holds it, rather than walking the port fallback onto a neighbouring port and
// opening the same SQLite file underneath the first one (foundation#46).
//
// The holder here is the test process rather than a first daemon. That is the
// same lock through the same package, and it removes the race that makes the
// two-daemon version flaky: the port only reaches the lock file after the
// holder has bound a listener, so a second daemon started too early would read
// a truthful "unknown" port and the assertion would be a coin flip. Holding it
// from the test means the PID and port asserted on are known exactly.
//
// It still discriminates: a trellisd that does not take the lock starts and
// serves instead of exiting, which is the failure this test exists to catch.

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/foundation/pkg/instance"
)

// heldPort is the port the fake holder publishes. Any value will do; it is
// asserted on verbatim, so it must not be one trellisd could bind by accident.
const heldPort = 18999

// daemonRefusalTimeout bounds the refusal. It is generous: the build is done
// before the clock starts, so this measures only start-up to exit.
const daemonRefusalTimeout = 30 * time.Second

func TestSecondDaemonRefusesAHeldDataDir(t *testing.T) {
	dataDir := t.TempDir()

	lock, err := instance.Acquire(dataDir)
	if err != nil {
		t.Fatalf("acquire the lock the daemon must lose: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	if err := lock.SetPort(heldPort); err != nil {
		t.Fatalf("publish the holder's port: %v", err)
	}

	out, exitCode := runDaemon(t, dataDir, buildDaemon(t))

	if exitCode != 1 {
		t.Errorf("second trellisd exit code = %d, want 1\noutput:\n%s", exitCode, out)
	}
	if pid := strconv.Itoa(os.Getpid()); !strings.Contains(out, pid) {
		t.Errorf("refusal does not name the holder's pid %s\noutput:\n%s", pid, out)
	}
	if port := strconv.Itoa(heldPort); !strings.Contains(out, port) {
		t.Errorf("refusal does not name the holder's port %s\noutput:\n%s", port, out)
	}
}

// TestSecondDaemonNamesTheFirstDaemonsPort is the row's acceptance with a real
// first daemon, which is what makes it the acceptance: the test above holds the
// lock itself and so publishes the port itself, and passes on a trellisd that
// takes the lock and never calls SetPort. Only a first daemon can prove the
// port a second one reads was published by the first.
//
// The cost is a race the test above does not have -- the port only reaches the
// lock file after the fallback has settled -- so the second daemon is not
// started until the first has logged the address it bound, and that address is
// what the refusal is checked against.
func TestSecondDaemonNamesTheFirstDaemonsPort(t *testing.T) {
	dataDir := t.TempDir()
	bin := buildDaemon(t)

	first := exec.Command(bin)
	first.Env = daemonEnv(dataDir)
	stdout, err := first.StdoutPipe()
	if err != nil {
		t.Fatalf("pipe the first daemon's output: %v", err)
	}
	first.Stderr = first.Stdout
	if err := first.Start(); err != nil {
		t.Fatalf("start the first daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = first.Process.Kill()
		_ = first.Wait()
	})

	port := waitForListeningPort(t, stdout)

	out, exitCode := runDaemon(t, dataDir, bin)
	if exitCode != 1 {
		t.Errorf("second trellisd exit code = %d, want 1\noutput:\n%s", exitCode, out)
	}
	if pid := strconv.Itoa(first.Process.Pid); !strings.Contains(out, pid) {
		t.Errorf("refusal does not name the first daemon's pid %s\noutput:\n%s", pid, out)
	}
	if !strings.Contains(out, port) {
		t.Errorf("refusal does not name the port the first daemon bound (%s)\noutput:\n%s", port, out)
	}
}

// waitForListeningPort reads the first daemon's log until it says what it bound.
func waitForListeningPort(t *testing.T, out io.Reader) string {
	t.Helper()

	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "trellisd listening") {
			continue
		}
		_, addr, found := strings.Cut(line, "addr=")
		if !found {
			t.Fatalf("the listening line names no address: %s", line)
		}
		addr, _, _ = strings.Cut(addr, " ")
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("parse the bound address %q: %v", addr, err)
		}
		return port
	}
	t.Fatalf("the first daemon never logged an address: %v", scanner.Err())
	return ""
}

// buildDaemon builds trellisd once per test. Go's build cache makes the second
// call cheap; building in-test is what keeps the binary under test the one the
// package's own source produces.
func buildDaemon(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "trellisd")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build trellisd: %v\n%s", err, out)
	}
	return bin
}

// daemonEnv runs a daemon against dataDir on an ephemeral port, so a daemon that
// wrongly gets as far as binding cannot collide with anything on the host.
func daemonEnv(dataDir string) []string {
	return append(os.Environ(),
		"TRELLIS_DATA_DIR="+dataDir,
		"TRELLIS_ADDR=127.0.0.1:0",
	)
}

// runDaemon builds trellisd and runs it against dataDir, returning everything
// it wrote and its exit code. A daemon that does not exit on its own is killed
// and the test fails naming that: starting successfully is the defect.
func runDaemon(t *testing.T, dataDir, bin string) (output string, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), daemonRefusalTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = daemonEnv(dataDir)

	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("trellisd was still running after %s on a data dir another instance holds\noutput:\n%s",
			daemonRefusalTimeout, out)
	}
	if err == nil {
		return string(out), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("run trellisd: %v\n%s", err, out)
	}
	return string(out), exit.ExitCode()
}
