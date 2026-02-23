package cmd

import (
	"github.com/josh/goggle/pkg/gog"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with GOG",
	RunE: func(cmd *cobra.Command, args []string) error {
		return gog.Login()
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
