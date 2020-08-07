package outil

import (
	"testing"
)

func TestImproveCoverage(_ *testing.T) {
	RandString(8)
	RandInt(8)
	UniqueID()
}
