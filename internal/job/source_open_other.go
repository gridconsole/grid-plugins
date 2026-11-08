//go:build !unix

package job

import "os"

func openSource(path string) (*os.File, error) { return os.Open(path) }
