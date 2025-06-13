package vfs

import (
	"go/types"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/goplus/xgolsw/xgo"
	xfs "github.com/qiniu/x/http/fs"
)

type MapFile = xgo.File
type MapFileImpl = xgo.FileImpl
type MapFS = xgo.Project

// RangeSpriteNames iterates sprite names.
func RangeSpriteNames(rootFS *MapFS, f func(name string) bool) {
	rootFS.RangeFiles(func(filename string) bool {
		name := path.Base(filename)
		if strings.HasSuffix(name, ".spx") {
			return f(name[:len(name)-4])
		}
		return true
	})
}

// HasSpriteType checks if there is specified sprite type.
func HasSpriteType(rootFS *MapFS, typ types.Type) (has bool) {
	pkg, _, _, _ := rootFS.TypeInfo()
	RangeSpriteNames(rootFS, func(name string) bool {
		if obj := pkg.Scope().Lookup(name); obj != nil && obj.Type() == typ {
			has = true
			return false
		}
		return true
	})
	return
}

// ListSpxFiles returns a list of .spx files in the rootFS.
func ListSpxFiles(rootFS *MapFS) (files []string, err error) {
	rootFS.RangeFiles(func(path string) bool {
		if strings.HasSuffix(path, ".spx") {
			files = append(files, path)
		}
		return true
	})
	return
}

// WithOverlay returns a new MapFS with overlay files.
func WithOverlay(rootFS *MapFS, overlay map[string]MapFile) *MapFS {
	ret := rootFS.Snapshot()
	for k, v := range overlay {
		ret.PutFile(k, v)
	}
	return ret
}

// ReadFile reads a file from the rootFS.
func ReadFile(rootFS *MapFS, name string) ([]byte, error) {
	ret, ok := rootFS.File(name)
	if !ok {
		return nil, fs.ErrNotExist
	}
	return ret.Content, nil
}

type SubFS struct {
	root *MapFS
	base string
}

func (fs SubFS) ReadFile(name string) ([]byte, error) {
	return ReadFile(fs.root, fs.base+"/"+name)
}

func (fs SubFS) Readdir(name string) (ret []fs.FileInfo, err error) {
	prefix := fs.base + "/" + name + "/"
	entries := map[string]int{}
	fs.root.RangeFileContents(func(path string, file xgo.File) bool {
		if strings.HasPrefix(path, prefix) {
			name := path[len(prefix):]
			if i := strings.Index(name, "/"); i >= 0 {
				entries[name[:i]] = -1
			} else {
				entries[name] = len(file.Content)
			}
		}
		return true
	})
	for name, size := range entries {
		if size < 0 {
			ret = append(ret, xfs.NewDirInfo(name))
		} else {
			ret = append(ret, xfs.NewFileInfo(name, int64(size)))
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name() < ret[j].Name()
	})
	return
}

func Sub(rootFS *MapFS, base string) SubFS {
	return SubFS{rootFS, base}
}
