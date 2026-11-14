// Package webboard hosts the explicitly requested, read-only loopback board.
package webboard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	assets "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

func handler(project *config.Project, authority, token string) http.Handler {
	base := "/" + token + "/"
	origin := "http://" + authority
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce, err := capability()
		if err != nil {
			http.Error(w, "cannot generate page nonce", 500)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'nonce-"+nonce+"'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		origins := r.Header.Values("Origin")
		if r.Host != authority || len(origins) > 1 || (len(origins) == 1 && origins[0] != origin) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "read-only board", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, base) {
			http.NotFound(w, r)
			return
		}
		route := strings.TrimPrefix(r.URL.Path, base)
		var data []byte
		kind := "text/javascript; charset=utf-8"
		switch route {
		case "":
			kind = "text/html; charset=utf-8"
			data = []byte(fmt.Sprintf(page, nonce))
		case "sessions.js", "toasts.js", "render.js", "browser.js":
			data, err = assets.BoardFiles.ReadFile("board-view/" + route)
		case "snapshot", "brief":
			// Re-resolve config on every read, just as the editor's command reads do.
			var p *config.Project
			p, err = config.Find(project.Root)
			if err == nil {
				var info os.FileInfo
				info, err = os.Stat(p.Work)
				if err == nil && !info.IsDir() {
					err = fmt.Errorf("cards folder is not a directory")
				}
			}
			if err != nil {
				http.Error(w, "cannot read project configuration or cards folder", 500)
				return
			}
			loaded := board.Load(p, nil, nil)
			if route == "snapshot" {
				kind = "application/json; charset=utf-8"
				data, err = json.Marshal(struct {
					Board *board.Board `json:"board"`
					Owner string       `json:"owner"`
				}{loaded, p.Owner})
			} else {
				kind = "text/plain; charset=utf-8"
				data, err = readBrief(p, loaded, r.URL.Query().Get("id"))
				if err != nil {
					http.NotFound(w, r)
					return
				}
			}
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "cannot read board asset", 500)
			return
		}
		w.Header().Set("Content-Type", kind)
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
	})
}

// Root.Open (Go 1.24) confines symlinks as well as lexical paths, including a
// concurrent symlink replacement. It never opens a board-supplied absolute path.
func readBrief(p *config.Project, loaded *board.Board, id string) ([]byte, error) {
	for _, col := range loaded.Columns {
		for _, card := range col.Cards {
			if card.ID != id {
				continue
			}
			rel, err := filepath.Rel(p.Work, filepath.Join(p.Root, card.Path))
			if err != nil || !filepath.IsLocal(rel) || filepath.Ext(rel) != ".md" {
				return nil, os.ErrPermission
			}
			root, err := os.OpenRoot(p.Work)
			if err != nil {
				return nil, err
			}
			defer root.Close()
			f, err := root.Open(rel)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			info, err := f.Stat()
			if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
				return nil, os.ErrPermission
			}
			data, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
			if len(data) > 4<<20 {
				return nil, os.ErrPermission
			}
			return data, err
		}
	}
	return nil, os.ErrNotExist
}

func capability() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

const page = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Kanbanzai Board</title><style id="board-style" nonce="%s"></style>
<script src="sessions.js" defer></script><script src="toasts.js" defer></script><script src="render.js" defer></script><script src="browser.js" defer></script>
</head><body><div id="error" role="alert"></div><main id="content"></main>
<section id="brief" hidden><button id="brief-close" type="button">Back to board</button><h1 id="brief-title"></h1><pre id="brief-text"></pre></section>
</body></html>`
