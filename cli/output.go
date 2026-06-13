package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"
)

// Format is an output encoding.
type Format string

const (
	FormatAuto  Format = "auto"
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatJSONL Format = "jsonl"
	FormatCSV   Format = "csv"
	FormatTSV   Format = "tsv"
	FormatYAML  Format = "yaml"
	FormatURL   Format = "url"
	FormatRaw   Format = "raw"
)

// Output renders records in a chosen format. It streams where the format allows.
type Output struct {
	format   Format
	fields   []string
	noHeader bool
	tmpl     *template.Template
	w        io.Writer

	tw         *tabwriter.Writer
	csvw       *csv.Writer
	cols       []string
	headerDone bool
	jsonOpen   bool
	jsonFirst  bool
	count      int

	// suppress drops every record (used by --dry-run, which only prints the
	// requests that would be made).
	suppress bool
}

// NewOutput builds an Output. format "auto" should already be resolved by caller.
func NewOutput(w io.Writer, format Format, fields []string, noHeader bool, tmpl string) (*Output, error) {
	o := &Output{format: format, fields: fields, noHeader: noHeader, w: w, jsonFirst: true}
	if tmpl != "" {
		t, err := template.New("row").Parse(tmpl)
		if err != nil {
			return nil, fmt.Errorf("bad template: %w", err)
		}
		o.tmpl = t
		o.format = FormatRaw
	}
	switch o.format {
	case FormatTable:
		o.tw = tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	case FormatCSV:
		o.csvw = csv.NewWriter(w)
	case FormatTSV:
		o.csvw = csv.NewWriter(w)
		o.csvw.Comma = '\t'
	}
	return o, nil
}

// Emit renders one record.
func (o *Output) Emit(v any) error {
	if o.suppress {
		return nil
	}
	o.count++
	switch o.format {
	case FormatJSON:
		if !o.jsonOpen {
			_, _ = io.WriteString(o.w, "[")
			o.jsonOpen = true
		}
		if !o.jsonFirst {
			_, _ = io.WriteString(o.w, ",")
		}
		o.jsonFirst = false
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(o.w, "\n  ")
		_, _ = o.w.Write(b)
		return nil
	case FormatJSONL:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, _ = o.w.Write(b)
		_, _ = io.WriteString(o.w, "\n")
		return nil
	case FormatRaw:
		if o.tmpl != nil {
			m := toMap(v)
			if err := o.tmpl.Execute(o.w, m); err != nil {
				return err
			}
			_, _ = io.WriteString(o.w, "\n")
			return nil
		}
		b, _ := json.MarshalIndent(v, "", "  ")
		_, _ = o.w.Write(b)
		_, _ = io.WriteString(o.w, "\n")
		return nil
	case FormatYAML:
		return o.emitYAML(v)
	case FormatURL:
		return o.emitURL(v)
	case FormatCSV, FormatTSV:
		return o.emitCSV(v)
	default: // table
		return o.emitTable(v)
	}
}

func (o *Output) columnsFor(v any) ([]string, map[string]string) {
	keys, vals := flatten(v)
	if len(o.fields) > 0 {
		return o.fields, vals
	}
	return keys, vals
}

func (o *Output) emitTable(v any) error {
	cols, vals := o.columnsFor(v)
	if !o.headerDone {
		o.cols = cols
		if !o.noHeader {
			_, _ = fmt.Fprintln(o.tw, strings.Join(cols, "\t"))
		}
		o.headerDone = true
	}
	row := make([]string, len(o.cols))
	for i, c := range o.cols {
		row[i] = oneLine(vals[c])
	}
	_, _ = fmt.Fprintln(o.tw, strings.Join(row, "\t"))
	return nil
}

func (o *Output) emitCSV(v any) error {
	cols, vals := o.columnsFor(v)
	if !o.headerDone {
		o.cols = cols
		if !o.noHeader {
			_ = o.csvw.Write(cols)
		}
		o.headerDone = true
	}
	row := make([]string, len(o.cols))
	for i, c := range o.cols {
		row[i] = vals[c]
	}
	return o.csvw.Write(row)
}

func (o *Output) emitYAML(v any) error {
	cols, vals := o.columnsFor(v)
	_, _ = fmt.Fprintf(o.w, "- ")
	first := true
	for _, c := range cols {
		if !first {
			_, _ = fmt.Fprintf(o.w, "  ")
		}
		first = false
		_, _ = fmt.Fprintf(o.w, "%s: %s\n", c, yamlVal(vals[c]))
	}
	return nil
}

func (o *Output) emitURL(v any) error {
	_, vals := flatten(v)
	for _, k := range []string{"url", "note_id", "user_id", "comment_id", "link", "id", "name"} {
		if s, ok := vals[k]; ok && s != "" {
			_, _ = fmt.Fprintln(o.w, s)
			return nil
		}
	}
	return nil
}

// Close flushes any buffered output.
func (o *Output) Close() error {
	if o.suppress {
		return nil
	}
	switch o.format {
	case FormatTable:
		return o.tw.Flush()
	case FormatCSV, FormatTSV:
		o.csvw.Flush()
		return o.csvw.Error()
	case FormatJSON:
		if !o.jsonOpen {
			_, _ = io.WriteString(o.w, "[]\n")
		} else {
			_, _ = io.WriteString(o.w, "\n]\n")
		}
	}
	return nil
}

// flatten returns ordered json keys and their string values for a struct.
func flatten(v any) ([]string, map[string]string) {
	vals := map[string]string{}
	var keys []string
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		// fall back to map via json
		m := toMap(v)
		for k, val := range m {
			keys = append(keys, k)
			vals[k] = scalarStr(val)
		}
		return keys, vals
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = f.Name
		}
		if name == "-" {
			continue
		}
		keys = append(keys, name)
		vals[name] = scalarStr(rv.Field(i).Interface())
	}
	return keys, vals
}

func scalarStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", t)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Len() == 0 {
			return ""
		}
		parts := make([]string, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			parts[i] = scalarStr(rv.Index(i).Interface())
		}
		return strings.Join(parts, "; ")
	case reflect.Struct, reflect.Map:
		b, _ := json.Marshal(v)
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len([]rune(s)) > 80 {
		r := []rune(s)
		return string(r[:79]) + "…"
	}
	return s
}

func yamlVal(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#\n\"'") {
		return strconv.Quote(s)
	}
	return s
}

func toMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	return m
}
