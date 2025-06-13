//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"
	"time"

	"github.com/goplus/xgolsw/internal/pkgdata"
	"github.com/goplus/xgolsw/internal/server"
	"github.com/goplus/xgolsw/internal/vfs"
	"github.com/goplus/xgolsw/jsonrpc2"
	"github.com/goplus/xgolsw/xgo"
)

// Spxls implements a lightweight XGo language server for spx that runs in the
// browser using WebAssembly.
type Spxls struct {
	messageReplier js.Value
	server         *server.Server
}

// NewSpxls creates a new instance of [Spxls].
func NewSpxls(this js.Value, args []js.Value) any {
	if len(args) != 2 {
		return errors.New("NewSpxls: expected 2 arguments")
	}
	if args[0].Type() != js.TypeFunction {
		return errors.New("NewSpxls: filesProvider argument must be a function")
	}
	if args[1].Type() != js.TypeFunction {
		return errors.New("NewSpxls: messageReplier argument must be a function")
	}
	filesProvider := args[0]
	s := &Spxls{
		messageReplier: args[1],
	}

	filesMapGetter := func() map[string]vfs.MapFile {
		files := filesProvider.Invoke()
		return ConvertJSFilesToMap(files)
	}
	s.server = server.New(xgo.NewProject(nil, filesMapGetter, xgo.FeatAll), s, filesMapGetter)
	return js.ValueOf(map[string]any{
		"handleMessage": JSFuncOfWithError(s.HandleMessage),
	})
}

// HandleMessage handles incoming LSP messages from the client.
func (s *Spxls) HandleMessage(this js.Value, args []js.Value) any {
	if len(args) != 1 {
		return errors.New("Spxls.HandleMessage: expected 1 argument")
	}
	if args[0].Type() != js.TypeObject {
		return errors.New("Spxls.HandleMessage: message argument must be an object")
	}
	rawMessage := js.Global().Get("JSON").Call("stringify", args[0]).String()
	message, err := jsonrpc2.DecodeMessage([]byte(rawMessage))
	if err != nil {
		return fmt.Errorf("Spxls.HandleMessage: %w", err)
	}
	if err := s.server.HandleMessage(message); err != nil {
		return fmt.Errorf("Spxls.HandleMessage: %w", err)
	}
	return nil
}

// ReplyMessage sends a message back to the client via s.messageReplier.
func (s *Spxls) ReplyMessage(m jsonrpc2.Message) (err error) {
	rawMessage, err := json.Marshal(m)
	if err != nil {
		return err
	}

	// Catch potential panics during JavaScript execution.
	defer func() {
		if r := recover(); r != nil {
			if jsErr, ok := r.(js.Error); ok {
				err = fmt.Errorf("client error: %w", jsErr)
			} else {
				err = fmt.Errorf("client panic: %v", r)
			}
		}
	}()

	message := js.Global().Get("JSON").Call("parse", string(rawMessage))
	s.messageReplier.Invoke(message)
	return nil
}

// SetCustomPkgdataZip sets custom package data that will be used with higher
// priority than the embedded package data.
func SetCustomPkgdataZip(this js.Value, args []js.Value) any {
	if len(args) != 1 {
		return errors.New("SetCustomPkgdataZip: expected 1 argument")
	}
	if args[0].Type() != js.TypeObject || !args[0].InstanceOf(js.Global().Get("Uint8Array")) {
		return errors.New("SetCustomPkgdataZip: argument must be a Uint8Array")
	}
	customPkgdataZip := JSUint8ArrayToBytes(args[0])
	pkgdata.SetCustomPkgdataZip(customPkgdataZip)
	return nil
}

// SetClassfileAutoImportedPackages sets the auto-imported packages for the
// classfile specified by id.
func SetClassfileAutoImportedPackages(this js.Value, args []js.Value) any {
	if len(args) != 2 {
		return errors.New("SetClassfileAutoImportedPackages: expected 2 argument")
	}
	if args[0].Type() != js.TypeString {
		return errors.New("SetClassfileAutoImportedPackages: first argument must be a string")
	}
	if args[1].Type() != js.TypeObject {
		return errors.New("SetClassfileAutoImportedPackages: argument must be an object")
	}

	id := args[0].String()

	pkgs := make(map[string]string)
	keys := js.Global().Get("Object").Call("keys", args[1])
	for i := range keys.Length() {
		key := keys.Index(i).String()
		value := args[1].Get(key)
		if value.Type() != js.TypeString {
			return errors.New("SetClassfileAutoImportedPackages: all values must be strings")
		}
		pkgs[key] = value.String()
	}

	xgo.SetClassfileAutoImportedPackages(id, pkgs)
	return nil
}

// JSFuncOfWithError returns a function to be used by JavaScript that can return
// an error.
func JSFuncOfWithError(fn func(this js.Value, args []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		result := fn(this, args)
		if err, ok := result.(error); ok {
			return js.Global().Get("Error").New(err.Error())
		}
		return result
	})
}

// JSUint8ArrayToBytes converts a JavaScript Uint8Array to a []byte.
func JSUint8ArrayToBytes(uint8Array js.Value) []byte {
	b := make([]byte, uint8Array.Length())
	js.CopyBytesToGo(b, uint8Array)
	return b
}

// ConvertJSFilesToMap converts a JavaScript object of files to a map.
func ConvertJSFilesToMap(files js.Value) map[string]vfs.MapFile {
	if files.Type() != js.TypeObject {
		return nil
	}
	keys := js.Global().Get("Object").Call("keys", files)
	result := make(map[string]vfs.MapFile, keys.Length())
	for i := range keys.Length() {
		key := keys.Index(i).String()
		value := files.Get(key)
		if value.InstanceOf(js.Global().Get("Object")) {
			result[key] = &vfs.MapFileImpl{
				Content: JSUint8ArrayToBytes(value.Get("content")),
				ModTime: time.UnixMilli(int64(value.Get("modTime").Int())),
			}
		}
	}
	return result
}

func main() {
	js.Global().Set("NewSpxls", JSFuncOfWithError(NewSpxls))
	js.Global().Set("SetCustomPkgdataZip", JSFuncOfWithError(SetCustomPkgdataZip))
	js.Global().Set("SetClassfileAutoImportedPackages", JSFuncOfWithError(SetClassfileAutoImportedPackages))
	select {}
}
