package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// The dead-code gate answers two questions that a purely lexical scan cannot:
//
//  1. Is a declaration referenced by name at all? (the original check)
//  2. Is a *method* dead even though it is exported? A method is only
//     legitimately callable without a direct call site when it satisfies an
//     interface the program actually uses. A method that implements no such
//     interface and is never called is dead — this is the `MarkCompleted` case
//     the previous version of this gate could not see, because it skipped every
//     exported name.
//
// Liveness is judged by direct reference (the identifier appears somewhere) plus
// interface satisfaction (a method kept alive only by an interface it implements
// is not dead). A method reached purely by reflection (reflect.MethodByName)
// would defeat the second signal; this tree has no such call sites, so the gate
// stays sound without a reflection escape hatch.
type deadCodeBaseline struct {
	Allowed []string `json:"allowed"`
	Version int      `json:"version"`
}

const deadCodeBaselineVersion = 3

type deadDecl struct {
	id   string
	kind string
	pos  token.Position
	// ifaceImpl marks a method whose receiver type satisfies an interface the
	// program references. Such a method is reachable through dynamic dispatch
	// and is therefore never reported.
	ifaceImpl bool
}

// loadTypedPackages loads every internal package plus the command binaries with
// full type information and dependency type info, so interface satisfaction can
// be computed against the whole program. Tests are included so a method that is
// exercised only by a test is not mistaken for dead code; declarations that live
// in _test.go files are excluded when reporting (see collectDeclarations).
func loadTypedPackages() ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedModule,
		Dir:   filepath.Join(getRoot(), "apps", "api"),
		Tests: true,
	}
	return packages.Load(cfg, "./internal/...", "./cmd/...", "./tests/...")
}

// collectDeclarations records every declaration the gate can judge: unexported
// package-level objects (the original scope) and every method of an internal
// type, exported or not, because a method's liveness is a reachability question
// rather than a visibility one.
func collectDeclarations(pkgs []*packages.Package, ifaceImpl map[string]bool) map[string]deadDecl {
	decls := map[string]deadDecl{}

	for _, pkg := range pkgs {
		if pkg.Types == nil || pkg.TypesInfo == nil {
			continue
		}
		if !isInternal(pkg.PkgPath) {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			if !isAnalyzable(name) {
				continue
			}
			obj := scope.Lookup(name)
			key := objectKey(pkg.PkgPath, obj)
			decls[key] = deadDecl{
				id:   key,
				pos:  packagePosition(pkg, obj),
				kind: objectKind(obj),
			}
		}

		for ident, obj := range pkg.TypesInfo.Defs {
			fn, ok := obj.(*types.Func)
			if !ok || fn == nil {
				continue
			}
			pos := pkg.Fset.Position(ident.Pos())
			if strings.HasSuffix(pos.Filename, "_test.go") {
				continue // Test helpers are not production dead code.
			}
			sig, ok := fn.Type().(*types.Signature)
			if !ok || sig.Recv() == nil {
				continue
			}
			recv := deref(sig.Recv().Type())
			if _, isIface := recv.Underlying().(*types.Interface); isIface {
				continue // Interface method declarations are the contract itself.
			}
			named, ok := recv.(*types.Named)
			if !ok {
				continue
			}
			key := methodKey(pkg.PkgPath, fn)
			decls[key] = deadDecl{
				id:        key,
				pos:       pos,
				kind:      "method",
				ifaceImpl: ifaceImpl[namedKey(named)],
			}
		}
	}
	return decls
}

// isInternal reports whether a package path belongs to this module's internal
// tree, which is the gate's scope.
func isInternal(pkgPath string) bool {
	return strings.Contains(pkgPath, "/internal/")
}

// isAnalyzable reports whether a package-level name is judged by the gate:
// unexported, not blank, not main/init. Exported package-level funcs stay out of
// scope because they are the package API and are legitimately callable from
// outside the module.
func isAnalyzable(name string) bool {
	if name == "" || name == "_" {
		return false
	}
	if name == "main" || name == "init" {
		return false
	}
	r := []rune(name)[0]
	return r >= 'a' && r <= 'z'
}

