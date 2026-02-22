package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ResolveGrafanaRange(since, from, to string) (string, string, error) {
	since = strings.TrimSpace(since)
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)

	if from != "" || to != "" {
		if from == "" || to == "" {
			return "", "", fmt.Errorf("both --from and --to are required when either is set")
		}
		f, err := normalizeTimeInput(from)
		if err != nil {
			return "", "", fmt.Errorf("invalid --from: %w", err)
		}
		tv, err := normalizeTimeInput(to)
		if err != nil {
			return "", "", fmt.Errorf("invalid --to: %w", err)
		}
		return f, tv, nil
	}

	if since == "" {
		since = "1h"
	}
	if strings.HasPrefix(since, "now-") {
		return since, "now", nil
	}
	return "now-" + since, "now", nil
}

func normalizeTimeInput(v string) (string, error) {
	if strings.HasPrefix(v, "now") {
		return v, nil
	}
	// unix seconds or ms
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		return v, nil
	}
	formats := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, v); err == nil {
			return strconv.FormatInt(t.UnixMilli(), 10), nil
		}
	}
	return "", fmt.Errorf("unsupported time format")
}
