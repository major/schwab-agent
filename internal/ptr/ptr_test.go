package ptr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/major/schwab-agent/internal/ptr"
)

func TestTo(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		p := ptr.To(42)
		assert.Equal(t, 42, *p)
	})

	t.Run("string", func(t *testing.T) {
		p := ptr.To("hello")
		assert.Equal(t, "hello", *p)
	})

	t.Run("bool", func(t *testing.T) {
		p := ptr.To(true)
		assert.Equal(t, true, *p)
	})
}
