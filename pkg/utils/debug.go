package utils

import (
	"log"
	"os"
)

// DebugLog 输出调试日志
func DebugLog(format string, args ...interface{}) {
	if os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true" {
		log.Printf("[DEBUG] "+format, args...)
	}
}
