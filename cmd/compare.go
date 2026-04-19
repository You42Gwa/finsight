package cmd

import (
	"fmt"
	"sync"

	"github.com/You42Gwa/finsight/internal/dart"
	"github.com/You42Gwa/finsight/internal/output"
	"github.com/spf13/cobra"
)

var (
	compareType      string
	compareFsDiv     string
	compareOutputFmt string
)

var compareCmd = &cobra.Command{
	Use:   "compare <종목코드1|회사명1> <종목코드2|회사명2>",
	Short: "두 종목 재무지표 비교",
	Long: `두 종목의 재무지표를 나란히 비교합니다.
두 종목을 병렬 조회한 뒤 비교표를 출력합니다.

예시:
  finsight compare 005930 000660
  finsight compare 삼성전자 SK하이닉스
  finsight compare 005930 000660 --type half
  finsight compare 005930 000660 --output json`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		dartAPIKey := mustDartKey()

		type result struct {
			entry output.CompareEntry
			err   error
		}

		results := make([]result, 2)
		var wg sync.WaitGroup

		fsCode := dart.FsDivCodes[compareFsDiv]
		if fsCode == "" {
			fsCode = "CFS"
		}

		for i, query := range args {
			wg.Add(1)
			go func(idx int, q string) {
				defer wg.Done()
				corp, err := dart.FindCompany(dartAPIKey, q)
				if err != nil {
					results[idx].err = err
					return
				}
				if corp == nil {
					results[idx].err = fmt.Errorf(`"%s"에 해당하는 기업을 찾을 수 없습니다`, q)
					return
				}
				fin, _, _, err := dart.FetchLatestFinancials(dartAPIKey, corp.CorpCode, corp.CorpName, compareType, fsCode)
				if err != nil {
					results[idx].err = err
					return
				}
				results[idx].entry = output.CompareEntry{
					CorpName:  corp.CorpName,
					StockCode: corp.StockCode,
					Fin:       fin,
				}
			}(i, query)
		}
		wg.Wait()

		for _, r := range results {
			if r.err != nil {
				output.PrintError(r.err.Error())
				return
			}
		}

		output.PrintRule(fmt.Sprintf("finsight — 비교: %s vs %s",
			results[0].entry.CorpName, results[1].entry.CorpName))

		output.PrintCompare([]output.CompareEntry{results[0].entry, results[1].entry},
			compareOutputFmt == "json")
	},
}

func init() {
	rootCmd.AddCommand(compareCmd)
	compareCmd.Flags().StringVar(&compareType, "type", "annual", "보고서 유형: annual | half | q1 | q3")
	compareCmd.Flags().StringVar(&compareFsDiv, "fs", "연결", "재무제표 구분: 연결 | 별도")
	compareCmd.Flags().StringVarP(&compareOutputFmt, "output", "o", "table", "출력 형식: table | json")
}
