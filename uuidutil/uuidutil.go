package uuidutil

import "encoding/hex"

func Encode(src [16]byte) string {
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], src[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], src[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], src[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], src[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], src[10:16])

	return string(buf)
}
