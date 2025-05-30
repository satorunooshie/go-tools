// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package completion

import (
	"go/ast"

	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/util/astutil"
)

const (
	BREAK       = "break"
	CASE        = "case"
	CHAN        = "chan"
	CONST       = "const"
	CONTINUE    = "continue"
	DEFAULT     = "default"
	DEFER       = "defer"
	ELSE        = "else"
	FALLTHROUGH = "fallthrough"
	FOR         = "for"
	FUNC        = "func"
	GO          = "go"
	GOTO        = "goto"
	IF          = "if"
	IMPORT      = "import"
	INTERFACE   = "interface"
	MAP         = "map"
	PACKAGE     = "package"
	RANGE       = "range"
	RETURN      = "return"
	SELECT      = "select"
	STRUCT      = "struct"
	SWITCH      = "switch"
	TYPE        = "type"
	VAR         = "var"
)

// addKeywordCompletions offers keyword candidates appropriate at the position.
func (c *completer) addKeywordCompletions() {
	seen := make(map[string]bool)

	if c.wantTypeName() && c.inference.objType == nil {
		// If we want a type name but don't have an expected obj type,
		// include "interface", "struct", "func", "chan", and "map".

		// "interface" and "struct" are more common declaring named types.
		// Give them a higher score if we are in a type declaration.
		structIntf, funcChanMap := stdScore, highScore
		if len(c.path) > 1 {
			if _, namedDecl := c.path[1].(*ast.TypeSpec); namedDecl {
				structIntf, funcChanMap = highScore, stdScore
			}
		}

		c.addKeywordItems(seen, structIntf, STRUCT, INTERFACE)
		c.addKeywordItems(seen, funcChanMap, FUNC, CHAN, MAP)
	}

	// If we are at the file scope, only offer decl keywords. We don't
	// get *ast.Idents at the file scope because non-keyword identifiers
	// turn into *ast.BadDecl, not *ast.Ident.
	if len(c.path) == 1 || is[*ast.File](c.path[1]) {
		c.addKeywordItems(seen, stdScore, TYPE, CONST, VAR, FUNC, IMPORT)
		return
	} else if _, ok := c.path[0].(*ast.Ident); !ok {
		// Otherwise only offer keywords if the client is completing an identifier.
		return
	}

	if len(c.path) > 2 {
		// Offer "range" if we are in ast.ForStmt.Init. This is what the
		// AST looks like before "range" is typed, e.g. "for i := r<>".
		if loop, ok := c.path[2].(*ast.ForStmt); ok && loop.Init != nil && astutil.NodeContains(loop.Init, c.pos) {
			c.addKeywordItems(seen, stdScore, RANGE)
		}
	}

	// Only suggest keywords if we are beginning a statement.
	switch n := c.path[1].(type) {
	case *ast.BlockStmt, *ast.ExprStmt:
		// OK - our ident must be at beginning of statement.
	case *ast.CommClause:
		// Make sure we aren't in the Comm statement.
		if !n.Colon.IsValid() || c.pos <= n.Colon {
			return
		}
	case *ast.CaseClause:
		// Make sure we aren't in the case List.
		if !n.Colon.IsValid() || c.pos <= n.Colon {
			return
		}
	default:
		return
	}

	// Filter out keywords depending on scope
	// Skip the first one because we want to look at the enclosing scopes
	path := c.path[1:]
	for i, n := range path {
		switch node := n.(type) {
		case *ast.CaseClause:
			// only recommend "fallthrough" and "break" within the bodies of a case clause
			if c.pos > node.Colon {
				c.addKeywordItems(seen, stdScore, BREAK)
				// "fallthrough" is only valid in switch statements.
				// A case clause is always nested within a block statement in a switch statement,
				// that block statement is nested within either a TypeSwitchStmt or a SwitchStmt.
				if i+2 >= len(path) {
					continue
				}
				if _, ok := path[i+2].(*ast.SwitchStmt); ok {
					c.addKeywordItems(seen, stdScore, FALLTHROUGH)
				}
			}
		case *ast.CommClause:
			if c.pos > node.Colon {
				c.addKeywordItems(seen, stdScore, BREAK)
			}
		case *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.SwitchStmt:
			// if there is no default case yet, it's highly likely to add a default in switch.
			// we don't offer 'default' anymore if user has used it already in current switch.
			if !hasDefaultClause(node) {
				c.addKeywordItems(seen, highScore, CASE, DEFAULT)
			}
		case *ast.ForStmt, *ast.RangeStmt:
			c.addKeywordItems(seen, stdScore, BREAK, CONTINUE)
		// This is a bit weak, functions allow for many keywords
		case *ast.FuncDecl:
			if node.Body != nil && c.pos > node.Body.Lbrace {
				// requireReturnObj checks whether user must provide some objects after return.
				requireReturnObj := func(sig *ast.FuncType) bool {
					results := sig.Results
					if results == nil || results.List == nil {
						return false // nothing to return
					}
					// If any result is named, allow a bare return.
					for _, r := range results.List {
						for _, name := range r.Names {
							if name.Name != "_" {
								return false
							}
						}
					}
					return true
				}
				ret := RETURN
				if requireReturnObj(node.Type) {
					// as user must return something, we offer a space after return.
					// function literal inside a function will be affected by outer function,
					// but 'go fmt' will help to remove the ending space.
					// the benefit is greater than introducing an unnecessary space.
					ret += " "
				}

				c.addKeywordItems(seen, stdScore, DEFER, ret, FOR, GO, SWITCH, SELECT, IF, ELSE, VAR, CONST, GOTO, TYPE)
			}
		}
	}
}

// hasDefaultClause reports whether the given node contains a direct default case.
// It does not traverse child nodes to look for nested default clauses,
// and returns false if the node is not a switch statement.
func hasDefaultClause(node ast.Node) bool {
	var cases []ast.Stmt
	switch node := node.(type) {
	case *ast.TypeSwitchStmt:
		cases = node.Body.List
	case *ast.SelectStmt:
		cases = node.Body.List
	case *ast.SwitchStmt:
		cases = node.Body.List
	}
	for _, c := range cases {
		if clause, ok := c.(*ast.CaseClause); ok &&
			clause.List == nil { // default case
			return true
		}
	}
	return false
}

// addKeywordItems dedupes and adds completion items for the specified
// keywords with the specified score.
func (c *completer) addKeywordItems(seen map[string]bool, score float64, kws ...string) {
	for _, kw := range kws {
		if seen[kw] {
			continue
		}
		seen[kw] = true

		if matchScore := c.matcher.Score(kw); matchScore > 0 {
			c.items = append(c.items, CompletionItem{
				Label:      kw,
				Kind:       protocol.KeywordCompletion,
				InsertText: kw,
				Score:      score * float64(matchScore),
			})
		}
	}
}
