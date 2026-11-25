// Package contract holds this service to openapi.yaml, the contract the
// front end generates, and to contract-addendum.yaml, what this service
// serves before the front end has it: it finds the operation a request is
// for and checks a response against the statuses and schemas the operation
// declares. Tests use it; the service itself never reads the contract.
package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

// Prefix is where the contract's paths are served, its one server URL.
const Prefix = "/api"

// File is the contract's path from the backend's root, and AddendumFile
// the addendum's. The addendum refers to the contract's parts by this name,
// such as "openapi.yaml#/components/responses/Unauthenticated".
const (
	File         = "openapi.yaml"
	AddendumFile = "contract-addendum.yaml"
)

// base is where both files are found by the schema compiler, so that a
// reference from one to the other resolves.
const base = "https://contract.dgs.test/"

// How an addendum entry relates to the generated contract, in its x-change.
const (
	// ChangeNew is an operation the contract lacks.
	ChangeNew = "new"
	// ChangeChanged is an operation the contract has, with shapes this
	// service changes.
	ChangeChanged = "changed"
)

// internalSchema is the answer to an unexpected failure, which any
// operation may give: an Error whose code is internal (httpx.CodeInternal).
// The contract's ServiceErrorCode does not list that code, so it is
// checked here rather than against the contract's Error.
const internalSchema = `{
	"type": "object",
	"properties": {
		"code": {"const": "internal"},
		"message": {"type": "string"},
		"fields": {"type": "object", "additionalProperties": {"type": "string"}}
	},
	"required": ["code", "message", "fields"],
	"additionalProperties": false
}`

// Contract is openapi.yaml with its addendum, compiled.
type Contract struct {
	ops []*Operation
	// stale are the addendum's entries the contract has caught up with.
	stale    []string
	internal *jsonschema.Schema
	// errorBody is the contract's Error, the body of every refusal.
	errorBody *jsonschema.Schema
	// gaps are statuses let through that operations do not declare yet.
	gaps map[string]map[int]bool
}

// Gap is a status this service answers that an operation does not declare
// yet, with why. It belongs in the front end's routes, which the contract
// is generated from; until it is there, Checked lets the status through,
// checking only that its body is an Error.
type Gap struct {
	Operation string
	Status    int
	Why       string
}

// WithGaps is the contract letting gaps through.
func (c *Contract) WithGaps(gaps ...Gap) *Contract {
	out := *c
	out.gaps = map[string]map[int]bool{}
	for _, g := range gaps {
		if out.gaps[g.Operation] == nil {
			out.gaps[g.Operation] = map[int]bool{}
		}
		out.gaps[g.Operation][g.Status] = true
	}
	return &out
}

// check is op.Check, letting the contract's gaps through.
func (c *Contract) check(op *Operation, status int, header http.Header, body []byte) error {
	if c.gaps[op.ID][status] {
		gap := *op
		gap.responses = map[int]*jsonschema.Schema{status: c.errorBody}
		return gap.Check(status, header, body)
	}
	return op.Check(status, header, body)
}

// Operation is one method on one path of the contract.
type Operation struct {
	// ID is the operation's operationId, such as auth.getCurrentUser.
	ID     string
	Method string
	// Path is the template under Prefix, such as /classrooms/{classroomId}.
	Path string
	// WaitsFor is, for an operation from the addendum, the front-end change
	// it waits for; it is empty for one from openapi.yaml.
	WaitsFor string
	segs     []string
	// src is the file the operation is from, raw the operation as written
	// and shared the parameters its path declares for every method.
	src    *source
	raw    map[string]any
	shared any
	// responses maps each declared status to its body's schema, nil for a
	// response without a body.
	responses map[int]*jsonschema.Schema
	internal  *jsonschema.Schema
}

