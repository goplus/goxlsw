package server

import (
	xgoast "github.com/goplus/xgo/ast"
	"github.com/goplus/xgolsw/pkgdoc"
)

// SpxReferencePkg is a reference to an imported package.
type SpxReferencePkg struct {
	PkgPath string
	Pkg     *pkgdoc.PkgDoc
	Node    *xgoast.ImportSpec
}
