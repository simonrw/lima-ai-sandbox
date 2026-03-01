package main

import (
	"os"

	"github.com/simonrw/lima-ai-sandbox/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
