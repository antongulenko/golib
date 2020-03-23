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

func (s *AbstractTestSuite) Run(t *testing.T) {
	suite.Run(t, s)
}

func (s *AbstractTestSuite) SubTest(name string, test func()) {
	oldT := s.t
	oldAssertions := s.Assertions
	s.T().Run(name, func(t *testing.T) {
		s.t = t
		s.Assertions = require.New(t)
		test()
	})
	s.t = oldT
	s.Assertions = oldAssertions
}

func (s *AbstractTestSuite) SubTestSuite(testingSuite suite.TestingSuite) {
	suite.Run(s.t, testingSuite)
}
