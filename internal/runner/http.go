package runner

import (
	"io"
)

func readBody(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}
