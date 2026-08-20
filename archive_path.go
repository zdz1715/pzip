package pzip

import (
	"fmt"
	"path"
	"strings"
)

func validateStripOptions(prefix string, components int) error {
	if prefix != "" && components != 0 {
		return fmt.Errorf("strip prefix and strip components are mutually exclusive")
	}
	if components < 0 {
		return fmt.Errorf("invalid strip components %d: must not be negative", components)
	}
	return nil
}

func stripArchivePath(name, prefix string, components int) (string, bool) {
	name = path.Clean(name)
	if prefix != "" {
		prefix = path.Clean(prefix)
		if name == prefix {
			return "", false
		}
		name, ok := strings.CutPrefix(name, prefix+"/")
		if !ok {
			return "", false
		}
		return name, true
	}

	for range components {
		_, rest, ok := strings.Cut(name, "/")
		if !ok {
			return "", false
		}
		name = rest
	}
	return name, name != "" && name != "."
}
