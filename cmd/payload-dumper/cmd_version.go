package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/constant"
	"github.com/xishang0128/payload-dumper-go/dumper"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: i18n.I18nMsg.App.VersionCmdShort,
	Long:  i18n.I18nMsg.App.VersionCmdLong,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", i18n.I18nMsg.App.VersionTitle)
		fmt.Printf("%s: %s(%s)\n", i18n.I18nMsg.App.VersionLabel, constant.Version, constant.BuildTime)
		fmt.Printf("%s: %s\n", i18n.I18nMsg.App.GoVersionLabel, runtime.Version())
		fmt.Printf("%s: %s/%s\n", i18n.I18nMsg.App.PlatformLabel, runtime.GOOS, runtime.GOARCH)
		fmt.Printf("%s: %s\n", i18n.I18nMsg.App.XZImplementationLabel, dumper.GetXZImplementation())

		fmt.Printf("\n")
		if dumper.GetXZImplementation() == "CGO" {
			fmt.Printf("%s\n", i18n.I18nMsg.App.PerformanceFastMessage)
		} else {
			fmt.Printf("%s\n", i18n.I18nMsg.App.PerformanceSlowMessage)
			fmt.Printf("%s\n", i18n.I18nMsg.App.PerformanceSlowAdvice)
		}
	},
}

func initVersionCmd() {
	rootCmd.AddCommand(versionCmd)
}
