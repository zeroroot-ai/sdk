// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The manifest guard (ADR-0097, ADR-0094, sdk#130).
//
// A component declares itself in its own binary at check-in. No code in the
// sdk parses a manifest file that a component author writes. component.yaml
// had five schemas in three repos, and each one was added for a local reason.
// This guard stops a sixth one.
//
// The guard asserts two rules on each non-test Go file of the module:
//
//   - manifest-file: no string literal names component.yaml, permissions.yaml
//     or plugin.yaml (or the .yml form). A path join that produces one of
//     these names starts from such a literal.
//   - component-decode: no YAML decode has a target that describes a
//     component. A target describes a component when its keys, nested types
//     included, hold methods, secrets or system dependencies, or hold kind,
//     name and version together.
//
// These inputs stay legal, and the fixture proves each one:
//
//   - taxonomy.yaml and ontology.yaml are codegen inputs, like a .proto. The
//     ontology target does not describe a component. The taxonomy target has
//     the same shape as a component, so it has an exemption.
//   - A signed catalog manifest is an operator-side artifact that the
//     platform makes from the binary of a component. Its target holds a
//     signature key, so it is not an authored manifest.
//   - Helm values are deployment input that the operator owns. Their targets
//     do not describe a component.
//
// An exemption names one finding by its content: the rule, the package and
// the literal or the decoded type. It states a reason. An exemption that no
// finding matches fails the guard, so the list cannot rot.

// manifestFileNames are the manifest files that a component author writes.
var manifestFileNames = map[string]bool{
	"component.yaml":   true,
	"component.yml":    true,
	"permissions.yaml": true,
	"permissions.yml":  true,
	"plugin.yaml":      true,
	"plugin.yml":       true,
}

// componentKeys are keys that only a component declaration holds.
var componentKeys = []string{"methods", "secrets", "system_dependencies", "systemdependencies"}

// identityKeys together describe a component: a kind, a name and a version.
var identityKeys = []string{"kind", "name", "version"}

// signedKeys mark an operator-side artifact that the platform signed.
var signedKeys = []string{"signature", "signatures"}

// yamlModules are the YAML packages whose decode the guard inspects.
var yamlModules = []string{
	"gopkg.in/yaml.v2",
	"gopkg.in/yaml.v3",
	"go.yaml.in/yaml/v2",
	"go.yaml.in/yaml/v3",
	"go.yaml.in/yaml/v4",
	"sigs.k8s.io/yaml",
	"github.com/goccy/go-yaml",
	"github.com/ghodss/yaml",
	"buf.build/go/protoyaml",
}

// yamlDecodeFuncs are the package-level decode functions. A Decode method on
// any receiver in a file that imports a YAML package also counts.
var yamlDecodeFuncs = map[string]bool{"Unmarshal": true, "UnmarshalStrict": true, "UnmarshalWithOptions": true}

const (
	ruleManifestFile    = "manifest-file"
	ruleComponentDecode = "component-decode"
	ruleUnresolved      = "unresolved-target"
)

// manifestExemption names one finding by its content.
type manifestExemption struct {
	rule   string // ruleManifestFile or ruleComponentDecode
	pkg    string // import path of the package that holds the finding
	target string // the literal, or the decoded type as "<import path>.<Name>"
	reason string
}

// manifestExemptions is the one list of findings that the guard accepts.
var manifestExemptions = []manifestExemption{
	{
		rule:   ruleComponentDecode,
		pkg:    "github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/parser",
		target: "github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/schema.Taxonomy",
		reason: "taxonomy.yaml is a codegen input, like a .proto. Its kind is core or extension, and its names are the names of node types, not of a component.",
	},
}

// manifestFinding is one breach of a rule.
type manifestFinding struct {
	rule, file, pkg, target string
}

func (f manifestFinding) String() string {
	switch f.rule {
	case ruleManifestFile:
		return fmt.Sprintf("%s: the literal %q names a component-authored manifest file (ADR-0097)", f.file, f.target)
	case ruleComponentDecode:
		return fmt.Sprintf("%s: a YAML decode into %s, which describes a component (ADR-0097)", f.file, f.target)
	default:
		return fmt.Sprintf("%s: a YAML decode into %s, which the guard cannot resolve. Decode into a named type or a declared variable", f.file, f.target)
	}
}

