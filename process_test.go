package matchguru

import (
	"testing"
)

func TestProcess(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Premier League England",
			input:    `<l fullname="Premier League England">Premier</l>`,
			expected: `<a href="/league/609">Premier</a>`,
		},
		{
			name:     "EFL Championship England",
			input:    `<l fullname="EFL Championship England">Champ</l>`,
			expected: `<a href="/league/9">Champ</a>`,
		},
		{
			name:     "FA Cup",
			input:    `<l fullname="FA Cup">FA</l>`,
			expected: `<a href="/league/24">FA</a>`,
		},
		{
			name:     "Nonexistent League",
			input:    `<l fullname="Nonexistent League">Nonexistent</l>`,
			expected: `Nonexistent`,
		},
		{
			name:     "Bundesliga",
			input:    `<l fullname="Bundesliga">Bundes</l>`,
			expected: `<a href="/league/82">Bundes</a>`,
		},
		{
			name:     "Serie A Italy",
			input:    `<l fullname="Serie A Italy">Serie A</l>`,
			expected: `<a href="/league/384">Serie A</a>`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := process(test.input)
			if output != test.expected {
				t.Errorf("process(%q) = %q; want %q", test.input, output, test.expected)
			}
		})
	}
}
