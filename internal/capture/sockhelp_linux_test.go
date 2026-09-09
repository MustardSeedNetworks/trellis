// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

import "golang.org/x/sys/unix"

// The socket helpers the frame-loop tests use, kept apart so the test bodies
// read as intent rather than syscall detail.

func unixSocketpair() ([2]int, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		return [2]int{}, err
	}
	return [2]int{fds[0], fds[1]}, nil
}

func closeFD(fd int) error { return unix.Close(fd) }

func writeFD(fd int, b []byte) error {
	_, err := unix.Write(fd, b)
	return err
}
