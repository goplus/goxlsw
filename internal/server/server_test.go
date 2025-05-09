package server

import (
	"github.com/goplus/goxlsw/gop"
	"github.com/goplus/goxlsw/internal/vfs"
)

func newMapFSWithoutModTime(files map[string][]byte) *vfs.MapFS {
	return gop.NewProject(nil, func() map[string]vfs.MapFile {
		fileMap := make(map[string]vfs.MapFile)
		for k, v := range files {
			fileMap[k] = &vfs.MapFileImpl{Content: v}
		}
		return fileMap
	}, gop.FeatAll)
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