// typeDecl is one named type and the file that declares it.
type typeDecl struct {
	expr ast.Expr
	file *ast.File
	pkg  string
}

// moduleIndex holds the parsed non-test files of a module.
type moduleIndex struct {
	module string
	files  map[string][]*ast.File         // import path -> files
	paths  map[*ast.File]string           // file -> path relative to the root
	types  map[string]map[string]typeDecl // import path -> type name -> decl
	parsed int
}

// readModulePath returns the module path that root/go.mod states.
func readModulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Clean(filepath.Join(root, "go.mod")))
	if err != nil {
		return "", fmt.Errorf("read the go.mod of %s: %w", root, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", fmt.Errorf("%s/go.mod states no module path", root)
}

// indexModule parses each non-test .go file under root. It skips testdata,
// vendor and the worktree and git directories, as the Go tool does.
func indexModule(root string) (*moduleIndex, error) {
	module, err := readModulePath(root)
	if err != nil {
		return nil, err
	}
	idx := &moduleIndex{
		module: module,
		files:  map[string][]*ast.File{},
		paths:  map[*ast.File]string{},
		types:  map[string]map[string]typeDecl{},
	}
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case "testdata", "vendor", ".worktrees", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, p, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", p, parseErr)
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		pkg := module
		if dir := path.Dir(rel); dir != "." {
			pkg = module + "/" + dir
		}
		idx.files[pkg] = append(idx.files[pkg], file)
		idx.paths[file] = rel
		idx.parsed++
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts := spec.(*ast.TypeSpec)
				if idx.types[pkg] == nil {
					idx.types[pkg] = map[string]typeDecl{}
				}
				idx.types[pkg][ts.Name.Name] = typeDecl{expr: ts.Type, file: file, pkg: pkg}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return idx, nil
}

// importName returns the name under which a file refers to an import.
func importName(imp *ast.ImportSpec) (name, importPath string) {
	importPath, _ = strconv.Unquote(imp.Path.Value)
	if imp.Name != nil {
		return imp.Name.Name, importPath
	}
	base := path.Base(importPath)
	switch {
	case strings.HasPrefix(base, "yaml.v"), base == "go-yaml":
		return "yaml", importPath
	case len(base) > 1 && base[0] == 'v' && strings.Trim(base[1:], "0123456789") == "":
		// A major version suffix such as yaml/v3.
		return path.Base(path.Dir(importPath)), importPath
	}
	return base, importPath
}

// fileImports maps each import name of a file to its import path.
func fileImports(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range file.Imports {
		name, importPath := importName(imp)
		out[name] = importPath
	}
	return out
}

func isYAMLModule(importPath string) bool {
	for _, m := range yamlModules {
		if importPath == m || strings.HasPrefix(importPath, m+"/") {
			return true
		}
	}
	return false
}

// scanManifestParsers applies both rules to each indexed file.
func scanManifestParsers(idx *moduleIndex) []manifestFinding {
	var findings []manifestFinding
	pkgs := make([]string, 0, len(idx.files))
	for pkg := range idx.files {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		for _, file := range idx.files[pkg] {
			findings = append(findings, scanManifestFile(idx, pkg, file)...)
		}
	}
	return findings
}

func scanManifestFile(idx *moduleIndex, pkg string, file *ast.File) []manifestFinding {
	var findings []manifestFinding
	rel := idx.paths[file]
	imports := fileImports(file)
	yamlNames := map[string]bool{}
	for name, importPath := range imports {
		if isYAMLModule(importPath) {
			yamlNames[name] = true
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			if node.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(node.Value)
			if err != nil {
				return true
			}
			if manifestFileNames[strings.ToLower(path.Base(filepath.ToSlash(value)))] {
				findings = append(findings, manifestFinding{ruleManifestFile, rel, pkg, value})
			}
		case *ast.CallExpr:
			if len(yamlNames) == 0 {
				return true
			}
			target := decodeTarget(node, yamlNames)
			if target == nil {
				return true
			}
			typ, name := resolveTarget(pkg, file, target)
			if typ == nil {
				findings = append(findings, manifestFinding{ruleUnresolved, rel, pkg, exprString(target)})
				return true
			}
			keys := map[string]int{}
			collectKeys(idx, pkg, file, typ, 0, keys, map[string]bool{})
			if describesComponent(keys) {
				findings = append(findings, manifestFinding{ruleComponentDecode, rel, pkg, name})
			}
		}
		return true
	})
	return findings
}

