package golib

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// An example of how to use this type can be found in flags_test.go

type AbstractTestSuite struct {
	t *testing.T
	*require.Assertions
}

func (s *AbstractTestSuite) T() *testing.T {
	return s.t
}

func (s *AbstractTestSuite) SetT(t *testing.T) {
	s.t = t
	s.Assertions = require.New(t)
}

func (s *AbstractTestSuite) SubTest(name string, test func()) {
	oldT := s.t
	oldAssertions := s.Assertions
	defer func() {
		s.t = oldT
		s.Assertions = oldAssertions
	}()
	oldT.Run(name, func(t *testing.T) {
		s.SetT(t)
		test()
	})
}

func (s *AbstractTestSuite) SubTestSuite(testingSuite suite.TestingSuite) {
	suite.Run(s.t, testingSuite)
}
