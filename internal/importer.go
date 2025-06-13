package internal

import (
	"fmt"
	"go/types"
	"sync"

	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/internal/pkgdata"
	"golang.org/x/tools/go/gcexportdata"
)

// importer implements [types.Importer].
type importer struct {
	mu     sync.Mutex
	fset   *xgotoken.FileSet
	loaded map[string]*types.Package
}

// newImporter creates a new instance of [importer].
func newImporter() *importer {
	loaded := make(map[string]*types.Package)
	loaded["unsafe"] = types.Unsafe
	return &importer{
		fset:   xgotoken.NewFileSet(),
		loaded: loaded,
	}
}

// Import implements [types.Importer].
func (imp *importer) Import(path string) (*types.Package, error) {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	if pkg, ok := imp.loaded[path]; ok && pkg.Complete() {
		return pkg, nil
	}

	export, err := pkgdata.OpenExport(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open package export file: %w", err)
	}
	defer export.Close()

	pkg, err := gcexportdata.Read(export, imp.fset, imp.loaded, path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package export data: %w", err)
	}
	return pkg, nil
}

// Importer is the global instance of [importer].
var Importer = newImporter()
