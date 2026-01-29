package main

import (
	"log"
	"os"
)

// debugLog 输出调试日志
func debugLog(format string, args ...interface{}) {
	if os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true" {
		log.Printf("[DEBUG] "+format, args...)
	}
}
