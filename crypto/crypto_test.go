package crypto

import (
	"strings"
	"testing"
)

func TestRandStringLengthAndAlphabet(t *testing.T) {
	for _, n := range []int{0, 1, 8, 16, 128, 256, 6000} {
		s := RandString(n)
		if len(s) != n {
			t.Fatalf("RandString(%d) returned %d characters", n, len(s))
		}
		for _, c := range s {
			if !strings.ContainsRune(letterBytes, c) {
				t.Fatalf("RandString(%d) returned %q outside the alphabet", n, c)
			}
		}
	}
}

func TestRandStringNegative(t *testing.T) {
	if s := RandString(-1); s != "" {
		t.Fatalf("RandString(-1) = %q, want empty", s)
	}
}

func TestRandStringNoCollisions(t *testing.T) {
	seen := make(map[string]struct{}, 100000)
	for i := 0; i < 100000; i++ {
		s := RandString(16)
		if _, ok := seen[s]; ok {
			t.Fatalf("RandString(16) repeated %q after %d calls", s, i)
		}
		seen[s] = struct{}{}
	}
}

func TestRandStringUniform(t *testing.T) {
	counts := make(map[byte]int, len(letterBytes))
	s := RandString(520000)
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}
	for i := 0; i < len(letterBytes); i++ {
		c := counts[letterBytes[i]]
		if c < 9000 || c > 11000 {
			t.Fatalf("letter %q appeared %d times, want about 10000", letterBytes[i], c)
		}
	}
}
