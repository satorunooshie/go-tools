// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package misc

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/settings"
	. "golang.org/x/tools/gopls/internal/test/integration"
)

// This test exercises the filtering of code actions in generated files.
// Most code actions, being potential edits, are discarded, but
// some (GoTest, GoDoc) are pure queries, and so are allowed.
func TestCodeActionsInGeneratedFiles(t *testing.T) {
	const src = `
-- go.mod --
module example.com
go 1.19

-- src/a.go --
package a

func f() { g() }
func g() {}
-- gen/a.go --
// Code generated by hand; DO NOT EDIT.
package a

func f() { g() }
func g() {}
`

	Run(t, src, func(t *testing.T, env *Env) {
		check := func(filename string, wantKind ...protocol.CodeActionKind) {
			env.OpenFile(filename)
			loc := env.RegexpSearch(filename, `g\(\)`)
			actions, err := env.Editor.CodeAction(env.Ctx, loc, nil, protocol.CodeActionUnknownTrigger)
			if err != nil {
				t.Fatal(err)
			}

			type kinds = map[protocol.CodeActionKind]bool
			got := make(kinds)
			for _, act := range actions {
				got[act.Kind] = true
			}
			want := make(kinds)
			for _, kind := range wantKind {
				want[kind] = true
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("%s: unexpected CodeActionKinds: (-want +got):\n%s",
					filename, diff)
				t.Log(actions)
			}
		}

		check("src/a.go",
			settings.AddTest,
			settings.GoAssembly,
			settings.GoDoc,
			settings.GoFreeSymbols,
			settings.GoplsDocFeatures,
			settings.RefactorInlineCall)
		check("gen/a.go",
			settings.GoAssembly,
			settings.GoDoc,
			settings.GoFreeSymbols,
			settings.GoplsDocFeatures)
	})
}

// Test refactor.inline.call is not included in automatically triggered code action
// unless users want refactoring.
//
// (The mechanism behind this behavior has changed. It was added when
// we used to interpret CodeAction(Only=[]) as "all kinds", which was
// a distracting nuisance (too many lightbulbs); this was fixed by
// adding special logic to refactor.inline.call to respect the trigger
// kind; but now we do this for all actions (for similar reasons) and
// interpret Only=[] as Only=[quickfix] unless triggerKind=invoked;
// except that the test client always requests CodeAction(Only=[""]).
// So, we should remove the special logic from refactorInlineCall
// and vary the Only parameter used by the test client.)
func TestVSCodeIssue65167(t *testing.T) {
	const vim1 = `package main

func main() {
	Func()  // range to be selected
}

func Func() int { return 0 }
`

	Run(t, "", func(t *testing.T, env *Env) {
		env.CreateBuffer("main.go", vim1)
		for _, trigger := range []protocol.CodeActionTriggerKind{
			protocol.CodeActionUnknownTrigger,
			protocol.CodeActionInvoked,
			protocol.CodeActionAutomatic,
		} {
			t.Run(fmt.Sprintf("trigger=%v", trigger), func(t *testing.T) {
				for _, selectedRange := range []bool{false, true} {
					t.Run(fmt.Sprintf("range=%t", selectedRange), func(t *testing.T) {
						loc := env.RegexpSearch("main.go", "Func")
						if !selectedRange {
							// assume the cursor is placed at the beginning of `Func`, so end==start.
							loc.Range.End = loc.Range.Start
						}
						actions := env.CodeAction(loc, nil, trigger)
						want := trigger != protocol.CodeActionAutomatic || selectedRange
						if got := slices.ContainsFunc(actions, func(act protocol.CodeAction) bool {
							return act.Kind == settings.RefactorInlineCall
						}); got != want {
							t.Errorf("got refactor.inline.call = %t, want %t", got, want)
						}
					})
				}
			})
		}
	})
}