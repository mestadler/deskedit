package gpu

import (
	"os"
	"strings"
)

func statExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func fileContains(path, needle string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), needle)
}
