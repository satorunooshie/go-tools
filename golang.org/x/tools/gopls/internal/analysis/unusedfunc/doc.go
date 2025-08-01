// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package unusedfunc defines an analyzer that checks for unused
// functions and methods
//
// # Analyzer unusedfunc
//
// unusedfunc: check for unused functions, methods, etc
//
// The unusedfunc analyzer reports functions and methods that are
// never referenced outside of their own declaration.
//
// A function is considered unused if it is unexported and not
// referenced (except within its own declaration).
//
// A method is considered unused if it is unexported, not referenced
// (except within its own declaration), and its name does not match
// that of any method of an interface type declared within the same
// package.
//
// The tool may report false positives in some situations, for
// example:
//
//   - for a declaration of an unexported function that is referenced
//     from another package using the go:linkname mechanism, if the
//     declaration's doc comment does not also have a go:linkname
//     comment.
//
//     (Such code is in any case strongly discouraged: linkname
//     annotations, if they must be used at all, should be used on both
//     the declaration and the alias.)
//
//   - for compiler intrinsics in the "runtime" package that, though
//     never referenced, are known to the compiler and are called
//     indirectly by compiled object code.
//
//   - for functions called only from assembly.
//
//   - for functions called only from files whose build tags are not
//     selected in the current build configuration.
//
// Since these situations are relatively common in the low-level parts
// of the runtime, this analyzer ignores the standard library.
// See https://go.dev/issue/71686 and https://go.dev/issue/74130 for
// further discussion of these limitations.
//
// The unusedfunc algorithm is not as precise as the
// golang.org/x/tools/cmd/deadcode tool, but it has the advantage that
// it runs within the modular analysis framework, enabling near
// real-time feedback within gopls.
//
// The unusedfunc analyzer also reports unused types, vars, and
// constants. Enums--constants defined with iota--are ignored since
// even the unused values must remain present to preserve the logical
// ordering.
package unusedfunc
