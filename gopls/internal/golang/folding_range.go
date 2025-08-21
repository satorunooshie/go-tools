// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"cmp"
	"context"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/cache/parsego"
	"golang.org/x/tools/gopls/internal/file"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/util/bug"
	"golang.org/x/tools/gopls/internal/util/safetoken"
)

// FoldingRange gets all of the folding range for f.
func FoldingRange(ctx context.Context, snapshot *cache.Snapshot, fh file.Handle, lineFoldingOnly bool) ([]protocol.FoldingRange, error) {
	// TODO(suzmue): consider limiting the number of folding ranges returned, and
	// implement a way to prioritize folding ranges in that case.
	pgf, err := snapshot.ParseGo(ctx, fh, parsego.Full)
	if err != nil {
		return nil, err
	}

	// With parse errors, we wouldn't be able to produce accurate folding info.
	// LSP protocol (3.16) currently does not have a way to handle this case
	// (https://github.com/microsoft/language-server-protocol/issues/1200).
	// We cannot return an error either because we are afraid some editors
	// may not handle errors nicely. As a workaround, we now return an empty
	// result and let the client handle this case by double check the file
	// contents (i.e. if the file is not empty and the folding range result
	// is empty, raise an internal error).
	if pgf.ParseErr != nil {
		return nil, nil
	}

	// Get folding ranges for comments separately as they are not walked by ast.Inspect.
	ranges := commentsFoldingRange(pgf)

	// Walk the ast and collect folding ranges.
	filter := []ast.Node{
		(*ast.BasicLit)(nil),
		(*ast.BlockStmt)(nil),
		(*ast.CallExpr)(nil),
		(*ast.CaseClause)(nil),
		(*ast.CommClause)(nil),
		(*ast.CompositeLit)(nil),
		(*ast.FieldList)(nil),
		(*ast.GenDecl)(nil),
	}
	for cur := range pgf.Cursor.Preorder(filter...) {
		// TODO(suzmue): include trailing empty lines before the closing
		// parenthesis/brace.
		var kind protocol.FoldingRangeKind
		// start and end define the range of content to fold away.
		var start, end token.Pos
		switch n := cur.Node().(type) {
		case *ast.BlockStmt:
			// Fold between positions of or lines between "{" and "}".
			start, end = bracketedFoldingRange(n.Lbrace, n.Rbrace)

		case *ast.CaseClause:
			// Fold from position of ":" to end.
			start, end = n.Colon+1, n.End()

		case *ast.CommClause:
			// Fold from position of ":" to end.
			start, end = n.Colon+1, n.End()

		case *ast.CallExpr:
			// Fold between positions of or lines between "(" and ")".
			start, end = bracketedFoldingRange(n.Lparen, n.Rparen)

		case *ast.FieldList:
			// Fold between positions of or lines between opening parenthesis/brace and closing parenthesis/brace.
			start, end = bracketedFoldingRange(n.Opening, n.Closing)

		case *ast.GenDecl:
			// If this is an import declaration, set the kind to be protocol.Imports.
			if n.Tok == token.IMPORT {
				kind = protocol.Imports
			}
			// Fold between positions of or lines between "(" and ")".
			start, end = bracketedFoldingRange(n.Lparen, n.Rparen)

		case *ast.BasicLit:
			// Fold raw string literals from position of "`" to position of "`".
			if n.Kind == token.STRING && len(n.Value) >= 2 && n.Value[0] == '`' && n.Value[len(n.Value)-1] == '`' {
				start, end = n.Pos(), n.End()
			}

		case *ast.CompositeLit:
			// Fold between positions of or lines between "{" and "}".
			start, end = bracketedFoldingRange(n.Lbrace, n.Rbrace)

		default:
			panic(n)
		}

		// Check that folding positions are valid.
		if !start.IsValid() || !end.IsValid() {
			continue
		}
		if start == end {
			// Nothing to fold.
			continue
		}
		// in line folding mode, do not fold if the start and end lines are the same.
		if lineFoldingOnly && safetoken.Line(pgf.Tok, start) == safetoken.Line(pgf.Tok, end) {
			continue
		}
		rng, err := pgf.PosRange(start, end)
		if err != nil {
			bug.Reportf("failed to create range: %s", err) // can't happen
			continue
		}
		ranges = append(ranges, foldingRange(kind, rng))
	}

	// Sort by start position.
	slices.SortFunc(ranges, func(x, y protocol.FoldingRange) int {
		if d := cmp.Compare(*x.StartLine, *y.StartLine); d != 0 {
			return d
		}
		return cmp.Compare(*x.StartCharacter, *y.StartCharacter)
	})

	return ranges, nil
}

