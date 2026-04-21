package utils

import (
	"fmt"

	"github.com/qmaru/minitools/v2/file"
	"github.com/qmaru/minitools/v2/hashx/blake3"
)

var (
	FileSuite   = file.New()
	Blake3Suite = blake3.New()
)

func PrettySize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func PrettyHash(h string) string {
	if len(h) <= 8 {
		return h
	}
	return h[:8]
}
