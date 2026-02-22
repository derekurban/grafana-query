package util

import (
	"sort"
	"strings"
)

func ExpandAlias(expr string, aliases map[string]string) string {
	expr = strings.TrimSpace(expr)
	if !strings.HasPrefix(expr, "@") {
		return expr
	}
	key := strings.TrimPrefix(expr, "@")
	if v, ok := aliases[key]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return expr
}

func InjectDefaultLabels(expr string, labels map[string]string) string {
	expr = strings.TrimSpace(expr)
	if len(labels) == 0 || !strings.HasPrefix(expr, "{") {
		return expr
	}
	close := strings.Index(expr, "}")
	if close < 1 {
		return expr
	}

	selector := expr[1:close]
	tail := expr[close+1:]

	present := map[string]bool{}
	for _, p := range strings.Split(selector, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i := strings.IndexAny(p, "=!~"); i > 0 {
			present[strings.TrimSpace(p[:i])] = true
		}
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		if !present[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return expr
	}

	parts := []string{}
	if strings.TrimSpace(selector) != "" {
		parts = append(parts, strings.TrimSpace(selector))
	}
	for _, k := range keys {
		v := strings.ReplaceAll(labels[k], `"`, `\"`)
		parts = append(parts, k+`="`+v+`"`)
	}
	return "{" + strings.Join(parts, ", ") + "}" + tail
}
