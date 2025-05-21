package server

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"html/template"
	"regexp"
	"slices"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/goplus/gogen"
	gopast "github.com/goplus/gop/ast"
	goptoken "github.com/goplus/gop/token"
	"github.com/goplus/gop/x/typesutil"
	"github.com/goplus/goxlsw/internal/util"
)

// unwrapPointerType returns the underlying type of t. For pointer types, it
// returns the element type that the pointer points to. For non-pointer types,
// it returns the type unchanged.
func unwrapPointerType(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// getStringLitOrConstValue attempts to get the value from a string literal or
// constant. It returns the string value and true if successful, or empty string
// and false if the expression is not a string literal or constant, or if the
// value cannot be determined.
func getStringLitOrConstValue(expr gopast.Expr, tv types.TypeAndValue) (string, bool) {
	switch e := expr.(type) {
	case *gopast.BasicLit:
		if e.Kind != goptoken.STRING {
			return "", false
		}
		v, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return v, true
	case *gopast.Ident:
		if tv.Value != nil && tv.Value.Kind() == constant.String {
			// If it's a constant, we can get its value.
			return constant.StringVal(tv.Value), true
		}
		// There is nothing we can do for string variables.
		return "", false
	default:
		return "", false
	}
}

// deduplicateLocations deduplicates locations.
func deduplicateLocations(locations []Location) []Location {
	result := make([]Location, 0, len(locations))
	seen := make(map[string]struct{})
	for _, loc := range locations {
		key := fmt.Sprintf("%s:%d:%d", loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, loc)
		}
	}
	return result
}

// toLowerCamelCase converts the first character of a Go identifier to lowercase.
func toLowerCamelCase(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]|32) + s[1:]
}

// walkStruct walks a struct and calls the given onMember for each field and
// method. If onMember returns false, the walk is stopped.
func walkStruct(named *types.Named, onMember func(member types.Object, selector *types.Named) bool) {
	walked := make(map[*types.Named]struct{})
	seenMembers := make(map[string]struct{})
	var walk func(named *types.Named, namedPath []*types.Named) bool
	walk = func(named *types.Named, namedPath []*types.Named) bool {
		if _, ok := walked[named]; ok {
			return true
		}
		walked[named] = struct{}{}

		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			return true
		}

		selector := named
		for _, named := range namedPath {
			if !isExportedOrMainPkgObject(named.Obj()) {
				break
			}
			selector = named

			if isSpxPkgObject(selector.Obj()) && (selector == GetSpxGameType() || selector == GetSpxSpriteImplType()) {
				break
			}
		}

		for i := range st.NumFields() {
			field := st.Field(i)
			if _, ok := seenMembers[field.Name()]; ok || !isExportedOrMainPkgObject(field) {
				continue
			}
			seenMembers[field.Name()] = struct{}{}

			if !onMember(field, selector) {
				return false
			}
		}
		for i := range named.NumMethods() {
			method := named.Method(i)
			if _, ok := seenMembers[method.Name()]; ok || !isExportedOrMainPkgObject(method) {
				continue
			}
			seenMembers[method.Name()] = struct{}{}

			if !onMember(method, selector) {
				return false
			}
		}
		for i := range st.NumFields() {
			field := st.Field(i)
			if !field.Embedded() {
				continue
			}
			fieldType := unwrapPointerType(field.Type())
			namedField, ok := fieldType.(*types.Named)
			if !ok || !isNamedStructType(namedField) {
				continue
			}

			if !walk(namedField, append(namedPath, namedField)) {
				return false
			}
		}
		return true
	}
	walk(named, []*types.Named{named})
}

// isNamedStructType reports whether the given named type is a struct type.
func isNamedStructType(named *types.Named) bool {
	_, ok := named.Underlying().(*types.Struct)
	return ok
}

// gopOverloadFuncNameRE is the regular expression of the Go+ overloaded
// function name.
var gopOverloadFuncNameRE = regexp.MustCompile(`^(.+)__([0-9a-z])$`)

