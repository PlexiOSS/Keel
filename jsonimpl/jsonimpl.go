// Adapted from https://github.com/Anti-Raid/Sandwich-Daemon/blob/master/sandwichjson/json.go
package jsonimpl

import (
	"encoding/json"
	"io"
	"runtime"

	"github.com/bytedance/sonic"
)

var UseSonic = runtime.GOARCH == "amd64" && runtime.GOOS == "linux"

func Unmarshal(data []byte, v any) error {
	if UseSonic {
		return sonic.ConfigDefault.Unmarshal(data, v)
	} else {
		return json.Unmarshal(data, v)
	}
}

func UnmarshalReader(reader io.Reader, v any) error {
	if UseSonic {
		return sonic.ConfigDefault.NewDecoder(reader).Decode(v)
	} else {
		return json.NewDecoder(reader).Decode(v)
	}
}

func Marshal(v any) ([]byte, error) {
	if UseSonic {
		return sonic.ConfigDefault.Marshal(v)
	} else {
		return json.Marshal(v)
	}
}

func MarshalToWriter(writer io.Writer, v any) error {
	if UseSonic {
		return sonic.ConfigDefault.NewEncoder(writer).Encode(v)
	} else {
		return json.NewEncoder(writer).Encode(v)
	}
}
