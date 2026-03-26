package runner

import (
	"fmt"
	"io"
)

const maxResponseBodyBytes = 10 * 1024 * 1024 // 10 MB

func readBody(reader io.Reader) ([]byte, error) {
	// Read one extra byte to detect overflow.
	data, err := io.ReadAll(io.LimitReader(reader, maxResponseBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes limit", maxResponseBodyBytes)
	}
	return data, nil
}
