package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/You42Gwa/finsight/internal/dart"
	"github.com/You42Gwa/finsight/internal/output"
	"github.com/You42Gwa/finsight/internal/upstage"
	"github.com/spf13/cobra"
)

var (
	reportTicker     string
	reportCompany    string
	reportType       string
	reportFsDiv      string
	reportOutputFmt  string
)

var reportCmd = &cobra.Command{
	Use:   "report [파일]",
	Short: "풀 파이프라인 — PDF 또는 종목코드/회사명",
	Long: `재무제표 풀 분석 리포트를 생성합니다.

두 가지 입력 경로:
  1. 로컬 파일  → document-classify + information-extract + Solar Pro 3
  2. DART 조회  → XBRL 재무제표 자동 수집    + Solar Pro 3

예시:
  finsight report ./samsung_2024q3.pdf
  finsight report --ticker 005930
  finsight report --company 삼성전자 --type annual
  finsight report --ticker 035420 --type half --fs 별도
  finsight report --ticker 005930 --output json`,
	Run: func(cmd *cobra.Command, args []string) {
		hasFile := len(args) > 0
		hasDart := reportTicker != "" || reportCompany != ""

		if !hasFile && !hasDart {
			output.PrintError("PDF 파일 경로 또는 --ticker / --company 옵션을 지정하세요.")
			_ = cmd.Help()
			return
		}
		if hasDart {
			runDartReport()
		} else {
			runPDFReport(args[0])
		}
	},
}

func runDartReport() {
	dartAPIKey := mustDartKey()
	upstageAPIKey := mustUpstageKey()

	query := reportTicker
	if query == "" {
		query = reportCompany
	}

	if !output.AgentMode {
		output.PrintRule("finsight — DART: " + query)
	}
	if !output.NoColor {
		fmt.Printf("1/3  기업 조회 중: %s ...\n", query)
	}
	corp, err := dart.FindCompany(dartAPIKey, query)
	if err != nil {
		output.PrintError(err.Error())
		return
	}
	if corp == nil {
		output.PrintError(fmt.Sprintf(`"%s"에 해당하는 기업을 찾을 수 없습니다. finsight search 로 먼저 검색하세요.`, query))
		return
	}
	if !output.AgentMode {
		output.PrintCorpInfo(corp.CorpName, corp.StockCode, corp.CorpCode)
	}

	if !output.NoColor {
		fmt.Printf("2/3  DART 재무제표 조회 중 (fs=%s) ...\n", reportFsDiv)
	}
	fsCode := dart.FsDivCodes[reportFsDiv]
	if fsCode == "" {
		fsCode = "CFS"
	}
	fin, _, reprtLabel, err := dart.FetchLatestFinancials(dartAPIKey, corp.CorpCode, corp.CorpName, reportType, fsCode)
	if err != nil {
		output.PrintError(err.Error())
		return
	}

	if reportOutputFmt == "json" {
		type out struct {
			Corp      interface{} `json:"corp"`
			Financials interface{} `json:"financials"`
		}
		data, _ := json.MarshalIndent(out{corp, fin}, "", "  ")
		fmt.Println(string(data))
		return
	}

	if !output.NoColor {
		fmt.Println("3/3  Solar Pro 3 분석 중 (reasoning=high) ...")
	}
	analysis, err := upstage.AnalyzeFinancials(upstageAPIKey, fin, reprtLabel)
	if err != nil {
		output.PrintError(err.Error())
		return
	}

	if output.AgentMode {
		output.PrintAgentReport(corp.CorpName, corp.StockCode, analysis, fin)
		return
	}
	output.PrintFinancials(fin, false)
	output.PrintAnalysis(analysis)
	output.PrintFooter("finsight  ·  DART OpenAPI + Upstage Solar Pro 3")
}

func runPDFReport(filePath string) {
	apiKey := mustUpstageKey()

	if !output.AgentMode {
		output.PrintRule("finsight — " + filePath)
	}
	if !output.NoColor {
		fmt.Println("1/3  문서 분류 중 ...")
	}
	docType, confidence, err := upstage.ClassifyDocument(apiKey, filePath)
	if err != nil {
		output.PrintError("문서 분류 실패 (건너뜀): " + err.Error())
	} else if !output.AgentMode {
		output.PrintDocType(docType, confidence)
	}

	if !output.NoColor {
		fmt.Println("2/3  재무지표 추출 중 ...")
	}
	fin, err := upstage.ExtractFinancials(apiKey, filePath)
	if err != nil {
		output.PrintError(err.Error())
		return
	}

	if reportOutputFmt == "json" {
		type out struct {
			DocType    string      `json:"doc_type"`
			Confidence float64     `json:"confidence"`
			Financials interface{} `json:"financials"`
		}
		data, _ := json.MarshalIndent(out{docType, confidence, fin}, "", "  ")
		fmt.Println(string(data))
		return
	}

	if !output.NoColor {
		fmt.Println("3/3  Solar Pro 3 분석 중 (reasoning=high) ...")
	}
	analysis, err := upstage.AnalyzeFinancials(apiKey, fin, docType)
	if err != nil {
		output.PrintError(err.Error())
		return
	}

	if output.AgentMode {
		output.PrintAgentReport(fin.CompanyName, "", analysis, fin)
		return
	}
	output.PrintFinancials(fin, false)
	output.PrintAnalysis(analysis)
	output.PrintFooter("finsight  ·  Upstage document-parse + information-extract + Solar Pro 3")
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.Flags().StringVarP(&reportTicker, "ticker", "t", "", "종목코드 (예: 005930)")
	reportCmd.Flags().StringVarP(&reportCompany, "company", "c", "", "회사명 (예: 삼성전자)")
	reportCmd.Flags().StringVar(&reportType, "type", "annual", "보고서 유형: annual | half | q1 | q3")
	reportCmd.Flags().StringVar(&reportFsDiv, "fs", "연결", "재무제표 구분: 연결 | 별도")
	reportCmd.Flags().StringVarP(&reportOutputFmt, "output", "o", "report", "출력 형식: report | json")
}
