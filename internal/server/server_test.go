package server

import (
	"github.com/goplus/xgolsw/internal/vfs"
	"github.com/goplus/xgolsw/xgo"
)

func newMapFSWithoutModTime(files map[string][]byte) *vfs.MapFS {
	return xgo.NewProject(nil, func() map[string]vfs.MapFile {
		fileMap := make(map[string]vfs.MapFile)
		for k, v := range files {
			fileMap[k] = &vfs.MapFileImpl{Content: v}
		}
		return fileMap
	}, xgo.FeatAll)
}

func fileMapGetter(files map[string][]byte) func() map[string]vfs.MapFile {
	return func() map[string]vfs.MapFile {
		fileMap := make(map[string]vfs.MapFile)
		for k, v := range files {
			fileMap[k] = &vfs.MapFileImpl{Content: v}
		}
		return fileMap
	}
}
