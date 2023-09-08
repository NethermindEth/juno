//go:build windows
// +build windows

package colors

func EnableColor() {
	// TODO implement color check for windows
}
func DisableColor() {}

func WithColor(s string, c Color) string {
	return s
}
