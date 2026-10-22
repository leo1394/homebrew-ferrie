//go:build !windows

package selector

import (
    "bufio"
    "errors"
    "os"
    "time"

    "golang.org/x/sys/unix"
)

func enableSelectorTerminal(in, out *os.File) (func(), error) { return func() {}, nil }

func waitForInput(reader *bufio.Reader, fd int, timeout time.Duration) (bool, error) {
	if reader.Buffered() > 0 {
		return true, nil
	}
	if fd < 0 {
		return false, nil
	}

	descriptors := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		count, err := unix.Poll(descriptors, int(timeout/time.Millisecond))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return false, err
		}
		if count == 0 {
			return false, nil
		}
		if descriptors[0].Revents&unix.POLLNVAL != 0 {
			return false, errors.New("selector: terminal input descriptor is invalid")
		}
		return descriptors[0].Revents&(unix.POLLIN|unix.POLLHUP) != 0, nil
	}
}
