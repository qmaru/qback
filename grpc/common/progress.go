package common

import (
	"fmt"
	"log"
)

// ShowProgress 显示进度条
func ShowProgress(chunk int64, filechunks int64) {
	progress := float64(chunk) / float64(filechunks) * 100
	if progress != 0 {
		log.Printf("Progress: %.2f%%\n", progress)
	}
}

func FormatSpeed(bytesPerSecond float64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytesPerSecond >= GB:
		return fmt.Sprintf("%.2f GB/s", bytesPerSecond/GB)
	case bytesPerSecond >= MB:
		return fmt.Sprintf("%.2f MB/s", bytesPerSecond/MB)
	case bytesPerSecond >= KB:
		return fmt.Sprintf("%.2f KB/s", bytesPerSecond/KB)
	default:
		return fmt.Sprintf("%.2f B/s", bytesPerSecond)
	}
}
