// Package apilint checks handler DTOs, swag annotations, and domain json tags.
package apilint

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer enforces the API boundary between handlers and domain types.
var Analyzer = &analysis.Analyzer{
	Name:     "apilint",
	Doc:      "checks API DTO, annotation, and domain-tag conventions",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	switch {
	case strings.HasSuffix(pass.Pkg.Path(), "/internal/model"):
		checkModel(pass, ins)
	case strings.HasSuffix(pass.Pkg.Path(), "/internal/handler"):
		checkHandler(pass, ins)
	}
	if strings.Contains(pass.Pkg.Path(), "/internal/") {
		checkErrorCause(pass, ins)
	}
	return nil, nil
}

// checkErrorCause reports `if err != nil` branches that never use err, which
// drops the cause before the error handler can log it.
func checkErrorCause(pass *analysis.Pass, ins *inspector.Inspector) {
	ins.Preorder([]ast.Node{(*ast.IfStmt)(nil)}, func(n ast.Node) {
		stmt := n.(*ast.IfStmt)
		if skipFile(pass, stmt.Pos()) {
			return
		}
		obj := nonNilErrorCheck(pass, stmt.Cond)
		if obj == nil || usesObject(pass, stmt.Body, obj) {
			return
		}
		pass.Reportf(stmt.Pos(), "error-cause: %s is dropped; wrap it or pass it as the cause", obj.Name())
	})
}

func nonNilErrorCheck(pass *analysis.Pass, cond ast.Expr) types.Object {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return nil
	}
	ident, ok := bin.X.(*ast.Ident)
	if !ok || !isNil(pass, bin.Y) {
		return nil
	}
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil || !isError(obj.Type()) {
		return nil
	}
	return obj
}

func isNil(pass *analysis.Pass, expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	_, isNilObj := pass.TypesInfo.ObjectOf(ident).(*types.Nil)
	return isNilObj
}

func usesObject(pass *analysis.Pass, body *ast.BlockStmt, obj types.Object) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && pass.TypesInfo.ObjectOf(ident) == obj {
			found = true
		}
		return !found
	})
	return found
}

func checkModel(pass *analysis.Pass, ins *inspector.Inspector) {
	ins.Preorder([]ast.Node{(*ast.Field)(nil)}, func(n ast.Node) {
		field := n.(*ast.Field)
		if skipFile(pass, field.Pos()) || field.Tag == nil {
			return
		}
		if strings.Contains(tagText(field.Tag), "json:") {
			pass.Reportf(field.Pos(), "model-no-wire-tags: domain types must not declare json tags")
		}
	})
}

func checkHandler(pass *analysis.Pass, ins *inspector.Inspector) {
	ins.Preorder([]ast.Node{
		(*ast.TypeSpec)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.CallExpr)(nil),
	}, func(n ast.Node) {
		if skipFile(pass, n.Pos()) {
			return
		}
		switch node := n.(type) {
		case *ast.TypeSpec:
			checkDTO(pass, node)
		case *ast.FuncDecl:
			checkHandlerMethod(pass, node)
		case *ast.CallExpr:
			checkCall(pass, node)
		}
	})
}

func checkDTO(pass *analysis.Pass, spec *ast.TypeSpec) {
	st, ok := spec.Type.(*ast.StructType)
	if !ok || spec.Name == nil || st.Fields == nil {
		return
	}
	name := spec.Name.Name
	switch {
	case strings.HasSuffix(name, "Request"), strings.HasSuffix(name, "Params"):
		for _, field := range st.Fields.List {
			checkRequestField(pass, field)
		}
	case strings.HasSuffix(name, "Response"):
		for _, field := range st.Fields.List {
			checkResponseField(pass, field)
		}
	}
}

func checkRequestField(pass *analysis.Pass, field *ast.Field) {
	if !exported(field) {
		return
	}
	tag := parsedTag(field)
	if tag.Get("json") == "" && tag.Get("query") == "" {
		pass.Reportf(field.Pos(), "dto-explicit-fields: missing json or query tag")
	}
	validate := tagOptions(tag.Get("validate"))
	validateOptional := validate["omitempty"]
	if !validate["required"] && !validateOptional {
		pass.Reportf(field.Pos(), "dto-explicit-fields: validate tag must include required or omitempty")
		return
	}
	// swag --requiredByDefault reads json omitempty, the validator reads validate.
	// Query fields take optionality from @Param, so only json fields are compared.
	if tag.Get("json") != "" && tagOptions(tag.Get("json"))["omitempty"] != validateOptional {
		pass.Reportf(field.Pos(), "dto-explicit-fields: json omitempty and validate omitempty must agree")
	}
}

