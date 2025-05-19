// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mcp

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/lsprpc"
	"golang.org/x/tools/gopls/internal/util/moremaps"
	"golang.org/x/tools/internal/mcp"
)

// Serve start an MCP server serving at the input address.
// The server receives LSP session events on the specified channel, which the
// caller is responsible for closing. The server runs until the context is
// canceled.
func Serve(ctx context.Context, address string, eventChan <-chan lsprpc.SessionEvent, cache *cache.Cache, isDaemon bool) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	// TODO(hxjiang): expose the MCP server address to the LSP client.
	if isDaemon {
		log.Printf("Gopls MCP daemon: listening on address %s...", listener.Addr())
	}
	defer log.Printf("Gopls MCP server: exiting")

	svr := http.Server{
		Handler: HTTPHandler(eventChan, cache, isDaemon),
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	// Run the server until cancellation.
	go func() {
		<-ctx.Done()
		svr.Close()
	}()
	return svr.Serve(listener)
}

// HTTPHandler returns an HTTP handler for handling requests from MCP client.
func HTTPHandler(eventChan <-chan lsprpc.SessionEvent, cache *cache.Cache, isDaemon bool) http.Handler {
	var (
		mu          sync.Mutex                         // lock for mcpHandlers.
		mcpHandlers = make(map[string]*mcp.SSEHandler) // map from lsp session ids to MCP sse handlers.
	)

	// Spin up go routine listen to the session event channel until channel close.
	go func() {
		for event := range eventChan {
			mu.Lock()
			switch event.Type {
			case lsprpc.SessionStart:
				mcpHandlers[event.Session.ID()] = mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
					return newServer(cache, event.Session)
				})
			case lsprpc.SessionEnd:
				delete(mcpHandlers, event.Session.ID())
			}
			mu.Unlock()
		}
	}()

	// In daemon mode, gopls serves mcp server at ADDRESS/sessions/$SESSIONID.
	// Otherwise, gopls serves mcp server at ADDRESS.
	mux := http.NewServeMux()
	if isDaemon {
		mux.HandleFunc("/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
			sessionID := r.PathValue("id")

			mu.Lock()
			handler := mcpHandlers[sessionID]
			mu.Unlock()

			if handler == nil {
				http.Error(w, fmt.Sprintf("session %s not established", sessionID), http.StatusNotFound)
				return
			}

			handler.ServeHTTP(w, r)
		})
	} else {
		// TODO(hxjiang): should gopls serve only at a specific path?
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			// When not in daemon mode, gopls has at most one LSP session.
			_, handler, ok := moremaps.Arbitrary(mcpHandlers)
			mu.Unlock()

			if !ok {
				http.Error(w, "session not established", http.StatusNotFound)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
	return mux
}

func newServer(_ *cache.Cache, session *cache.Session) *mcp.Server {
	s := mcp.NewServer("golang", "v0.1", nil)

	// TODO(hxjiang): replace dummy tool with tools which use cache and session.
	s.AddTools(mcp.NewTool("hello_world", "Say hello to someone", helloHandler(session)))
	return s
}

type HelloParams struct {
	Name string `json:"name" mcp:"the name to say hi to"`
}

func helloHandler(_ *cache.Session) func(ctx context.Context, cc *mcp.ServerSession, request *HelloParams) ([]*mcp.Content, error) {
	return func(ctx context.Context, cc *mcp.ServerSession, request *HelloParams) ([]*mcp.Content, error) {
		return []*mcp.Content{
			mcp.NewTextContent("Hi " + request.Name),
		}, nil
	}
}
