package dstruct

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshal(t *testing.T) {
	person := Person{
		FullName: FullName{
			FirstName: "test",
			LastName:  "hello",
		},
		Occupation: Occupation{
			JobTitle:   "hehe",
			Department: "Hoho",
		},
		IntSlice:  []int{1, 2, 34, 56},
		StrSlice:  []string{"ab", "cf"},
		LeftEmpty: "hohoho",
		RawValue:  []string{"eee", "ncdw"},
	}

	m := make(map[string][]string)

	err := Marshal(person, m)

	require.NoError(t, err)
}
