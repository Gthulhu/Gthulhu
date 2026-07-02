package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/Gthulhu/Gthulhu/internal/daemon"
	"github.com/Gthulhu/Gthulhu/internal/schedext"
	"github.com/Gthulhu/Gthulhu/internal/scheduler"
)

const (
	modeScheduler = "scheduler"
	modeDaemon    = "daemon"
)

func Run(args []string) error {
	if isGlobalHelpRequest(args) {
		printRootUsage(os.Stdout, os.Args[0])
		return nil
	}

	mode, modeArgs := resolveModeAndArgs(args)
	switch mode {
	case modeDaemon:
		return daemon.Run(modeArgs)
	default:
		return scheduler.Run(modeArgs)
	}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, schedext.ErrUnsupported) {
		return schedext.UnsupportedExitCode
	}
	return 1
}

func isGlobalHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	first := args[0]
	return first == "help" || first == "-h" || first == "--help"
}

func resolveModeAndArgs(args []string) (string, []string) {
	if len(args) == 0 {
		return modeScheduler, args
	}
	if args[0] == modeScheduler || args[0] == modeDaemon {
		return args[0], args[1:]
	}
	return modeScheduler, args
}

func printRootUsage(w *os.File, binary string) {
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  %s [scheduler flags]            # default mode (backward compatible)\n", binary)
	fmt.Fprintf(w, "  %s scheduler [scheduler flags]  # explicit scheduler mode\n", binary)
	fmt.Fprintf(w, "  %s daemon [daemon flags]        # supervisor mode\n\n", binary)
	fmt.Fprintf(w, "Scheduler flags:\n")
	fmt.Fprintf(w, "  -config string\tPath to YAML configuration file\n")
	fmt.Fprintf(w, "  -help\t\tShow scheduler help message\n")
	fmt.Fprintf(w, "  -explain\tExplain configuration options\n\n")
	fmt.Fprintf(w, "Daemon flags:\n")
	fmt.Fprintf(w, "  -config string\tPath to YAML configuration file passed to child scheduler\n")
	fmt.Fprintf(w, "  -restart-delay duration\tDelay before restarting child scheduler (default 2s)\n")
	fmt.Fprintf(w, "  -scheduler-bin string\tPath to scheduler binary (default: current executable)\n")
}
