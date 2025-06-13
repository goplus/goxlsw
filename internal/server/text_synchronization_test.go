package server

import (
	"errors"
	"go/token"
	"testing"
	"time"

	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/jsonrpc2"
	"github.com/goplus/xgolsw/protocol"
	"github.com/goplus/xgolsw/xgo"
)

// MockReplier implements a message replier for testing
type MockReplier struct {
	notifications []*jsonrpc2.Notification
}

// ReplyMessage records notifications for later verification
func (m *MockReplier) ReplyMessage(msg jsonrpc2.Message) error {
	if n, ok := msg.(*jsonrpc2.Notification); ok {
		m.notifications = append(m.notifications, n)
	}
	return nil
}

func file(text string) xgo.File {
	return &xgo.FileImpl{Content: []byte(text)}
}

// strPtr returns a pointer to the given string
func strPtr(s string) *string {
	return &s
}

func TestModifyFiles(t *testing.T) {
	tests := []struct {
		name    string
		initial map[string]xgo.File
		changes []FileChange
		want    map[string]string // path -> expected content
	}{
		{
			name:    "add new files",
			initial: map[string]xgo.File{},
			changes: []FileChange{
				{
					Path:    "new.go",
					Content: []byte("package main"),
					Version: 100,
				},
			},
			want: map[string]string{
				"new.go": "package main",
			},
		},
		{
			name: "update existing file with newer version",
			initial: map[string]xgo.File{
				"main.go": &xgo.FileImpl{
					Content: []byte("old content"),
					ModTime: time.UnixMilli(100),
				},
			},
			changes: []FileChange{
				{
					Path:    "main.go",
					Content: []byte("new content"),
					Version: 200,
				},
			},
			want: map[string]string{
				"main.go": "new content",
			},
		},
		{
			name: "ignore older version update",
			initial: map[string]xgo.File{
				"main.go": &xgo.FileImpl{
					Content: []byte("current content"),
					Version: 200,
				},
			},
			changes: []FileChange{
				{
					Path:    "main.go",
					Content: []byte("old content"),
					Version: 100,
				},
			},
			want: map[string]string{
				"main.go": "current content",
			},
		},
		{
			name: "multiple file changes",
			initial: map[string]xgo.File{
				"file1.go": &xgo.FileImpl{
					Content: []byte("content1"),
					ModTime: time.UnixMilli(100),
				},
				"file2.go": &xgo.FileImpl{
					Content: []byte("content2"),
					ModTime: time.UnixMilli(100),
				},
			},
			changes: []FileChange{
				{
					Path:    "file1.go",
					Content: []byte("new content1"),
					Version: 200,
				},
				{
					Path:    "file3.go",
					Content: []byte("content3"),
					Version: 200,
				},
			},
			want: map[string]string{
				"file1.go": "new content1",
				"file2.go": "content2",
				"file3.go": "content3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new project with initial files
			proj := xgo.NewProject(token.NewFileSet(), tt.initial, xgo.FeatAll)

			// Create a TestServer that extends the real Server
			server := &Server{
				workspaceRootFS: proj,
			}

			// Apply changes
			server.ModifyFiles(tt.changes)

			// Verify results
			for path, wantContent := range tt.want {
				file, ok := proj.File(path)
				if !ok {
					t.Errorf("file %s not found", path)
					continue
				}
				if got := string(file.Content); got != wantContent {
					t.Errorf("%s file %s content = %q, want %q", tt.name, path, got, wantContent)
				}
			}

			// Verify no extra files exist
			count := 0
			proj.RangeFiles(func(path string) bool {
				count++
				if _, ok := tt.want[path]; !ok {
					t.Errorf("unexpected file: %s", path)
				}
				return true
			})
			if count != len(tt.want) {
				t.Errorf("got %d files, want %d", count, len(tt.want))
			}
		})
	}
}

