package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkdownLinkFilter(t *testing.T) {
	tests := []struct {
		name     string
		chunks   []string
		expected []string
	}{
		{
			name: "simple text without links",
			chunks: []string{
				"**hi there**, ",
				"no links here, ",
				"and here",
				"",
			},
			expected: []string{
				"**hi there**, ",
				"no links here, ",
				"and here",
				"",
			},
		},
		{
			name: "text with never ending link",
			chunks: []string{
				"**hi there**, ",
				"no links here, [",
				"and here",
				"",
			},
			expected: []string{
				"**hi there**, ",
				"",
				"",
				"no links here, [and here",
			},
		},
		{
			name: "text with never ending link",
			chunks: []string{
				"**hi there**, ",
				"no links here, [",
				"and here [",
				"",
			},
			expected: []string{
				"**hi there**, ",
				"",
				"no links here, [",
				"and here [",
			},
		},
		{
			name: "not a link just [] brackets",
			chunks: []string{
				"**hi there**, ",
				"[no links here], ",
				"and here",
				"",
			},
			expected: []string{
				"**hi there**, ",
				"",
				"",
				"[no links here], and here",
			},
		},
		{
			name: "not a link just [) brackets",
			chunks: []string{
				"**hi there**, ",
				"[no links here), ",
				"and here",
				"",
			},
			expected: []string{
				"**hi there**, ",
				"",
				"",
				"[no links here), and here",
			},
		},
		{
			name: "link with text multiple chunks",
			chunks: []string{
				"**hi there**, ",
				"[no links here](https://",
				"example.com), ",
				"and here",
				"",
			},
			expected: []string{
				"**hi there**, ",
				"",
				", ",
				"and here", "",
			},
		},
		{
			name: "link with text one chunks",
			chunks: []string{
				"**hi there**, ",
				"[no links here](https://example.com), ",
				"and here",
				"",
			},
			expected: []string{
				"**hi there**, ",
				", ",
				"and here",
				"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &ExternalLinkFilter{}
			var result []string
			for _, chunk := range tt.chunks {
				result = append(result, filter.ProcessChunk(chunk))
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}
