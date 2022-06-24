package util

import "fmt"

// FormatSize returns a human-readable string representation of a size in bytes.
func FormatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MiB", float64(size)/1024/1024)
	}
	return fmt.Sprintf("%.1f GiB", float64(size)/1024/1024/1024)
}
