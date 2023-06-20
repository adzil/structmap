/*
Copyright 2023 Fadhli Dzil Ikram.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package structmap_test

import (
	"testing"

	"github.com/adzil/structmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	t.Run("WithoutPointer", func(t *testing.T) {
		var empty struct{}

		err := structmap.Unmarshal(nil, empty)
		assert.ErrorContains(t, err, "pointer")
	})

	t.Run("WithoutStruct", func(t *testing.T) {
		var empty string

		err := structmap.Unmarshal(nil, &empty)
		assert.ErrorContains(t, err, "cannot unmarshal into string")
	})

	t.Run("WithInvalidStructOption", func(t *testing.T) {
		type emptyStruct struct {
			Nested struct{} `map:",required"`
		}

		var empty emptyStruct

		err := structmap.Unmarshal(nil, &empty)
		assert.ErrorContains(t, err, "required")
	})

	t.Run("WithInvalidFieldOption", func(t *testing.T) {
		type emptyStruct struct {
			Field string `map:",unknownopt"`
		}

		var empty emptyStruct

		err := structmap.Unmarshal(nil, &empty)
		assert.ErrorContains(t, err, "unknownopt")
	})

	t.Run("WithUnknownType", func(t *testing.T) {
		type emptyStruct struct {
			Float64 float64
		}

		var empty *emptyStruct

		err := structmap.Unmarshal(nil, &empty)
		assert.ErrorContains(t, err, "cannot unmarshal into float64")
	})

	t.Run("WithUnknownSliceType", func(t *testing.T) {
		type emptyStruct struct {
			Float64 []float64
		}

		var empty emptyStruct

		err := structmap.Unmarshal(nil, &empty)
		assert.ErrorContains(t, err, "cannot unmarshal into slice of float64")
	})

	t.Run("WithNestedPointer", func(t *testing.T) {
		type emptyStruct struct {
			Field string
		}

		var empty *emptyStruct

		err := structmap.Unmarshal(nil, &empty)
		assert.NoError(t, err)
	})

	t.Run("WithFieldNames", func(t *testing.T) {
		type testStruct struct {
			Ignored    string `map:"-"`
			NotIgnored string `map:"-,"`
			Message    string `map:"message"`
		}

		expected := testStruct{
			Ignored:    "valueHere",
			NotIgnored: "valueThere",
			Message:    "itsHere",
		}

		input := map[string][]string{
			"Ignored": {"valueThere"},
			"-":       {"valueThere"},
			"Message": {"itsThere"},
			"message": {"itsHere"},
		}

		actual := testStruct{
			Ignored:    "valueHere",
			NotIgnored: "valueHere",
		}

		err := structmap.Unmarshal(input, &actual)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})
}
