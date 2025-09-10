package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/xishang/payload-dumper-go/common/i18n"
)

var rootCmd *cobra.Command

func init() {
	// Initialize language support first
	i18n.InitLanguage()

	// Initialize root command with localized strings
	rootCmd = &cobra.Command{
		Use:   "payload-dumper",
		Short: i18n.I18nMsg.App.AppDescription,
		Long:  i18n.I18nMsg.App.AppLongDescription,
	}

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
