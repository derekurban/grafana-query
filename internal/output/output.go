package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

func PrintJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func PrintRaw(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func PrintTable(rows []map[string]any, preferred []string) {
	if len(rows) == 0 {
		fmt.Println("(no rows)")
		return
	}
	cols := columns(rows, preferred)
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(cols, "\t"))
	for _, r := range rows {
		vals := make([]string, 0, len(cols))
		for _, c := range cols {
			vals = append(vals, fmt.Sprint(r[c]))
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}
	_ = w.Flush()
}

func PrintCSV(rows []map[string]any, preferred []string) error {
	if len(rows) == 0 {
		return nil
	}
	cols := columns(rows, preferred)
	w := csv.NewWriter(os.Stdout)
	if err := w.Write(cols); err != nil {
		return err
	}
	for _, r := range rows {
		rec := make([]string, 0, len(cols))
		for _, c := range cols {
			rec = append(rec, fmt.Sprint(r[c]))
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func columns(rows []map[string]any, preferred []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, p := range preferred {
		if p == "" || seen[p] {
			continue
		}
		for _, r := range rows {
			if _, ok := r[p]; ok {
				seen[p] = true
				out = append(out, p)
				break
			}
		}
	}
	extra := []string{}
	for _, r := range rows {
		for k := range r {
			if !seen[k] {
				extra = append(extra, k)
				seen[k] = true
			}
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}
