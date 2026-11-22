package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateClearsToNullKeepsBooleanNo(t *testing.T) {
	boolean := &FieldSpec{Name: "on_offer", Type: "BOOLEAN"}
	assert.False(t, updateClearsToNull(boolean, "on_offer", "0"), "no is stored as 0")
	assert.True(t, updateClearsToNull(boolean, "on_offer", ""), "empty still clears")
	assert.False(t, updateClearsToNull(&FieldSpec{Type: "TINYINT(1)"}, "x", "0"))

	number := &FieldSpec{Name: "owner", Type: "INTEGER"}
	assert.True(t, updateClearsToNull(number, "owner", "0"), "0 still clears other nullable integers")
}

func TestIsBooleanTypeName(t *testing.T) {
	for _, name := range []string{"tinyint", "TINYINT(1)", "bool", " Boolean "} {
		assert.True(t, IsBooleanTypeName(name), name)
	}
	for _, name := range []string{"INTEGER", "int64", "TEXT", ""} {
		assert.False(t, IsBooleanTypeName(name), name)
	}
}
