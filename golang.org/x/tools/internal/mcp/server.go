// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/url"
	"slices"
	"sync"

	jsonrpc2 "golang.org/x/tools/internal/jsonrpc2_v2"
)

// A Server is an instance of an MCP server.
//
// Servers expose server-side MCP features, which can serve one or more MCP
// sessions by using [Server.Start] or [Server.Run].
type Server struct {
	// fixed at creation
	name    string
	version string
	opts    ServerOptions

	mu        sync.Mutex
	prompts   *featureSet[*ServerPrompt]
	tools     *featureSet[*ServerTool]
	resources *featureSet[*ServerResource]
	sessions  []*ServerSession
}

// ServerOptions is used to configure behavior of the server.
type ServerOptions struct {
	Instructions string
}

// NewServer creates a new MCP server. The resulting server has no features:
// add features using [Server.AddTools]. (TODO: support more features).
//
// The server can be connected to one or more MCP clients using [Server.Start]
// or [Server.Run].
//
// If non-nil, the provided options is used to configure the server.
func NewServer(name, version string, opts *ServerOptions) *Server {
	if opts == nil {
		opts = new(ServerOptions)
	}
	return &Server{
		name:      name,
		version:   version,
		opts:      *opts,
		prompts:   newFeatureSet(func(p *ServerPrompt) string { return p.Prompt.Name }),
		tools:     newFeatureSet(func(t *ServerTool) string { return t.Tool.Name }),
		resources: newFeatureSet(func(r *ServerResource) string { return r.Resource.URI }),
	}
}

// AddPrompts adds the given prompts to the server,
// replacing any with the same names.
func (s *Server) AddPrompts(prompts ...*ServerPrompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts.add(prompts...)
	// Assume there was a change, since add replaces existing prompts.
	// (It's possible a prompt was replaced with an identical one, but not worth checking.)
	// TODO(rfindley): notify connected clients
}

// RemovePrompts removes the prompts with the given names.
// It is not an error to remove a nonexistent prompt.
func (s *Server) RemovePrompts(names ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prompts.remove(names...) {
		// TODO: notify
	}
}

// AddTools adds the given tools to the server,
// replacing any with the same names.
func (s *Server) AddTools(tools ...*ServerTool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools.add(tools...)
	// Assume there was a change, since add replaces existing tools.
	// (It's possible a tool was replaced with an identical one, but not worth checking.)
	// TODO(rfindley): notify connected clients
}

// RemoveTools removes the tools with the given names.
// It is not an error to remove a nonexistent tool.
func (s *Server) RemoveTools(names ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tools.remove(names...) {
		// TODO: notify
	}
}

// ResourceNotFoundError returns an error indicating that a resource being read could
// not be found.
func ResourceNotFoundError(uri string) error {
	return &jsonrpc2.WireError{
		Code:    codeResourceNotFound,
		Message: "Resource not found",
		Data:    json.RawMessage(fmt.Sprintf(`{"uri":%q}`, uri)),
	}
}

// The error code to return when a resource isn't found.
// See https://modelcontextprotocol.io/specification/2025-03-26/server/resources#error-handling
// However, the code they chose in in the wrong space
// (see https://github.com/modelcontextprotocol/modelcontextprotocol/issues/509).
// so we pick a different one, arbirarily for now (until they fix it).
// The immediate problem is that jsonprc2 defines -32002 as "server closing".
const codeResourceNotFound = -31002

// A ResourceHandler is a function that reads a resource.
// If it cannot find the resource, it should return the result of calling [ResourceNotFoundError].
type ResourceHandler func(context.Context, *ServerSession, *ReadResourceParams) (*ReadResourceResult, error)

// A ServerResource associates a Resource with its handler.
type ServerResource struct {
	Resource *Resource
	Handler  ResourceHandler
}

// AddResource adds the given resource to the server and associates it with
// a [ResourceHandler], which will be called when the client calls [ClientSession.ReadResource].
// If a resource with the same URI already exists, this one replaces it.
// AddResource panics if a resource URI is invalid or not absolute (has an empty scheme).
func (s *Server) AddResources(resources ...*ServerResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range resources {
		u, err := url.Parse(r.Resource.URI)
		if err != nil {
			panic(err) // url.Parse includes the URI in the error
		}
		if !u.IsAbs() {
			panic(fmt.Errorf("URI %s needs a scheme", r.Resource.URI))
		}
		s.resources.add(r)
	}
	// TODO: notify
}

