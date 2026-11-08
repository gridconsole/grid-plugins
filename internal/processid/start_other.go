//go:build !unix

package processid

// Start cannot establish process identity on unsupported platforms.
func Start(int) string { return "" }