// referencedInterfaces returns the set of interfaces the program actually
// mentions. An interface nobody references can never hold a live value, so it
// cannot justify keeping a method. Empty interfaces (any / interface{}) are
// excluded: every type satisfies them, so they carry no information.
func referencedInterfaces(pkgs []*packages.Package) []*types.Interface {
	seenPkg := map[*packages.Package]bool{}
	seenIface := map[*types.Interface]bool{}
	var out []*types.Interface

	add := func(t types.Type) {
		iface, ok := t.Underlying().(*types.Interface)
		if !ok || iface.NumMethods() == 0 || seenIface[iface] {
			return
		}
		seenIface[iface] = true
		out = append(out, iface)
	}

	var walk func(p *packages.Package)
	walk = func(p *packages.Package) {
		if p == nil || seenPkg[p] || p.Types == nil {
			return
		}
		seenPkg[p] = true
		if p.TypesInfo != nil {
			for _, obj := range p.TypesInfo.Uses {
				if obj == nil {
					continue
				}
				if tn, ok := obj.(*types.TypeName); ok {
					add(tn.Type())
				}
			}
		}
		for _, imp := range p.Imports {
			walk(imp)
		}
	}
	for _, p := range pkgs {
		walk(p)
	}

	if errObj := types.Universe.Lookup("error"); errObj != nil {
		add(errObj.Type())
	}
	return out
}

// interfaceImplementors maps "pkgPath.TypeName" -> true for every internal named
// type that satisfies one of the referenced interfaces.
func interfaceImplementors(pkgs []*packages.Package, ifaces []*types.Interface) map[string]bool {
	out := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg.Types == nil {
			continue
		}
		for _, name := range pkg.Types.Scope().Names() {
			obj := pkg.Types.Scope().Lookup(name)
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok {
				continue
			}
			for _, iface := range ifaces {
				if types.Implements(named, iface) || types.Implements(types.NewPointer(named), iface) {
					out[namedKey(named)] = true
					break
				}
			}
		}
	}
	return out
}

func namedKey(named *types.Named) string {
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

func deref(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

func objectKey(pkgPath string, obj types.Object) string {
	return pkgPath + "." + obj.Name()
}

func methodKey(pkgPath string, fn *types.Func) string {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return pkgPath + "." + fn.Name()
	}
	recv := deref(sig.Recv().Type())
	return namedKeyOf(recv) + "." + fn.Name()
}

func namedKeyOf(t types.Type) string {
	if named, ok := t.(*types.Named); ok {
		if named.Obj() != nil && named.Obj().Pkg() != nil {
			return named.Obj().Pkg().Path() + "." + named.Obj().Name()
		}
		if named.Obj() != nil {
			return named.Obj().Name()
		}
	}
	return t.String()
}

func objectKind(obj types.Object) string {
	switch obj.(type) {
	case *types.Func:
		return "func"
	case *types.TypeName:
		return "type"
	case *types.Var:
		return "var"
	case *types.Const:
		return "const"
	default:
		return "other"
	}
}

func packagePosition(pkg *packages.Package, obj types.Object) token.Position {
	if obj != nil && obj.Pos().IsValid() {
		return pkg.Fset.Position(obj.Pos())
	}
	return token.Position{Filename: pkg.PkgPath}
}

// collectUses records every object referenced by name anywhere in the loaded
// packages, so declarations that are merely mentioned (including via a method
// value or a selector) are never reported.
func collectUses(pkgs []*packages.Package) map[string]bool {
	used := map[string]bool{}

	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, obj := range pkg.TypesInfo.Uses {
			if obj == nil || obj.Pkg() == nil {
				continue
			}
			used[objectKey(obj.Pkg().Path(), obj)] = true
			if fn, ok := obj.(*types.Func); ok {
				if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
					used[methodKey(obj.Pkg().Path(), fn)] = true
				}
			}
		}

		for _, sel := range pkg.TypesInfo.Selections {
			obj := sel.Obj()
			if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil {
				used[methodKey(fn.Pkg().Path(), fn)] = true
				used[objectKey(fn.Pkg().Path(), fn)] = true
			}
		}
	}
	return used
}