// decodeTarget returns the target argument of a YAML decode call, or nil.
func decodeTarget(call *ast.CallExpr, yamlNames map[string]bool) ast.Expr {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	if x, ok := sel.X.(*ast.Ident); ok && yamlNames[x.Name] {
		if yamlDecodeFuncs[sel.Sel.Name] && len(call.Args) >= 2 {
			return call.Args[1]
		}
		return nil
	}
	if sel.Sel.Name == "Decode" && len(call.Args) == 1 {
		return call.Args[0]
	}
	return nil
}

func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.UnaryExpr:
		return x.Op.String() + exprString(x.X)
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(x.X)
	}
	return reflect.TypeOf(e).String()
}

// resolveTarget finds the type of a decode target in a file of pkg. It
// returns the type expression and a name for it, or a nil type.
func resolveTarget(pkg string, file *ast.File, target ast.Expr) (typ ast.Expr, name string) {
	expr := target
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = u.X
	}
	switch x := expr.(type) {
	case *ast.CompositeLit:
		typ = x.Type
	case *ast.Ident:
		typ = identType(x)
	case *ast.CallExpr:
		// new(T)
		if fn, ok := x.Fun.(*ast.Ident); ok && fn.Name == "new" && len(x.Args) == 1 {
			typ = x.Args[0]
		}
	}
	if typ == nil {
		return nil, ""
	}
	for star, ok := typ.(*ast.StarExpr); ok; star, ok = typ.(*ast.StarExpr) {
		typ = star.X
	}
	switch t := typ.(type) {
	case *ast.Ident:
		return t, pkg + "." + t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return t, fileImports(file)[x.Name] + "." + t.Sel.Name
		}
	}
	return typ, pkg + ".<anonymous " + reflect.TypeOf(typ).Elem().Name() + ">"
}

// identType returns the declared type of a local identifier, or nil.
func identType(id *ast.Ident) ast.Expr {
	if id.Obj == nil {
		return nil
	}
	switch decl := id.Obj.Decl.(type) {
	case *ast.ValueSpec:
		if decl.Type != nil {
			return decl.Type
		}
		for i, name := range decl.Names {
			if name.Name == id.Name && i < len(decl.Values) {
				return valueType(decl.Values[i])
			}
		}
	case *ast.AssignStmt:
		for i, lhs := range decl.Lhs {
			if l, ok := lhs.(*ast.Ident); ok && l.Name == id.Name && i < len(decl.Rhs) {
				return valueType(decl.Rhs[i])
			}
		}
	case *ast.Field:
		return decl.Type
	}
	return nil
}

// valueType returns the type of T{}, &T{} or new(T), or nil.
func valueType(v ast.Expr) ast.Expr {
	if u, ok := v.(*ast.UnaryExpr); ok && u.Op == token.AND {
		v = u.X
	}
	switch x := v.(type) {
	case *ast.CompositeLit:
		return x.Type
	case *ast.CallExpr:
		if fn, ok := x.Fun.(*ast.Ident); ok && fn.Name == "new" && len(x.Args) == 1 {
			return x.Args[0]
		}
	}
	return nil
}

// maxKeyDepth is the deepest level whose keys count. Level 0 is the
// target, and level 1 is each struct directly under it, such as metadata or
// spec. A key deeper down describes a part, not the document.
const maxKeyDepth = 1

