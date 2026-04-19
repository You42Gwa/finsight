package cmd

import (
	"fmt"
	"time"

	"github.com/You42Gwa/finsight/internal/dart"
	"github.com/You42Gwa/finsight/internal/output"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache [status|refresh|clear]",
	Short: "DART 기업 코드 캐시 관리",
	Long: `DART 기업 코드 캐시를 관리합니다.

  status   캐시 상태 확인 (기본값)
  refresh  캐시 강제 갱신 (~40,000개 기업 코드 다운로드)
  clear    캐시 삭제`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		action := "status"
		if len(args) > 0 {
			action = args[0]
		}

		switch action {
		case "status":
			runCacheStatus()
		case "refresh":
			runCacheRefresh()
		case "clear":
			runCacheClear()
		default:
			output.PrintError("알 수 없는 동작: " + action + "  (status | refresh | clear)")
		}
	},
}

func runCacheStatus() {
	exists, count, modTime, size := dart.CacheStatus()
	if !exists {
		msg := "캐시 없음. 'finsight cache refresh' 로 초기화하세요."
		if output.NoColor {
			fmt.Println(msg)
		} else {
			output.Tip(msg)
		}
		return
	}
	age := time.Since(modTime)
	if output.NoColor {
		fmt.Printf("기업 수:   %d개\n", count)
		fmt.Printf("파일 크기: %.1f KB\n", float64(size)/1024)
		fmt.Printf("캐시 나이: %.1f시간 전\n", age.Hours())
	} else {
		fmt.Printf("  기업 수   %d개\n", count)
		fmt.Printf("  파일 크기 %.1f KB\n", float64(size)/1024)
		fmt.Printf("  캐시 나이 %.1f시간 전\n", age.Hours())
	}
}

func runCacheRefresh() {
	apiKey := mustDartKey()
	if !output.NoColor {
		fmt.Println("DART 기업 코드 다운로드 중 (수 초 소요) ...")
	}
	count, err := dart.RefreshCache(apiKey)
	if err != nil {
		output.PrintError(err.Error())
		return
	}
	fmt.Printf("갱신 완료: %d개 기업 코드\n", count)
}

func runCacheClear() {
	dart.ClearCache()
	fmt.Println("캐시를 삭제했습니다.")
}

func init() {
	rootCmd.AddCommand(cacheCmd)
}
