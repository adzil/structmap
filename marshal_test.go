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
