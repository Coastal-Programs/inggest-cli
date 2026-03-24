package output_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// captureStdout replaces os.Stdout with a pipe for the duration of fn,
// returning everything written to it.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	fnErr := fn()
	w.Close()
	os.Stdout = old
	return <-done, fnErr
}

// captureStderr replaces os.Stderr with a pipe for the duration of fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	fn()
	w.Close()
	os.Stderr = old
	return <-done
}

type testRow struct {
	Name  string
	Value int
}

// --- Print: JSON format ---

func TestPrint_JSON(t *testing.T) {
	data := testRow{Name: "foo", Value: 42}
	got, err := captureStdout(t, func() error {
		return output.Print(data, output.FormatJSON)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result testRow
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, got)
	}
	if result.Name != "foo" || result.Value != 42 {
		t.Errorf("got %+v, want {Name:foo Value:42}", result)
	}
}

func TestPrint_JSON_Unmarshalable(t *testing.T) {
	_, err := captureStdout(t, func() error {
		return output.Print(make(chan int), output.FormatJSON)
	})
	if err == nil {
		t.Error("expected error for un-marshalable type, got nil")
	}
}

func TestPrint_DefaultFormat_IsJSON(t *testing.T) {
	data := testRow{Name: "default", Value: 0}
	got, err := captureStdout(t, func() error {
		return output.Print(data, output.Format("unknown"))
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &result); err != nil {
		t.Fatalf("unknown format should fall back to JSON, got: %q", got)
	}
}

// --- Print: text format ---

func TestPrint_Text_Slice(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return output.Print([]string{"alpha", "beta"}, output.FormatText)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Errorf("expected alpha and beta in output, got: %q", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestPrint_Text_Struct(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return output.Print(testRow{Name: "bar", Value: 7}, output.FormatText)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Name: bar") {
		t.Errorf("expected 'Name: bar', got: %q", got)
	}
	if !strings.Contains(got, "Value: 7") {
		t.Errorf("expected 'Value: 7', got: %q", got)
	}
}

func TestPrint_Text_PointerToStruct(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return output.Print(&testRow{Name: "ptr", Value: 99}, output.FormatText)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Name: ptr") {
		t.Errorf("expected 'Name: ptr', got: %q", got)
	}
}

func TestPrint_Text_Map(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return output.Print(map[string]string{"key": "val"}, output.FormatText)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "key: val") {
		t.Errorf("expected 'key: val', got: %q", got)
	}
}

func TestPrint_Text_Scalar(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return output.Print("hello", output.FormatText)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("expected 'hello', got: %q", got)
	}
}

// --- Print: table format ---

func TestPrint_Table_StructSlice(t *testing.T) {
	rows := []testRow{
		{Name: "foo", Value: 1},
		{Name: "bar", Value: 2},
	}
	got, err := captureStdout(t, func() error {
		return output.Print(rows, output.FormatTable)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (header + 2 rows), got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "NAME") || !strings.Contains(lines[0], "VALUE") {
		t.Errorf("header line missing column names: %q", lines[0])
	}
	if !strings.Contains(lines[1], "foo") {
		t.Errorf("expected 'foo' in row 1: %q", lines[1])
	}
	if !strings.Contains(lines[2], "bar") {
		t.Errorf("expected 'bar' in row 2: %q", lines[2])
	}
}

func TestPrint_Table_PointerSlice(t *testing.T) {
	rows := []*testRow{
		{Name: "ptr1", Value: 10},
		{Name: "ptr2", Value: 20},
	}
	got, err := captureStdout(t, func() error {
		return output.Print(rows, output.FormatTable)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "ptr1") {
		t.Errorf("expected table with NAME header and ptr1 row, got: %q", got)
	}
}

func TestPrint_Table_EmptySlice(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return output.Print([]testRow{}, output.FormatTable)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty output for empty slice, got: %q", got)
	}
}

func TestPrint_Table_NonStructSlice_FallsBackToText(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return output.Print([]string{"x", "y"}, output.FormatTable)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "x") || !strings.Contains(got, "y") {
		t.Errorf("expected x and y in fallback text output, got: %q", got)
	}
}

func TestPrint_Table_PointerToSlice(t *testing.T) {
	rows := []testRow{
		{Name: "a", Value: 1},
		{Name: "b", Value: 2},
	}
	got, err := captureStdout(t, func() error {
		return output.Print(&rows, output.FormatTable)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Errorf("expected table with NAME header and rows a/b, got: %q", got)
	}
}

func TestPrint_Text_SliceWithUnmarshalableElement(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return output.Print([]any{make(chan int)}, output.FormatText)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// formatValue falls back to fmt.Sprintf for chan, which produces something like "0xc..."
	if strings.TrimSpace(got) == "" {
		t.Error("expected non-empty output for chan element")
	}
}

// --- PrintError ---

func TestPrintError_WithError(t *testing.T) {
	got := captureStderr(t, func() {
		output.PrintError("something went wrong", errors.New("underlying cause"))
	})
	var payload map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &payload); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\noutput: %q", err, got)
	}
	if payload["error"] != "something went wrong" {
		t.Errorf("error field: got %q, want %q", payload["error"], "something went wrong")
	}
	if payload["detail"] != "underlying cause" {
		t.Errorf("detail field: got %q, want %q", payload["detail"], "underlying cause")
	}
}

func TestPrintError_NilError(t *testing.T) {
	got := captureStderr(t, func() {
		output.PrintError("no detail", nil)
	})
	var payload map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &payload); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\noutput: %q", err, got)
	}
	if payload["error"] != "no detail" {
		t.Errorf("error field: got %q, want %q", payload["error"], "no detail")
	}
	if payload["detail"] != "" {
		t.Errorf("detail field: got %q, want empty string", payload["detail"])
	}
}
