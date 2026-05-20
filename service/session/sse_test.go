package session

import "testing"

func TestFormatSSEDataPreservesMultilineChunks(t *testing.T) {
	got := formatSSEData("line 1\nline 2")
	want := "data: line 1\ndata: line 2\n\n"
	if got != want {
		t.Fatalf("formatSSEData() = %q, want %q", got, want)
	}
}
