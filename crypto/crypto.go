package crypto

import (
	"crypto/rand"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const letterIdxMask = 1<<6 - 1

func fill(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic("crypto: failed to read random bytes: " + err.Error())
	}
}

func RandString(n int) string {
	if n <= 0 {
		return ""
	}

	b := make([]byte, n)
	buf := make([]byte, n+n/4+16)
	fill(buf)

	for i, read := 0, 0; i < n; {
		if read == len(buf) {
			fill(buf)
			read = 0
		}

		idx := int(buf[read] & letterIdxMask)
		read++

		if idx >= len(letterBytes) {
			continue
		}

		b[i] = letterBytes[idx]
		i++
	}

	return string(b)
}