// isGopOverloadedFuncName reports whether the given function name is a Go+
// overloaded function name.
func isGopOverloadedFuncName(name string) bool {
	return gopOverloadFuncNameRE.MatchString(name)
}

// parseGopFuncName parses the Go+ overloaded function name.
func parseGopFuncName(name string) (parsedName string, overloadID *string) {
	parsedName = name
	if matches := gopOverloadFuncNameRE.FindStringSubmatch(parsedName); len(matches) == 3 {
		parsedName = matches[1]
		overloadID = &matches[2]
	}
	parsedName = toLowerCamelCase(parsedName)
	return
}

// isGopOverloadableFunc reports whether the given function is a Go+ overloadable
// function with a signature like `func(__gop_overload_args__ interface{_()})`.
func isGopOverloadableFunc(fun *types.Func) bool {
	typ, _ := gogen.CheckSigFuncExObjects(fun.Type().(*types.Signature))
	return typ != nil
}

// isUnexpandableGopOverloadableFunc checks if given function is a Unexpandable-Gop-Overloadable-Func.
// "Unexpandable-Gop-Overloadable-Func" is a function that
// 1. is overloadable: has a signature like `func(__gop_overload_args__ interface{_()})`
// 2. but not expandable: can not be expanded into overloads
// A typical example is method `GetWidget` on spx `Game`.
func isUnexpandableGopOverloadableFunc(fun *types.Func) bool {
	sig := fun.Type().(*types.Signature)
	if _, ok := gogen.CheckSigFuncEx(sig); ok { // is `func(__gop_overload_args__ interface{_()})`
		if t, _ := gogen.CheckSigFuncExObjects(sig); t == nil { // not expandable
			return true
		}
	}
	return false
}

// expandGopOverloadableFunc expands the given Go+ function with a signature
// like `func(__gop_overload_args__ interface{_()})` to all its overloads. It
// returns nil if the function is not qualified for overload expansion.
func expandGopOverloadableFunc(fun *types.Func) []*types.Func {
	typ, objs := gogen.CheckSigFuncExObjects(fun.Type().(*types.Signature))
	if typ == nil {
		return nil
	}
	overloads := make([]*types.Func, 0, len(objs))
	for _, obj := range objs {
		overloads = append(overloads, obj.(*types.Func))
	}
	return overloads
}

// spxEventHandlerFuncNameRE is the regular expression of the spx event handler
// function name.
var spxEventHandlerFuncNameRE = regexp.MustCompile(`^on[A-Z]\w*$`)

// isSpxEventHandlerFuncName reports whether the given function name is an
// spx event handler function name.
func isSpxEventHandlerFuncName(name string) bool {
	return spxEventHandlerFuncNameRE.MatchString(name)
}

// isSpxPkgObject reports whether the given object is defined in the spx package.
func isSpxPkgObject(obj types.Object) bool {
	return obj != nil && obj.Pkg() == GetSpxPkg()
}

// isBuiltinObject reports whether the given object is a builtin object.
func isBuiltinObject(obj types.Object) bool {
	return obj != nil && util.PackagePath(obj.Pkg()) == "builtin"
}

// isMainPkgObject reports whether the given object is defined in the main package.
func isMainPkgObject(obj types.Object) bool {
	return obj != nil && util.PackagePath(obj.Pkg()) == "main"
}

// isExportedOrMainPkgObject reports whether the given object is exported or
// defined in the main package.
func isExportedOrMainPkgObject(obj types.Object) bool {
	return obj != nil && (obj.Exported() || isMainPkgObject(obj))
}

// isRenameableObject reports whether the given object can be renamed.
func isRenameableObject(obj types.Object) bool {
	if !isMainPkgObject(obj) || obj.Parent() == types.Universe {
		return false
	}
	switch obj.(type) {
	case *types.Var, *types.Const, *types.TypeName, *types.Func, *types.Label:
		return true
	case *types.PkgName:
		return false
	}
	return false
}