// collectKeys records the YAML keys of a type at the given level, and the keys
// of its direct child structs one level down. It follows named types across
// the packages of the module. keys maps each key to the lowest level of it.
func collectKeys(idx *moduleIndex, pkg string, file *ast.File, typ ast.Expr, depth int, keys map[string]int, onPath map[string]bool) {
	if depth > maxKeyDepth {
		return
	}
	follow := func(target, name string) {
		decl, ok := idx.types[target][name]
		id := target + "." + name
		if !ok || onPath[id] {
			return
		}
		onPath[id] = true
		collectKeys(idx, decl.pkg, decl.file, decl.expr, depth, keys, onPath)
		delete(onPath, id)
	}
	switch t := typ.(type) {
	case *ast.StarExpr:
		collectKeys(idx, pkg, file, t.X, depth, keys, onPath)
	case *ast.ArrayType:
		collectKeys(idx, pkg, file, t.Elt, depth, keys, onPath)
	case *ast.MapType:
		collectKeys(idx, pkg, file, t.Value, depth, keys, onPath)
	case *ast.Ident:
		follow(pkg, t.Name)
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			follow(fileImports(file)[x.Name], t.Sel.Name)
		}
	case *ast.StructType:
		record := func(key string) {
			if old, ok := keys[key]; !ok || depth < old {
				keys[key] = depth
			}
		}
		for _, field := range t.Fields.List {
			key, inline := yamlKey(field)
			if key == "-" {
				continue
			}
			for _, name := range field.Names {
				if key == "" {
					record(strings.ToLower(name.Name))
				} else {
					record(key)
				}
			}
			if inline {
				// An inlined struct adds its keys at this level.
				collectKeys(idx, pkg, file, field.Type, depth, keys, onPath)
				continue
			}
			collectKeys(idx, pkg, file, field.Type, depth+1, keys, onPath)
		}
	}
}

// yamlKey returns the YAML key that a struct tag states, and whether the
// field is inlined.
func yamlKey(field *ast.Field) (key string, inline bool) {
	if field.Tag == nil {
		return "", len(field.Names) == 0
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false
	}
	tag, ok := reflect.StructTag(raw).Lookup("yaml")
	if !ok {
		return "", len(field.Names) == 0
	}
	parts := strings.Split(tag, ",")
	for _, opt := range parts[1:] {
		if opt == "inline" {
			inline = true
		}
	}
	return strings.ToLower(parts[0]), inline || len(field.Names) == 0
}

// describesComponent reports whether a set of keys describes a component:
// methods, secrets or system dependencies at the top or one level down, or a
// kind at the top with a name and a version at the top or one level down. A
// signature key marks an operator-side artifact that the platform signed.
func describesComponent(keys map[string]int) bool {
	for _, k := range signedKeys {
		if _, ok := keys[k]; ok {
			return false
		}
	}
	for _, k := range componentKeys {
		if _, ok := keys[k]; ok {
			return true
		}
	}
	if depth, ok := keys["kind"]; !ok || depth != 0 {
		return false
	}
	for _, k := range identityKeys {
		if _, ok := keys[k]; !ok {
			return false
		}
	}
	return true
}

// applyManifestExemptions drops each finding that an exemption names. It
// returns the findings that stay and each exemption that matched nothing.
func applyManifestExemptions(findings []manifestFinding, exemptions []manifestExemption) (kept []manifestFinding, stale []manifestExemption) {
	used := make([]bool, len(exemptions))
	for _, f := range findings {
		exempt := false
		for i, e := range exemptions {
			if e.rule == f.rule && e.pkg == f.pkg && e.target == f.target {
				used[i] = true
				exempt = true
			}
		}
		if !exempt {
			kept = append(kept, f)
		}
	}
	for i, e := range exemptions {
		if !used[i] {
			stale = append(stale, e)
		}
	}
	return kept, stale
}

// TestManifestParserGuard walks the module and fails on a parser of a
// component-authored manifest.
func TestManifestParserGuard(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(thisFile)

	idx, err := indexModule(repoRoot)
	if err != nil {
		t.Fatalf("index the module: %v", err)
	}
	// A floor. A walk that parsed almost nothing reads as a clean module.
	if idx.parsed < 200 {
		t.Fatalf("parsed %d Go files under %s, want at least 200: the walk did not reach the module", idx.parsed, repoRoot)
	}
	for _, e := range manifestExemptions {
		if e.reason == "" {
			t.Errorf("the exemption for %s in %s states no reason", e.target, e.pkg)
		}
	}
	kept, stale := applyManifestExemptions(scanManifestParsers(idx), manifestExemptions)
	for _, f := range kept {
		t.Error(f.String())
	}
	for _, e := range stale {
		t.Errorf("the exemption for %s %s in %s matches nothing. Delete it: %s", e.rule, e.target, e.pkg, e.reason)
	}
}

