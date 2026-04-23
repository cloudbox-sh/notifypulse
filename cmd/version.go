package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the notifypulse version",
	RunE: func(cmd *cobra.Command, args []string) error {
		return emit(map[string]string{"version": Version}, func() {
			fmt.Println(styles.Accent.Render("notifypulse") + " " + Version)
		})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
