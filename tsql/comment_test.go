package tsql

import (
	"strings"
	"testing"
)

func TestCommentSafe(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"a\nb", "a b"},
		{"a\r\nb", "a  b"},
		{"a\tb\x00c", "a b c"},
		{"plain é", "plain é"},
	} {
		if got := CommentSafe(tc.in); got != tc.want {
			t.Errorf("CommentSafe(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBlockCommentSafe(t *testing.T) {
	for _, s := range []string{"*/", "/*", "*/*", "/*/", "**//", "a*/*/b", "/**/", "x"} {
		got := BlockCommentSafe(s)
		if strings.Contains(got, "*/") || strings.Contains(got, "/*") {
			t.Errorf("BlockCommentSafe(%q) = %q, still holds a delimiter", s, got)
		}
		if strings.ReplaceAll(got, " ", "") != strings.ReplaceAll(s, " ", "") {
			t.Errorf("BlockCommentSafe(%q) = %q, changed more than spacing", s, got)
		}
	}
}
