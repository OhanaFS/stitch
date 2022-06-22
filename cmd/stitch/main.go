package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/OhanaFS/stitch/cmd/stitch/cmd"
)

var subcommands = map[string]*flag.FlagSet{
	cmd.ReedsolomonCmd.Name(): cmd.ReedsolomonCmd,
	cmd.PipelineCmd.Name():    cmd.PipelineCmd,
}

func run() int {
	var command *flag.FlagSet

	subcommandNames := []string{}
	for name := range subcommands {
		subcommandNames = append(subcommandNames, name)
	}

	if len(os.Args) < 2 {
		log.Fatalf("You must specify a subcommand. Valid subcommands are: %s\n", strings.Join(subcommandNames, ", "))
	}

	command = subcommands[os.Args[1]]
	if command == nil {
		log.Fatalf("unknown subcommand '%s'. Available commands are: %s\n", os.Args[1], strings.Join(subcommandNames, ", "))
	}

	command.Parse(os.Args[2:])

	switch command.Name() {
	case cmd.ReedsolomonCmd.Name():
		return cmd.RunReedSolomonCmd()
	case cmd.PipelineCmd.Name():
		return cmd.RunPipelineCmd()
	}

	return 0
}

func main() {
	os.Exit(run())
}