// TestDidOpen tests the didOpen handler functionality
func TestDidOpen(t *testing.T) {
	tests := []struct {
		name            string
		params          *protocol.DidOpenTextDocumentParams
		expectedPath    string
		expectedContent string
		wantErr         bool
	}{
		{
			name: "basic open",
			params: &protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:     "file://workspace/echo.spx",
					Version: 1,
					Text:    "echo \"100\""},
			},
			expectedPath:    "echo.spx",
			expectedContent: "echo \"100\"",
			wantErr:         false,
		},
		{
			name: "open file with function",
			params: &protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:     "file://workspace/test_func.spx",
					Version: 2,
					Text:    "onStart {\n say \"Hello, World!\"\n}",
				},
			},
			expectedPath:    "test_func.spx",
			expectedContent: "onStart {\n say \"Hello, World!\"\n}",
			wantErr:         false,
		},
		{
			name: "open file with unicode content",
			params: &protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:     "file://workspace/i18n.spx",
					Version: 3,
					Text:    "onStart {\n say \"你好，世界!\"\n}",
				},
			},
			expectedPath:    "i18n.spx",
			expectedContent: "onStart {\n say \"你好，世界!\"\n}",
			wantErr:         false,
		},
		{
			name: "URI conversion error",
			params: &protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:     "file://error_workspace/error.spx",
					Version: 1,
					Text:    "onStart {}",
				},
			},
			wantErr: true,
		},
		{
			name: "empty file content",
			params: &protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:     "file://workspace/empty.spx",
					Version: 1,
					Text:    "",
				},
			},
			expectedPath:    "empty.spx",
			expectedContent: "",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment with real Project instead of MockProject
			proj := xgo.NewProject(token.NewFileSet(), make(map[string]xgo.File), 0)
			proj.PutFile(tt.expectedPath, file("mock content"))
			mockReplier := &MockReplier{}

			// Create a TestServer that extends the real Server
			server := &Server{
				workspaceRootFS:  proj,
				replier:          mockReplier,
				workspaceRootURI: "file://workspace/",
			}

			// Execute test
			err := server.didOpen(tt.params)

			time.Sleep(1 * time.Second)
			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("didOpen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify file was correctly added to the project
				file, ok := proj.File(tt.expectedPath)
				if !ok {
					t.Errorf("File not found in project: %s", tt.expectedPath)
					return
				}

				if string(file.Content) != tt.expectedContent {
					t.Errorf("File %s content = %q, want %q", tt.expectedPath, string(file.Content), tt.expectedContent)
				}

				// If available, check file version
				if _, ok := proj.File(tt.expectedPath); ok {
					expectedVersion := int(tt.params.TextDocument.Version)
					// Note: In a real test, you might need to extract the version from the FileImpl
					// This depends on how version is stored in your implementation
					t.Logf("File opened with version: %d", expectedVersion)
				}
			}
		})
	}
}

// TestDidChange tests the didChange handler functionality
func TestDidChange(t *testing.T) {
	tests := []struct {
		name            string
		initialContent  string
		params          *protocol.DidChangeTextDocumentParams
		convertError    error
		expectedContent string
		wantErr         bool
	}{
		{
			name:           "full document replacement",
			initialContent: "package main",
			params: &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{
						URI: "file://workspace/test.xgo",
					},
					Version: 2,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{
					{Text: "package main\n\nfunc main() {}"},
				},
			},
			expectedContent: "package main\n\nfunc main() {}",
			wantErr:         false,
		},
		{
			name:           "incremental change",
			initialContent: "package main\n\nfunc main() {\n\t\n}",
			params: &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{
						URI: "file://workspace/test.xgo",
					},
					Version: 2,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{
					{
						Range: &protocol.Range{
							Start: protocol.Position{Line: 3, Character: 1},
							End:   protocol.Position{Line: 3, Character: 1},
						},
						Text: "fmt.Println(\"Hello, World!\")",
					},
				},
			},
			expectedContent: "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			wantErr:         false,
		},
		{
			name:           "multiple incremental changes",
			initialContent: "package main\n\nfunc main() {\n\t\n}",
			params: &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{
						URI: "file://workspace/test.xgo",
					},
					Version: 3,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{
					{
						Range: &protocol.Range{
							Start: protocol.Position{Line: 3, Character: 1},
							End:   protocol.Position{Line: 3, Character: 1},
						},
						Text: "fmt.Print(\"Hello",
					},
					{
						Range: &protocol.Range{
							Start: protocol.Position{Line: 3, Character: 17},
							End:   protocol.Position{Line: 3, Character: 17},
						},
						Text: ", World!\")",
					},
				},
			},
			expectedContent: "package main\n\nfunc main() {\n\tfmt.Print(\"Hello, World!\")\n}",
			wantErr:         false,
		},
		{
			name:           "URI conversion error",
			initialContent: "package main",
			params: &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{
						URI: "file://error/test.xgo",
					},
					Version: 2,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{
					{Text: "package main\n\nfunc main() {}"},
				},
			},
			convertError: errors.New("URI conversion failed"),
			wantErr:      true,
		},
		{
			name:           "empty changes array",
			initialContent: "package main",
			params: &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{
						URI: "file://workspace/test.xgo",
					},
					Version: 2,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment with initial file content
			files := make(map[string]xgo.File)
			path := "test.xgo"

			files[path] = &xgo.FileImpl{
				Content: []byte(tt.initialContent),
				ModTime: time.Time{},
			}

			proj := xgo.NewProject(token.NewFileSet(), files, xgo.FeatAll)
			mockReplier := &MockReplier{}

			// Create a TestServer that extends the real Server
			server := &Server{
				workspaceRootFS:  proj,
				replier:          mockReplier,
				workspaceRootURI: "file://workspace/",
			}

			// Execute test
			err := server.didChange(tt.params)

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("%s didChange() error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify file content was updated
				file, ok := proj.File(path)
				if !ok {
					t.Errorf("%s File not found in project: %s", tt.name, path)
					return
				}

				if string(file.Content) != tt.expectedContent {
					t.Errorf("%s File content = %q, want %q", tt.name, string(file.Content), tt.expectedContent)
				}

				// If available, check file version
				expectedVersion := int(tt.params.TextDocument.Version)
				// Note: For a real implementation, verify the version is stored correctly
				t.Logf("%s File changed with version: %d", tt.name, expectedVersion)
			}
		})
	}
}

