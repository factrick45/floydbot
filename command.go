package main

import (
	"strings"
)

func ComParse(message string, command string) ([]string, bool) {
	if !strings.HasPrefix(message, command) {
		return nil, false
	}
	return strings.Split(message, " "), true
}
