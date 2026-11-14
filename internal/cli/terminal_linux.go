package cli

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func terminalDevice(stream any) bool {
	f, ok := stream.(*os.File)
	if !ok {
		return false
	}
	var state syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&state)))
	return err == 0
}

func terminalRaw(f *os.File) (func(), error) {
	var saved syscall.Termios
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&saved)))
	if e != 0 {
		return nil, e
	}
	raw := saved
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	_, _, e = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCSETS, uintptr(unsafe.Pointer(&raw)))
	if e != 0 {
		return nil, fmt.Errorf("set terminal mode: %w", e)
	}
	return func() { syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCSETS, uintptr(unsafe.Pointer(&saved))) }, nil
}

func terminalSize(f *os.File) (int, int) {
	var size struct{ rows, cols, x, y uint16 }
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	if e != 0 || size.rows == 0 || size.cols == 0 {
		return 80, 24
	}
	return int(size.cols), int(size.rows)
}

func terminalOutput(_ *os.File) (func(), error) { return func() {}, nil }
