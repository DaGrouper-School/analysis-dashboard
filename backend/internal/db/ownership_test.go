package db

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// root is the backend's root, from this package.
var root = filepath.Join("..", "..")

// ownership is a row of the table ownership in docs/development.md: the
// table's owning package, and the packages also allowed to use it.
type ownership struct {
	owner string
	also  []string
}

// row matches a row of the ownership table: | `table` | `owner` | also |.
var row = regexp.MustCompile("^\\| `([a-z_]+)` \\| `([a-z0-9/_-]+)` \\|(.*)\\|\\s*$")

var pkgRef = regexp.MustCompile("`([a-z0-9/_-]+/[a-z0-9/_-]+)`")

func readOwnership(t *testing.T) map[string]ownership {
	t.Helper()
	f, err := os.Open(filepath.Join(root, "docs", "development.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	owners := map[string]ownership{}
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			in = line == "## Table ownership"
			continue
		}
		if m := row.FindStringSubmatch(line); in && m != nil {
			o := ownership{owner: m[2]}
			for _, ref := range pkgRef.FindAllStringSubmatch(m[3], -1) {
				o.also = append(o.also, ref[1])
			}
			owners[m[1]] = o
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(owners) == 0 {
		t.Fatal(`docs/development.md has no "Table ownership" table`)
	}
	return owners
}

var (
	// write matches a table being written; read, one being queried.
	write = regexp.MustCompile(`(?:INSERT INTO|UPDATE|DELETE FROM|MERGE INTO|TRUNCATE(?: TABLE)?|COPY)\s+([a-z_]+)`)
	read  = regexp.MustCompile(`(?:FROM|JOIN)\s+([a-z_]+)`)
	// create matches a table being created.
	create = regexp.MustCompile(`CREATE TABLE (?:IF NOT EXISTS )?([a-z_]+)`)
	// schemaChange matches statements that create, alter or drop tables.
	schemaChange = regexp.MustCompile(`(?:CREATE|ALTER|DROP) TABLE`)
	// advisory matches taking or releasing an advisory lock.
	advisory = regexp.MustCompile(`pg_(?:try_)?advisory_`)
)

// schemaChangers are the packages that change the schema outside
// migrations, and why.
var schemaChangers = map[string]string{
	"internal/db": "creates schema_migrations before the first migration",
}

// TestTableOwnership fails when code queries or writes a table it neither
// owns nor is listed as also using, when a table has no owner, when a
// package changes the schema outside migrations without a reason in
// schemaChangers, or when a package other than internal/db takes an
// advisory lock itself instead of through db.Lock or db.TryLock.
func TestTableOwnership(t *testing.T) {
	t.Parallel()
	owners := readOwnership(t)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		migration := filepath.Base(filepath.Dir(path)) == "migrations" && strings.HasSuffix(path, ".sql")
		code := strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
		if !migration && !code {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pkg := ""
		if code {
			rel, _ := filepath.Rel(root, filepath.Dir(path))
			pkg = filepath.ToSlash(rel)
		}
		checkFile(t.Errorf, owners, path, pkg, src)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// checkFile checks one migration (pkg "") or source file of pkg,
// reporting each problem.
func checkFile(report func(format string, args ...any), owners map[string]ownership, path, pkg string, src []byte) {
	for _, m := range create.FindAllSubmatch(src, -1) {
		if _, ok := owners[string(m[1])]; !ok {
			report("%s creates %s, which has no owner in docs/development.md", path, m[1])
		}
	}
	if pkg == "" { // a migration may use any table
		return
	}
	if schemaChange.Match(src) && schemaChangers[pkg] == "" {
		report("%s changes the schema; only migrations and the packages in schemaChangers may", path)
	}
	if pkg != "internal/db" && advisory.Match(src) {
		report("%s takes an advisory lock itself; take it through db.Lock or db.TryLock, with its key in internal/db/locks.go", path)
	}
	for _, m := range write.FindAllSubmatch(src, -1) {
		o, ok := owners[string(m[1])]
		switch {
		case !ok:
			report("%s writes %s, which has no owner in docs/development.md", path, m[1])
		case o.owner != pkg && !slices.Contains(o.also, pkg):
			report("%s writes %s, owned by %s", path, m[1], o.owner)
		}
	}
	for _, m := range read.FindAllSubmatch(src, -1) {
		if o, ok := owners[string(m[1])]; ok && o.owner != pkg && !slices.Contains(o.also, pkg) {
			report("%s queries %s, owned by %s", path, m[1], o.owner)
		}
	}
}

// TestOwnershipRows guards the parser: a row names a table, its owner, and
// the packages also allowed with their reasons.
func TestOwnershipRows(t *testing.T) {
	t.Parallel()
	m := row.FindStringSubmatch("| `classroom_members` | `internal/classrooms` | `internal/accounts` reads names alongside a member list |")
	if m == nil || m[1] != "classroom_members" || m[2] != "internal/classrooms" {
		t.Fatalf("row not parsed: %v", m)
	}
	if refs := pkgRef.FindAllStringSubmatch(m[3], -1); len(refs) != 1 || refs[0][1] != "internal/accounts" {
		t.Fatalf("also-users %v", refs)
	}
}

// TestTheCheckCatches: a query, a write and an advisory lock outside their
// owner, and a table created without one, are each reported.
func TestTheCheckCatches(t *testing.T) {
	t.Parallel()
	owners := map[string]ownership{"users": {owner: "internal/accounts"}, "labs": {owner: "internal/labs", also: []string{"internal/dashboards"}}}
	for name, c := range map[string]struct {
		pkg, src string
	}{
		"another's table queried": {"internal/labs", "SELECT id FROM users"},
		"another's table joined":  {"internal/labs", "SELECT 1 FROM labs l JOIN users u ON true"},
		"another's table written": {"internal/labs", "UPDATE users SET x = 1"},
		"an unowned table":        {"internal/labs", "INSERT INTO gadgets VALUES (1)"},
		"a lock taken directly":   {"internal/labs", "SELECT pg_try_advisory_xact_lock(1)"},
		"a table made in code":    {"internal/labs", "CREATE TABLE labs_copy (id int)"},
	} {
		reported := 0
		checkFile(func(string, ...any) { reported++ }, owners, name, c.pkg, []byte(c.src))
		if reported == 0 {
			t.Errorf("%s: not reported", name)
		}
	}
	checkFile(t.Errorf, owners, "fine", "internal/dashboards", []byte("SELECT id FROM labs"))
	checkFile(t.Errorf, owners, "fine", "internal/accounts", []byte("UPDATE users SET x = 1"))
}
