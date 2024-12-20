// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// -- plundered from the future (CL 605517, issue #68021) --

// TODO(adonovan): replace with ast.Directive after go1.24 (#68021).

// A directive is a comment line with special meaning to the Go
// toolchain or another tool. It has the form:
//
//	//tool:name args
//
// The "tool:" portion is missing for the three directives named
// line, extern, and export.
//
// See https://go.dev/doc/comment#Syntax for details of Go comment
// syntax and https://pkg.go.dev/cmd/compile#hdr-Compiler_Directives
// for details of directives used by the Go compiler.
type directive struct {
	Pos  token.Pos // of preceding "//"
	Tool string
	Name string
	Args string // may contain internal spaces
}

// directives returns the directives within the comment.
func directives(g *ast.CommentGroup) (res []*directive) {
	if g != nil {
		// Avoid (*ast.CommentGroup).Text() as it swallows directives.
		for _, c := range g.List {
			if len(c.Text) > 2 &&
				c.Text[1] == '/' &&
				c.Text[2] != ' ' &&
				isDirective(c.Text[2:]) {

				tool, nameargs, ok := strings.Cut(c.Text[2:], ":")
				if !ok {
					// Must be one of {line,extern,export}.
					tool, nameargs = "", tool
				}
				name, args, _ := strings.Cut(nameargs, " ") // tab??
				res = append(res, &directive{
					Pos:  c.Slash,
					Tool: tool,
					Name: name,
					Args: strings.TrimSpace(args),
				})
			}
		}
	}
	return
}

// isDirective reports whether c is a comment directive.
// This code is also in go/printer.
func isDirective(c string) bool {
	// "//line " is a line directive.
	// "//extern " is for gccgo.
	// "//export " is for cgo.
	// (The // has been removed.)
	if strings.HasPrefix(c, "line ") || strings.HasPrefix(c, "extern ") || strings.HasPrefix(c, "export ") {
		return true
	}

	// "//[a-z0-9]+:[a-z0-9]"
	// (The // has been removed.)
	colon := strings.Index(c, ":")
	if colon <= 0 || colon+1 >= len(c) {
		return false
	}
	for i := 0; i <= colon+1; i++ {
		if i == colon {
			continue
		}
		b := c[i]
		if !('a' <= b && b <= 'z' || '0' <= b && b <= '9') {
			return false
		}
	}
	return true
}
