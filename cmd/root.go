// Package cmd defines the finsight CLI commands using cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/You42Gwa/finsight/internal/output"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "finsight",
	Short: "Upstage API 기반 한국 재무공시 분석 CLI",
	Long: `finsight — Upstage API 기반 한국 재무공시 분석 CLI

DART 공시 문서를 AI로 파싱하고 핵심 재무지표를 추출하여 심층 분석을 제공합니다.

필요 환경변수:
  UPSTAGE_API_KEY  Upstage API 키       https://console.upstage.ai
  DART_API_KEY     DART OpenAPI 키      https://opendart.fss.or.kr
                   (search / report --ticker 사용 시 필요)`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	_ = godotenv.Load()
	rootCmd.PersistentFlags().BoolVar(&output.NoColor, "no-color", false, "AI agent 파이프용 plain text 출력")
	rootCmd.PersistentFlags().BoolVar(&output.AgentMode, "agent", false, "AI 에이전트용 요약 텍스트 출력 (--no-color 포함)")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if output.AgentMode {
			output.NoColor = true
		}
	}
}

// mustUpstageKey exits if UPSTAGE_API_KEY is not set.
func mustUpstageKey() string {
	key := os.Getenv("UPSTAGE_API_KEY")
	if key == "" {
		output.PrintError("UPSTAGE_API_KEY 환경변수가 설정되지 않았습니다.")
		os.Exit(1)
	}
	return key
}

// mustDartKey exits if DART_API_KEY is not set.
func mustDartKey() string {
	key := os.Getenv("DART_API_KEY")
	if key == "" {
		output.PrintError("DART_API_KEY 환경변수가 설정되지 않았습니다.\n  발급: https://opendart.fss.or.kr")
		os.Exit(1)
	}
	return key
}
