package uuidutil

import "testing"

func TestEncode(t *testing.T) {
	src := [16]byte{
		0x12, 0x3e, 0x45, 0x67,
		0xe8, 0x9b,
		0x12, 0xd3,
		0xa4, 0x56,
		0x42, 0x66, 0x14, 0x17, 0x40, 0x00,
	}

	want := "123e4567-e89b-12d3-a456-426614174000"

	if got := Encode(src); got != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestEncodeZero(t *testing.T) {
	want := "00000000-0000-0000-0000-000000000000"

	if got := Encode([16]byte{}); got != want {
		t.Errorf("Encode(zero) = %q, want %q", got, want)
	}
}

func TestEncodePadsSingleDigitBytes(t *testing.T) {
	var src [16]byte
	for i := range src {
		src[i] = 0x01
	}

	want := "01010101-0101-0101-0101-010101010101"

	if got := Encode(src); got != want {
		t.Errorf("Encode(all 0x01) = %q, want %q", got, want)
	}
}