// writeFixtureModule writes a module with the given files to a new directory.
func writeFixtureModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/fx\n\ngo 1.22\n"
	for rel, body := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestManifestParserGuard_Fixture is the failing fixture of the guard. Each
// red case must fail for its stated rule, and each legal input must pass.
func TestManifestParserGuard_Fixture(t *testing.T) {
	const yamlImport = "import \"gopkg.in/yaml.v3\"\n\n"
	files := map[string]string{
		// Red: a path join that produces component.yaml.
		"red/join/join.go": "package join\n\nimport \"path/filepath\"\n\n" +
			"func Path(dir string) string { return filepath.Join(dir, \"component.yaml\") }\n",
		// Red: a literal that names permissions.yaml.
		"red/perm/perm.go": "package perm\n\nconst File = \"./permissions.yaml\"\n",
		// Red: a decode whose target declares methods.
		"red/methods/methods.go": "package methods\n\n" + yamlImport +
			"type Component struct {\n\tMethods []string `yaml:\"methods\"`\n}\n\n" +
			"func Load(b []byte) (Component, error) {\n\tvar c Component\n\terr := yaml.Unmarshal(b, &c)\n\treturn c, err\n}\n",
		// Red: a decoder whose target nests kind, name and version, and whose
		// type lives in another package of the module.
		"red/nested/types/types.go": "package types\n\n" +
			"type Meta struct {\n\tName string\n\tVersion string\n}\n\n" +
			"type Manifest struct {\n\tKind string `yaml:\"kind\"`\n\tMetadata Meta `yaml:\"metadata\"`\n}\n",
		"red/nested/load.go": "package nested\n\nimport (\n\t\"strings\"\n\n\t\"example.com/fx/red/nested/types\"\n\t\"gopkg.in/yaml.v3\"\n)\n\n" +
			"func Load(s string) (*types.Manifest, error) {\n\tm := &types.Manifest{}\n\terr := yaml.NewDecoder(strings.NewReader(s)).Decode(m)\n\treturn m, err\n}\n",
		// Red: system dependencies in an anonymous struct.
		"red/sysdeps/sysdeps.go": "package sysdeps\n\n" + yamlImport +
			"func Load(b []byte) error {\n\tvar raw struct {\n\t\tSystemDependencies []string `yaml:\"system_dependencies\"`\n\t}\n\treturn yaml.Unmarshal(b, &raw)\n}\n",
		// Red: a target that the guard cannot resolve.
		"red/opaque/opaque.go": "package opaque\n\n" + yamlImport +
			"func Load(b []byte, out func() any) error { return yaml.Unmarshal(b, out()) }\n",

		// Legal: the taxonomy is a codegen input. It has the shape of a
		// component, so it passes by its exemption below.
		"ok/taxonomy/taxonomy.go": "package taxonomy\n\n" + yamlImport +
			"// The input file is taxonomy.yaml.\n" +
			"type Taxonomy struct {\n\tVersion string `yaml:\"version\"`\n\tKind string `yaml:\"kind\"`\n\tNodeTypes []NodeType `yaml:\"node_types\"`\n}\n\n" +
			"type NodeType struct {\n\tName string `yaml:\"name\"`\n\tCategory string `yaml:\"category\"`\n}\n\n" +
			"const Input = \"taxonomy.yaml\"\n\n" +
			"func Load(b []byte) (t Taxonomy, err error) { err = yaml.Unmarshal(b, &t); return }\n",
		// Legal: the ontology is a codegen input.
		"ok/ontology/ontology.go": "package ontology\n\n" + yamlImport +
			"func Parse(b []byte) error {\n\tvar raw struct {\n\t\tVersion string `yaml:\"version\"`\n\t\tPrefixes map[string]string `yaml:\"prefixes\"`\n\t}\n\treturn yaml.Unmarshal(b, &raw)\n}\n",
		// Legal: a signed catalog manifest is an operator-side artifact.
		"ok/catalog/catalog.go": "package catalog\n\n" + yamlImport +
			"type Entry struct {\n\tName string `yaml:\"name\"`\n\tVersion string `yaml:\"version\"`\n\tMethods []string `yaml:\"methods\"`\n\tSignature string `yaml:\"signature\"`\n}\n\n" +
			"func Load(b []byte) (*Entry, error) { e := new(Entry); return e, yaml.Unmarshal(b, e) }\n",
		// Legal: Helm values are operator-owned deployment input.
		"ok/helm/values.go": "package helm\n\n" + yamlImport +
			"type Values struct {\n\tImage struct {\n\t\tRepository string `yaml:\"repository\"`\n\t\tTag string `yaml:\"tag\"`\n\t} `yaml:\"image\"`\n\tReplicaCount int `yaml:\"replicaCount\"`\n}\n\n" +
			"func Load(b []byte) (v Values, err error) { err = yaml.Unmarshal(b, &v); return }\n",
		// Legal: an evaluation set has a name and a version, but its kind is
		// a key of each sample, not of the document.
		"ok/evalset/evalset.go": "package evalset\n\n" + yamlImport +
			"type Sample struct {\n\tKind string\n\tInput string\n}\n\n" +
			"type EvalSet struct {\n\tName string\n\tVersion string\n\tSamples []Sample\n}\n\n" +
			"func Load(b []byte) (s EvalSet, err error) { err = yaml.Unmarshal(b, &s); return }\n",
		// Legal: a comment may name a manifest file.
		"ok/doc/doc.go": "// Package doc once read component.yaml. It reads nothing now.\npackage doc\n",
		// Legal: a test file is not part of the shipped sdk.
		"ok/doc/doc_test.go": "package doc\n\nconst fixture = \"component.yaml\"\n",
		// Legal: a fixture under testdata is not part of the module.
		"ok/doc/testdata/x.go": "package x\n\nconst f = \"plugin.yaml\"\n",
	}
	root := writeFixtureModule(t, files)
	idx, err := indexModule(root)
	if err != nil {
		t.Fatalf("index the fixture: %v", err)
	}

	exemptions := []manifestExemption{{
		rule:   ruleComponentDecode,
		pkg:    "example.com/fx/ok/taxonomy",
		target: "example.com/fx/ok/taxonomy.Taxonomy",
		reason: "taxonomy.yaml is a codegen input",
	}}
	kept, stale := applyManifestExemptions(scanManifestParsers(idx), exemptions)
	if len(stale) != 0 {
		t.Errorf("the taxonomy exemption matched nothing: %v", stale)
	}
	got := map[string]string{}
	for _, f := range kept {
		got[f.file] = f.rule + " " + f.target
	}
	want := map[string]string{
		"red/join/join.go":       ruleManifestFile + " component.yaml",
		"red/perm/perm.go":       ruleManifestFile + " ./permissions.yaml",
		"red/methods/methods.go": ruleComponentDecode + " example.com/fx/red/methods.Component",
		"red/nested/load.go":     ruleComponentDecode + " example.com/fx/red/nested/types.Manifest",
		"red/sysdeps/sysdeps.go": ruleComponentDecode + " example.com/fx/red/sysdeps.<anonymous StructType>",
		"red/opaque/opaque.go":   ruleUnresolved + " *ast.CallExpr",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findings:\n got %v\nwant %v", got, want)
	}
}

// TestManifestParserGuard_Exemptions proves that an exemption drops only the
// finding that it names, and that an exemption with no target is reported.
func TestManifestParserGuard_Exemptions(t *testing.T) {
	findings := []manifestFinding{
		{ruleComponentDecode, "a/a.go", "m/a", "m/a.Manifest"},
		{ruleComponentDecode, "b/b.go", "m/b", "m/b.Manifest"},
		{ruleManifestFile, "a/a.go", "m/a", "plugin.yaml"},
	}
	exemptions := []manifestExemption{
		{ruleComponentDecode, "m/a", "m/a.Manifest", "kept until x"},
		{ruleComponentDecode, "m/gone", "m/gone.Manifest", "its target is deleted"},
	}
	kept, stale := applyManifestExemptions(findings, exemptions)
	if len(kept) != 2 || kept[0].pkg != "m/b" || kept[1].rule != ruleManifestFile {
		t.Errorf("kept = %v, want the m/b decode and the plugin.yaml literal", kept)
	}
	if len(stale) != 1 || stale[0].pkg != "m/gone" {
		t.Errorf("stale = %v, want the m/gone exemption", stale)
	}
}
