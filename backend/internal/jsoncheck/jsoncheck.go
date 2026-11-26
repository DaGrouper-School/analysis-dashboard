// Package jsoncheck decodes configuration strictly and reports every
// problem it finds at once, each with where it is, so whoever wrote the
// value can fix them all in one go.
package jsoncheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Invalid is a value refused, with every problem found.
type Invalid struct {
	// Subject names the value, such as privacy or the stages.
	Subject  string
	Problems []string
}

func (e *Invalid) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s is not valid:", e.Subject)
	for _, p := range e.Problems {
		b.WriteString("\n  - " + p)
	}
	return b.String()
}

// Fields are the problems by where each is, as an API's field errors:
// "texts.en.title: blank" is "blank" under texts.en.title. Problems about
// several fields at once are joined under the first part of their path.
func (e *Invalid) Fields() map[string]string {
	out := map[string]string{}
	for _, p := range e.Problems {
		at, msg, ok := strings.Cut(p, ": ")
		if !ok || strings.ContainsAny(at, " ") {
			at, msg = "value", p
		}
		if prev, dup := out[at]; dup {
			msg = prev + "; " + msg
		}
		out[at] = msg
	}
	return out
}

// Refuse is an *Invalid for subject when there are problems, and nil
// otherwise.
func Refuse(subject string, problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return &Invalid{Subject: subject, Problems: problems}
}

// Decode decodes raw as a T and checks its shape: JSON with nothing after
// it, no field T lacks, every field T has, null only where T's field is a
// pointer, and the right kind of value in each. Every problem is reported;
// for a value of the wrong kind, the first.
func Decode[T any](raw []byte) (T, []string) {
	var v T
	var in any
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&in); err != nil {
		return v, []string{describe(err)}
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return v, []string{"there is more after the value"}
	}
	var problems []string
	if err := json.Unmarshal(raw, &v); err != nil {
		problems = append(problems, describe(err))
	}
	emptyLists(reflect.ValueOf(&v))
	canonical, err := json.Marshal(v)
	if err != nil {
		return v, append(problems, err.Error())
	}
	var out any
	if err := json.Unmarshal(canonical, &out); err != nil {
		return v, append(problems, err.Error())
	}
	return v, append(problems, compare(in, out, "")...)
}

// Canonical is v as it is stored: re-encoded from its shape, so the same
// value always stores the same way.
func Canonical(v any) (json.RawMessage, error) { return json.Marshal(v) }

// emptyLists makes each nil list in v an empty one, so a list given as
// null is told apart from one given empty.
func emptyLists(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			emptyLists(v.Elem())
		}
	case reflect.Struct:
		for i := range v.NumField() {
			emptyLists(v.Field(i))
		}
	case reflect.Slice:
		if v.IsNil() && v.CanSet() {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		}
		for i := range v.Len() {
			emptyLists(v.Index(i))
		}
	}
}

// compare walks what was given, in, beside the value it decoded to, out,
// and reports each field out lacks, each field in lacks, and each null
// where out holds a value.
func compare(in, out any, path string) []string {
	if in == nil && out != nil {
		if path == "" {
			return []string{"the value is null"}
		}
		return []string{At(path) + ": null is not allowed"}
	}
	var problems []string
	switch o := out.(type) {
	case map[string]any:
		i, ok := in.(map[string]any)
		if !ok {
			return nil
		}
		for _, name := range sortedKeys(i) {
			if _, known := o[name]; !known {
				problems = append(problems, At(Join(path, name))+": not a field here")
			}
		}
		for _, name := range sortedKeys(o) {
			given, ok := i[name]
			if !ok {
				problems = append(problems, At(Join(path, name))+": missing")
				continue
			}
			problems = append(problems, compare(given, o[name], Join(path, name))...)
		}
	case []any:
		i, _ := in.([]any)
		for n := range min(len(i), len(o)) {
			problems = append(problems, compare(i[n], o[n], path+"["+strconv.Itoa(n)+"]")...)
		}
	}
	return problems
}

