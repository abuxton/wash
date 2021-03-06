package meta

import (
	"testing"

	"github.com/puppetlabs/wash/cmd/internal/find/parser/parsertest"
	"github.com/puppetlabs/wash/cmd/internal/find/parser/predicate"
	"github.com/puppetlabs/wash/cmd/internal/find/primary/numeric"
	"github.com/stretchr/testify/suite"
)

type NumericPredicateTestSuite struct {
	parsertest.Suite
}

func (s *NumericPredicateTestSuite) TestErrors() {
	s.RunTestCases(
		s.NPETC("", `expected a \+, -, or a digit`, true),
		s.NPETC("foo", "expected.*number.*foo", true),
		s.NPETC("--15", "expected.*positive", false),
	)
}

func (s *NumericPredicateTestSuite) TestValidInput() {
	s.RunTestCases(
		// Test a plain numeric value
		s.NPTC("200 -size", "-size", float64(200)),
		s.NPTC("+200 -size", "-size", float64(201)),
		s.NPTC("-200 -size", "-size", float64(199)),
		// Test a plain, negative numeric value
		s.NPTC("{200} -size", "-size", float64(-200)),
		s.NPTC("+{200} -size", "-size", float64(-199)),
		s.NPTC("-{200} -size", "-size", float64(-201)),
		// Test a size value
		s.NPTC("2G -size", "-size", float64(2*numeric.BytesOf('G'))),
		s.NPTC("+2G -size", "-size", float64(3*numeric.BytesOf('G'))),
		s.NPTC("-2G -size", "-size", float64(1*numeric.BytesOf('G'))),
	)
}

func (s *NumericPredicateTestSuite) TestNumericP_NotANumber() {
	np := numericP(func(n int64) bool {
		return n > 5
	})
	s.False(np.IsSatisfiedBy("6"))
	s.False(np.Negate().IsSatisfiedBy("3"))
}

func (s *NumericPredicateTestSuite) TestNumericP() {
	np := numericP(func(n int64) bool {
		return n > 5
	})

	s.True(np.IsSatisfiedBy(float64(6)))
	s.False(np.IsSatisfiedBy(float64(3)))

	// Test negation
	s.False(np.Negate().IsSatisfiedBy(float64(6)))
	s.True(np.Negate().IsSatisfiedBy(float64(3)))
}

func TestNumericPredicate(t *testing.T) {
	s := new(NumericPredicateTestSuite)
	s.Parser = predicate.ToParser(parseNumericPredicate)
	suite.Run(t, s)
}
