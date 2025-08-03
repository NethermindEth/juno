package statetestutils

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	once        sync.Once
	parsed      bool
	useNewState bool
)

func parseFlags() {
	flag.BoolVar(&useNewState, "use-new-state", false, "use new state implementation")
	fmt.Println("use-new-state", useNewState)

	cleanArgs := []string{os.Args[0]}
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-use-new-state" || strings.HasPrefix(arg, "-use-new-state=") {
			continue
		}
		cleanArgs = append(cleanArgs, arg)
	}
	os.Args = cleanArgs

	flag.Parse()
	parsed = true
}

func Parse() {
	once.Do(parseFlags)
}

func UseNewState() bool {
	if !parsed {
		Parse()
	}
	return useNewState
}
