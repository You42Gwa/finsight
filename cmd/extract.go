package cmd

import (
	"fmt"

	"github.com/You42Gwa/finsight/internal/output"
	"github.com/You42Gwa/finsight/internal/upstage"
	"github.com/spf13/cobra"
)

var extractOutputFmt string

var extractCmd = &cobra.Command{
	Use:   "extract <파일>",
	Short: "PDF에서 핵심 재무지표 구조화 추출",
	Long: `information-extract API로 재무지표를 추출합니다.
매출액, 영업이익, 순이익, 부채비율 등 10개 지표를 자동 추출합니다.

예시:
  finsight extract ./report.pdf
  finsight extract ./report.pdf --output json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		apiKey := mustUpstageKey()

		if !output.NoColor {
			fmt.Printf("재무지표 추출 중: %s ...\n", filePath)
		}

		fin, err := upstage.ExtractFinancials(apiKey, filePath)
		if err != nil {
			output.PrintError(err.Error())
			return
		}
		output.PrintFinancials(fin, extractOutputFmt == "json")
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().StringVarP(&extractOutputFmt, "output", "o", "table", "출력 형식: table | json")
}
