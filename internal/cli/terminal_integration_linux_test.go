package cli

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func ptyIoctl(f *os.File, request uintptr, ptr unsafe.Pointer) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), request, uintptr(ptr))
	if err != 0 {
		return err
	}
	return nil
}

func TestTerminalBoardRefreshAndRestore(t *testing.T) {
	p := newProject(t, "")
	path := writeCard(t, p, "W-01", "Before-refresh", "todo", "")
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no pseudo terminal: %v", err)
	}
	defer master.Close()
	var unlock int32
	if err = ptyIoctl(master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		t.Fatal(err)
	}
	var number uint32
	if err = ptyIoctl(master, syscall.TIOCGPTN, unsafe.Pointer(&number)); err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	size := struct{ rows, cols, x, y uint16 }{10, 28, 0, 0}
	if err = ptyIoctl(slave, syscall.TIOCSWINSZ, unsafe.Pointer(&size)); err != nil {
		t.Fatal(err)
	}
	var before, after syscall.Termios
	if err = ptyIoctl(slave, syscall.TCGETS, unsafe.Pointer(&before)); err != nil {
		t.Fatal(err)
	}
	chunks := make(chan string, 100)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				chunks <- string(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	result := make(chan error, 1)
	go func() { result <- runTerminalBoard(p, slave, slave) }()
	waitFor := func(want string) string {
		t.Helper()
		deadline := time.After(5 * time.Second)
		var text strings.Builder
		for {
			select {
			case chunk := <-chunks:
				text.WriteString(chunk)
				if strings.Contains(text.String(), want) {
					return text.String()
				}
			case <-deadline:
				t.Fatalf("did not see %q in terminal output %q", want, text.String())
			}
		}
	}
	waitFor("Before-refresh")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "Before-refresh", "After-refresh")), 0600); err != nil {
		t.Fatal(err)
	}
	waitFor("After-refresh")
	if err = os.Rename(p.Work, p.Work+"-hidden"); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(p.Work+"-hidden", p.Work)
	if output := waitFor("Board unavailable"); !strings.Contains(output, "After-refresh") {
		t.Fatalf("last good board was lost: %q", output)
	}
	if err = os.Rename(p.Work+"-hidden", p.Work); err != nil {
		t.Fatal(err)
	}
	if _, err = master.Write([]byte("\x1b[C")); err != nil {
		t.Fatal(err)
	}
	waitFor("Design/Designing")
	size.cols = 90
	if err = ptyIoctl(slave, syscall.TIOCSWINSZ, unsafe.Pointer(&size)); err != nil {
		t.Fatal(err)
	}
	waitFor("Design/Designed")
	if _, err = master.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("terminal did not quit")
	}
	if err = ptyIoctl(slave, syscall.TCGETS, unsafe.Pointer(&after)); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("terminal mode was not restored")
	}
}
