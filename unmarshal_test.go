package dstruct

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

type RawValue []string

func (x *RawValue) UnmarshalValue(v []string) error {
	*x = v

	return nil
}

type FullName struct {
	FirstName string `map:"first_name"`
	LastName  string `map:",required,omitempty"`
}

type Occupation struct {
	JobTitle   string
	Department string
}

type Person struct {
	FullName
	Occupation Occupation
	Age        int
	RawValue   RawValue
	StrSlice   []string
	IntSlice   []int `map:"int_slice[]"`
	LeftEmpty  string
}

func TestUnmarshal(t *testing.T) {
	person := Person{
		LeftEmpty: "abcd",
	}

	val := url.Values{
		"first_name":  []string{"abcd"},
		"LastName":    []string{"hehehe"},
		"Age":         []string{"21"},
		"RawValue":    []string{"hello", "world"},
		"StrSlice":    []string{"World", "hello"},
		"int_slice[]": []string{"13", "23", "25"},
	}

	err := Unmarshal(val, &person)
	require.NoError(t, err)
}

func BenchmarkUnmarshal(b *testing.B) {
	val := url.Values{
		"first_name":  []string{"abcd"},
		"LastName":    []string{"hehehe"},
		"Age":         []string{"21"},
		"RawValue":    []string{"hello", "world"},
		"StrSlice":    []string{"World", "hello"},
		"int_slice[]": []string{"13", "23", "25"},
	}

	person := Person{
		LeftEmpty: "abcd",
	}

	for n := 0; n < b.N; n++ {
		if err := Unmarshal(val, &person); err != nil {
			b.Fatal(err)
		}
	}
}
