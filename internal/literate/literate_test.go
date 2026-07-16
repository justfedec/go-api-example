package literate

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestExtractSingleBlock(t *testing.T) {
	md := strings.Join([]string{
		"# Title",     // 1
		"",            // 2
		"Some prose.", // 3
		"",            // 4
		"```inkdown",  // 5
		`print("hi")`, // 6
		"let x = 1",   // 7
		"```",         // 8
		"",            // 9
		"More prose.", // 10
	}, "\n") + "\n"

	p, err := Extract(md)
	if err != nil {
		t.Fatal(err)
	}
	wantCode := "print(\"hi\")\nlet x = 1\n"
	if p.Code != wantCode {
		t.Errorf("Code = %q, want %q", p.Code, wantCode)
	}
	if want := []int{6, 7}; !reflect.DeepEqual(p.LineMap, want) {
		t.Errorf("LineMap = %v, want %v", p.LineMap, want)
	}
}

func TestExtractTanglesBlocksInOrder(t *testing.T) {
	md := strings.Join([]string{
		"intro",      // 1
		"```inkdown", // 2
		"let a = 1",  // 3
		"```",        // 4
		"middle",     // 5
		"~~~inkdown", // 6
		"let b = 2",  // 7
		"~~~",        // 8
	}, "\n")

	p, err := Extract(md)
	if err != nil {
		t.Fatal(err)
	}
	if want := "let a = 1\nlet b = 2\n"; p.Code != want {
		t.Errorf("Code = %q, want %q", p.Code, want)
	}
	if want := []int{3, 7}; !reflect.DeepEqual(p.LineMap, want) {
		t.Errorf("LineMap = %v, want %v", p.LineMap, want)
	}
}

func TestExtractSkipsNonInkdownAndExampleBlocks(t *testing.T) {
	md := strings.Join([]string{
		"```go",              // ignored: other language
		"package main",       //
		"```",                //
		"```inkdown example", // ignored: example
		"print(1)",           //
		"```",                //
		"```",                // ignored: no info string
		"plain",              //
		"```",                //
		"```inkdown",         // compiled
		"print(2)",           // 11
		"```",                //
		"``` inkdown extra",  // compiled: leading space in info, extra word
		"print(3)",           // 14
		"```",                //
	}, "\n")

	p, err := Extract(md)
	if err != nil {
		t.Fatal(err)
	}
	if want := "print(2)\nprint(3)\n"; p.Code != want {
		t.Errorf("Code = %q, want %q", p.Code, want)
	}
	if want := []int{11, 14}; !reflect.DeepEqual(p.LineMap, want) {
		t.Errorf("LineMap = %v, want %v", p.LineMap, want)
	}
}

func TestExtractNestedFencesOfOtherKind(t *testing.T) {
	// Inside a ~~~ block, ``` lines are content; and a ```markdown block
	// showing an inkdown block must not leak code.
	md := strings.Join([]string{
		"~~~inkdown",   // 1
		"let a = 1",    // 2
		"```inkdown",   // 3: content, not a fence for us... but IS extracted as code
		"~~~",          // 4: closes
		"````markdown", // 5: four backticks
		"```inkdown",   // 6: content of the markdown block
		"print(9)",     // 7: content
		"```",          // 8: content (only 3 backticks, opener had 4)
		"````",         // 9: closes
	}, "\n")

	p, err := Extract(md)
	if err != nil {
		t.Fatal(err)
	}
	if want := "let a = 1\n```inkdown\n"; p.Code != want {
		t.Errorf("Code = %q, want %q", p.Code, want)
	}
}

func TestExtractUnclosedBlockRunsToEOF(t *testing.T) {
	md := "```inkdown\nprint(1)\nprint(2)"
	p, err := Extract(md)
	if err != nil {
		t.Fatal(err)
	}
	if want := "print(1)\nprint(2)\n"; p.Code != want {
		t.Errorf("Code = %q, want %q", p.Code, want)
	}
	if want := []int{2, 3}; !reflect.DeepEqual(p.LineMap, want) {
		t.Errorf("LineMap = %v, want %v", p.LineMap, want)
	}
}

func TestExtractIndentedFenceIsNotAFence(t *testing.T) {
	md := "  ```inkdown\nprint(1)\n  ```\n```inkdown\nprint(2)\n```\n"
	p, err := Extract(md)
	if err != nil {
		t.Fatal(err)
	}
	if want := "print(2)\n"; p.Code != want {
		t.Errorf("Code = %q, want %q", p.Code, want)
	}
}

func TestExtractNoCode(t *testing.T) {
	_, err := Extract("# just prose\n\n```go\nx\n```\n")
	if !errors.Is(err, ErrNoCode) {
		t.Errorf("err = %v, want ErrNoCode", err)
	}
}

func TestMdLine(t *testing.T) {
	p := &Program{LineMap: []int{5, 6, 10}}
	for _, tc := range []struct{ in, want int }{
		{1, 5}, {2, 6}, {3, 10},
		{0, 5},  // clamped low
		{9, 10}, // clamped high
	} {
		if got := p.MdLine(tc.in); got != tc.want {
			t.Errorf("MdLine(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
