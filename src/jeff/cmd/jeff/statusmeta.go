package main

import (
	"fmt"
	"strings"
)

func extractStatusLines(output string) []string {
	lines := strings.Split(output, "\n")
	var out []string
	started := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "OpenAI Codex") {
			continue
		}
		if line == "--------" {
			if started {
				break
			}
			started = true
			continue
		}
		if !started {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "user") || strings.HasPrefix(lower, "codex") {
			break
		}
		out = append(out, anonymizeSessionLine(line))
	}
	return out
}

func anonymizeSessionLine(line string) string {
	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, "session id") {
		return line
	}
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return line
	}
	value := strings.TrimSpace(parts[1])
	segments := strings.Split(value, "-")
	for i, seg := range segments {
		runes := []rune(seg)
		for j := 1; j < len(runes); j++ {
			runes[j] = '*'
		}
		segments[i] = string(runes)
	}
	return fmt.Sprintf("%s: %s", strings.TrimSpace(parts[0]), strings.Join(segments, "-"))
}
