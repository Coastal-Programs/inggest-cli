package output

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
)

type Format string

const (
	FormatJSON  Format = "json"
	FormatText  Format = "text"
	FormatTable Format = "table"
)

// Print renders data in the requested format.
func Print(data any, format Format) error {
	switch format {
	case FormatText:
		return printText(data)
	case FormatTable:
		return printTable(data)
	default:
		return printJSON(data)
	}
}

// PrintError writes a JSON-encoded error to stderr.
func PrintError(msg string, err error) {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	payload := map[string]string{"error": msg, "detail": detail}
	b, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Fprintln(os.Stderr, string(b))
}

func printJSON(data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func printText(data any) error {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Slice:
		for i := range v.Len() {
			fmt.Println(formatValue(v.Index(i).Interface()))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			fmt.Printf("%s: %v\n", k, v.MapIndex(k))
		}
	case reflect.Struct:
		t := v.Type()
		for i := range v.NumField() {
			fmt.Printf("%s: %v\n", t.Field(i).Name, v.Field(i).Interface())
		}
	default:
		fmt.Println(formatValue(data))
	}
	return nil
}

func printTable(data any) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice || v.Len() == 0 {
		return printText(data)
	}
	elem := v.Index(0)
	if elem.Kind() == reflect.Ptr {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return printText(data)
	}
	t := elem.Type()
	headers := make([]string, t.NumField())
	for i := range t.NumField() {
		headers[i] = strings.ToUpper(t.Field(i).Name)
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for i := range v.Len() {
		row := v.Index(i)
		if row.Kind() == reflect.Ptr {
			row = row.Elem()
		}
		cells := make([]string, row.NumField())
		for j := range row.NumField() {
			cells[j] = fmt.Sprintf("%v", row.Field(j).Interface())
		}
		fmt.Fprintln(w, strings.Join(cells, "\t"))
	}
	return w.Flush()
}

func formatValue(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
