package golib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// An example of how to use this type can be found in flags_test.go

type AbstractTestSuite struct {
	t *testing.T
	*require.Assertions
}

func (suite *AbstractTestSuite) T() *testing.T {
	return suite.t
}

func (suite *AbstractTestSuite) SetT(t *testing.T) {
	suite.t = t
	suite.Assertions = require.New(t)
}
