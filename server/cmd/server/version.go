package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Build   = ""
	Commit  = "HEAD"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build info",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("tink-server version %s (build %s, %s)\n", Version, Build, Commit)
	},
}
