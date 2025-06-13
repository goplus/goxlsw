package server

import (
	"bytes"
	"unicode/utf16"
	"unicode/utf8"

	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// ToPtr returns a pointer to the value.
func ToPtr[T any](v T) *T {
	return &v
}

// FromPtr returns the value from a pointer. It returns the zero value of type T
// if the pointer is nil.
func FromPtr[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// DedupeLocations deduplicates locations.
func DedupeLocations(locations []Location) []Location {
	result := make([]Location, 0, len(locations))
	seen := make(map[Location]struct{}, len(locations))
	for _, loc := range locations {
		if _, ok := seen[loc]; !ok {
			seen[loc] = struct{}{}
			result = append(result, loc)
		}
	}
	return result
}

// UTF16Len calculates the UTF-16 length of the given string.
func UTF16Len(s string) int {
	var length int
	for _, r := range s {
		length += utf16.RuneLen(r)
	}
	return length
}

// UTF16PosToUTF8Offset converts a UTF-16 code unit position to a UTF-8 byte
// offset in the given string.
func UTF16PosToUTF8Offset(s string, utf16Pos int) int {
	if utf16Pos <= 0 {
		return 0
	}

	var utf16Units, utf8Bytes int
	for _, r := range s {
		nextUTF16Units := utf16Units + utf16.RuneLen(r)
		if nextUTF16Units > utf16Pos {
			break
		}
		utf16Units = nextUTF16Units
		utf8Bytes += utf8.RuneLen(r)
	}
	return utf8Bytes
}

// PositionOffset converts an LSP position (line, character) to a byte offset in the document.
// It calculates the offset by:
//  1. Finding the starting byte offset of the requested line
//  2. Adding the character offset within that line, converting from UTF-16 to UTF-8 if needed
//
// Parameters:
//   - content: The file content as a byte array
//   - position: The LSP position with line and character numbers (0-based)
//
// Returns the byte offset from the beginning of the document
func PositionOffset(content []byte, position Position) int {
	// If content is empty or position is beyond the content, return 0
	if len(content) == 0 {
		return 0
	}

	// Find all line start positions in the document
	lineStarts := []int{0} // First line always starts at position 0
	for i, b := range content {
		if b == '\n' {
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
	utf8Offset := UTF16PosToUTF8Offset(string(lineContent), int(position.Character))

	// Ensure the final offset doesn't exceed the content length
	return lineOffset + utf8Offset
}

// FromPosition converts a [xgotoken.Position] to a [Position].
func FromPosition(proj *xgo.Project, astFile *xgoast.File, position xgotoken.Position) Position {
	tokenFile := xgoutil.NodeTokenFile(proj, astFile)

	line := position.Line
	lineStart := int(tokenFile.LineStart(line))
	relLineStart := lineStart - tokenFile.Base()
	lineContent := astFile.Code[relLineStart : relLineStart+position.Column-1]

	return Position{
		Line:      uint32(position.Line - 1),
		Character: uint32(UTF16Len(string(lineContent))),
	}
}

// ToPosition converts a [Position] to a [xgotoken.Position].
func ToPosition(proj *xgo.Project, astFile *xgoast.File, position Position) xgotoken.Position {
	tokenFile := xgoutil.NodeTokenFile(proj, astFile)

	line := min(int(position.Line)+1, tokenFile.LineCount())
	lineStart := int(tokenFile.LineStart(line))
	relLineStart := lineStart - tokenFile.Base()
	lineContent := astFile.Code[relLineStart:]
	if i := bytes.IndexByte(lineContent, '\n'); i >= 0 {
		lineContent = lineContent[:i]
	}
	utf8Offset := UTF16PosToUTF8Offset(string(lineContent), int(position.Character))
	column := utf8Offset + 1

	return xgotoken.Position{
		Filename: tokenFile.Name(),
		Offset:   relLineStart + utf8Offset,
		Line:     line,
		Column:   column,
	}
}

// PosAt returns the [xgotoken.Pos] of the given position in the given AST file.
func PosAt(proj *xgo.Project, astFile *xgoast.File, position Position) xgotoken.Pos {
	tokenFile := xgoutil.NodeTokenFile(proj, astFile)
	if int(position.Line) > tokenFile.LineCount()-1 {
		return xgotoken.Pos(tokenFile.Base() + tokenFile.Size()) // EOF
	}
	return tokenFile.Pos(ToPosition(proj, astFile, position).Offset)
}

// RangeForASTFilePosition returns a [Range] for the given [xgotoken.Position]
// in the given AST file.
func RangeForASTFilePosition(proj *xgo.Project, astFile *xgoast.File, position xgotoken.Position) Range {
	p := FromPosition(proj, astFile, position)
	return Range{Start: p, End: p}
}

// RangeForASTFileNode returns the [Range] for the given node in the given AST file.
func RangeForASTFileNode(proj *xgo.Project, astFile *xgoast.File, node xgoast.Node) Range {
	fset := proj.Fset
	return Range{
		Start: FromPosition(proj, astFile, fset.Position(node.Pos())),
		End:   FromPosition(proj, astFile, fset.Position(node.End())),
	}
}

// RangeForPos returns the [Range] for the given position.
func RangeForPos(proj *xgo.Project, pos xgotoken.Pos) Range {
	return RangeForASTFilePosition(proj, xgoutil.PosASTFile(proj, pos), proj.Fset.Position(pos))
}

// RangeForPosEnd returns the [Range] for the given pos and end positions.
func RangeForPosEnd(proj *xgo.Project, pos, end xgotoken.Pos) Range {
	astFile := xgoutil.PosASTFile(proj, pos)
	return Range{
		Start: FromPosition(proj, astFile, proj.Fset.Position(pos)),
		End:   FromPosition(proj, astFile, proj.Fset.Position(end)),
	}
}

// RangeForNode returns the [Range] for the given node.
func RangeForNode(proj *xgo.Project, node xgoast.Node) Range {
	return RangeForASTFileNode(proj, xgoutil.NodeASTFile(proj, node), node)
}

// IsRangesOverlap reports whether two ranges overlap.
func IsRangesOverlap(a, b Range) bool {
	return a.Start.Line <= b.End.Line && (a.Start.Line != b.End.Line || a.Start.Character <= b.End.Character) &&
		b.Start.Line <= a.End.Line && (b.Start.Line != a.End.Line || b.Start.Character <= a.End.Character)
}