// getSimplifiedTypeString returns the string representation of the given type,
// with the spx package name omitted while other packages use their short names.
func getSimplifiedTypeString(typ types.Type) string {
	return types.TypeString(typ, func(p *types.Package) string {
		if p == GetSpxPkg() {
			return ""
		}
		return p.Name()
	})
}

// isTypeCompatible checks if two types are compatible.
func isTypeCompatible(got, want types.Type) bool {
	if got == nil || want == nil {
		return false
	}

	if types.AssignableTo(got, want) {
		return true
	}

	switch want := want.(type) {
	case *types.Interface:
		return types.Implements(got, want)
	case *types.Pointer:
		if gotPtr, ok := got.(*types.Pointer); ok {
			return types.Identical(want.Elem(), gotPtr.Elem())
		}
		return types.Identical(got, want.Elem())
	case *types.Slice:
		gotSlice, ok := got.(*types.Slice)
		return ok && types.Identical(want.Elem(), gotSlice.Elem())
	case *types.Chan:
		gotCh, ok := got.(*types.Chan)
		return ok && types.Identical(want.Elem(), gotCh.Elem()) &&
			(want.Dir() == types.SendRecv || want.Dir() == gotCh.Dir())
	}

	if _, ok := got.(*types.Named); ok {
		return types.Identical(got, want)
	}

	return false
}

// attr transforms given string value to an HTML attribute value (with quotes).
func attr(value string) string {
	return fmt.Sprintf(`"%s"`, template.HTMLEscapeString(value))
}

// utf16OffsetToUTF8 converts a UTF-16 offset to a UTF-8 offset in the given string.
func utf16OffsetToUTF8(s string, utf16Offset int) int {
	if utf16Offset <= 0 {
		return 0
	}

	var utf16Units, utf8Bytes int
	for _, r := range s {
		if utf16Units >= utf16Offset {
			break
		}
		utf16Units += utf16.RuneLen(r)
		utf8Bytes += utf8.RuneLen(r)
	}
	return utf8Bytes
}

// utf8OffsetToUTF16 converts a UTF-8 offset to a UTF-16 offset in the given string.
func utf8OffsetToUTF16(s string, utf8Offset int) int {
	if utf8Offset <= 0 {
		return 0
	}

	var utf8Bytes, utf16Units int
	for _, r := range s {
		if utf8Bytes >= utf8Offset {
			break
		}
		utf8Bytes += utf8.RuneLen(r)
		utf16Units += utf16.RuneLen(r)
	}
	return utf16Units
}

// positionOffset converts an LSP position (line, character) to a byte offset in the document.
// It calculates the offset by:
// 1. Finding the starting byte offset of the requested line
// 2. Adding the character offset within that line, converting from UTF-16 to UTF-8 if needed
//
// Parameters:
// - content: The file content as a byte array
// - position: The LSP position with line and character numbers (0-based)
//
// Returns the byte offset from the beginning of the document
func positionOffset(content []byte, position Position) int {
	// If content is empty or position is beyond the content, return 0
	if len(content) == 0 {
		return 0
	}

	// Find all line start positions in the document
	lineStarts := []int{0} // First line always starts at position 0
	for i := range len(content) {
		if content[i] == '\n' {
			lineStarts = append(lineStarts, i+1) // Next line starts after the newline
		}
	}

	// Ensure the requested line is within range
	lineIndex := int(position.Line)
	if lineIndex >= len(lineStarts) {
		// If line is beyond available lines, return the end of content
		return len(content)
	}

	// Get the starting offset of the requested line
	lineOffset := lineStarts[lineIndex]

	// Extract the content of the requested line
	lineEndOffset := len(content)
	if lineIndex+1 < len(lineStarts) {
		lineEndOffset = lineStarts[lineIndex+1] - 1 // -1 to exclude the newline character
	}

	// Ensure we don't go beyond the end of content
	if lineOffset >= len(content) {
		return len(content)
	}

	lineContent := content[lineOffset:min(lineEndOffset, len(content))]

	// Convert UTF-16 character offset to UTF-8 byte offset
	utf8Offset := utf16OffsetToUTF8(string(lineContent), int(position.Character))

	// Ensure the final offset doesn't exceed the content length
	return lineOffset + utf8Offset
}

