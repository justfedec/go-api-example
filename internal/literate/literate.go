// Package literate extracts Inkdown code from Markdown documents.
//
// An Inkdown program is a Markdown file; only fenced code blocks whose info
// string starts with "inkdown" (and does not contain "example") are compiled.
// All such blocks are concatenated in order of appearance ("tangle"). The
// extractor keeps a map from extracted line numbers back to lines of the
// original document so that every later stage can report diagnostics against
// the Markdown source.
package literate

import (
	"errors"
	"strings"
)

// Program is the tangled source of one Markdown document.
type Program struct {
	// Code is the concatenation of all compiling code blocks. Every line,
	// including the last, is terminated by '\n'.
	Code string

	// LineMap maps 1-based line numbers of Code to 1-based line numbers of
	// the Markdown document: LineMap[i-1] is the document line of Code line i.
	LineMap []int
}

// MdLine translates a 1-based line number of the extracted code into the
// corresponding 1-based line number of the Markdown document. Out-of-range
// values are clamped so diagnostics never fail to render.
func (p *Program) MdLine(codeLine int) int {
	if len(p.LineMap) == 0 {
		return codeLine
	}
	if codeLine < 1 {
		codeLine = 1
	}
	if codeLine > len(p.LineMap) {
		codeLine = len(p.LineMap)
	}
	return p.LineMap[codeLine-1]
}

// ErrNoCode is returned when the document contains no compiling code blocks.
var ErrNoCode = errors.New("no ```inkdown code blocks found (fences must start at column 0)")

// Extract tangles all compiling code blocks of a Markdown document.
func Extract(markdown string) (*Program, error) {
	var (
		prog      Program
		code      strings.Builder
		inBlock   bool
		fenceChar byte
		fenceLen  int
		compiling bool
	)

	lines := strings.Split(markdown, "\n")
	// A trailing newline produces one final empty element; it is not a line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for i, line := range lines {
		mdLine := i + 1
		if !inBlock {
			ch, n, info, ok := openingFence(line)
			if !ok {
				continue
			}
			inBlock = true
			fenceChar, fenceLen = ch, n
			compiling = isInkdownInfo(info)
			continue
		}
		if isClosingFence(line, fenceChar, fenceLen) {
			inBlock = false
			continue
		}
		if compiling {
			code.WriteString(line)
			code.WriteByte('\n')
			prog.LineMap = append(prog.LineMap, mdLine)
		}
	}
	// An unclosed fence runs to the end of the file (CommonMark behavior).

	if len(prog.LineMap) == 0 {
		return nil, ErrNoCode
	}
	prog.Code = code.String()
	return &prog, nil
}

// openingFence reports whether line opens a fenced code block at column 0,
// returning the fence character, its length, and the info string.
func openingFence(line string) (ch byte, n int, info string, ok bool) {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return 0, 0, "", false
	}
	ch = line[0]
	n = runLen(line, ch)
	if n < 3 {
		return 0, 0, "", false
	}
	info = strings.TrimSpace(line[n:])
	// CommonMark: the info string of a backtick fence cannot contain backticks
	// (that would be an inline code span, not a fence).
	if ch == '`' && strings.ContainsRune(info, '`') {
		return 0, 0, "", false
	}
	return ch, n, info, true
}

// isClosingFence reports whether line closes a block opened with n or more
// ch characters: same fence character, at least as many, nothing else but
// trailing whitespace.
func isClosingFence(line string, ch byte, n int) bool {
	got := runLen(line, ch)
	return got >= n && strings.TrimSpace(line[got:]) == ""
}

func runLen(s string, ch byte) int {
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	return n
}

// isInkdownInfo reports whether an info string marks a compiling block: the
// first word is "inkdown" and no word is "example".
func isInkdownInfo(info string) bool {
	fields := strings.Fields(info)
	if len(fields) == 0 || fields[0] != "inkdown" {
		return false
	}
	for _, f := range fields[1:] {
		if f == "example" {
			return false
		}
	}
	return true
}