// RemoveResources removes the resources with the given URIs.
// It is not an error to remove a nonexistent resource.
func (s *Server) RemoveResources(uris ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources.remove(uris...)
}

// Sessions returns an iterator that yields the current set of server sessions.
func (s *Server) Sessions() iter.Seq[*ServerSession] {
	s.mu.Lock()
	clients := slices.Clone(s.sessions)
	s.mu.Unlock()
	return slices.Values(clients)
}

func (s *Server) listPrompts(_ context.Context, _ *ServerSession, params *ListPromptsParams) (*ListPromptsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := new(ListPromptsResult)
	for p := range s.prompts.all() {
		res.Prompts = append(res.Prompts, p.Prompt)
	}
	return res, nil
}

func (s *Server) getPrompt(ctx context.Context, cc *ServerSession, params *GetPromptParams) (*GetPromptResult, error) {
	s.mu.Lock()
	prompt, ok := s.prompts.get(params.Name)
	s.mu.Unlock()
	if !ok {
		// TODO: surface the error code over the wire, instead of flattening it into the string.
		return nil, fmt.Errorf("%s: unknown prompt %q", jsonrpc2.ErrInvalidParams, params.Name)
	}
	return prompt.Handler(ctx, cc, params.Arguments)
}

func (s *Server) listTools(_ context.Context, _ *ServerSession, params *ListToolsParams) (*ListToolsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := new(ListToolsResult)
	for t := range s.tools.all() {
		res.Tools = append(res.Tools, t.Tool)
	}
	return res, nil
}

func (s *Server) callTool(ctx context.Context, cc *ServerSession, params *CallToolParams) (*CallToolResult, error) {
	s.mu.Lock()
	tool, ok := s.tools.get(params.Name)
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%s: unknown tool %q", jsonrpc2.ErrInvalidParams, params.Name)
	}
	return tool.Handler(ctx, cc, params)
}

func (s *Server) listResources(_ context.Context, _ *ServerSession, params *ListResourcesParams) (*ListResourcesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := new(ListResourcesResult)
	for r := range s.resources.all() {
		res.Resources = append(res.Resources, r.Resource)
	}
	return res, nil
}

func (s *Server) readResource(ctx context.Context, ss *ServerSession, params *ReadResourceParams) (*ReadResourceResult, error) {
	uri := params.URI
	// Look up the resource URI in the list we have.
	// This is a security check as well as an information lookup.
	s.mu.Lock()
	resource, ok := s.resources.get(uri)
	s.mu.Unlock()
	if !ok {
		// Don't expose the server configuration to the client.
		// Treat an unregistered resource the same as a registered one that couldn't be found.
		return nil, ResourceNotFoundError(uri)
	}
	res, err := resource.Handler(ctx, ss, params)
	if err != nil {
		return nil, err
	}
	if res == nil || res.Contents == nil {
		return nil, fmt.Errorf("reading resource %s: read handler returned nil information", uri)
	}
	// As a convenience, populate some fields.
	if res.Contents.URI == "" {
		res.Contents.URI = uri
	}
	if res.Contents.MIMEType == "" {
		res.Contents.MIMEType = resource.Resource.MIMEType
	}
	return res, nil
}

// Run runs the server over the given transport, which must be persistent.
//
// Run blocks until the client terminates the connection.
func (s *Server) Run(ctx context.Context, t Transport) error {
	ss, err := s.Connect(ctx, t)
	if err != nil {
		return err
	}
	return ss.Wait()
}

// bind implements the binder[*ServerSession] interface, so that Servers can
// be connected using [connect].
func (s *Server) bind(conn *jsonrpc2.Connection) *ServerSession {
	cc := &ServerSession{conn: conn, server: s}
	s.mu.Lock()
	s.sessions = append(s.sessions, cc)
	s.mu.Unlock()
	return cc
}

// disconnect implements the binder[*ServerSession] interface, so that
// Servers can be connected using [connect].
func (s *Server) disconnect(cc *ServerSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = slices.DeleteFunc(s.sessions, func(cc2 *ServerSession) bool {
		return cc2 == cc
	})
}

// Connect connects the MCP server over the given transport and starts handling
// messages.
//
// It returns a connection object that may be used to terminate the connection
// (with [Connection.Close]), or await client termination (with
// [Connection.Wait]).
func (s *Server) Connect(ctx context.Context, t Transport) (*ServerSession, error) {
	return connect(ctx, t, s)
}

