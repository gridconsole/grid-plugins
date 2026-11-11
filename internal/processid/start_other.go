//go:build !unix && !windows

package processid

// Start cannot establish process identity on platforms with no Unix or Windows reader.
func Start(int) string { return "" }
