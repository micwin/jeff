package main

import "strings"

func joinArgs(args []string) string {
	return strings.TrimSpace(strings.Join(args, " "))
}
