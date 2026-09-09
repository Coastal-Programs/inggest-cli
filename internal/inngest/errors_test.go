package inngest

import (
	"strings"
	"testing"
)

func TestTruncateBody_ShortBodyUnchanged(t *testing.T) {
	body := "short error body"
	if got := truncateBody(body); got != body {
		t.Errorf("expected body unchanged, got %q", got)
	}
}

func TestTruncateBody_AtMaxLenUnchanged(t *testing.T) {
	body := strings.Repeat("a", truncateBodyMaxLen)
	if got := truncateBody(body); got != body {
		t.Errorf("expected body of exactly max len unchanged, got len %d", len(got))
	}
}

func TestTruncateBody_LongBodyTruncated(t *testing.T) {
	body := strings.Repeat("a", truncateBodyMaxLen+50)

	got := truncateBody(body)

	want := strings.Repeat("a", truncateBodyMaxLen) + "...(truncated)"
	if got != want {
		t.Errorf("expected truncated body, got %q", got)
	}
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Errorf("expected truncation marker suffix, got %q", got)
	}
}