// Load reads and compiles the contract at path and the addendum at
// addendum. An addendum entry marked new adds an operation, and one marked
// changed replaces the contract's operation on its method and path; an
// entry that is malformed, or changes an operation the contract lacks, is
// refused.
func Load(path, addendum string) (*Contract, error) {
	comp := jsonschema.NewCompiler()
	comp.DefaultDraft(jsonschema.Draft2020)
	gen, err := addSource(comp, path, File)
	if err != nil {
		return nil, err
	}
	add, err := addSource(comp, addendum, AddendumFile)
	if err != nil {
		return nil, err
	}
	const internalURL = "urn:dgs:internal.json"
	internal, err := jsonschema.UnmarshalJSON(strings.NewReader(internalSchema))
	if err != nil {
		return nil, err
	}
	if err := comp.AddResource(internalURL, internal); err != nil {
		return nil, err
	}
	c := &Contract{}
	if c.internal, err = comp.Compile(internalURL); err != nil {
		return nil, err
	}
	l := loader{comp: comp, sources: map[string]*source{File: gen, AddendumFile: add}, internal: c.internal}
	if c.errorBody, err = l.schema(gen.url() + "#/components/schemas/Error"); err != nil {
		return nil, err
	}
	if c.ops, err = l.operations(gen); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	entries, err := l.operations(add)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", addendum, err)
	}
	if err := l.merge(c, entries); err != nil {
		return nil, fmt.Errorf("%s: %w", addendum, err)
	}
	return c, nil
}

// Find loads the contract and its addendum from the backend's root, found
// from the working directory upward, as tests run in their package's
// directory.
func Find() (*Contract, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return Load(filepath.Join(dir, File), filepath.Join(dir, AddendumFile))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errors.New("no go.mod above the working directory")
		}
		dir = parent
	}
}

// Stale lists the addendum's entries the generated contract has caught up
// with, each saying what to do: an entry marked changed whose shapes the
// contract now has, and one marked new for an operation the contract now
// has. Each is removed from the addendum, or, if the front end adopted it
// differently, marked changed.
func (c *Contract) Stale() []string { return slices.Clone(c.stale) }

// Operations lists every operation, in the contract's order.
func (c *Contract) Operations() []*Operation { return slices.Clone(c.ops) }

// Match is the operation a request for method and path (Prefix included)
// is for, or nil. A literal segment wins over a parameter, so
// /classrooms/mine is not taken for /classrooms/{classroomId}.
func (c *Contract) Match(method, path string) *Operation {
	rest, ok := strings.CutPrefix(path, Prefix)
	if !ok || (rest != "" && rest[0] != '/') {
		return nil
	}
	segs := split(rest)
	var best *Operation
	bestLiterals := -1
	for _, op := range c.ops {
		if op.Method != method {
			continue
		}
		if n, ok := op.match(segs); ok && n > bestLiterals {
			best, bestLiterals = op, n
		}
	}
	return best
}

// match reports whether segs fill the operation's path, and how many of
// its segments are literal.
func (o *Operation) match(segs []string) (int, bool) {
	if len(segs) != len(o.segs) {
		return 0, false
	}
	literals := 0
	for i, s := range o.segs {
		switch {
		case strings.HasPrefix(s, "{"):
			if segs[i] == "" {
				return 0, false
			}
		case s == segs[i]:
			literals++
		default:
			return 0, false
		}
	}
	return literals, true
}

// Example is a path that matches the operation, with each parameter
// filled in, Prefix included.
func (o *Operation) Example() string {
	segs := make([]string, len(o.segs))
	for i, s := range o.segs {
		if strings.HasPrefix(s, "{") {
			s = "x"
		}
		segs[i] = s
	}
	return Prefix + "/" + strings.Join(segs, "/")
}

// Check reports how a response breaks the operation's contract: a status
// it does not declare, a body where it declares none, or a body that does
// not match its schema. Any operation may answer 500 internal.
func (o *Operation) Check(status int, header http.Header, body []byte) error {
	schema, ok := o.responses[status]
	switch {
	case !ok && status == http.StatusInternalServerError:
		schema = o.internal
	case !ok:
		return fmt.Errorf("%s %s answered %d, which the contract does not declare", o.Method, o.Path, status)
	}
	if schema == nil {
		if len(body) > 0 {
			return fmt.Errorf("%s %s answered %d with a body, which the contract declares without one", o.Method, o.Path, status)
		}
		return nil
	}
	if mt, _, err := mime.ParseMediaType(header.Get("Content-Type")); err != nil || mt != "application/json" {
		return fmt.Errorf("%s %s answered %d as %q, not application/json", o.Method, o.Path, status, header.Get("Content-Type"))
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s %s answered %d with a body that is not JSON: %w", o.Method, o.Path, status, err)
	}
	if err := schema.Validate(v); err != nil {
		return fmt.Errorf("%s %s answered %d with a body the contract refuses: %w", o.Method, o.Path, status, err)
	}
	return nil
}

