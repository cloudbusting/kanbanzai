package cli

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var getConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleMode")
var setConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")
var getConsoleScreenBufferInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleScreenBufferInfo")

func terminalDevice(stream any) bool {
	f, ok := stream.(*os.File)
	if !ok {
		return false
	}
	var mode uint32
	result, _, _ := getConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode)))
	return result != 0
}

func terminalRaw(f *os.File) (func(), error) {
	var old uint32
	if r, _, e := getConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&old))); r == 0 {
		return nil, e
	}
	// Disable line and echo input; retain processed Ctrl-C and enable VT keys.
	if r, _, e := setConsoleMode.Call(f.Fd(), uintptr((old&^6)|0x200)); r == 0 {
		return nil, fmt.Errorf("set terminal mode: %w", e)
	}
	return func() { setConsoleMode.Call(f.Fd(), uintptr(old)) }, nil
}

func terminalSize(f *os.File) (int, int) {
	var info struct {
		size, cursor struct{ x, y int16 }
		attr         uint16
		window       struct{ left, top, right, bottom int16 }
		max          struct{ x, y int16 }
	}
	if r, _, _ := getConsoleScreenBufferInfo.Call(f.Fd(), uintptr(unsafe.Pointer(&info))); r == 0 {
		return 80, 24
	}
	w, h := int(info.window.right-info.window.left+1), int(info.window.bottom-info.window.top+1)
	if w < 1 || h < 1 {
		return 80, 24
	}
	return w, h
}

func terminalOutput(f *os.File) (func(), error) {
	var old uint32
	if r, _, e := getConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&old))); r == 0 {
		return nil, e
	}
	if r, _, e := setConsoleMode.Call(f.Fd(), uintptr(old|4)); r == 0 {
		return nil, e
	}
	return func() { setConsoleMode.Call(f.Fd(), uintptr(old)) }, nil
}
