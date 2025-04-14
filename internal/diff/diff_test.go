// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type addr struct {
	Line string `json:"line"`
	City string `json:"city"`
}

type contact struct {
	Name    string `json:"name"`
	Address addr   `json:"address"`
}

func TestBasicDiffs(t *testing.T) {
	a := assert.New(t)
	left := contact{
		Name: "John Doe",
		Address: addr{
			Line: "1st st",
			City: "San Jose",
		},
	}
	opts := Options{}
	right := left
	out, err := Objects(left, right, opts)
	require.Nil(t, err)
	a.Equal("", string(out))

	right.Name = "Jane Doe"
	right.Address.City = "San Francisco"
	out, err = Objects(left, right, opts)
	require.Nil(t, err)
	outStr := string(out)
	a.Contains(outStr, "address:\n")
	a.Contains(outStr, "  line: 1st st\n")
	a.Contains(outStr, "-  city: San Jose\n")
	a.Contains(outStr, "+  city: San Francisco\n")
	a.Contains(outStr, "-name: John Doe\n")
	a.Contains(outStr, "+name: Jane Doe\n")

	out, err = Objects(left, nil, opts)
	require.Nil(t, err)
	outStr = string(out)
	a.Contains(outStr, "-name: John Doe\n")
	a.Contains(outStr, "-address:\n")
	a.Contains(outStr, "-  city: San Jose\n")
	a.Contains(outStr, "-  line: 1st st\n")

	out, err = Objects([]byte{}, right, opts)
	require.Nil(t, err)
	outStr = string(out)
	a.Contains(outStr, "+name: Jane Doe\n")
	a.Contains(outStr, "+address:\n")
	a.Contains(outStr, "+  city: San Francisco\n")
	a.Contains(outStr, "+  line: 1st st\n")
}

func TestContextLines(t *testing.T) {
	a := assert.New(t)
	left := []contact{
		{
			Name: "John Doe",
			Address: addr{
				Line: "1st st",
				City: "San Jose",
			},
		},
		{
			Name: "Jane Doe",
			Address: addr{
				Line: "1st st",
				City: "San Francisco",
			},
		},
	}
	right := []contact{left[0], left[1]}
	right[1].Address.Line = "2nd st"

	out, err := Objects(left, right, Options{Context: 2})
	require.Nil(t, err)
	outStr := string(out)
	a.NotContains(outStr, "John Doe")
	a.Contains(outStr, "- address:\n")
	a.Contains(outStr, "  city: San Francisco\n")
	a.Contains(outStr, "-    line: 1st st\n")
	a.Contains(outStr, "+    line: 2nd st\n")
	a.Contains(outStr, "  city: San Francisco\n")
}

func TestColorize(t *testing.T) {
	a := assert.New(t)
	left := contact{
		Name: "John Doe",
		Address: addr{
			Line: "1st st",
			City: "San Jose",
		},
	}
	opts := Options{Colorize: true}
	right := left
	right.Address.Line = "2nd st"
	out, err := Objects(left, right, opts)
	require.Nil(t, err)
	outStr := string(out)
	a.Contains(outStr, escRed+"-  line: 1st st\n"+escReset)
	a.Contains(outStr, escGreen+"+  line: 2nd st\n"+escReset)
}
