package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Update routes by type, so a message nobody wrote a case for is dropped in
// silence, which is how a failed dispatch came to look like one that worked.
// The check is syntactic: it proves a handler exists, not that it is right.
func TestUpdate_EveryDeclaredMessageIsMatchedSomewhere(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	declared := map[string]token.Pos{}
	matched := map[string]bool{}

	for _, path := range packageSources(t) {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			collectMessageTypes(node, declared)
			collectMatchedTypes(node, matched)

			return true
		})
	}

	if len(declared) == 0 {
		t.Fatal("found no message types, so the check proves nothing")
	}

	for name, pos := range declared {
		if !matched[name] {
			t.Errorf("%s is declared at %s and matched nowhere, so Update drops it",
				name, fset.Position(pos))
		}
	}
}

func packageSources(t *testing.T) []string {
	t.Helper()

	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	sources := make([]string, 0, len(all))

	for _, path := range all {
		if !strings.HasSuffix(path, "_test.go") {
			sources = append(sources, path)
		}
	}

	if len(sources) == 0 {
		t.Fatalf("no sources under %s", mustGetwd(t))
	}

	return sources
}

func mustGetwd(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolving the working directory: %v", err)
	}

	return dir
}

func collectMessageTypes(node ast.Node, into map[string]token.Pos) {
	spec, ok := node.(*ast.TypeSpec)
	if ok && strings.HasSuffix(spec.Name.Name, "Msg") {
		into[spec.Name.Name] = spec.Pos()
	}
}

func collectMatchedTypes(node ast.Node, into map[string]bool) {
	switch n := node.(type) {
	case *ast.TypeAssertExpr:
		markMatched(into, n.Type)

	case *ast.TypeSwitchStmt:
		for _, clause := range n.Body.List {
			if caseClause, ok := clause.(*ast.CaseClause); ok {
				for _, expr := range caseClause.List {
					markMatched(into, expr)
				}
			}
		}
	}
}

func markMatched(into map[string]bool, expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.Ident:
		into[t.Name] = true
	case *ast.SelectorExpr:
		into[t.Sel.Name] = true
	}
}
