// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	jsonrpc2 "golang.org/x/tools/internal/jsonrpc2_v2"
	"golang.org/x/tools/internal/mcp/jsonschema"
)

type hiParams struct {
	Name string
}

func sayHi(ctx context.Context, cc *ServerSession, v hiParams) ([]*Content, error) {
	if err := cc.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping failed: %v", err)
	}
	return []*Content{NewTextContent("hi " + v.Name)}, nil
}

func TestEndToEnd(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer("testServer", "v1.0.0", nil)

	// The 'greet' tool says hi.
	s.AddTools(NewTool("greet", "say hi", sayHi))

	// The 'fail' tool returns this error.
	failure := errors.New("mcp failure")
	s.AddTools(
		NewTool("fail", "just fail", func(context.Context, *ServerSession, struct{}) ([]*Content, error) {
			return nil, failure
		}),
	)

	s.AddPrompts(
		NewPrompt("code_review", "do a code review", func(_ context.Context, _ *ServerSession, params struct{ Code string }) (*GetPromptResult, error) {
			return &GetPromptResult{
				Description: "Code review prompt",
				Messages: []*PromptMessage{
					{Role: "user", Content: NewTextContent("Please review the following code: " + params.Code)},
				},
			}, nil
		}),
		NewPrompt("fail", "", func(_ context.Context, _ *ServerSession, params struct{}) (*GetPromptResult, error) {
			return nil, failure
		}),
	)

	// Connect the server.
	ss, err := s.Connect(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Collect(s.Sessions()); len(got) != 1 {
		t.Errorf("after connection, Clients() has length %d, want 1", len(got))
	}

	// Wait for the server to exit after the client closes its connection.
	var clientWG sync.WaitGroup
	clientWG.Add(1)
	go func() {
		if err := ss.Wait(); err != nil {
			t.Errorf("server failed: %v", err)
		}
		clientWG.Done()
	}()

	c := NewClient("testClient", "v1.0.0", nil)
	c.AddRoots(&Root{URI: "file:///root"})

	// Connect the client.
	cs, err := c.Connect(ctx, ct)
	if err != nil {
		t.Fatal(err)
	}

	if err := cs.Ping(ctx, nil); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	t.Run("prompts", func(t *testing.T) {
		res, err := cs.ListPrompts(ctx, nil)
		if err != nil {
			t.Errorf("prompts/list failed: %v", err)
		}
		wantPrompts := []*Prompt{
			{
				Name:        "code_review",
				Description: "do a code review",
				Arguments:   []*PromptArgument{{Name: "Code", Required: true}},
			},
			{Name: "fail"},
		}
		if diff := cmp.Diff(wantPrompts, res.Prompts); diff != "" {
			t.Fatalf("prompts/list mismatch (-want +got):\n%s", diff)
		}

		gotReview, err := cs.GetPrompt(ctx, &GetPromptParams{Name: "code_review", Arguments: map[string]string{"Code": "1+1"}})
		if err != nil {
			t.Fatal(err)
		}
		wantReview := &GetPromptResult{
			Description: "Code review prompt",
			Messages: []*PromptMessage{{
				Content: NewTextContent("Please review the following code: 1+1"),
				Role:    "user",
			}},
		}
		if diff := cmp.Diff(wantReview, gotReview); diff != "" {
			t.Errorf("prompts/get 'code_review' mismatch (-want +got):\n%s", diff)
		}

		if _, err := cs.GetPrompt(ctx, &GetPromptParams{Name: "fail"}); err == nil || !strings.Contains(err.Error(), failure.Error()) {
			t.Errorf("fail returned unexpected error: got %v, want containing %v", err, failure)
		}
	})

	t.Run("tools", func(t *testing.T) {
		res, err := cs.ListTools(ctx, nil)
		if err != nil {
			t.Errorf("tools/list failed: %v", err)
		}
		wantTools := []*Tool{
			{
				Name:        "fail",
				Description: "just fail",
				InputSchema: &jsonschema.Schema{
					Type:                 "object",
					AdditionalProperties: falseSchema,
				},
			},
			{
				Name:        "greet",
				Description: "say hi",
				InputSchema: &jsonschema.Schema{
					Type:     "object",
					Required: []string{"Name"},
					Properties: map[string]*jsonschema.Schema{
						"Name": {Type: "string"},
					},
					AdditionalProperties: falseSchema,
				},
			},
		}
		if diff := cmp.Diff(wantTools, res.Tools, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
			t.Fatalf("tools/list mismatch (-want +got):\n%s", diff)
		}

		gotHi, err := cs.CallTool(ctx, "greet", map[string]any{"name": "user"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		wantHi := &CallToolResult{
			Content: []*Content{{Type: "text", Text: "hi user"}},
		}
		if diff := cmp.Diff(wantHi, gotHi); diff != "" {
			t.Errorf("tools/call 'greet' mismatch (-want +got):\n%s", diff)
		}

		gotFail, err := cs.CallTool(ctx, "fail", map[string]any{}, nil)
		// Counter-intuitively, when a tool fails, we don't expect an RPC error for
		// call tool: instead, the failure is embedded in the result.
		if err != nil {
			t.Fatal(err)
		}
		wantFail := &CallToolResult{
			IsError: true,
			Content: []*Content{{Type: "text", Text: failure.Error()}},
		}
		if diff := cmp.Diff(wantFail, gotFail); diff != "" {
			t.Errorf("tools/call 'fail' mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("resources", func(t *testing.T) {
		resource1 := &Resource{
			Name:     "public",
			MIMEType: "text/plain",
			URI:      "file:///file1.txt",
		}
		resource2 := &Resource{
			Name:     "public", // names are not unique IDs
			MIMEType: "text/plain",
			URI:      "file:///nonexistent.txt",
		}

		readHandler := func(_ context.Context, _ *ServerSession, p *ReadResourceParams) (*ReadResourceResult, error) {
			if p.URI == "file:///file1.txt" {
				return &ReadResourceResult{
					Contents: &ResourceContents{
						Text: "file contents",
					},
				}, nil
			}
			return nil, ResourceNotFoundError(p.URI)
		}
		s.AddResources(
			&ServerResource{resource1, readHandler},
			&ServerResource{resource2, readHandler})

		lrres, err := cs.ListResources(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff([]*Resource{resource1, resource2}, lrres.Resources); diff != "" {
			t.Errorf("resources/list mismatch (-want, +got):\n%s", diff)
		}

		for _, tt := range []struct {
			uri      string
			mimeType string // "": not found; "text/plain": resource; "text/template": template
		}{
			{"file:///file1.txt", "text/plain"},
			{"file:///nonexistent.txt", ""},
			// TODO(jba): add resource template cases when we implement them
		} {
			rres, err := cs.ReadResource(ctx, &ReadResourceParams{URI: tt.uri})
			if err != nil {
				var werr *jsonrpc2.WireError
				if errors.As(err, &werr) && werr.Code == codeResourceNotFound {
					if tt.mimeType != "" {
						t.Errorf("%s: not found but expected it to be", tt.uri)
					}
				} else {
					t.Fatalf("reading %s: %v", tt.uri, err)
				}
			} else {
				if got := rres.Contents.URI; got != tt.uri {
					t.Errorf("got uri %q, want %q", got, tt.uri)
				}
				if got := rres.Contents.MIMEType; got != tt.mimeType {
					t.Errorf("%s: got MIME type %q, want %q", tt.uri, got, tt.mimeType)
				}
			}
		}
	})
	t.Run("roots", func(t *testing.T) {
		// Take the server's first ServerSession.
		var sc *ServerSession
		for sc = range s.Sessions() {
			break
		}

		rootRes, err := sc.ListRoots(ctx, &ListRootsParams{})
		if err != nil {
			t.Fatal(err)
		}
		gotRoots := rootRes.Roots
		wantRoots := slices.Collect(c.roots.all())
		if diff := cmp.Diff(wantRoots, gotRoots); diff != "" {
			t.Errorf("roots/list mismatch (-want +got):\n%s", diff)
		}
	})

	// Disconnect.
	cs.Close()
	clientWG.Wait()

	// After disconnecting, neither client nor server should have any
	// connections.
	for range s.Sessions() {
		t.Errorf("unexpected client after disconnection")
	}
}

// basicConnection returns a new basic client-server connection configured with
// the provided tools.
//
// The caller should cancel either the client connection or server connection
// when the connections are no longer needed.
func basicConnection(t *testing.T, tools ...*ServerTool) (*ServerSession, *ClientSession) {
	t.Helper()

	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer("testServer", "v1.0.0", nil)

	// The 'greet' tool says hi.
	s.AddTools(tools...)
	ss, err := s.Connect(ctx, st)
	if err != nil {
		t.Fatal(err)
	}

	c := NewClient("testClient", "v1.0.0", nil)
	cs, err := c.Connect(ctx, ct)
	if err != nil {
		t.Fatal(err)
	}
	return ss, cs
}

func TestServerClosing(t *testing.T) {
	cc, c := basicConnection(t, NewTool("greet", "say hi", sayHi))
	defer c.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if err := c.Wait(); err != nil {
			t.Errorf("server connection failed: %v", err)
		}
		wg.Done()
	}()
	if _, err := c.CallTool(ctx, "greet", map[string]any{"name": "user"}, nil); err != nil {
		t.Fatalf("after connecting: %v", err)
	}
	cc.Close()
	wg.Wait()
	if _, err := c.CallTool(ctx, "greet", map[string]any{"name": "user"}, nil); !errors.Is(err, ErrConnectionClosed) {
		t.Errorf("after disconnection, got error %v, want EOF", err)
	}
}

func TestBatching(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer("testServer", "v1.0.0", nil)
	_, err := s.Connect(ctx, st)
	if err != nil {
		t.Fatal(err)
	}

	c := NewClient("testClient", "v1.0.0", nil)
	// TODO: this test is broken, because increasing the batch size here causes
	// 'initialize' to block. Therefore, we can only test with a size of 1.
	const batchSize = 1
	BatchSize(ct, batchSize)
	cs, err := c.Connect(ctx, ct)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	errs := make(chan error, batchSize)
	for i := range batchSize {
		go func() {
			_, err := cs.ListTools(ctx, nil)
			errs <- err
		}()
		time.Sleep(2 * time.Millisecond)
		if i < batchSize-1 {
			select {
			case <-errs:
				t.Errorf("ListTools: unexpected result for incomplete batch: %v", err)
			default:
			}
		}
	}
}

func TestCancellation(t *testing.T) {
	var (
		start     = make(chan struct{})
		cancelled = make(chan struct{}, 1) // don't block the request
	)

	slowRequest := func(ctx context.Context, cc *ServerSession, v struct{}) ([]*Content, error) {
		start <- struct{}{}
		select {
		case <-ctx.Done():
			cancelled <- struct{}{}
		case <-time.After(5 * time.Second):
			return nil, nil
		}
		return nil, nil
	}
	_, sc := basicConnection(t, NewTool("slow", "a slow request", slowRequest))
	defer sc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go sc.CallTool(ctx, "slow", map[string]any{}, nil)
	<-start
	cancel()
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for cancellation")
	}
}

func TestAddMiddleware(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()
	s := NewServer("testServer", "v1.0.0", nil)
	ss, err := s.Connect(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	// Wait for the server to exit after the client closes its connection.
	var clientWG sync.WaitGroup
	clientWG.Add(1)
	go func() {
		if err := ss.Wait(); err != nil {
			t.Errorf("server failed: %v", err)
		}
		clientWG.Done()
	}()

	var buf bytes.Buffer
	buf.WriteByte('\n')

	// traceCalls creates a middleware function that prints the method before and after each call
	// with the given prefix.
	traceCalls := func(prefix string) func(ServerMethodHandler) ServerMethodHandler {
		return func(d ServerMethodHandler) ServerMethodHandler {
			return func(ctx context.Context, ss *ServerSession, method string, params any) (any, error) {
				fmt.Fprintf(&buf, "%s >%s\n", prefix, method)
				defer fmt.Fprintf(&buf, "%s <%s\n", prefix, method)
				return d(ctx, ss, method, params)
			}
		}
	}

	// "1" is the outer middleware layer, called first; then "2" is called, and finally
	// the default dispatcher.
	s.AddMiddleware(traceCalls("1"), traceCalls("2"))

	c := NewClient("testClient", "v1.0.0", nil)
	cs, err := c.Connect(ctx, ct)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Fatal(err)
	}
	want := `
1 >initialize
2 >initialize
2 <initialize
1 <initialize
1 >tools/list
2 >tools/list
2 <tools/list
1 <tools/list
`
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

var falseSchema = &jsonschema.Schema{Not: &jsonschema.Schema{}}
