package strip

import (
	"testing"
)

func TestStrip(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// --- Comment stripping ---
		{
			name:  "line comment at EOL",
			input: `{"a": 1} // comment`,
			want:  `{"a": 1} `,
		},
		{
			name:  "line comment on own line",
			input: "// comment\n{\"a\": 1}",
			want:  "\n{\"a\": 1}",
		},
		{
			name:  "inline line comment preserves newline",
			input: "{\"a\": 1, // end of line\n\"b\": 2}",
			want:  "{\"a\": 1, \n\"b\": 2}",
		},
		{
			name:  "inline block comment",
			input: `{"a": /* inline */ 1}`,
			want:  `{"a":  1}`,
		},
		{
			name:  "multi-line block comment",
			input: "{\"a\": 1,\n/* multi\nline\n*/\n\"b\": 2}",
			want:  "{\"a\": 1,\n\n\"b\": 2}",
		},
		{
			name:  "block comment with many stars",
			input: `{"a": /****** stars ******/ 1}`,
			want:  `{"a":  1}`,
		},
		{
			name:    "unterminated block comment",
			input:   `{"a": /* unterminated`,
			wantErr: true,
		},
		{
			name:    "unterminated block comment star state",
			input:   `{"a": /*`,
			wantErr: true,
		},

		// --- Trailing commas ---
		{
			name:  "trailing comma before closing brace",
			input: `{"a": 1,}`,
			want:  `{"a": 1}`,
		},
		{
			name:  "trailing comma before closing bracket",
			input: `[1, 2, 3,]`,
			want:  `[1, 2, 3]`,
		},
		{
			name:  "trailing comma with whitespace",
			input: "{\"a\": 1,\n}",
			want:  "{\"a\": 1\n}",
		},
		{
			name:  "trailing comma with multiple spaces",
			input: `{"a": 1,   }`,
			want:  `{"a": 1   }`,
		},
		{
			name:  "nested structures trailing commas",
			input: `{"a": [1, 2,], "b": {"c": 3,},}`,
			want:  `{"a": [1, 2], "b": {"c": 3}}`,
		},
		{
			name:  "no trailing commas pass-through",
			input: `{"a": 1, "b": [2, 3]}`,
			want:  `{"a": 1, "b": [2, 3]}`,
		},

		// --- String preservation ---
		{
			name:  "line comment syntax inside string",
			input: `{"url": "https://example.com"}`,
			want:  `{"url": "https://example.com"}`,
		},
		{
			name:  "block comment syntax inside string",
			input: `{"s": "/* not a comment */"}`,
			want:  `{"s": "/* not a comment */"}`,
		},
		{
			name:  "trailing comma inside string",
			input: `{"s": "a,"}`,
			want:  `{"s": "a,"}`,
		},
		{
			name:  "escaped quote inside string",
			input: `{"s": "he said \"hi\""}`,
			want:  `{"s": "he said \"hi\""}`,
		},
		{
			name:  "escaped backslash before closing quote",
			input: `{"s": "\\"}`,
			want:  `{"s": "\\"}`,
		},
		{
			name:  "URL with double slash in string",
			input: `{"url": "http://example.com/path"}`,
			want:  `{"url": "http://example.com/path"}`,
		},

		// --- Error cases ---
		{
			name:    "unterminated string",
			input:   `{"a": "unterminated`,
			wantErr: true,
		},
		{
			name:    "unterminated string after escape",
			input:   "{\"a\": \"esc\\",
			wantErr: true,
		},

		// --- Integration ---
		{
			name: "full JSONC document",
			input: `{
  // top-level comment
  "name": "test", /* inline */
  "values": [
    1,
    2, // trailing
    3,
  ],
  "nested": {
    "x": 1,
  },
}`,
			// Leading spaces before // are preserved; spaces before /* */ are preserved.
			want: "{\n  \n  \"name\": \"test\", \n  \"values\": [\n    1,\n    2, \n    3\n  ],\n  \"nested\": {\n    \"x\": 1\n  }\n}",
		},
		{
			name:  "empty input",
			input: ``,
			want:  ``,
		},
		{
			name:  "valid JSON pass-through",
			input: `{"a": 1, "b": [2, 3]}`,
			want:  `{"a": 1, "b": [2, 3]}`,
		},
		{
			name:  "lone slash at EOF",
			input: `{"a": 1}/`,
			want:  `{"a": 1}/`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Strip([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Strip() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("Strip() mismatch:\ninput: %q\ngot:   %q\nwant:  %q", tt.input, string(got), tt.want)
			}
		})
	}
}
