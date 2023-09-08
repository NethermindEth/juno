package colors

type Color int

// ANSI color chars
const (
	BOLD      Color = 1
	BLACK     Color = 31
	RED       Color = BLACK + 1
	GREEN     Color = RED + 1
	YELLOW    Color = GREEN + 1
	BLUE      Color = YELLOW + 1
	MAGENTA   Color = BLUE + 1
	CYAN      Color = MAGENTA + 1
	WHITE     Color = CYAN + 1
	DARK_GRAY Color = 90
)

// ColorFunc is an alias type for a coloring function that accepts string and returns colorized string
type ColorFunc = func(s string) string

func Reset(s string) string {
	return s
}

// TODO implement more colors

func Black(s string) string {
	return WithColor(s, BLACK)
}

func Red(s string) string {
	return WithColor(s, RED)
}

func Green(s string) string {
	return WithColor(s, GREEN)
}

func Yellow(s string) string {
	return WithColor(s, YELLOW)
}

func Blue(s string) string {
	return WithColor(s, BLUE)
}

func White(s string) string {
	return WithColor(s, WHITE)
}
