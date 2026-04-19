package cmd

import (
	"fmt"

	"github.com/finsight-cli/finsight/internal/output"
	"github.com/finsight-cli/finsight/internal/upstage"
	"github.com/spf13/cobra"
)

var analyzeSkipClassify bool

var analyzeCmd = &cobra.Command{
	Use:   "analyze <파일>",
	Short: "재무제표 AI 심층 분석 (Solar Pro 3)",
	Long: `문서 분류 → 재무지표 추출 → Solar Pro 3 심층 분석을 수행합니다.
reasoning_effort=high 모드로 수익성·안정성·현금흐름을 평가합니다.

예시:
  finsight analyze ./report.pdf
  finsight analyze ./report.pdf --skip-classify`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		apiKey := mustUpstageKey()

		docType, confidence := "", 0.0

		if !analyzeSkipClassify {
			if !output.NoColor {
				fmt.Println("1/3  문서 분류 중 ...")
			}
			var err error
			docType, confidence, err = upstage.ClassifyDocument(apiKey, filePath)
			if err != nil {
				output.PrintError("문서 분류 실패 (건너뜀): " + err.Error())
			} else {
				output.PrintDocType(docType, confidence)
			}
		}

		if !output.NoColor {
			fmt.Println("2/3  재무지표 추출 중 ...")
		}
		fin, err := upstage.ExtractFinancials(apiKey, filePath)
		if err != nil {
			output.PrintError(err.Error())
			return
		}
		output.PrintFinancials(fin, false)

		if !output.NoColor {
			fmt.Println("3/3  Solar Pro 3 분석 중 (reasoning=high) ...")
		}
		analysis, err := upstage.AnalyzeFinancials(apiKey, fin, docType)
		if err != nil {
			output.PrintError(err.Error())
			return
		}
		output.PrintAnalysis(analysis)
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().BoolVar(&analyzeSkipClassify, "skip-classify", false, "문서 분류 단계 생략")
}