// TestDidSave tests the didSave handler functionality
func TestDidSave(t *testing.T) {
	tests := []struct {
		name            string
		initialContent  string
		params          *protocol.DidSaveTextDocumentParams
		expectedContent string
		contentChanged  bool
		wantErr         bool
	}{
		{
			name:           "save with text",
			initialContent: "package main",
			params: &protocol.DidSaveTextDocumentParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: "file://workspace/test.xgo",
				},
				Text: strPtr("package main\n\nfunc main() {}"),
			},
			expectedContent: "package main\n\nfunc main() {}",
			contentChanged:  true,
			wantErr:         false,
		},
		{
			name:           "save without text",
			initialContent: "package main",
			params: &protocol.DidSaveTextDocumentParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: "file://workspace/test.xgo",
				},
				Text: nil,
			},
			expectedContent: "package main", // Content should not change
			contentChanged:  false,
			wantErr:         false,
		},
		{
			name:           "URI conversion error",
			initialContent: "package main",
			params: &protocol.DidSaveTextDocumentParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: "file://error/test.xgo",
				},
				Text: strPtr("package main\n\nfunc main() {}"),
			},
			contentChanged: false,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			fset := xgotoken.NewFileSet()
			files := make(map[string]xgo.File)
			path := "test.xgo"

			files[path] = &xgo.FileImpl{
				Content: []byte(tt.initialContent),
				ModTime: time.Time{},
			}

			proj := xgo.NewProject(fset, files, xgo.FeatAST)
			mockReplier := &MockReplier{}

			// Create a TestServer
			server := &Server{
				workspaceRootFS:  proj,
				replier:          mockReplier,
				workspaceRootURI: "file://workspace/",
			}

			// Execute test
			err := server.didSave(tt.params)

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("%s didSave() error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify file content
				file, ok := proj.File(path)
				if !ok {
					t.Errorf("%s File not found in project: %s", tt.name, path)
					return
				}

				if string(file.Content) != tt.expectedContent {
					t.Errorf("%s File content = %q, want %q", tt.name, string(file.Content), tt.expectedContent)
				}
			}
		})
	}
}

