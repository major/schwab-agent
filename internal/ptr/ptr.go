// Package ptr provides a generic helper for creating pointers to values.
package ptr

// To returns a pointer to the given value. Useful for initializing
// pointer fields in struct literals.
//
//go:fix inline
func To[T any](v T) *T { return new(v) }
