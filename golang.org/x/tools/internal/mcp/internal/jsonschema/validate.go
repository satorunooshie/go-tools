// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// The value of the "$schema" keyword for the version that we can validate.
const draft202012 = "https://json-schema.org/draft/2020-12/schema"

// Temporary definition of ResolvedSchema.
// The full definition deals with references between schemas, specifically the $id, $anchor and $ref keywords.
// We'll ignore that for now.
type ResolvedSchema struct {
	root *Schema
}

// Validate validates the instance, which must be a JSON value, against the schema.
// It returns nil if validation is successful or an error if it is not.
func (rs *ResolvedSchema) Validate(instance any) error {
	if s := rs.root.Schema; s != "" && s != draft202012 {
		return fmt.Errorf("cannot validate version %s, only %s", s, draft202012)
	}
	st := &state{rs: rs}
	var pathBuffer [4]any
	return st.validate(reflect.ValueOf(instance), st.rs.root, nil, pathBuffer[:0])
}

// state is the state of single call to ResolvedSchema.Validate.
type state struct {
	rs    *ResolvedSchema
	depth int
}

// validate validates the reflected value of the instance.
// It keeps track of the path within the instance for better error messages.
func (st *state) validate(instance reflect.Value, schema *Schema, callerAnns *annotations, path []any) (err error) {
	defer func() {
		if err != nil {
			if p := formatPath(path); p != "" {
				err = fmt.Errorf("%s: %w", p, err)
			}
		}
	}()

	st.depth++
	defer func() { st.depth-- }()
	if st.depth >= 100 {
		return fmt.Errorf("max recursion depth of %d reached", st.depth)
	}

	// Treat the nil schema like the empty schema, as accepting everything.
	if schema == nil {
		return nil
	}

	// Step through interfaces.
	if instance.IsValid() && instance.Kind() == reflect.Interface {
		instance = instance.Elem()
	}

	// type: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.1.1
	if schema.Type != "" || schema.Types != nil {
		gotType, ok := jsonType(instance)
		if !ok {
			return fmt.Errorf("%v of type %[1]T is not a valid JSON value", instance)
		}
		if schema.Type != "" {
			// "number" subsumes integers
			if !(gotType == schema.Type ||
				gotType == "integer" && schema.Type == "number") {
				return fmt.Errorf("type: %s has type %q, want %q", instance, gotType, schema.Type)
			}
		} else {
			if !(slices.Contains(schema.Types, gotType) || (gotType == "integer" && slices.Contains(schema.Types, "number"))) {
				return fmt.Errorf("type: %s has type %q, want one of %q",
					instance, gotType, strings.Join(schema.Types, ", "))
			}
		}
	}
	// enum: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.1.2
	if schema.Enum != nil {
		ok := false
		for _, e := range schema.Enum {
			if equalValue(reflect.ValueOf(e), instance) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("enum: %v does not equal any of: %v", instance, schema.Enum)
		}
	}

	// const: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.1.3
	if schema.Const != nil {
		if !equalValue(reflect.ValueOf(*schema.Const), instance) {
			return fmt.Errorf("const: %v does not equal %v", instance, *schema.Const)
		}
	}

	// numbers: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.2
	if schema.MultipleOf != nil || schema.Minimum != nil || schema.Maximum != nil || schema.ExclusiveMinimum != nil || schema.ExclusiveMaximum != nil {
		n, ok := jsonNumber(instance)
		if ok { // these keywords don't apply to non-numbers
			if schema.MultipleOf != nil {
				// TODO: validate MultipleOf as non-zero.
				// The test suite assumes floats.
				nf, _ := n.Float64() // don't care if it's exact or not
				if _, f := math.Modf(nf / *schema.MultipleOf); f != 0 {
					return fmt.Errorf("multipleOf: %s is not a multiple of %f", n, *schema.MultipleOf)
				}
			}

			m := new(big.Rat) // reuse for all of the following
			cmp := func(f float64) int { return n.Cmp(m.SetFloat64(f)) }

			if schema.Minimum != nil && cmp(*schema.Minimum) < 0 {
				return fmt.Errorf("minimum: %s is less than %f", n, *schema.Minimum)
			}
			if schema.Maximum != nil && cmp(*schema.Maximum) > 0 {
				return fmt.Errorf("maximum: %s is greater than %f", n, *schema.Maximum)
			}
			if schema.ExclusiveMinimum != nil && cmp(*schema.ExclusiveMinimum) <= 0 {
				return fmt.Errorf("exclusiveMinimum: %s is less than or equal to %f", n, *schema.ExclusiveMinimum)
			}
			if schema.ExclusiveMaximum != nil && cmp(*schema.ExclusiveMaximum) >= 0 {
				return fmt.Errorf("exclusiveMaximum: %s is greater than or equal to %f", n, *schema.ExclusiveMaximum)
			}
		}
	}

	// strings: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.3
	if instance.Kind() == reflect.String && (schema.MinLength != nil || schema.MaxLength != nil || schema.Pattern != "") {
		str := instance.String()
		n := utf8.RuneCountInString(str)
		if schema.MinLength != nil {
			if m := int(*schema.MinLength); n < m {
				return fmt.Errorf("minLength: %q contains %d Unicode code points, fewer than %d", str, n, m)
			}
		}
		if schema.MaxLength != nil {
			if m := int(*schema.MaxLength); n > m {
				return fmt.Errorf("maxLength: %q contains %d Unicode code points, more than %d", str, n, m)
			}
		}

		if schema.Pattern != "" {
			// TODO(jba): compile regexps during schema validation.
			m, err := regexp.MatchString(schema.Pattern, str)
			if err != nil {
				return err
			}
			if !m {
				return fmt.Errorf("pattern: %q does not match pattern %q", str, schema.Pattern)
			}
		}
	}

	// logic
	// https://json-schema.org/draft/2020-12/json-schema-core#section-10.2
	// These must happen before arrays and objects because if they evaluate an item or property,
	// then the unevaluatedItems/Properties schemas don't apply to it.
	// See https://json-schema.org/draft/2020-12/json-schema-core#section-11.2, paragraph 4.
	//
	// If any of these fail, then validation fails, even if there is an unevaluatedXXX
	// keyword in the schema. The spec is unclear about this, but that is the intention.

	var anns annotations // all the annotations for this call and child calls

	valid := func(s *Schema, anns *annotations) bool { return st.validate(instance, s, anns, path) == nil }

	if schema.AllOf != nil {
		for _, ss := range schema.AllOf {
			if err := st.validate(instance, ss, &anns, path); err != nil {
				return err
			}
		}
	}
	if schema.AnyOf != nil {
		// We must visit them all, to collect annotations.
		ok := false
		for _, ss := range schema.AnyOf {
			if valid(ss, &anns) {
				ok = true
			}
		}
		if !ok {
			return fmt.Errorf("anyOf: did not validate against any of %v", schema.AnyOf)
		}
	}
	if schema.OneOf != nil {
		// Exactly one.
		var okSchema *Schema
		for _, ss := range schema.OneOf {
			if valid(ss, &anns) {
				if okSchema != nil {
					return fmt.Errorf("oneOf: validated against both %v and %v", okSchema, ss)
				}
				okSchema = ss
			}
		}
		if okSchema == nil {
			return fmt.Errorf("oneOf: did not validate against any of %v", schema.OneOf)
		}
	}
	if schema.Not != nil {
		// Ignore annotations from "not".
		if valid(schema.Not, nil) {
			return fmt.Errorf("not: validated against %v", schema.Not)
		}
	}
	if schema.If != nil {
		var ss *Schema
		if valid(schema.If, &anns) {
			ss = schema.Then
		} else {
			ss = schema.Else
		}
		if ss != nil {
			if err := st.validate(instance, ss, &anns, path); err != nil {
				return err
			}
		}
	}

	// arrays
	if instance.Kind() == reflect.Array || instance.Kind() == reflect.Slice {
		// https://json-schema.org/draft/2020-12/json-schema-core#section-10.3.1
		// This validate call doesn't collect annotations for the items of the instance; they are separate
		// instances in their own right.
		// TODO(jba): if the test suite doesn't cover this case, add a test. For example, nested arrays.
		for i, ischema := range schema.PrefixItems {
			if i >= instance.Len() {
				break // shorter is OK
			}
			if err := st.validate(instance.Index(i), ischema, nil, append(path, i)); err != nil {
				return err
			}
		}
		anns.noteEndIndex(min(len(schema.PrefixItems), instance.Len()))

		if schema.Items != nil {
			for i := len(schema.PrefixItems); i < instance.Len(); i++ {
				if err := st.validate(instance.Index(i), schema.Items, nil, append(path, i)); err != nil {
					return err
				}
			}
			// Note that all the items in this array have been validated.
			anns.allItems = true
		}

		nContains := 0
		if schema.Contains != nil {
			for i := range instance.Len() {
				if err := st.validate(instance.Index(i), schema.Contains, nil, append(path, i)); err == nil {
					nContains++
					anns.noteIndex(i)
				}
			}
			if nContains == 0 && (schema.MinContains == nil || int(*schema.MinContains) > 0) {
				return fmt.Errorf("contains: %s does not have an item matching %s",
					instance, schema.Contains)
			}
		}

		// https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.4
		// TODO(jba): check that these next four keywords' values are integers.
		if schema.MinContains != nil && schema.Contains != nil {
			if m := int(*schema.MinContains); nContains < m {
				return fmt.Errorf("minContains: contains validated %d items, less than %d", nContains, m)
			}
		}
		if schema.MaxContains != nil && schema.Contains != nil {
			if m := int(*schema.MaxContains); nContains > m {
				return fmt.Errorf("maxContains: contains validated %d items, greater than %d", nContains, m)
			}
		}
		if schema.MinItems != nil {
			if m := int(*schema.MinItems); instance.Len() < m {
				return fmt.Errorf("minItems: array length %d is less than %d", instance.Len(), m)
			}
		}
		if schema.MaxItems != nil {
			if m := int(*schema.MaxItems); instance.Len() > m {
				return fmt.Errorf("minItems: array length %d is greater than %d", instance.Len(), m)
			}
		}
		if schema.UniqueItems {
			// Determine uniqueness with O(n²) comparisons.
			// TODO: optimize via hashing.
			for i := range instance.Len() {
				for j := i + 1; j < instance.Len(); j++ {
					if equalValue(instance.Index(i), instance.Index(j)) {
						return fmt.Errorf("uniqueItems: array items %d and %d are equal", i, j)
					}
				}
			}
		}
		// https://json-schema.org/draft/2020-12/json-schema-core#section-11.2
		if schema.UnevaluatedItems != nil && !anns.allItems {
			// Apply this subschema to all items in the array that haven't been successfully validated.
			// That includes validations by subschemas on the same instance, like allOf.
			for i := anns.endIndex; i < instance.Len(); i++ {
				if !anns.evaluatedIndexes[i] {
					if err := st.validate(instance.Index(i), schema.UnevaluatedItems, nil, append(path, i)); err != nil {
						return err
					}
				}
			}
			anns.allItems = true
		}
	}

	if callerAnns != nil {
		// Our caller wants to know what we've validated.
		callerAnns.merge(&anns)
	}
	return nil
}

func formatPath(path []any) string {
	var b strings.Builder
	for i, p := range path {
		if n, ok := p.(int); ok {
			fmt.Fprintf(&b, "[%d]", n)
		} else {
			if i > 0 {
				b.WriteByte('.')
			}
			fmt.Fprintf(&b, "%q", p)
		}
	}
	return b.String()
}
