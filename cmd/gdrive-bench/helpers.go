package main

import (
	"io"
	"strings"
)

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func strReader(s string) io.Reader {
	return strings.NewReader(s)
}
