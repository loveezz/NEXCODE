// hash-password reads a password from stdin; no plaintext is written to disk or logs.
package main

import (
	"fmt"
	"io"
	"nexcode/internal/auth"
	"os"
	"strings"
)

func main() {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1024))
	if err != nil {
		os.Exit(1)
	}
	hash, err := auth.HashPassword(strings.TrimRight(string(data), "\r\n"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