func reachableFromEntryPoints(pkgs []*packages.Package, used map[string]bool) {
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil || pkg.Syntax == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					if obj, ok := pkg.TypesInfo.Uses[id]; ok && obj != nil && obj.Pkg() != nil {
						used[objectKey(obj.Pkg().Path(), obj)] = true
					}
				}
				return true
			})
		}
	}
}

func verifyDeadCode() bool {
	pkgs, err := loadTypedPackages()
	if err != nil {
		fmt.Printf("    [ERROR] loading packages: %v\n", err)
		return false
	}
	if packages.PrintErrors(pkgs) > 0 {
		fmt.Printf("    [FAIL] the tree does not type-check; fix build errors first\n")
		return false
	}

	ifaces := referencedInterfaces(pkgs)
	impls := interfaceImplementors(pkgs, ifaces)
	decls := collectDeclarations(pkgs, impls)
	used := collectUses(pkgs)
	reachableFromEntryPoints(pkgs, used)

	allowed := map[string]bool{}
	for _, id := range loadDeadCodeBaseline() {
		allowed[id] = true
	}

	var dead []deadDecl
	for id, d := range decls {
		if used[id] || allowed[id] || d.ifaceImpl {
			continue
		}
		dead = append(dead, d)
	}
	sort.Slice(dead, func(i, j int) bool {
		if dead[i].pos.Filename != dead[j].pos.Filename {
			return dead[i].pos.Filename < dead[j].pos.Filename
		}
		return dead[i].pos.Line < dead[j].pos.Line
	})

	if len(dead) == 0 {
		fmt.Printf("    [OK] %d declaration(s) type-checked, none unreferenced\n", len(decls))
		return true
	}

	for _, d := range dead {
		fmt.Printf("      %s:%d  %s (%s)\n", displayPath(d.pos.Filename), d.pos.Line, shortID(d.id), d.kind)
	}
	fmt.Printf("    [FAIL] %d unreferenced declaration(s)\n", len(dead))
	fmt.Printf("    Remove them, or freeze with `make deadcode-baseline`.\n")
	return false
}

func displayPath(path string) string {
	root := getRoot()
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}

func shortID(id string) string {
	const prefix = "github.com/VinnsEdesigner/vyzorix/apps/api/"
	return strings.TrimPrefix(id, prefix)
}

func loadDeadCodeBaseline() []string {
	path := filepath.Join(getRoot(), "apps", "api", "config", "deadcode-baseline.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var b deadCodeBaseline
	if jsonUnmarshal(raw, &b) != nil {
		return nil
	}
	return b.Allowed
}

func exportDeadCodeBaseline() error {
	pkgs, err := loadTypedPackages()
	if err != nil {
		return err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("the tree does not type-check")
	}

	ifaces := referencedInterfaces(pkgs)
	impls := interfaceImplementors(pkgs, ifaces)
	decls := collectDeclarations(pkgs, impls)
	used := collectUses(pkgs)
	reachableFromEntryPoints(pkgs, used)

	allowed := []string{}
	for id, d := range decls {
		if !used[id] && !d.ifaceImpl {
			allowed = append(allowed, id)
		}
	}
	sort.Strings(allowed)

	out, err := jsonMarshalIndent(deadCodeBaseline{
		Version: deadCodeBaselineVersion,
		Allowed: allowed,
	})
	if err != nil {
		return err
	}

	dir := filepath.Join(getRoot(), "apps", "api", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "deadcode-baseline.json"), out, 0o644)
}
