// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package modernize

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
	"golang.org/x/tools/internal/analysisinternal"
	"golang.org/x/tools/internal/astutil/edge"
)

// stringsseq offers a fix to replace a call to strings.Split with
// SplitSeq or strings.Fields with FieldsSeq
// when it is the operand of a range loop, either directly:
//
//	for _, line := range strings.Split() {...}
//
// or indirectly, if the variable's sole use is the range statement:
//
//	lines := strings.Split()
//	for _, line := range lines {...}
//
// Variants:
// - bytes.SplitSeq
// - bytes.FieldsSeq
func stringsseq(pass *analysis.Pass) {
	if !analysisinternal.Imports(pass.Pkg, "strings") &&
		!analysisinternal.Imports(pass.Pkg, "bytes") {
		return
	}
	info := pass.TypesInfo
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	for curFile := range filesUsing(inspect, info, "go1.24") {
		for curRange := range curFile.Preorder((*ast.RangeStmt)(nil)) {
			rng := curRange.Node().(*ast.RangeStmt)

			// Reject "for i, line := ..." since SplitSeq is not an iter.Seq2.
			// (We require that i is blank.)
			if id, ok := rng.Key.(*ast.Ident); ok && id.Name != "_" {
				continue
			}

			// Find the call operand of the range statement,
			// whether direct or indirect.
			call, ok := rng.X.(*ast.CallExpr)
			if !ok {
				if id, ok := rng.X.(*ast.Ident); ok {
					if v, ok := info.Uses[id].(*types.Var); ok {
						if curRange.ParentEdge() == edge.BlockStmt_List && curRange.ParentIndex() > 0 {
							curPrev, _ := curRange.PrevSibling()
							if assign, ok := curPrev.Node().(*ast.AssignStmt); ok &&
								assign.Tok == token.DEFINE &&
								len(assign.Lhs) == 1 &&
								len(assign.Rhs) == 1 &&
								info.Defs[assign.Lhs[0].(*ast.Ident)] == v &&
								soleUse(info, v) == id {
								// Have:
								//    lines := ...
								//    for _, line := range lines {...}
								// and no other uses of lines.
								call, _ = assign.Rhs[0].(*ast.CallExpr)
							}
						}
					}
				}
			}

			if call != nil {
				var edits []analysis.TextEdit
				if rng.Key != nil {
					// Delete (blank) RangeStmt.Key:
					//  for _, line := -> for line :=
					//  for _, _    := -> for
					//  for _       := -> for
					end := rng.Range
					if rng.Value != nil {
						end = rng.Value.Pos()
					}
					edits = append(edits, analysis.TextEdit{
						Pos: rng.Key.Pos(),
						End: end,
					})
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					continue
				}

				obj := typeutil.Callee(info, call)
				if analysisinternal.IsFunctionNamed(obj, "strings", "Split", "Fields") ||
					analysisinternal.IsFunctionNamed(obj, "bytes", "Split", "Fields") {
					oldFnName := obj.Name()
					seqFnName := fmt.Sprintf("%sSeq", oldFnName)
					pass.Report(analysis.Diagnostic{
						Pos:      sel.Pos(),
						End:      sel.End(),
						Category: "stringsseq",
						Message:  fmt.Sprintf("Ranging over %s is more efficient", seqFnName),
						SuggestedFixes: []analysis.SuggestedFix{{
							Message: fmt.Sprintf("Replace %s with %s", oldFnName, seqFnName),
							TextEdits: append(edits, analysis.TextEdit{
								Pos:     sel.Sel.Pos(),
								End:     sel.Sel.End(),
								NewText: []byte(seqFnName)}),
						}},
					})
				}
			}
		}
	}
}
