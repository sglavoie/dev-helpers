package buildcmd

import "strings"

func wrapLongLinesWithBackslashes(sb *strings.Builder) {
	chunks := splitIntoChunks(sb.String(), 80)
	sb.Reset()
	for i, chunk := range chunks {
		sb.WriteString(chunk)
		if i < len(chunks)-1 {
			sb.WriteString(" \\\n")
		}
	}
}

func splitIntoChunks(s string, chunkSize int) []string {
	var chunks []string
	for len(s) > chunkSize {
		i := strings.LastIndex(s[:chunkSize], " ")
		if i == -1 {
			i = chunkSize
		}
		chunks = append(chunks, s[:i])
		s = s[i:]
	}
	chunks = append(chunks, s)
	return chunks
}
