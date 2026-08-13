package leadpush

import (
	"bytes"
	"encoding/json"
)

// Optional represents a JSON property that can be omitted, set to a value, or
// explicitly set to null. Its zero value is omitted by fields tagged with
// json:",omitzero".
type Optional[T any] struct {
	value T
	set   bool
	null  bool
}

// Ptr returns a pointer to value. It is useful for optional request fields
// where a zero value such as false or an empty string must still be sent.
func Ptr[T any](value T) *T {
	return &value
}

// Some returns an Optional set to value.
func Some[T any](value T) Optional[T] {
	return Optional[T]{value: value, set: true}
}

// Null returns an Optional explicitly set to JSON null.
func Null[T any]() Optional[T] {
	return Optional[T]{set: true, null: true}
}

// IsZero reports whether the property is unset and should be omitted.
func (o Optional[T]) IsZero() bool {
	return !o.set
}

// IsSet reports whether the property was set, including when it was set to
// null.
func (o Optional[T]) IsSet() bool {
	return o.set
}

// IsNull reports whether the property was explicitly set to null.
func (o Optional[T]) IsNull() bool {
	return o.set && o.null
}

// Value returns the configured value and true when the property contains a
// non-null value.
func (o Optional[T]) Value() (T, bool) {
	return o.value, o.set && !o.null
}

// MarshalJSON implements json.Marshaler.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.set || o.null {
		return []byte("null"), nil
	}

	return json.Marshal(o.value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.null = true
		var zero T
		o.value = zero
		return nil
	}

	o.null = false
	return json.Unmarshal(data, &o.value)
}
