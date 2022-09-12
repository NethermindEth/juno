package prof

import (
	"errors"
	"net/http"
	_ "net/http/pprof"

	"github.com/fatih/color"
)

func Serve(ch chan<- error) {
	// DEBUG.
	color.Magenta("prof: visit http://localhost:8080/debug/pprof/profile?seconds=30")

	go func(ch chan<- error) {
		if err := http.ListenAndServe("localhost:8080", nil); !errors.Is(err, http.ErrServerClosed) {
			ch <- err
		}
		close(ch)
	}(ch)
}
