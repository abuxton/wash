// Package cmd implements Wash's CLI using https://github.com/spf13/cobra.
package cmd

import (
	"github.com/Benchkram/errz"
	cmdutil "github.com/puppetlabs/wash/cmd/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Unfortunately, cobra.Command.Execute() can only return error objects.
// Thus, the only way for us to let each command configure its own exit
// code is to wrap that value in an error object. This should be OK since
// we want the commands to handle their own errors.
type exitCode struct {
	value int
}

// Required to implement the error interface
func (e exitCode) Error() string {
	return ""
}

// This munging's necessary to ensure that all commandMain functions return
// an exit code while also letting them be used as RunE functions that can
// be passed into Cobra. Otherwise, Go's type-checker will complain even though
// exitCode is an error object.
type commandMain func(cmd *cobra.Command, args []string) exitCode
type runE func(cmd *cobra.Command, args []string) error

func toRunE(main commandMain) runE {
	return func(cmd *cobra.Command, args []string) error {
		return main(cmd, args)
	}
}

func rootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		// TODO: Set this to "" when we're ready to ship so that
		// when we alias our custom commands, someone typing in
		// e.g. `meta --help` will not see `wash meta` in the usage
		Use:  "wash",
		RunE: toRunE(rootMain),
		// Need to set these so that Cobra will not output the usage +
		// error object when Execute() returns an error, which will always
		// happen in our case because the exitCode object is technically
		// an error.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.Flags().String("loglevel", "info", "Set the logging level")
	errz.Fatal(viper.BindPFlag("loglevel", rootCmd.Flags().Lookup("loglevel")))

	rootCmd.Flags().String("logfile", "", "Set the log file's location. Defaults to stdout")
	errz.Fatal(viper.BindPFlag("logfile", rootCmd.Flags().Lookup("logfile")))

	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(serverCommand())
	rootCmd.AddCommand(metaCommand())
	rootCmd.AddCommand(listCommand())
	rootCmd.AddCommand(execCommand())
	rootCmd.AddCommand(psCommand())
	rootCmd.AddCommand(findCommand())
	rootCmd.AddCommand(clearCommand())
	rootCmd.AddCommand(tailCommand())
	rootCmd.AddCommand(historyCommand())

	return rootCmd
}

// Execute executes the root command, returning the exit code
func Execute() int {
	err := rootCommand().Execute()
	if err == nil {
		// This can happen if the user invokes `wash` without any
		// arguments, or if they invoke a help command.
		return 0
	}

	exitCode, ok := err.(exitCode)
	if !ok {
		// err is something Cobra-related, like e.g. a malformed
		// flag. Print the error, then return.
		cmdutil.ErrPrintf("Error: %v\n", err)
		return 1
	}

	return exitCode.value
}
