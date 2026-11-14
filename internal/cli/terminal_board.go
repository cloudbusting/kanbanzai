package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

func runTerminalBoard(project *config.Project, in, out *os.File) error {
	restoreInput, err := terminalRaw(in)
	if err != nil {
		return err
	}
	defer restoreInput()
	restoreOutput, err := terminalOutput(out)
	if err != nil {
		return err
	}
	defer restoreOutput()
	defer fmt.Fprint(out, "\x1b[?25h\x1b[?1049l")
	if _, err := fmt.Fprint(out, "\x1b[?1049h\x1b[?25l"); err != nil {
		return err
	}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	keys := make(chan byte, 16)
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer close(keys)
		buf := make([]byte, 1)
		for {
			n, err := in.Read(buf)
			if n > 0 {
				select {
				case keys <- buf[0]:
				case <-done:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	col, row, escape := 0, 0, 0
	var previous *board.Board
	redraw := func() error {
		var readErr error
		if _, readErr = os.ReadDir(project.Work); readErr == nil {
			for _, folder := range project.Folders {
				if _, readErr = os.ReadDir(filepath.Join(project.Work, folder)); readErr != nil {
					break
				}
			}
		}
		var b *board.Board
		if readErr != nil {
			if previous == nil {
				return fmt.Errorf("board unavailable: %w", readErr)
			}
			copy := *previous
			copy.Problems = append([]string{"Board unavailable: " + readErr.Error()}, previous.Problems...)
			b = &copy
		} else {
			b = board.Load(project, nil, nil)
			previous = b
		}
		if len(b.Columns) == 0 {
			col = 0
		} else if col >= len(b.Columns) {
			col = len(b.Columns) - 1
		}
		maxRows := 0
		for _, c := range b.Columns {
			if len(c.Cards) > maxRows {
				maxRows = len(c.Cards)
			}
		}
		if row >= maxRows {
			row = maxRows - 1
		}
		if row < 0 {
			row = 0
		}
		w, h := terminalSize(out)
		_, err := fmt.Fprint(out, "\x1b[H\x1b[2J", terminalRender(b, w, h, col, row))
		return err
	}
	if err := redraw(); err != nil {
		return err
	}
	for {
		select {
		case <-interrupt:
			return nil
		case <-ticker.C:
			if err := redraw(); err != nil {
				return err
			}
		case key, ok := <-keys:
			if !ok {
				return nil
			}
			if escape == 1 {
				if key == '[' {
					escape = 2
				} else {
					escape = 0
				}
				continue
			}
			if escape == 2 {
				escape = 0
				switch key {
				case 'A':
					key = 'k'
				case 'B':
					key = 'j'
				case 'C':
					key = 'l'
				case 'D':
					key = 'h'
				}
			}
			switch key {
			case 27:
				escape = 1
				continue
			case 'q', 3:
				return nil
			case 'h':
				if col > 0 {
					col--
				}
			case 'l':
				col++
			case 'k':
				if row > 0 {
					row--
				}
			case 'j':
				row++
			}
			if err := redraw(); err != nil {
				return err
			}
		}
	}
}
