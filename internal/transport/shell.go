package transport

import "strings"

// ShellQuote preserves a single argument when a command crosses a POSIX shell.
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func ShellCommand(command []string) string {
	args := make([]string, len(command))
	for i, arg := range command {
		args[i] = ShellQuote(arg)
	}
	return strings.Join(args, " ")
}
