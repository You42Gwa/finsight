package cmd

import (
	"fmt"

	"github.com/finsight-cli/finsight/internal/output"
	"github.com/finsight-cli/finsight/internal/upstage"
	"github.com/spf13/cobra"
)

var (
	parseMode       string
	parseTablesOnly bool
)

var parseCmd = &cobra.Command{
	Use:   "parse <파일>",
	Short: "PDF → Markdown 변환 (표 구조 보존)",
	Long: `document-parse API로 PDF를 Markdown으로 변환합니다.
재무제표의 표 구조를 보존하여 Markdown 테이블로 변환합니다.

예시:
  finsight parse ./report.pdf
  finsight parse ./report.pdf --tables-only
  finsight parse ./report.pdf --mode enhanced`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		apiKey := mustUpstageKey()

		if !output.NoColor {
			fmt.Printf("문서 파싱 중: %s (mode=%s) ...\n", filePath, parseMode)
		}

		result, err := upstage.ParseDocument(apiKey, filePath, parseMode)
		if err != nil {
			output.PrintError(err.Error())
			return
		}

		if parseTablesOnly {
			count := 0
			for _, el := range result.Elements {
				if el.Category == "table" && el.Content.Markdown != "" {
					if count > 0 {
						fmt.Print("\n---\n\n")
					}
					fmt.Println(el.Content.Markdown)
					count++
				}
			}
			if count == 0 {
				output.PrintError("표(table) 요소를 찾을 수 없습니다.")
			}
			return
		}

		if !output.NoColor {
			fmt.Printf("처리 완료: %d 페이지\n", result.Usage.Pages)
		}
		output.PrintParse(result.Content.Markdown)
	},
}

func init() {
	rootCmd.AddCommand(parseCmd)
	parseCmd.Flags().StringVarP(&parseMode, "mode", "m", "auto", "파싱 모드: standard | enhanced | auto")
	parseCmd.Flags().BoolVarP(&parseTablesOnly, "tables-only", "t", false, "표(table) 요소만 출력")
}
