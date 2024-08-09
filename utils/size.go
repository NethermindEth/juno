package utils

import (
	"fmt"
)

type DataSize float64

func (d DataSize) String() string {
	switch {
	case d >= Terabyte:
		return fmt.Sprintf("%.2f TiB", d/Terabyte)
	case d >= Gigabyte:
		return fmt.Sprintf("%.2f GiB", d/Gigabyte)
	case d >= Megabyte:
		return fmt.Sprintf("%.2f MiB", d/Megabyte)
	case d >= Kilobyte:
		return fmt.Sprintf("%.2f KiB", d/Kilobyte)
	}
	return fmt.Sprintf("%.2f B", d)
}
