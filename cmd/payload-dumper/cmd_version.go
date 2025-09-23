package main

import (
	"fmt"
	"runtime"
	"sort"

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
		fmt.Println(i18n.I18nMsg.App.VersionTitle)
		fmt.Println(i18n.I18nMsg.App.VersionLabel, ": ", constant.Version, "(", constant.BuildTime, ")")
		fmt.Println(i18n.I18nMsg.App.GoVersionLabel, ": ", runtime.Version())
		fmt.Println(i18n.I18nMsg.App.PlatformLabel, ": ", runtime.GOOS, "/", runtime.GOARCH)

		fmt.Println(i18n.I18nMsg.App.CompressionImplementationsTitle)
		implementations := dumper.GetCompressionImplementations()

		algorithms := make([]string, 0, len(implementations))
		for algo := range implementations {
			algorithms = append(algorithms, algo)
		}
		sort.Strings(algorithms)

		hasCGO := false
		for _, algo := range algorithms {
			impl := implementations[algo]
			implType := i18n.I18nMsg.App.PureGoImplementation
			if impl.IsCGO {
				implType = i18n.I18nMsg.App.CGOImplementation
				hasCGO = true
			}
			fmt.Println("  ", algo, ": ", implType)
		}

		if hasCGO {
			fmt.Println(i18n.I18nMsg.App.CGOPerformanceMessage)
		} else {
			fmt.Println(i18n.I18nMsg.App.PureGoPerformanceMessage)
			fmt.Println(i18n.I18nMsg.App.PerformanceAdvice)
		}

		fmt.Println(i18n.I18nMsg.App.CompatibilityLabel, ": ", i18n.I18nMsg.App.XZImplementationLabel, ": ", dumper.GetXZImplementation())
	},
}

func initVersionCmd() {
	rootCmd.AddCommand(versionCmd)
}
