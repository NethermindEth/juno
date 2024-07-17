package utils

import (
	"fmt"
)

type DataSize float64

//nolint:mnd
func (d DataSize) String() string {
	switch {
	case d >= 1099511627776:
		return fmt.Sprintf("%.2f TiB", d/1099511627776)
	case d >= 1073741824:
		return fmt.Sprintf("%.2f GiB", d/1073741824)
	case d >= 1048576:
		return fmt.Sprintf("%.2f MiB", d/1048576)
	case d >= 1024:
		return fmt.Sprintf("%.2f KiB", d/1024)
	}
	return fmt.Sprintf("%.2f B", d)
}
