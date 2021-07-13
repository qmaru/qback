package utils

import (
	"log"
)

// SetProgress 显示进度条
func SetProgress(chunk int64, filechunks int64) {
	progress := float64(chunk) / float64(filechunks) * 100
	if progress != 0 {
		log.Printf("Progress: %.2f%%\n", progress)
	}
}
