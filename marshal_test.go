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

func TestMarshal(t *testing.T) {
	t.Run("WithFieldNames", func(t *testing.T) {
		type testStruct struct {
			Ignored    string `map:"-"`
			NotIgnored string `map:"-,"`
			Message    string `map:"message"`
		}

		expected := map[string][]string{
			"-":       {"valueThere"},
			"message": {"itsHere"},
		}

		input := testStruct{
			Ignored:    "valueHere",
			NotIgnored: "valueThere",
			Message:    "itsHere",
		}

		actual := make(map[string][]string)

		err := structmap.Marshal(input, actual)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})
}
