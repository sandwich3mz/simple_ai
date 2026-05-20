package rag

import (
	"fmt"
	"strings"
	"testing"
)

func TestSplitTextIntoChunksSplitsAndOverlapsLongText(t *testing.T) {
	chunks := splitTextIntoChunks("abcdefghijklmnopqrstuv", 10, 3)

	want := []string{
		"abcdefghij",
		"hijklmnopq",
		"opqrstuv",
	}
	if len(chunks) != len(want) {
		t.Fatalf("chunk count = %d, want %d: %#v", len(chunks), len(want), chunks)
	}
	for i := range want {
		if chunks[i] != want[i] {
			t.Fatalf("chunk %d = %q, want %q", i, chunks[i], want[i])
		}
	}
}

func TestSplitTextIntoChunksPrefersParagraphBoundaries(t *testing.T) {
	text := strings.Join([]string{
		"first paragraph",
		"second paragraph",
		"third paragraph",
	}, "\n\n")

	chunks := splitTextIntoChunks(text, 25, 5)
	if len(chunks) < 2 {
		t.Fatalf("chunk count = %d, want at least 2", len(chunks))
	}
	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			t.Fatalf("chunk %d is empty", i)
		}
		if len([]rune(chunk)) > 30 {
			t.Fatalf("chunk %d too large: %d runes", i, len([]rune(chunk)))
		}
	}
}

func TestSplitTextIntoChunksRejectsInvalidParameters(t *testing.T) {
	tests := []struct {
		size    int
		overlap int
	}{
		{size: 0, overlap: 0},
		{size: 10, overlap: 10},
		{size: 10, overlap: 11},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("size_%d_overlap_%d", tt.size, tt.overlap), func(t *testing.T) {
			if chunks := splitTextIntoChunks("hello", tt.size, tt.overlap); len(chunks) != 0 {
				t.Fatalf("chunks = %#v, want empty result", chunks)
			}
		})
	}
}
