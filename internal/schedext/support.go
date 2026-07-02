package schedext

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

const (
	UnsupportedExitCode = 78
	minKernelMajor      = 6
	minKernelMinor      = 12
)

var ErrUnsupported = errors.New("sched_ext is unsupported on this kernel")

func CheckSupport() error {
	var uts syscall.Utsname
	if err := syscall.Uname(&uts); err != nil {
		return fmt.Errorf("%w: failed to read kernel version: %v", ErrUnsupported, err)
	}
	release := cStringToGoString(uts.Release[:])
	major, minor, err := parseKernelMajorMinor(release)
	if err != nil {
		return fmt.Errorf("%w: failed to parse kernel release %q: %v", ErrUnsupported, release, err)
	}
	if major < minKernelMajor || (major == minKernelMajor && minor < minKernelMinor) {
		return fmt.Errorf("%w: Linux kernel %s is older than required %d.%d+", ErrUnsupported, release, minKernelMajor, minKernelMinor)
	}
	if _, err := os.Stat("/sys/kernel/sched_ext"); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: /sys/kernel/sched_ext is missing; enable CONFIG_SCHED_CLASS_EXT", ErrUnsupported)
		}
		return fmt.Errorf("%w: cannot access /sys/kernel/sched_ext: %v", ErrUnsupported, err)
	}
	return nil
}

func parseKernelMajorMinor(release string) (int, int, error) {
	var major, minor int
	if _, err := fmt.Sscanf(release, "%d.%d", &major, &minor); err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

func cStringToGoString(chars []int8) string {
	var b strings.Builder
	b.Grow(len(chars))
	for _, c := range chars {
		if c == 0 {
			break
		}
		b.WriteByte(byte(c))
	}
	return b.String()
}
