package golib

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type FlagsTestSuite struct {
	AbstractTestSuite
}

func TestFlags(t *testing.T) {
	suite.Run(t, new(FlagsTestSuite))
}

func (s *FlagsTestSuite) TestConvertLabels() {
	s.Equal(map[string]string{"hello": "world", "super": "cool"}, ParseMap("hello=world,super=cool"))
	s.Equal(map[string]string{"hello": "world", "invalid": ""}, ParseMap("hello=world,invalid"))
	s.Empty(ParseMap(""))
	s.Equal(map[string]string{"hello": "world", "super": "cool"}, ParseMap("hello = world , super= cool"))
}

func (s *FlagsTestSuite) TestConvertList() {
	s.Equal([]string{"test", "super", "cool"}, ParseSlice("test,super,cool"))
	s.Equal([]string{"hello"}, ParseSlice("hello,"))
	s.Equal([]string{"hello"}, ParseSlice("hello"))
	s.Equal([]string{"hello"}, ParseSlice(",hello"))
	s.Equal([]string{"hello"}, ParseSlice(",,,hello"))
	s.Empty(ParseSlice(""))
	s.Empty(ParseSlice("  "))
	s.Equal([]string{"test", "super", "cool"}, ParseSlice("test , super, cool"))
}
