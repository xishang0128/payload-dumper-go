package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/xishang/payload-dumper-go/common/file"
	"github.com/xishang/payload-dumper-go/common/i18n"
)

var (
	rootCmd   *cobra.Command
	userAgent string
)

func init() {
	i18n.InitLanguage()

	rootCmd = &cobra.Command{
		Use:   "payload-dumper",
		Short: i18n.I18nMsg.App.AppDescription,
		Long:  i18n.I18nMsg.App.AppLongDescription,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if userAgent != "" {
				file.SetUserAgent(userAgent)
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&userAgent, "user-agent", "", i18n.I18nMsg.Common.FlagUserAgent)

	initExtractCmd()
	initListCmd()
	initMetadataCmd()
	initExtractDiffCmd()
	initVersionCmd()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
