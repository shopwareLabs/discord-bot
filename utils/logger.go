package utils

import (
	"log"
	"os"
)

// Logger is the global logger instance
var Logger = log.New(os.Stdout, "[Discord-SSO] ", log.LstdFlags|log.Lshortfile)