// TestDidClose tests the didClose handler functionality
func TestDidClose(t *testing.T) {
	tests := []struct {
		name    string
		params  *protocol.DidCloseTextDocumentParams
		wantErr bool
	}{
		{
			name: "close document",
			params: &protocol.DidCloseTextDocumentParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: "file://workspace/test.xgo",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			fset := xgotoken.NewFileSet()
			files := make(map[string]xgo.File)
			path := "/test.xgo"

			files[path] = &xgo.FileImpl{
				Content: []byte("package main"),
				ModTime: time.Time{},
			}

			proj := xgo.NewProject(fset, files, xgo.FeatAST)
			mockReplier := &MockReplier{}

			// Create a TestServer
			server := &Server{
				workspaceRootFS:  proj,
				replier:          mockReplier,
				workspaceRootURI: "file://workspace/",
			}

			// Execute test
			err := server.didClose(tt.params)

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("didClose() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

// TestChangedText tests the changedText function for processing document content changes
func TestChangedText(t *testing.T) {
	tests := []struct {
		name           string
		initialContent string
		changes        []protocol.TextDocumentContentChangeEvent
		want           string
		wantErr        bool
	}{
		{
			name:           "full document replacement",
			initialContent: "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Text: "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
				},
			},
			want:    "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			wantErr: false,
		},
		{
			name:           "incremental change - add comma",
			initialContent: "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 3, Character: 19},
						End:   protocol.Position{Line: 3, Character: 19},
					},
					Text: ",",
				},
			},
			want:    "package main\n\nfunc main() {\n\tfmt.Println(\"Hello,\")\n}",
			wantErr: false,
		},
		{
			name:           "incremental change - replace word",
			initialContent: "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 3, Character: 14},
						End:   protocol.Position{Line: 3, Character: 19},
					},
					Text: "World",
				},
			},
			want:    "package main\n\nfunc main() {\n\tfmt.Println(\"World\")\n}",
			wantErr: false,
		},
		{
			name:           "multiple incremental changes",
			initialContent: "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 3, Character: 14},
						End:   protocol.Position{Line: 3, Character: 19},
					},
					Text: "World",
				},
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 3, Character: 19},
						End:   protocol.Position{Line: 3, Character: 19},
					},
					Text: "!",
				},
			},
			want:    "package main\n\nfunc main() {\n\tfmt.Println(\"World!\")\n}",
			wantErr: false,
		},
		{
			name:           "empty changes array",
			initialContent: "package main",
			changes:        []protocol.TextDocumentContentChangeEvent{},
			want:           "",
			wantErr:        true,
		},
		{
			name:           "invalid range - end before start",
			initialContent: "package main",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 0, Character: 5},
						End:   protocol.Position{Line: 0, Character: 3},
					},
					Text: "invalid",
				},
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			fset := xgotoken.NewFileSet()
			files := make(map[string]xgo.File)
			path := "/test.xgo"

			// Create initial file
			files[path] = &xgo.FileImpl{
				Content: []byte(tt.initialContent),
				ModTime: time.Now(),
			}

			proj := xgo.NewProject(fset, files, xgo.FeatAST)

			// For AST parsing to work, we need a real file with content
			// parsed into the AST before we can apply changes
			_, err := proj.AST(path)
			if err != nil {
				t.Fatalf("Failed to setup test: %v", err)
			}

			server := &Server{
				workspaceRootFS: proj,
			}

			// Execute test
			got, err := server.changedText(path, tt.changes)

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("changedText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if string(got) != tt.want {
					t.Errorf("%s changedText() = %q, want %q", tt.name, string(got), tt.want)
				}
			}
		})
	}
}

