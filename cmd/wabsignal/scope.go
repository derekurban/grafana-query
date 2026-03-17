package wabsignal

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/derekurban/wabii-signal/internal/cfg"
)

func applyProjectScope(signal, expr string, project *cfg.Project, noProjectScope bool) string {
	if noProjectScope || project == nil {
		return strings.TrimSpace(expr)
	}
	services := project.AllServices()

	scoped := strings.TrimSpace(expr)
	switch signal {
	case "logs":
		if len(services) > 0 {
			scoped = injectLogQLScope(scoped, project.QueryScope.LogServiceLabel, services)
		}
		return scoped
	case "metrics":
		if len(services) > 0 {
			scoped = injectPromQLScope(scoped, project.QueryScope.MetricServiceLabel, services)
		}
		return scoped
	case "traces":
		if len(services) > 0 {
			scoped = injectTraceQLScope(scoped, project.QueryScope.TraceServiceAttr, services)
		}
		return scoped
	default:
		return scoped
	}
}

func injectLogQLScope(expr, label string, services []string) string {
	op, value := matcherForServices(services)
	matcher := fmt.Sprintf(`%s%s"%s"`, label, op, escapeQueryValue(value))
	if strings.TrimSpace(expr) == "" {
		return "{" + matcher + "}"
	}
	if scoped, ok := injectLabelMatcher(expr, label, matcher); ok {
		return scoped
	}
	if strings.HasPrefix(strings.TrimSpace(expr), "|") {
		return "{" + matcher + "} " + strings.TrimSpace(expr)
	}
	return strings.TrimSpace(expr)
}

func injectPromQLScope(expr, label string, services []string) string {
	op, value := matcherForServices(services)
	matcher := fmt.Sprintf(`%s%s"%s"`, label, op, escapeQueryValue(value))
	if strings.TrimSpace(expr) == "" {
		return "{" + matcher + "}"
	}
	if scoped, ok := injectLabelMatcher(expr, label, matcher); ok {
		return scoped
	}
	return injectPromMatcherWithoutSelector(expr, matcher)
}

func injectSingleLabelScope(expr, label, value string) string {
	matcher := fmt.Sprintf(`%s="%s"`, label, escapeQueryValue(value))
	if scoped, ok := injectLabelMatcher(expr, label, matcher); ok {
		return scoped
	}
	if strings.TrimSpace(expr) == "" {
		return "{" + matcher + "}"
	}
	if strings.HasPrefix(strings.TrimSpace(expr), "|") {
		return "{" + matcher + "} " + strings.TrimSpace(expr)
	}
	return strings.TrimSpace(expr)
}

func injectSinglePromScope(expr, label, value string) string {
	matcher := fmt.Sprintf(`%s="%s"`, label, escapeQueryValue(value))
	if scoped, ok := injectLabelMatcher(expr, label, matcher); ok {
		return scoped
	}
	return injectPromMatcherWithoutSelector(expr, matcher)
}

func injectTraceQLScope(expr, attr string, services []string) string {
	op, value := matcherForServices(services)
	matcher := fmt.Sprintf(`%s %s "%s"`, attr, op, escapeQueryValue(value))
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return "{ " + matcher + " }"
	}
	if scoped, ok := injectTraceMatcher(expr, attr, matcher); ok {
		return scoped
	}
	return "{ " + matcher + " } && (" + trimmed + ")"
}

func injectSingleTraceScope(expr, attr, value string) string {
	matcher := fmt.Sprintf(`%s = "%s"`, attr, escapeQueryValue(value))
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return "{ " + matcher + " }"
	}
	if scoped, ok := injectTraceMatcher(expr, attr, matcher); ok {
		return scoped
	}
	return "{ " + matcher + " } && (" + trimmed + ")"
}

func injectLabelMatcher(expr, label, matcher string) (string, bool) {
	start := strings.Index(expr, "{")
	if start < 0 {
		return expr, false
	}
	end := strings.Index(expr[start:], "}")
	if end < 0 {
		return expr, false
	}
	end += start

	selector := strings.TrimSpace(expr[start+1 : end])
	if selectorHasLabel(selector, label) {
		return expr, true
	}

	if selector == "" {
		selector = matcher
	} else {
		selector = selector + ", " + matcher
	}
	return expr[:start+1] + selector + expr[end:], true
}

func injectTraceMatcher(expr, attr, matcher string) (string, bool) {
	start := strings.Index(expr, "{")
	if start < 0 {
		return expr, false
	}
	end := strings.Index(expr[start:], "}")
	if end < 0 {
		return expr, false
	}
	end += start

	selector := strings.TrimSpace(expr[start+1 : end])
	if selectorHasTraceAttr(selector, attr) {
		return expr, true
	}
	if selector == "" {
		selector = matcher
	} else {
		selector = selector + " && " + matcher
	}
	return expr[:start+1] + " " + selector + " " + expr[end:], true
}

func injectPromMatcherWithoutSelector(expr, matcher string) string {
	const identChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_:"
	keywords := map[string]bool{
		"and":         true,
		"bool":        true,
		"by":          true,
		"group_left":  true,
		"group_right": true,
		"ignoring":    true,
		"offset":      true,
		"on":          true,
		"or":          true,
		"unless":      true,
		"without":     true,
	}

	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_", rune(ch)) {
			continue
		}
		if i > 0 && strings.ContainsRune(identChars, rune(expr[i-1])) {
			continue
		}
		j := i + 1
		for j < len(expr) && strings.ContainsRune(identChars, rune(expr[j])) {
			j++
		}
		ident := expr[i:j]
		if keywords[strings.ToLower(ident)] {
			i = j - 1
			continue
		}
		if j < len(expr) && expr[j] == '(' {
			i = j - 1
			continue
		}
		if j < len(expr) && expr[j] == '{' {
			if scoped, ok := injectLabelMatcher(expr, "", matcher); ok {
				return scoped
			}
			return expr
		}
		if j == len(expr) || isPromBoundary(expr[j]) || expr[j] == '[' {
			return expr[:j] + "{" + matcher + "}" + expr[j:]
		}
		i = j - 1
	}
	return strings.TrimSpace(expr)
}

func isPromBoundary(ch byte) bool {
	return strings.ContainsRune(" \t\n\r+-*/%^,)=><", rune(ch))
}

func selectorHasLabel(selector, label string) bool {
	if strings.TrimSpace(label) == "" {
		return false
	}
	for _, part := range strings.Split(selector, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if index := strings.IndexAny(part, "=!~"); index > 0 && strings.TrimSpace(part[:index]) == label {
			return true
		}
	}
	return false
}

func selectorHasTraceAttr(selector, attr string) bool {
	if strings.TrimSpace(attr) == "" {
		return false
	}
	return strings.Contains(selector, attr)
}

func matcherForServices(services []string) (string, string) {
	if len(services) == 1 {
		return "=", services[0]
	}
	parts := make([]string, 0, len(services))
	for _, service := range services {
		parts = append(parts, regexp.QuoteMeta(strings.TrimSpace(service)))
	}
	return "=~", strings.Join(parts, "|")
}

func escapeQueryValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(strings.TrimSpace(value))
}