// WalkNodesFromInterval walks the AST path starting from a position interval,
// calling walkFn for each node. The function walks from the smallest enclosing
// node outward through parent nodes. If walkFn returns false, the walk stops.
func WalkNodesFromInterval(root *gopast.File, start, end goptoken.Pos, walkFn func(node ast.Node) bool) {
	path, _ := util.PathEnclosingInterval(root, start, end)
	for _, node := range path {
		if !walkFn(node) {
			break
		}
	}
}

// createCallExprFromBranchStmt attempts to create a call expression from a
// branch statement. This handles cases in spx where the `Sprite.Goto` method
// is intended to precede the goto statement.
func createCallExprFromBranchStmt(typeInfo *typesutil.Info, stmt *gopast.BranchStmt) *gopast.CallExpr {
	if stmt.Tok != goptoken.GOTO {
		// Currently, we only need to handle goto statements.
		return nil
	}

	for ident, obj := range typeInfo.Uses {
		if ident.Pos() == stmt.TokPos && ident.End() == stmt.TokPos+goptoken.Pos(len(stmt.Tok.String())) {
			if _, ok := obj.(*types.Func); ok {
				return &gopast.CallExpr{
					Fun:  ident,
					Args: []gopast.Expr{stmt.Label},
				}
			}
			break
		}
	}
	return nil
}

// funcFromCallExpr returns the function object from a call expression.
func funcFromCallExpr(typeInfo *typesutil.Info, expr *gopast.CallExpr) *types.Func {
	var ident *gopast.Ident
	switch fun := expr.Fun.(type) {
	case *gopast.Ident:
		ident = fun
	case *gopast.SelectorExpr:
		ident = fun.Sel
	default:
		return nil
	}

	obj := typeInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}
	fun, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	return fun
}

// walkCallExprArgs walks the arguments of a call expression and calls the
// provided walkFn for each argument. It does nothing if the function is not
// found or if the function is Go+ FuncEx type.
func walkCallExprArgs(typeInfo *typesutil.Info, expr *gopast.CallExpr, walkFn func(fun *types.Func, params *types.Tuple, paramIndex int, arg gopast.Expr, argIndex int) bool) {
	fun := funcFromCallExpr(typeInfo, expr)
	if fun == nil {
		return
	}
	sig := fun.Signature()
	if _, ok := gogen.CheckFuncEx(sig); ok {
		return
	}

	params := sig.Params()
	if util.IsGopPackage(fun.Pkg()) {
		_, methodName, ok := util.SplitGoptMethodName(fun.Name(), false)
		if ok {
			vars := make([]*types.Var, 0, params.Len()-1)
			if _, ok := util.SplitGopxFuncName(methodName); ok {
				typeParams := fun.Signature().TypeParams()
				if typeParams != nil {
					vars = slices.Grow(vars, typeParams.Len())
					for i := range typeParams.Len() {
						typeParam := typeParams.At(i)
						param := types.NewParam(goptoken.NoPos, typeParam.Obj().Pkg(), typeParam.Obj().Name(), typeParam.Constraint().Underlying())
						vars = append(vars, param)
					}
				}
			}
			for i := 1; i < params.Len(); i++ {
				vars = append(vars, params.At(i))
			}
			params = types.NewTuple(vars...)
		}
	}

	totalParams := params.Len()
	for i, arg := range expr.Args {
		paramIndex := i
		if paramIndex >= totalParams {
			if !sig.Variadic() || totalParams == 0 {
				break
			}
			paramIndex = totalParams - 1
		}

		if !walkFn(fun, params, paramIndex, arg, i) {
			break
		}
	}
}

// rangesOverlap checks if two ranges overlap.
func rangesOverlap(a, b Range) bool {
	return a.Start.Line <= b.End.Line && (a.Start.Line != b.End.Line || a.Start.Character <= b.End.Character) &&
		b.Start.Line <= a.End.Line && (b.Start.Line != a.End.Line || b.Start.Character <= a.End.Character)
}
