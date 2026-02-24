package cmd

import "testing"

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tags removed",
			input: "<p>Hello <b>world</b></p>",
			want:  "Hello world",
		},
		{
			name:  "entities unescaped",
			input: "rock &amp; roll",
			want:  "rock & roll",
		},
		{
			name:  "tags and entities combined",
			input: "<p>A &gt; B &amp; C &lt; D</p>",
			want:  "A > B & C < D",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "nested tags",
			input: "<div><p><span>deep</span></p></div>",
			want:  "deep",
		},
		{
			name:  "self-closing tags",
			input: "line1<br/>line2",
			want:  "line1line2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