func split(path string) []string {
	return strings.Split(strings.TrimPrefix(path, "/"), "/")
}

// source is one of the two files, parsed.
type source struct {
	name string
	doc  map[string]any
}

func (s *source) url() string { return base + s.name }

// addSource reads the file at path and adds it to comp as name.
func addSource(comp *jsonschema.Compiler, path, name string) (*source, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src := &source{name: name}
	if err := yaml.Unmarshal(raw, &src.doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if src.doc == nil {
		src.doc = map[string]any{}
	}
	// The schema compiler takes JSON values, with numbers as json.Number.
	js, err := json.Marshal(src.doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(js))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := comp.AddResource(src.url(), value); err != nil {
		return nil, err
	}
	return src, nil
}

// loader compiles the parts of the contract an operation needs.
type loader struct {
	comp     *jsonschema.Compiler
	sources  map[string]*source
	internal *jsonschema.Schema
}

var methods = []string{"get", "put", "post", "delete", "patch", "head", "options"}

// operations lists every operation in src, in path order.
func (l loader) operations(src *source) ([]*Operation, error) {
	paths, _ := src.doc["paths"].(map[string]any)
	keys := make([]string, 0, len(paths))
	for p := range paths {
		keys = append(keys, p)
	}
	slices.Sort(keys)
	var ops []*Operation
	for _, p := range keys {
		item, _ := paths[p].(map[string]any)
		for _, m := range methods {
			raw, ok := item[m].(map[string]any)
			if !ok {
				continue
			}
			op, err := l.operation(src, p, m, raw)
			if err != nil {
				return nil, err
			}
			op.shared = item["parameters"]
			ops = append(ops, op)
		}
	}
	return ops, nil
}

func (l loader) operation(src *source, path, method string, raw map[string]any) (*Operation, error) {
	id, _ := raw["operationId"].(string)
	op := &Operation{ID: id, Method: strings.ToUpper(method), Path: path, segs: split(path),
		src: src, raw: raw, responses: map[int]*jsonschema.Schema{}}
	responses, _ := raw["responses"].(map[string]any)
	for code, r := range responses {
		status, err := strconv.Atoi(code)
		if err != nil {
			return nil, fmt.Errorf("%s %s: response %q is not a status code", op.Method, path, code)
		}
		at := src.url() + "#/paths/" + escape(path) + "/" + method + "/responses/" + code
		resp, _ := r.(map[string]any)
		if ref, ok := resp["$ref"].(string); ok {
			target, pointer, node := l.resolve(src, ref)
			if resp, _ = node.(map[string]any); resp == nil {
				return nil, fmt.Errorf("%s %s: %s does not exist", op.Method, path, ref)
			}
			at = target.url() + pointer
		}
		content, _ := resp["content"].(map[string]any)
		if _, ok := content["application/json"]; !ok {
			op.responses[status] = nil
			continue
		}
		s, err := l.schema(at + "/content/application~1json/schema")
		if err != nil {
			return nil, fmt.Errorf("%s %s %d: %w", op.Method, path, status, err)
		}
		op.responses[status] = s
	}
	op.internal = l.internal
	return op, nil
}

func (l loader) schema(url string) (*jsonschema.Schema, error) {
	return l.comp.Compile(url)
}

// merge applies the addendum's entries to c's operations, noting the ones
// the contract has caught up with.
func (l loader) merge(c *Contract, entries []*Operation) error {
	at := map[string]int{}
	for i, op := range c.ops {
		at[op.key()] = i
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Method + " " + e.Path
		if seen[e.key()] {
			return fmt.Errorf("%s is in the addendum twice", name)
		}
		seen[e.key()] = true
		if e.ID == "" {
			return fmt.Errorf("%s has no operationId", name)
		}
		waits, _ := e.raw["x-waits-for"].(string)
		if e.WaitsFor = strings.TrimSpace(waits); e.WaitsFor == "" {
			return fmt.Errorf("%s: x-waits-for names no front-end change", name)
		}
		i, has := at[e.key()]
		switch change, _ := e.raw["x-change"].(string); change {
		case ChangeNew:
			if !has {
				c.ops = append(c.ops, e)
				continue
			}
			c.stale = append(c.stale, fmt.Sprintf("%s is marked new, but %s has it now as %s: remove the entry if they match, or mark it changed",
				name, File, c.ops[i].ID))
		case ChangeChanged:
			if !has {
				return fmt.Errorf("%s is marked changed, but %s has no such operation", name, File)
			}
			if gen := c.ops[i]; gen.ID != e.ID {
				return fmt.Errorf("%s is %s in %s, not %s", name, gen.ID, File, e.ID)
			}
			if reflect.DeepEqual(l.shapes(c.ops[i]), l.shapes(e)) {
				c.stale = append(c.stale, fmt.Sprintf("%s is marked changed, but %s matches it now: remove the entry", name, File))
			}
		default:
			return fmt.Errorf("%s: x-change is %q, not %s or %s", name, change, ChangeNew, ChangeChanged)
		}
		c.ops[i] = e
	}
	return nil
}

// key is the operation's method and path, with its parameters unnamed, so
// /classrooms/{id} and /classrooms/{classroomId} are the same operation.
func (o *Operation) key() string {
	segs := slices.Clone(o.segs)
	for i, s := range segs {
		if strings.HasPrefix(s, "{") {
			segs[i] = "{}"
		}
	}
	return o.Method + " /" + strings.Join(segs, "/")
}

// shapes is what an operation takes and answers, with every reference
// followed and the prose left out, so an operation compares equal to one
// written differently that means the same.
func (l loader) shapes(o *Operation) any {
	var params []any
	for _, p := range [2]any{o.shared, o.raw["parameters"]} {
		list, _ := p.([]any)
		params = append(params, list...)
	}
	return []any{
		l.plain(o.src, params, false, nil),
		l.plain(o.src, o.raw["requestBody"], false, nil),
		l.plain(o.src, o.raw["responses"], true, nil),
	}
}

// prose are the keys that describe rather than shape.
var prose = []string{"description", "summary", "title", "example", "examples", "tags", "operationId", "deprecated"}

// named are the keys whose values are maps of names, not of keywords: a
// property called description is a shape, not prose.
var named = []string{"properties", "patternProperties", "$defs", "content", "responses", "headers"}

// plain is v from src with references followed and prose dropped. names
// says v's keys are names; seen are the references being followed, so a
// schema that refers to itself ends.
func (l loader) plain(src *source, v any, names bool, seen []string) any {
	switch v := v.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok && !names {
			target, pointer, node := l.resolve(src, ref)
			if target == nil || node == nil || slices.Contains(seen, target.name+pointer) {
				return map[string]any{"$ref": ref}
			}
			return l.plain(target, node, false, append(seen, target.name+pointer))
		}
		out := map[string]any{}
		for k, x := range v {
			if !names && (slices.Contains(prose, k) || strings.HasPrefix(k, "x-")) {
				continue
			}
			out[k] = l.plain(src, x, !names && slices.Contains(named, k), seen)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = l.plain(src, x, false, seen)
		}
		return out
	default:
		return v
	}
}

// resolve follows a reference from src, such as #/components/responses/X
// or, from the addendum, openapi.yaml#/components/schemas/Error, to the
// file it is in, the pointer within it, and what it points to (nil if
// nothing).
func (l loader) resolve(src *source, ref string) (*source, string, any) {
	file, frag, _ := strings.Cut(ref, "#")
	target := src
	if file != "" {
		if target = l.sources[file]; target == nil {
			return nil, "", nil
		}
	}
	var at any = target.doc
	for _, part := range strings.Split(strings.TrimPrefix(frag, "/"), "/") {
		m, ok := at.(map[string]any)
		if !ok {
			return target, "#" + frag, nil
		}
		at = m[unescape(part)]
	}
	return target, "#" + frag, at
}

// escape and unescape a JSON pointer segment.
func escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
func unescape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}
