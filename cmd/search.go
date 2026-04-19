package cmd

import (
	"fmt"

	"github.com/You42Gwa/finsight/internal/dart"
	"github.com/You42Gwa/finsight/internal/output"
	"github.com/spf13/cobra"
)

var (
	searchShowFilings bool
	searchLimit       int
)

var searchCmd = &cobra.Command{
	Use:   "search <쿼리>",
	Short: "DART 기업 검색 및 공시 목록 조회",
	Long: `DART에서 회사명(부분 일치) 또는 종목코드로 기업을 검색합니다.
상장사(종목코드 있음)가 우선 표시됩니다.

예시:
  finsight search 삼성전자
  finsight search 005930 --filings
  finsight search 카카오 --limit 3`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]
		apiKey := mustDartKey()

		if !output.NoColor {
			fmt.Println("기업 코드 조회 중 (첫 실행 시 시간이 걸릴 수 있습니다) ...")
		}

		var corps []dart.CorpCode
		var err error

		if isDigitsOnly(query) {
			corp, e := dart.SearchByStockCode(apiKey, query)
			if e != nil {
				output.PrintError(e.Error())
				return
			}
			if corp != nil {
				corps = []dart.CorpCode{*corp}
			}
		} else {
			corps, err = dart.SearchByName(apiKey, query)
			if err != nil {
				output.PrintError(err.Error())
				return
			}
		}

		if len(corps) == 0 {
			output.PrintError(fmt.Sprintf(`"%s"에 해당하는 기업을 찾을 수 없습니다.`, query))
			return
		}
		if len(corps) > searchLimit {
			corps = corps[:searchLimit]
		}

		output.PrintSearchHeader(query)
		for _, corp := range corps {
			output.PrintSearchRow(corp.CorpCode, corp.StockCode, corp.CorpName)
		}
		fmt.Println()

		if searchShowFilings {
			for _, corp := range corps {
				printFilings(apiKey, corp)
			}
		}

		output.Tip("팁: finsight report --ticker <종목코드> 로 재무 분석을 바로 실행할 수 있습니다.")
	},
}

func printFilings(apiKey string, corp dart.CorpCode) {
	filings, err := dart.GetFilings(apiKey, corp.CorpCode, "A,B,C", "20220101", 5)
	output.PrintFilingHeader(corp.CorpName)
	if err != nil || len(filings) == 0 {
		fmt.Println("  공시 내역 없음")
		return
	}
	for _, f := range filings {
		output.PrintFilingRow(f.RceptDt, f.RceptNo, f.ReportNm)
	}
}

func isDigitsOnly(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().BoolVarP(&searchShowFilings, "filings", "f", false, "최근 공시 목록도 함께 표시")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 5, "최대 표시 기업 수")
}
