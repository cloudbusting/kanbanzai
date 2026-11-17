// Package protocol is the store plugin protocol: one JSON request on a plugin's standard input, one JSON answer on its
// standard output, one process per request. docs/store-plugins.md is its public description.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/cloudbusting/kanbanzai/internal/store"
)

// Version is the protocol's version: a request carries it, and a plugin's hello answers it.
const Version = 1

// The methods.
const (
	MethodHello  = "hello"
	MethodList   = "list"
	MethodGet    = "get"
	MethodPut    = "put"
	MethodCreate = "create"
)

// Ticket is one ticket: list answers it without text, get with text.
type Ticket struct {
	ID    string `json:"id"`
	Rev   string `json:"rev"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type,omitempty"`
	Stage string `json:"stage,omitempty"`
	Text  string `json:"text,omitempty"`
}

// Fields are what put says about the brief it carries, so a plugin can map them without parsing Markdown.
type Fields struct {
	Stage string `json:"stage"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// Hello is a plugin's description of itself.
type Hello struct {
	Protocol     int      `json:"protocol"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

// Backend is what a plugin does: the protocol's methods, as Go.
type Backend interface {
	Hello() (Hello, error)
	List(query string) ([]Ticket, error)
	Get(ids []string) ([]Ticket, error)
	Put(id, baseRev, text string, fields Fields) (string, error)
	Create(cardType, title, text string) (id, rev string, err error)
}

// Request is one request on the wire.
type Request struct {
	Protocol int               `json:"protocol"`
	Method   string            `json:"method"`
	Settings map[string]string `json:"settings"`
	Query    string            `json:"query,omitempty"`
	IDs      []string          `json:"ids,omitempty"`
	ID       string            `json:"id,omitempty"`
	BaseRev  string            `json:"base_rev,omitempty"`
	Text     string            `json:"text,omitempty"`
	Fields   *Fields           `json:"fields,omitempty"`
	Type     string            `json:"type,omitempty"`
	Title    string            `json:"title,omitempty"`
}

// Reply is one answer on the wire: the method's fields, or an error.
type Reply struct {
	Protocol     int      `json:"protocol,omitempty"`
	Name         string   `json:"name,omitempty"`
	Version      string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Tickets      []Ticket `json:"tickets,omitempty"`
	ID           string   `json:"id,omitempty"`
	Rev          string   `json:"rev,omitempty"`
	Error        *Error   `json:"error,omitempty"`
}

// Error is a refusal on the wire.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorOf is the wire form of an error: a store refusal keeps its code; anything else is unavailable.
func ErrorOf(err error) *Error {
	var refusal *store.Error
	if errors.As(err, &refusal) {
		return &Error{Code: refusal.Code, Message: refusal.Error()}
	}
	return &Error{Code: store.CodeUnavailable, Message: err.Error()}
}

// Err is the store refusal an error answer stands for; an unknown code is unavailable.
func (e *Error) Err() error {
	switch e.Code {
	case store.CodeConflict, store.CodeNotFound, store.CodeExists, store.CodeRefused, store.CodeUnavailable,
		store.CodeUnsupported:
		return store.Refuse(e.Code, e.Message)
	}
	return store.Refuse(store.CodeUnavailable, fmt.Sprintf("%s: %s", e.Code, e.Message))
}

// Serve answers one request from in on out, with the backend open makes from the request's settings. It is the
// whole of a plugin's main.
func Serve(in io.Reader, out io.Writer, open func(settings map[string]string) (Backend, error)) error {
	var request Request
	decoder := json.NewDecoder(io.LimitReader(in, 64<<20))
	if err := decoder.Decode(&request); err != nil {
		return answer(out, Reply{Error: &Error{Code: store.CodeRefused, Message: "the request is not JSON: " + err.Error()}})
	}
	if request.Protocol != Version {
		return answer(out, Reply{Error: &Error{Code: store.CodeUnsupported,
			Message: fmt.Sprintf("protocol %d is not supported; this plugin speaks %d", request.Protocol, Version)}})
	}
	backend, err := open(request.Settings)
	if err != nil {
		return answer(out, Reply{Error: ErrorOf(err)})
	}
	reply, err := dispatch(backend, request)
	if err != nil {
		reply = Reply{Error: ErrorOf(err)}
	}
	return answer(out, reply)
}

func dispatch(backend Backend, request Request) (Reply, error) {
	switch request.Method {
	case MethodHello:
		hello, err := backend.Hello()
		return Reply{Protocol: hello.Protocol, Name: hello.Name, Version: hello.Version, Capabilities: hello.Capabilities}, err
	case MethodList:
		tickets, err := backend.List(request.Query)
		return Reply{Tickets: nonNil(tickets)}, err
	case MethodGet:
		tickets, err := backend.Get(request.IDs)
		return Reply{Tickets: nonNil(tickets)}, err
	case MethodPut:
		fields := Fields{}
		if request.Fields != nil {
			fields = *request.Fields
		}
		rev, err := backend.Put(request.ID, request.BaseRev, request.Text, fields)
		return Reply{Rev: rev}, err
	case MethodCreate:
		id, rev, err := backend.Create(request.Type, request.Title, request.Text)
		return Reply{ID: id, Rev: rev}, err
	}
	return Reply{}, store.Refuse(store.CodeUnsupported, fmt.Sprintf("unknown method %q", request.Method))
}

func nonNil(tickets []Ticket) []Ticket {
	if tickets == nil {
		return []Ticket{}
	}
	return tickets
}

func answer(out io.Writer, reply Reply) error {
	return json.NewEncoder(out).Encode(reply)
}