// bracketedFoldingRange returns the folding range for nodes with parentheses/braces/brackets
// that potentially can take up multiple lines.
func bracketedFoldingRange(open, close token.Pos) (token.Pos, token.Pos) {
	if !open.IsValid() || !close.IsValid() {
		return token.NoPos, token.NoPos
	}
	if open+1 == close {
		// Nothing to fold: (), {} or [].
		return token.NoPos, token.NoPos
	}

	// Clients with "LineFoldingOnly" set to true can fold only full lines.
	// This is checked in the caller.
	//
	// Clients that support folding ranges can display them in various ways
	// (e.g., how are folding ranges marked? is the final line displayed?).
	// The most common client
	// is vscode, which displays the first line followed by ..., and then does not
	// display any other lines in the range, but other clients might also display
	// final line of the range. For example, the following code
	//
	//	var x = []string{"a",
	//	"b",
	//	"c" }
	//
	// can be folded (in vscode) to
	//
	// var x = []string{"a", ...
	//
	// or in some other client
	//
	//	var x = []string{"a", ...
	//	"c" }
	//
	// This is a change in behavior. The old code would not fold this example,
	// nor would it have folded
	//
	// func foo() { // a non-godoc comment
	//  ...
	// }
	// which seems wrong.

	return open + 1, close
}

// commentsFoldingRange returns the folding ranges for all comment blocks in file.
// The folding range starts at the end of the first line of the comment block, and ends at the end of the
// comment block and has kind protocol.Comment.
func commentsFoldingRange(pgf *parsego.File) (comments []protocol.FoldingRange) {
	tokFile := pgf.Tok
	for _, commentGrp := range pgf.File.Comments {
		startGrpLine, endGrpLine := safetoken.Line(tokFile, commentGrp.Pos()), safetoken.Line(tokFile, commentGrp.End())
		if startGrpLine == endGrpLine {
			// Don't fold single line comments.
			continue
		}

		firstComment := commentGrp.List[0]
		startPos, endLinePos := firstComment.Pos(), firstComment.End()
		startCmmntLine, endCmmntLine := safetoken.Line(tokFile, startPos), safetoken.Line(tokFile, endLinePos)
		if startCmmntLine != endCmmntLine {
			// If the first comment spans multiple lines, then we want to have the
			// folding range start at the end of the first line.
			endLinePos = token.Pos(int(startPos) + len(strings.Split(firstComment.Text, "\n")[0]))
		}
		rng, err := pgf.PosRange(endLinePos, commentGrp.End())
		if err != nil {
			bug.Reportf("failed to create mapped range: %s", err) // can't happen
			continue
		}
		// Fold from the end of the first line comment to the end of the comment block.
		comments = append(comments, foldingRange(protocol.Comment, rng))
	}
	return comments
}

func foldingRange(kind protocol.FoldingRangeKind, rng protocol.Range) protocol.FoldingRange {
	return protocol.FoldingRange{
		// (I guess LSP doesn't use a protocol.Range here
		// because missing means something different from zero.)
		StartLine:      varOf(rng.Start.Line),
		StartCharacter: varOf(rng.Start.Character),
		EndLine:        varOf(rng.End.Line),
		EndCharacter:   varOf(rng.End.Character),
		Kind:           string(kind),
	}
}

// varOf returns a new variable whose value is x.
func varOf[T any](x T) *T { return &x }
