package taggedunion

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

func DecodeUnion(data []byte) (variant string, payload json.RawMessage, err error) {
	var unit string
	if err := json.Unmarshal(data, &unit); err == nil {
		return unit, nil, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", nil, fmt.Errorf("expected a string or a single-key object: %w", err)
	}

	if len(obj) != 1 {
		return "", nil, fmt.Errorf("expected exactly one variant key, got %d", len(obj))
	}

	for k, v := range obj {
		return k, v, nil
	}

	return "", nil, errors.New("unreachable")
}

func EncodeUnit(name string) ([]byte, error) {
	return json.Marshal(name)
}

func EncodeVariant(name string, inner any) ([]byte, error) {
	payload, err := json.Marshal(inner)
	if err != nil {
		return nil, err
	}

	key, err := json.Marshal(name)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.WriteByte('{')
	out.Write(key)
	out.WriteByte(':')
	out.Write(payload)
	out.WriteByte('}')

	return out.Bytes(), nil
}

type Unit struct{}

func UnitSet() *Unit { return &Unit{} }

func ExpectUnit(union, name string, payload json.RawMessage) error {
	if payload == nil || string(payload) == "null" {
		return nil
	}

	return fmt.Errorf("invalid type: expected unit for variant `%s` of %s", name, union)
}

func DecodeVariant(union, name string, payload json.RawMessage, into any) error {
	if payload == nil {
		return ErrVariantNeedsPayload(union, name)
	}

	return json.Unmarshal(payload, into)
}

func ErrUnknownVariant(union, variant string) error {
	return fmt.Errorf("unknown variant `%s` for %s", variant, union)
}

func ErrVariantNeedsPayload(union, variant string) error {
	return fmt.Errorf("variant `%s` of %s requires an object payload", variant, union)
}

func NonNil[T any](in []T) []T {
	if in == nil {
		return []T{}
	}

	return in
}
