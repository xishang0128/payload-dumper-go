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
		fmt.Printf("%s\n", i18n.I18nMsg.App.VersionTitle)
		fmt.Printf("%s: %s(%s)\n", i18n.I18nMsg.App.VersionLabel, constant.Version, constant.BuildTime)
		fmt.Printf("%s: %s\n", i18n.I18nMsg.App.GoVersionLabel, runtime.Version())
		fmt.Printf("%s: %s/%s\n", i18n.I18nMsg.App.PlatformLabel, runtime.GOOS, runtime.GOARCH)

		// Display compression implementations
		fmt.Printf("\n压缩算法实现:\n")
		implementations := dumper.GetCompressionImplementations()

		// Sort algorithms for consistent display
		algorithms := make([]string, 0, len(implementations))
		for algo := range implementations {
			algorithms = append(algorithms, algo)
		}
		sort.Strings(algorithms)

		hasCGO := false
		for _, algo := range algorithms {
			impl := implementations[algo]
			implType := "Pure Go"
			if impl.IsCGO {
				implType = "CGO"
				hasCGO = true
			}
			fmt.Printf("  %-8s: %s\n", algo, implType)
		}

		fmt.Printf("\n")
		if hasCGO {
			fmt.Printf("✅ 部分压缩算法使用高性能 CGO 实现\n")
		} else {
			fmt.Printf("⚠️  所有压缩算法使用标准 Pure Go 实现\n")
			fmt.Printf("💡 安装相关 C 库并启用 CGO 可获得更好的性能\n")
		}

		// Legacy XZ implementation info for backward compatibility
		fmt.Printf("\n[兼容性] %s: %s\n", i18n.I18nMsg.App.XZImplementationLabel, dumper.GetXZImplementation())
	},
}

func initVersionCmd() {
	rootCmd.AddCommand(versionCmd)
}
