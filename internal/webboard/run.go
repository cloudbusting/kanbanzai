package webboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

type Options struct {
	Port     int
	NoOpen   bool
	Out, Err io.Writer
	open     func(string) error
	ready    func(string)
}

func Serve(ctx context.Context, p *config.Project, opts Options) error {
	if opts.Port < 0 || opts.Port > 65535 {
		return fmt.Errorf("port must be from 0 through 65535")
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(opts.Port)))
	if err != nil {
		return fmt.Errorf("listen on loopback: %w", err)
	}
	defer listener.Close()
	token, err := capability()
	if err != nil {
		return fmt.Errorf("generate access token: %w", err)
	}
	server := &http.Server{
		Handler:           handler(p, listener.Addr().String(), token),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second,
		MaxHeaderBytes: 16 << 10, ErrorLog: log.New(io.Discard, "", 0),
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer server.Close()
	url := "http://" + listener.Addr().String() + "/" + token + "/"
	fmt.Fprintln(opts.Out, url)
	fmt.Fprintln(opts.Err, "Read-only loopback board. Keep this URL private. Press Ctrl-C to stop.")
	if !opts.NoOpen {
		open := opts.open
		if open == nil {
			open = openBrowser
		}
		if err := open(url); err != nil {
			fmt.Fprintf(opts.Err, "Browser not opened: %v. Open the printed URL manually.\n", err)
		}
	}
	if opts.ready != nil {
		opts.ready(url)
	}
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
		}
		<-done
		return nil
	}
}

// Run owns only this foreground command's lifetime; it never writes server state.
func Run(p *config.Project, opts Options) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := Serve(ctx, p, opts); err != nil {
		if opts.Err != nil {
			fmt.Fprintf(opts.Err, "kanbanzai: browser board: %v\n", err)
		}
		return 1
	}
	return 0
}

func browserCommand(goos, url string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "rundll32.exe", []string{"url.dll,FileProtocolHandler", url}
	default:
		return "xdg-open", []string{url}
	}
}

func openBrowser(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	name, args := browserCommand(runtime.GOOS, url)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	return cmd.Run()
}
