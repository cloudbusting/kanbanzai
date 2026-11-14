package cli

import (
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestBrowserBoardHelpAndInvalidModes(t *testing.T) {
	newProject(t, "")
	if code, out, _ := runCLI(t, "--help"); code != 0 || !strings.Contains(out, "--web") {
		t.Fatalf("browser option absent from help: %d %s", code, out)
	}
	for _, args := range [][]string{
		{"--web", "--json"}, {"--no-open"}, {"--port", "0"},
		{"--web", "--port", "-1"}, {"--web", "--port", "65536"},
		{"--web", "--port", "abc"}, {"--web", "extra"},
	} {
		if code, _, errOut := runCLI(t, append([]string{"board"}, args...)...); code != 2 {
			t.Errorf("%v: exit %d: %s", args, code, errOut)
		}
	}
}

func TestTerminalDeviceRejectsNull(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix null device")
	}
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if terminalDevice(f) {
		t.Fatal("null device is not an interactive terminal")
	}
}

func TestBrowserBoardReportsBindFailure(t *testing.T) {
	newProject(t, "")
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	code, _, message := runCLI(t, "board", "--web", "--no-open", "--port", port)
	if code != 1 || !strings.Contains(message, "listen on loopback") {
		t.Fatalf("bind error: %d %q", code, message)
	}
}

func TestBrowserFlagsValidateOutsideProject(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", t.TempDir())
	if code, _, _ := runCLI(t, "board", "--web", "--json"); code != 2 {
		t.Fatalf("invalid mode outside project: exit %d, want 2", code)
	}
}

func TestTerminalBoardModes(t *testing.T) {
	newProject(t, "")
	if code, out, _ := runCLI(t, "--help"); code != 0 || !strings.Contains(out, "--tui") {
		t.Errorf("terminal option absent from help: %d %s", code, out)
	}
	for _, args := range [][]string{{"--tui", "--json"}, {"--tui", "--web"}, {"--tui", "--port", "1234"}, {"--tui", "--no-open"}} {
		if code, _, errOut := runCLI(t, append([]string{"board"}, args...)...); code != 2 {
			t.Errorf("%v: exit %d, want usage error: %s", args, code, errOut)
		}
	}
	code, out, errOut := runCLI(t, "board", "--tui")
	if code != 1 || !strings.Contains(strings.ToLower(errOut), "interactive terminal") || strings.Contains(out, "\x1b") {
		t.Errorf("non-terminal: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}
