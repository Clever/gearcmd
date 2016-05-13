package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Clever/discovery-go"
	"github.com/Clever/gearcmd/baseworker"
	"github.com/Clever/gearcmd/gearcmd"
	"gopkg.in/Clever/kayvee-go.v3/logger"
)

var (
	lg = logger.New("gearcmd")
)

func main() {
	functionName := flag.String("name", "", "Name of the Gearman function")
	functionCmd := flag.String("cmd", "", "The command to run")
	gearmanHost := flag.String("host", "", "The Gearman host. If not specified the SERVICE_GEARMAND_TCP_HOST environment variable will be used")
	gearmanPort := flag.String("port", "", "The Gearman port. If not specified the SERVICE_GEARMAND_TCP_PORT environment variable will be used")
	parseArgs := flag.Bool("parseargs", true, "If false send the job payload directly to the cmd as its first argument without parsing it")
	printVersion := flag.Bool("version", false, "Print the version and exit")
	cmdTimeout := flag.Duration("cmdtimeout", 0, "Maximum time for the command to run before it will be killed, e.g. 2h, 30m, 2h30m")
	retryCount := flag.Int("retry", 0, "Number of times to retry the job if it fails")
	warningLength := flag.Int("warningLength", 5, "Number of warning lines to store and send back to the gearmn job")
	passSigterm := flag.Bool("pass-sigterm", false, "Whether or not to pass SIGTERM through to the worker process")
	flag.Parse()

	if *printVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	var err error
	if *gearmanHost == "" {
		if *gearmanHost, err = discovery.Host("gearmand", "tcp"); err != nil {
			exitWithError("must either specify a host argument or set an environment variable " +
				"that conforms to https://godoc.org/github.com/Clever/discovery-go")
		}
	}
	if *gearmanPort == "" {
		if *gearmanPort, err = discovery.Port("gearmand", "tcp"); err != nil {
			exitWithError("must either specify a port argument or set an environment variable " +
				"that conforms to https://godoc.org/github.com/Clever/discovery-go")
		}
	}

	if *functionName == "" {
		exitWithError("name not defined")
	}
	if *functionCmd == "" {
		exitWithError("cmd not defined")
	}

	config := gearcmd.TaskConfig{
		FunctionName: *functionName,
		FunctionCmd:  *functionCmd,
		WarningLines: *warningLength,
		ParseArgs:    *parseArgs,
		CmdTimeout:   *cmdTimeout,
		RetryCount:   *retryCount,
		Halt:         make(chan struct{}),
	}
	worker := baseworker.NewWorker(*functionName, config.Process)
	defer worker.Close()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM)
	go func() {
		<-sigc
		if *passSigterm {
			close(config.Halt)
		}
		worker.Shutdown()
		os.Exit(0)
	}()

	lg.InfoD("listening", logger.M{"job": *functionName})
	if err := worker.Listen(*gearmanHost, *gearmanPort); err != nil {
		lg.CriticalD("failure-case", logger.M{"error": err.Error()})
		os.Exit(1)
	}
}

// exitWithError prints out an error message and exits the process with an exit code of 1
func exitWithError(errorStr string) {
	lg.CriticalD("failure-case", logger.M{"error": errorStr})
	flag.PrintDefaults()
	os.Exit(1)

}
