// Package contract holds this service to openapi.yaml, the contract the
// front end generates: it finds the operation a request is for and checks a
// response against the statuses and schemas the operation declares. Tests
// use it; the service itself never reads the contract.
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
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

// Prefix is where the contract's paths are served, its one server URL.
const Prefix = "/api"

// File is the contract's path from the backend's root.
const File = "openapi.yaml"

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

// Contract is openapi.yaml, compiled.
type Contract struct {
	ops      []*Operation
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
	segs []string
	// responses maps each declared status to its body's schema, nil for a
	// response without a body.
	responses map[int]*jsonschema.Schema
	internal  *jsonschema.Schema
}

// Load reads and compiles the contract at path.
func Load(path string) (*Contract, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// The schema compiler takes JSON values, with numbers as json.Number.
	js, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(js))
	if err != nil {
		return nil, err
	}
	const url, internalURL = "urn:dgs:openapi.json", "urn:dgs:internal.json"
	comp := jsonschema.NewCompiler()
	comp.DefaultDraft(jsonschema.Draft2020)
	if err := comp.AddResource(url, value); err != nil {
		return nil, err
	}
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
	l := loader{comp: comp, url: url, doc: doc, internal: c.internal}
	if c.errorBody, err = l.schema("#/components/schemas/Error"); err != nil {
		return nil, err
	}
	if err := l.operations(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Find loads the contract from the backend's root, found from the working
// directory upward, as tests run in their package's directory.
func Find() (*Contract, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return Load(filepath.Join(dir, File))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errors.New("no go.mod above the working directory")
		}
		dir = parent
	}
}

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

// loader compiles the parts of the contract an operation needs.
type loader struct {
	comp     *jsonschema.Compiler
	url      string
	doc      map[string]any
	internal *jsonschema.Schema
}

var methods = []string{"get", "put", "post", "delete", "patch", "head", "options"}

// operations adds every operation in the contract to c.
func (l loader) operations(c *Contract) error {
	paths, _ := l.doc["paths"].(map[string]any)
	keys := make([]string, 0, len(paths))
	for p := range paths {
		keys = append(keys, p)
	}
	slices.Sort(keys)
	for _, p := range keys {
		item, _ := paths[p].(map[string]any)
		for _, m := range methods {
			raw, ok := item[m].(map[string]any)
			if !ok {
				continue
			}
			op, err := l.operation(p, m, raw)
			if err != nil {
				return err
			}
			c.ops = append(c.ops, op)
		}
	}
	return nil
}

func (l loader) operation(path, method string, raw map[string]any) (*Operation, error) {
	id, _ := raw["operationId"].(string)
	op := &Operation{ID: id, Method: strings.ToUpper(method), Path: path, segs: split(path),
		responses: map[int]*jsonschema.Schema{}}
	responses, _ := raw["responses"].(map[string]any)
	for code, r := range responses {
		status, err := strconv.Atoi(code)
		if err != nil {
			return nil, fmt.Errorf("%s %s: response %q is not a status code", op.Method, path, code)
		}
		at := "#/paths/" + escape(path) + "/" + method + "/responses/" + code
		resp, _ := r.(map[string]any)
		if ref, ok := resp["$ref"].(string); ok {
			at = ref
			resp = l.resolve(ref)
			if resp == nil {
				return nil, fmt.Errorf("%s %s: %s does not exist", op.Method, path, ref)
			}
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

func (l loader) schema(pointer string) (*jsonschema.Schema, error) {
	return l.comp.Compile(l.url + pointer)
}

// resolve follows a local reference such as #/components/responses/X.
func (l loader) resolve(ref string) map[string]any {
	var at any = l.doc
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		m, ok := at.(map[string]any)
		if !ok {
			return nil
		}
		at = m[unescape(part)]
	}
	m, _ := at.(map[string]any)
	return m
}

// escape and unescape a JSON pointer segment.
func escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
func unescape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}
