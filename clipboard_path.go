package main

import "strings"

// shellMeta lists the characters a shell reads as more than one word, so a
// path holding one of them has to be quoted before it is pasted.
const shellMeta = " \t\n'\"\\$`!*?()[]{}#~&;|<>"

// shellQuotedPath quotes a path for a shell, so joinedPaths' lines each paste
// as one word. A path needing nothing escaped is returned as it is.
func shellQuotedPath(path string) string {
	if path != "" && !strings.ContainsAny(path, shellMeta) {
		return path
	}
	var b strings.Builder
	b.Grow(len(path) + 2)
	b.WriteByte('\'')
	for _, r := range path {
		if r == '\'' {
			// A single quote cannot hold one: end the run, escape it, and
			// open the run again.
			b.WriteString(`'\''`)
			continue
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

// joinedPaths renders paths as the text "Copy Path" writes to the pasteboard:
// one absolute path per line, escaped so a shell takes each line as one word.
func joinedPaths(paths []string) string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, shellQuotedPath(path))
	}
	return strings.Join(out, "\n")
}