// tagOptions returns the comma-separated options of a tag value, keyed by name.
// The first element of a json tag is the field name and is skipped when it has no
// option form, which is harmless because names never equal required or omitempty.
func tagOptions(value string) map[string]bool {
	options := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		options[strings.SplitN(part, "=", 2)[0]] = true
	}
	return options
}

func checkResponseField(pass *analysis.Pass, field *ast.Field) {
	if !exported(field) || parsedTag(field).Get("json") != "" {
		return
	}
	pass.Reportf(field.Pos(), "dto-explicit-fields: response field needs a json tag")
}

func checkHandlerMethod(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn.Recv == nil || fn.Name == nil || !fn.Name.IsExported() || !isHandlerSignature(pass, fn) {
		return
	}
	doc := ""
	if fn.Doc != nil {
		doc = fn.Doc.Text()
	}
	var missing []string
	if !strings.Contains(doc, "@Router") {
		missing = append(missing, "@Router")
	}
	if !strings.Contains(doc, "@Success") {
		missing = append(missing, "@Success")
	}
	if !strings.Contains(doc, "@Failure") || !strings.Contains(doc, "ErrorResponse") {
		missing = append(missing, "@Failure ErrorResponse")
	}
	if len(missing) > 0 {
		pass.Reportf(fn.Name.Pos(), "handler-annotated: missing %s", strings.Join(missing, ", "))
	}
}

func checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	name := calledName(pass, call.Fun)
	switch name {
	case "respond":
		if len(call.Args) < 3 {
			return
		}
		if _, ok := handlerStruct(pass, pass.TypesInfo.TypeOf(call.Args[2]), false); !ok {
			pass.Reportf(call.Args[2].Pos(), "dto-only-responses: value must be a named struct declared in the handler package")
		}
	case "bindJSON", "bindQuery":
		if len(call.Args) < 2 {
			return
		}
		argType := pass.TypesInfo.TypeOf(call.Args[1])
		typeName, ok := handlerStruct(pass, argType, true)
		if !ok || (!strings.HasSuffix(typeName, "Request") && !strings.HasSuffix(typeName, "Params")) {
			pass.Reportf(call.Args[1].Pos(), "dto-only-requests: target must be a pointer to a handler struct ending in Request or Params")
		}
	}
}

func isHandlerSignature(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	params := fieldTypes(fn.Type.Params)
	results := fieldTypes(fn.Type.Results)
	if len(params) != 1 || len(results) != 1 {
		return false
	}
	return isFiberCtx(pass.TypesInfo.TypeOf(params[0])) && isError(pass.TypesInfo.TypeOf(results[0]))
}

func fieldTypes(list *ast.FieldList) []ast.Expr {
	if list == nil {
		return nil
	}
	var types []ast.Expr
	for _, field := range list.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			types = append(types, field.Type)
		}
	}
	return types
}

func isFiberCtx(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Name() == "Ctx" && named.Obj().Pkg().Name() == "fiber"
}

func isError(t types.Type) bool {
	errType := types.Universe.Lookup("error").Type()
	return types.Identical(t, errType)
}

func calledName(pass *analysis.Pass, fun ast.Expr) string {
	ident, ok := fun.(*ast.Ident)
	if !ok {
		return ""
	}
	fn, ok := pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok || fn.Pkg() != pass.Pkg {
		return ""
	}
	return fn.Name()
}

func handlerStruct(pass *analysis.Pass, t types.Type, pointer bool) (string, bool) {
	if t == nil {
		return "", false
	}
	if pointer {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return "", false
		}
		t = ptr.Elem()
	} else if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() != pass.Pkg {
		return "", false
	}
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return "", false
	}
	return named.Obj().Name(), true
}

func exported(field *ast.Field) bool {
	if len(field.Names) == 0 {
		return false
	}
	for _, name := range field.Names {
		if name.IsExported() {
			return true
		}
	}
	return false
}

func parsedTag(field *ast.Field) reflect.StructTag {
	return reflect.StructTag(tagText(field.Tag))
}

func tagText(lit *ast.BasicLit) string {
	if lit == nil || lit.Kind != token.STRING {
		return ""
	}
	text, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return text
}

func skipFile(pass *analysis.Pass, pos token.Pos) bool {
	file := pass.Fset.File(pos)
	return file == nil || strings.HasSuffix(file.Name(), "_test.go")
}
