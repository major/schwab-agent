package ptr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTo(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		p := new(42)
		assert.Equal(t, 42, *p)
	})

	t.Run("string", func(t *testing.T) {
		p := new("hello")
		assert.Equal(t, "hello", *p)
	})

	t.Run("bool", func(t *testing.T) {
		p := new(true)
		assert.Equal(t, true, *p)
	})
}