// A ServerSession is a logical connection from a single MCP client. Its
// methods can be used to send requests or notifications to the client. Create
// a session by calling [Server.Connect].
//
// Call [ServerSession.Close] to close the connection, or await client
// termination with [ServerSession.Wait].
type ServerSession struct {
	server *Server
	conn   *jsonrpc2.Connection

	mu               sync.Mutex
	initializeParams *initializeParams
	initialized      bool
}

// Ping makes an MCP "ping" request to the client.
func (ss *ServerSession) Ping(ctx context.Context, _ *PingParams) error {
	return call(ctx, ss.conn, "ping", nil, nil)
}

func (ss *ServerSession) ListRoots(ctx context.Context, params *ListRootsParams) (*ListRootsResult, error) {
	return standardCall[ListRootsResult](ctx, ss.conn, "roots/list", params)
}

func (ss *ServerSession) handle(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	ss.mu.Lock()
	initialized := ss.initialized
	ss.mu.Unlock()

	// From the spec:
	// "The client SHOULD NOT send requests other than pings before the server
	// has responded to the initialize request."
	switch req.Method {
	case "initialize", "ping":
	default:
		if !initialized {
			return nil, fmt.Errorf("method %q is invalid during session initialization", req.Method)
		}
	}

	// TODO: embed the incoming request ID in the client context (or, more likely,
	// a wrapper around it), so that we can correlate responses and notifications
	// to the handler; this is required for the new session-based transport.

	switch req.Method {
	case "initialize":
		return dispatch(ctx, ss, req, ss.initialize)

	case "ping":
		// The spec says that 'ping' expects an empty object result.
		return struct{}{}, nil

	case "prompts/list":
		return dispatch(ctx, ss, req, ss.server.listPrompts)

	case "prompts/get":
		return dispatch(ctx, ss, req, ss.server.getPrompt)

	case "tools/list":
		return dispatch(ctx, ss, req, ss.server.listTools)

	case "tools/call":
		return dispatch(ctx, ss, req, ss.server.callTool)

	case "resources/list":
		return dispatch(ctx, ss, req, ss.server.listResources)

	case "resources/read":
		return dispatch(ctx, ss, req, ss.server.readResource)

	case "notifications/initialized":
	}
	return nil, jsonrpc2.ErrNotHandled
}

func (ss *ServerSession) initialize(ctx context.Context, _ *ServerSession, params *initializeParams) (*initializeResult, error) {
	ss.mu.Lock()
	ss.initializeParams = params
	ss.mu.Unlock()

	// Mark the connection as initialized when this method exits. TODO:
	// Technically, the server should not be considered initialized until it has
	// *responded*, but we don't have adequate visibility into the jsonrpc2
	// connection to implement that easily. In any case, once we've initialized
	// here, we can handle requests.
	defer func() {
		ss.mu.Lock()
		ss.initialized = true
		ss.mu.Unlock()
	}()

	return &initializeResult{
		// TODO(rfindley): support multiple protocol versions.
		ProtocolVersion: "2024-11-05",
		Capabilities: &serverCapabilities{
			Prompts: &promptCapabilities{
				ListChanged: false, // not yet supported
			},
			Tools: &toolCapabilities{
				ListChanged: false, // not yet supported
			},
		},
		Instructions: ss.server.opts.Instructions,
		ServerInfo: &implementation{
			Name:    ss.server.name,
			Version: ss.server.version,
		},
	}, nil
}

// Close performs a graceful shutdown of the connection, preventing new
// requests from being handled, and waiting for ongoing requests to return.
// Close then terminates the connection.
func (ss *ServerSession) Close() error {
	return ss.conn.Close()
}

// Wait waits for the connection to be closed by the client.
func (ss *ServerSession) Wait() error {
	return ss.conn.Wait()
}

// dispatch turns a strongly type request handler into a jsonrpc2 handler.
//
// Importantly, it returns nil if the handler returned an error, which is a
// requirement of the jsonrpc2 package.
func dispatch[TParams, TResult any](ctx context.Context, conn *ServerSession, req *jsonrpc2.Request, f func(context.Context, *ServerSession, TParams) (TResult, error)) (any, error) {
	var params TParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, err
	}
	// Important: avoid returning a typed nil, as it can't be handled by the
	// jsonrpc2 package.
	res, err := f(ctx, conn, params)
	if err != nil {
		return nil, err
	}
	return res, nil
}
