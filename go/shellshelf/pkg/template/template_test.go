package template

import (
	"reflect"
	"testing"
)

func TestHasParams(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"no params", "echo hello", false},
		{"single param", "echo {{name}}", true},
		{"param with default", "echo {{name:world}}", true},
		{"multiple params", "ssh {{user}}@{{host}}", true},
		{"plain braces", "echo {not a param}", false},
		{"single brace", "echo {name}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasParams(tt.command); got != tt.want {
				t.Errorf("HasParams(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []Param
	}{
		{
			name:    "no params",
			command: "echo hello",
			want:    nil,
		},
		{
			name:    "single required param",
			command: "echo {{name}}",
			want: []Param{
				{Name: "name", Position: 0},
			},
		},
		{
			name:    "single param with default",
			command: "ssh {{user:root}}@localhost",
			want: []Param{
				{Name: "user", Default: "root", HasDefault: true, Position: 0},
			},
		},
		{
			name:    "multiple params",
			command: "ssh {{user:root}}@{{host}} -p {{port:22}}",
			want: []Param{
				{Name: "user", Default: "root", HasDefault: true, Position: 0},
				{Name: "host", Position: 1},
				{Name: "port", Default: "22", HasDefault: true, Position: 2},
			},
		},
		{
			name:    "duplicate param names uses first occurrence",
			command: "echo {{name}} and {{name}}",
			want: []Param{
				{Name: "name", Position: 0},
			},
		},
		{
			name:    "empty default value",
			command: "echo {{port:}}",
			want: []Param{
				{Name: "port", Default: "", HasDefault: true, Position: 0},
			},
		},
		{
			name:    "mixed required and optional",
			command: "curl -H 'Auth: {{token}}' {{url:http://localhost}}",
			want: []Param{
				{Name: "token", Position: 0},
				{Name: "url", Default: "http://localhost", HasDefault: true, Position: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.command)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) =\n  %+v\nwant\n  %+v", tt.command, got, tt.want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name    string
		command string
		values  map[string]string
		want    string
	}{
		{
			name:    "no params",
			command: "echo hello",
			values:  map[string]string{},
			want:    "echo hello",
		},
		{
			name:    "single param",
			command: "echo {{name}}",
			values:  map[string]string{"name": "world"},
			want:    "echo world",
		},
		{
			name:    "param with default replaced",
			command: "ssh {{user:root}}@{{host}}",
			values:  map[string]string{"user": "admin", "host": "prod-1"},
			want:    "ssh admin@prod-1",
		},
		{
			name:    "duplicate params all replaced",
			command: "echo {{name}} and {{name}}",
			values:  map[string]string{"name": "hello"},
			want:    "echo hello and hello",
		},
		{
			name:    "missing value keeps placeholder",
			command: "echo {{name}}",
			values:  map[string]string{},
			want:    "echo {{name}}",
		},
		{
			name:    "multiple params with defaults",
			command: "ssh {{user:root}}@{{host}} -p {{port:22}}",
			values:  map[string]string{"user": "deploy", "host": "web-1", "port": "2222"},
			want:    "ssh deploy@web-1 -p 2222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Render(tt.command, tt.values); got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParamNames(t *testing.T) {
	got := ParamNames("ssh {{user:root}}@{{host}} -p {{port:22}}")
	want := []string{"user", "host", "port"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParamNames() = %v, want %v", got, want)
	}
}

func TestResolveFromArgs(t *testing.T) {
	params := []Param{
		{Name: "user", Position: 0},
		{Name: "host", Position: 1},
	}

	t.Run("correct arg count", func(t *testing.T) {
		values, err := ResolveFromArgs(params, []string{"root", "prod-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := map[string]string{"user": "root", "host": "prod-1"}
		if !reflect.DeepEqual(values, want) {
			t.Errorf("got %v, want %v", values, want)
		}
	})

	t.Run("too few args", func(t *testing.T) {
		_, err := ResolveFromArgs(params, []string{"root"})
		if err == nil {
			t.Fatal("expected error for mismatched arg count")
		}
	})

	t.Run("too many args", func(t *testing.T) {
		_, err := ResolveFromArgs(params, []string{"root", "prod-1", "extra"})
		if err == nil {
			t.Fatal("expected error for mismatched arg count")
		}
	})
}
