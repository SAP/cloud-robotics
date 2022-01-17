package coretools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomString(t *testing.T) {
	type testCase struct {
		name string
		n    int
	}

	tests := []testCase{
		{
			name: "OneCharacter",
			n:    1,
		},
		{
			name: "TwoCharacters",
			n:    2,
		},
		{
			name: "FiveCharacters",
			n:    5,
		},
		{
			name: "TenCharacters",
			n:    10,
		},
		{
			name: "HundredCharacters",
			n:    100,
		},
		{
			name: "8bitsCharacters",
			n:    256,
		},
		{
			name: "ThousandCharacters",
			n:    1000,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				for i := 0; i < 100000; i++ {
					assert.Equal(t, test.n, len(RandomString(test.n)))
				}
			},
		)
	}

}
