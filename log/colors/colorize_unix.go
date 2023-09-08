//go:build !windows
// +build !windows

package colors

import "fmt"

// unix always has colors avaiable
var enabled = true

func EnableColor() { enabled = true }

func DisableColor() { enabled = false }

func WithColor(s string, c Color) string {
	if !enabled {
		return s
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", c, s)
}