// TestApplyIncrementalChanges tests the applyIncrementalChanges function
func TestApplyIncrementalChanges(t *testing.T) {
	tests := []struct {
		name           string
		initialContent string
		changes        []protocol.TextDocumentContentChangeEvent
		want           string
		wantErr        bool
	}{
		{
			name:           "add text at beginning",
			initialContent: "func main() {}",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 0},
					},
					Text: "package main\n\n",
				},
			},
			want:    "package main\n\nfunc main() {}",
			wantErr: false,
		},
		{
			name:           "add text in middle",
			initialContent: "func main() {}",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 0, Character: 13},
						End:   protocol.Position{Line: 0, Character: 13},
					},
					Text: "\n\tfmt.Println(\"Hello\")\n",
				},
			},
			want:    "func main() {\n\tfmt.Println(\"Hello\")\n}",
			wantErr: false,
		},
		{
			name:           "delete text",
			initialContent: "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 2, Character: 0},
						End:   protocol.Position{Line: 4, Character: 1},
					},
					Text: "",
				},
			},
			want:    "package main\n\n",
			wantErr: false,
		},
		{
			name:           "replace entire line",
			initialContent: "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 3, Character: 0},
						End:   protocol.Position{Line: 3, Character: 21},
					},
					Text: "\tfmt.Println(\"Hello, World!\")",
				},
			},
			want:    "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			wantErr: false,
		},
		{
			name:           "nil range",
			initialContent: "package main",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: nil,
					Text:  "new content",
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name:           "non-existent file",
			initialContent: "",
			changes: []protocol.TextDocumentContentChangeEvent{
				{
					Range: &protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 0},
					},
					Text: "package main",
				},
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			fset := xgotoken.NewFileSet()
			files := make(map[string]xgo.File)
			path := "/test.xgo"

			if tt.initialContent != "" {
				files[path] = &xgo.FileImpl{
					Content: []byte(tt.initialContent),
					ModTime: time.Now(),
				}
			}

			proj := xgo.NewProject(fset, files, xgo.FeatAST)

			// For tests with content, ensure we have AST
			if tt.initialContent != "" {
				_, err := proj.AST(path)
				if err != nil {
					t.Fatalf("Failed to setup test: %v", err)
				}
			}

			server := &Server{
				workspaceRootFS: proj,
			}

			// Execute test
			got, err := server.applyIncrementalChanges(path, tt.changes)

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("applyIncrementalChanges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if string(got) != tt.want {
					t.Errorf("%s applyIncrementalChanges() = %q, want %q", tt.name, string(got), tt.want)
				}
			}
		})
	}
}

// TestGetDiagnostics tests the getDiagnostics function for generating diagnostic information
func TestGetDiagnostics(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		path           string
		wantDiagCount  int
		wantSeverities []protocol.DiagnosticSeverity
		wantErr        bool
	}{
		{
			name:           "import errors",
			content:        "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			path:           "/test.xgo",
			wantDiagCount:  1,
			wantSeverities: []protocol.DiagnosticSeverity{SeverityError},
			wantErr:        false,
		},
		{
			name:           "syntax error",
			content:        "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\"\n}", // Missing closing parenthesis
			path:           "/syntax_error.xgo",
			wantDiagCount:  8,
			wantSeverities: []protocol.DiagnosticSeverity{SeverityError},
			wantErr:        false,
		},
		{
			name:           "type error",
			content:        "package main\n\nfunc main() {\n\tvar x int = \"string\"\n}", // Type mismatch
			path:           "/type_error.xgo",
			wantDiagCount:  1,
			wantSeverities: []protocol.DiagnosticSeverity{SeverityError},
			wantErr:        false,
		},
		{
			name:           "no error",
			content:        "package main\n\nfunc main() {\n\t}",
			path:           "/code_error.xgo",
			wantDiagCount:  0,
			wantSeverities: []protocol.DiagnosticSeverity{},
			wantErr:        false,
		},
		{
			name:           "multiple type errors",
			content:        "package main\n\nfunc main() {\n\tvar x int = \"string\"\n\tvar y bool = 42\n}",
			path:           "/multiple_errors.xgo",
			wantDiagCount:  2,
			wantSeverities: []protocol.DiagnosticSeverity{SeverityError},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			fset := xgotoken.NewFileSet()
			files := make(map[string]xgo.File)

			// Create the test file
			files[tt.path] = &xgo.FileImpl{
				Content: []byte(tt.content),
				ModTime: time.Now(),
			}

			// Create a mock Project that returns our predefined errors
			server := &Server{
				workspaceRootFS: xgo.NewProject(fset, files, xgo.FeatAll),
			}

			// Execute test
			diagnostics, err := server.getDiagnostics(tt.path)

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("getDiagnostics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// for _, d := range diagnostics {
			// 	t.Logf("%s Diagnostic: %v; Range: %d/%d %d/%d", tt.name, d.Message,
			// 		d.Range.Start.Line, d.Range.Start.Character, d.Range.End.Line, d.Range.End.Character)
			// }

			if len(diagnostics) != tt.wantDiagCount {
				t.Errorf("%s getDiagnostics() returned %v diagnostics, want %d", tt.name, len(diagnostics), tt.wantDiagCount)
			}

			// Check diagnostic severities
			for i, diag := range diagnostics {
				if i >= len(tt.wantSeverities) {
					break
				}
				if diag.Severity != tt.wantSeverities[i] {
					t.Errorf("diagnostic[%d] severity = %d, want %d", i, diag.Severity, tt.wantSeverities[i])
				}
			}
		})
	}
}
