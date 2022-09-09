package prof

// FIXME: Running the CPU profiler programmatically does not seem to
// work.

import (
	"errors"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/fatih/color"
)

type Prof struct {
	// TODO: This should be a slice of handlers containing the different
	// profiles.
	file *os.File
}

func (p *Prof) CPU() error {
	f, err := os.Create("juno_cpu.prof")
	if err != nil {
		return errors.New("prof: create CPU profile: " + err.Error())
	}

	// XXX: The default profiling rate may be too slow but this still
	// appears not to work.
	runtime.SetCPUProfileRate(524_288)

	if err := pprof.StartCPUProfile(f); err != nil {
		return errors.New("prof: start CPU profile: " + err.Error())
	}

	p.file = f

	return nil
}

func (p *Prof) Stop() error {
	if err := p.file.Close(); err != nil {
		return err
	}

	pprof.StopCPUProfile()
	return nil
}

func Serve(ch chan<- error) {
	// DEBUG.
  // 10-minute CPU profile.
	color.Magenta("prof: visit http://localhost:8080/debug/pprof/profile?minutes=10")

	go func(ch chan<- error) {
		if err := http.ListenAndServe("localhost:8080", nil); !errors.Is(err, http.ErrServerClosed) {
			ch <- err
		}
		close(ch)
	}(ch)
}
