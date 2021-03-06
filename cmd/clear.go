package cmd

import (
	"fmt"

	"github.com/puppetlabs/wash/api/client"
	cmdutil "github.com/puppetlabs/wash/cmd/util"
	"github.com/puppetlabs/wash/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func clearCommand() *cobra.Command {
	clearCmd := &cobra.Command{
		Use:   "clear [<path>]",
		Short: "Clears the cache at <path>, or the current directory if not specified",
		Args:  cobra.MaximumNArgs(1),
	}

	clearCmd.Flags().BoolP("verbose", "v", false, "Print paths that were cleared from the cache")
	if err := viper.BindPFlag("verbose", clearCmd.Flags().Lookup("verbose")); err != nil {
		cmdutil.ErrPrintf("%v\n", err)
	}

	clearCmd.RunE = toRunE(clearMain)

	return clearCmd
}

func clearMain(cmd *cobra.Command, args []string) exitCode {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	verbose := viper.GetBool("verbose")

	conn := client.ForUNIXSocket(config.Socket)
	cleared, err := conn.Clear(path)
	if err != nil {
		cmdutil.ErrPrintf("%v\n", err)
		return exitCode{1}
	}

	if verbose {
		for _, p := range cleared {
			fmt.Println("Cleared", p)
		}
	} else {
		fmt.Println("Cleared", path)
	}

	return exitCode{0}
}
