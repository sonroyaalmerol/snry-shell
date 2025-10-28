package clipboard_test

import (
	"testing"

	"github.com/sonroyaalmerol/snry-shell/internal/services/clipboard"
)

func TestParseCliphistList(t *testing.T) {
	input := "1\thello world\n2\tsome other text\n3\thttp://example.com\n"
	entries, err := clipboard.ParseCliphistList(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].ID != 1 || entries[0].Preview != "hello world" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[2].ID != 3 || entries[2].Preview != "http://example.com" {
		t.Fatalf("unexpected third entry: %+v", entries[2])
	}
}

func TestParseCliphistListEmpty(t *testing.T) {
	entries, err := clipboard.ParseCliphistList("")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseCliphistListSkipsMalformed(t *testing.T) {
	input := "1\tvalid entry\nnot-a-number\tskipped\n2\talso valid\n"
	entries, err := clipboard.ParseCliphistList(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 valid entries, got %d", len(entries))
	}
}
