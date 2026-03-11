package runner

import (
	"io"
)

const maxResponseBodyBytes = 10 * 1024 * 1024 // 10 MB

func readBody(reader io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, maxResponseBodyBytes))
}
