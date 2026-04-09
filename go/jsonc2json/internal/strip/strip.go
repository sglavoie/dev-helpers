package strip

import (
	"errors"
	"regexp"
)

type state int

const (
	stateDefault state = iota
	stateSlash
	stateLineComment
	stateBlockComment
	stateBlockCommentStar
	stateString
	stateStringEscape
)

var trailingCommaRe = regexp.MustCompile(`,(\s*[}\]])`)

// Strip removes JSONC comments and trailing commas from input, returning valid JSON.
func Strip(input []byte) ([]byte, error) {
	out := make([]byte, 0, len(input))
	s := stateDefault

	for _, b := range input {
		switch s {
		case stateDefault:
			switch b {
			case '"':
				out = append(out, b)
				s = stateString
			case '/':
				s = stateSlash
			default:
				out = append(out, b)
			}

		case stateSlash:
			switch b {
			case '/':
				s = stateLineComment
			case '*':
				s = stateBlockComment
			default:
				out = append(out, '/', b)
				s = stateDefault
			}

		case stateLineComment:
			if b == '\n' {
				out = append(out, '\n')
				s = stateDefault
			}

		case stateBlockComment:
			if b == '*' {
				s = stateBlockCommentStar
			}

		case stateBlockCommentStar:
			switch b {
			case '/':
				s = stateDefault
			case '*':
				// stay — handles sequences like /****/
			default:
				s = stateBlockComment
			}

		case stateString:
			out = append(out, b)
			switch b {
			case '\\':
				s = stateStringEscape
			case '"':
				s = stateDefault
			}

		case stateStringEscape:
			out = append(out, b)
			s = stateString
		}
	}

	// Check for unterminated constructs at EOF.
	switch s {
	case stateBlockComment, stateBlockCommentStar:
		return nil, errors.New("unterminated block comment")
	case stateString, stateStringEscape:
		return nil, errors.New("unterminated string")
	case stateSlash:
		out = append(out, '/')
	}

	// Pass 2: remove trailing commas before ] and }.
	out = trailingCommaRe.ReplaceAll(out, []byte("$1"))

	return out, nil
}