func sortedKeys[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// describe is a decoding error in the terms of whoever wrote the value.
func describe(err error) string {
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	switch {
	case errors.As(err, &typeErr):
		return fmt.Sprintf("%s: a %s where a %s belongs", At(typeErr.Field), typeErr.Value, kind(typeErr.Type.Kind()))
	case errors.As(err, &syntaxErr):
		return fmt.Sprintf("not JSON: %s at byte %d", syntaxErr.Error(), syntaxErr.Offset)
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "not JSON: the value ends early"
	}
	return strings.TrimPrefix(err.Error(), "json: ")
}

func kind(k reflect.Kind) string {
	switch k {
	case reflect.Int, reflect.Int64, reflect.Float64:
		return "number"
	case reflect.Slice:
		return "list"
	case reflect.Struct, reflect.Map:
		return "object"
	case reflect.Bool:
		return "true or false"
	}
	return k.String()
}

// Blank reports each text field of v that is empty or only spaces, other
// than those whose last part is named in optional, such as help.
func Blank(v any, path string, optional ...string) []string {
	var problems []string
	for _, t := range Texts(v, path) {
		name := t.Path[strings.LastIndexAny(t.Path, ".]")+1:]
		if strings.TrimSpace(t.Text) == "" && !slices.Contains(optional, name) {
			problems = append(problems, At(t.Path)+": blank")
		}
	}
	return problems
}

// Text is one text field of a value, and where it is.
type Text struct{ Path, Text string }

// Texts are the text fields of v, by path from path, in a fixed order.
func Texts(v any, path string) []Text {
	raw, _ := json.Marshal(v)
	var tree any
	_ = json.Unmarshal(raw, &tree)
	var out []Text
	var walk func(any, string)
	walk = func(node any, path string) {
		switch n := node.(type) {
		case string:
			out = append(out, Text{path, n})
		case map[string]any:
			for _, name := range sortedKeys(n) {
				walk(n[name], Join(path, name))
			}
		case []any:
			for i, item := range n {
				walk(item, path+"["+strconv.Itoa(i)+"]")
			}
		}
	}
	walk(tree, path)
	return out
}

// Slug is an ID of lower-case letters and digits, joined by - or _.
var Slug = regexp.MustCompile(`^[a-z0-9]+(?:[_-][a-z0-9]+)*$`)

// IDs checks each item's ID against pattern, and that none repeats.
func IDs[T any](path string, items []T, id func(T) string, pattern *regexp.Regexp) []string {
	var problems []string
	seen := map[string]int{}
	for i, item := range items {
		v := id(item)
		where := fmt.Sprintf("%s[%d]", path, i)
		if first, dup := seen[v]; dup {
			problems = append(problems, fmt.Sprintf("%s: %q repeats %s[%d]", where, v, path, first))
			continue
		}
		seen[v] = i
		if v != "" && !pattern.MatchString(v) {
			problems = append(problems, fmt.Sprintf("%s: %q is not an ID of lower-case letters, digits, - and _", where, v))
		}
	}
	return problems
}

var placeholder = regexp.MustCompile(`\{[^{}]*\}`)

// Placeholders reports each {name} in v's text that is not one of allowed,
// which the page fills in.
func Placeholders(path string, v any, allowed ...string) []string {
	var problems []string
	for _, field := range Texts(v, path) {
		for _, m := range placeholder.FindAllString(field.Text, -1) {
			if !slices.Contains(allowed, strings.Trim(m, "{}")) {
				problems = append(problems, fmt.Sprintf("%s: %s is not filled in; use {%s}", At(field.Path), m, strings.Join(allowed, "}, {")))
			}
		}
	}
	return problems
}

// HTTPSURL reports whether s is an absolute https URL.
func HTTPSURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// Join is path with name added.
func Join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// At is path for a message; the top of the value is "the value".
func At(path string) string {
	if path == "" {
		return "the value"
	}
	return path
}
