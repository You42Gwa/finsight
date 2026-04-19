// Package output handles terminal output with optional color support.
package output

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode"

	"github.com/fatih/color"
	"github.com/You42Gwa/finsight/internal/model"
)

// NoColor disables ANSI color output when true. Set via --no-color flag.
var NoColor bool

// AgentMode enables compact summary output and suppresses progress messages.
// Implies NoColor=true.
var AgentMode bool

var (
	styleBold   = color.New(color.Bold)
	styleCyan   = color.New(color.FgCyan)
	styleGreen  = color.New(color.FgGreen)
	styleYellow = color.New(color.FgYellow)
	styleRed    = color.New(color.FgRed)
	styleDim    = color.New(color.Faint)
)

func c(style *color.Color, s string) string {
	if NoColor {
		return s
	}
	return style.Sprint(s)
}

// visWidth returns the visible terminal width of s, counting CJK chars as 2.
func visWidth(s string) int {
	w := 0
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Han, r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func padRight(s string, width int) string {
	w := visWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// PrintError writes an error to stderr.
func PrintError(msg string) {
	fmt.Fprintln(os.Stderr, c(styleRed, "오류: ")+msg)
}

// PrintRule prints a section divider with title.
func PrintRule(title string) {
	line := strings.Repeat("━", 60)
	if NoColor {
		fmt.Println(strings.Repeat("=", 60))
		fmt.Println(title)
		fmt.Println(strings.Repeat("=", 60))
	} else {
		fmt.Println(c(styleBold, line))
		fmt.Println(c(styleBold, title))
		fmt.Println(c(styleBold, line))
	}
}

// PrintFooter prints a faint closing rule.
func PrintFooter(sub string) {
	if NoColor {
		fmt.Println(strings.Repeat("-", 60))
	} else {
		fmt.Println(c(styleDim, strings.Repeat("─", 60)))
		fmt.Println(c(styleDim, sub))
	}
}

// PrintDocType prints the document classification result.
func PrintDocType(docType string, confidence float64) {
	labels := map[string]string{
		"annual_report":  "사업보고서",
		"half_report":    "반기보고서",
		"quarter_report": "분기보고서",
		"audit_report":   "감사보고서",
		"prospectus":     "투자설명서/증권신고서",
		"others":         "기타",
	}
	label := labels[docType]
	if label == "" {
		label = docType
	}
	if NoColor {
		fmt.Printf("[문서 분류] %s  (신뢰도: %.2f)\n\n", label, confidence)
	} else {
		fmt.Printf("  %s  %s  신뢰도: %s\n\n",
			c(styleBold, "[문서 분류]"),
			c(styleCyan, label),
			c(styleGreen, fmt.Sprintf("%.2f", confidence)),
		)
	}
}

// PrintCorpInfo prints a company header line.
func PrintCorpInfo(corpName, stockCode, corpCode string) {
	if NoColor {
		fmt.Printf("[기업] %s  종목코드: %s  corp_code: %s\n\n", corpName, stockCode, corpCode)
	} else {
		fmt.Printf("\n  %s  종목코드: %s  corp_code: %s\n\n",
			c(styleBold, corpName),
			c(styleCyan, stockCode),
			c(styleDim, corpCode),
		)
	}
}

func fmtAmount(val float64, unit string) string {
	if val == 0 {
		return "N/A"
	}
	sign := ""
	if val < 0 {
		sign = "-"
	}
	abs := math.Abs(val)
	var s string
	switch {
	case abs >= 1e12:
		s = fmt.Sprintf("%.1f조", abs/1e12)
	case abs >= 1e8:
		s = fmt.Sprintf("%.0f억", abs/1e8)
	default:
		s = fmt.Sprintf("%.0f", abs)
	}
	return sign + s + " (" + unit + ")"
}

// PrintFinancials prints the financial metrics table.
// Pass asJSON=true for structured JSON output.
func PrintFinancials(fin *model.Financials, asJSON bool) {
	if asJSON {
		data, err := json.MarshalIndent(fin, "", "  ")
		if err != nil {
			PrintError("JSON 직렬화 실패: " + err.Error())
			return
		}
		fmt.Println(string(data))
		return
	}

	unit := fin.Unit
	if unit == "" {
		unit = "원"
	}
	currency := fin.Currency
	if currency == "" {
		currency = "KRW"
	}

	// Derived ratios
	opMargin, netMargin, debtRatio, roe := 0.0, 0.0, 0.0, 0.0
	if fin.Revenue != 0 {
		opMargin = fin.OperatingProfit / fin.Revenue * 100
		netMargin = fin.NetIncome / fin.Revenue * 100
	}
	if fin.TotalEquity != 0 {
		debtRatio = fin.TotalLiabilities / fin.TotalEquity * 100
		roe = fin.NetIncome / fin.TotalEquity * 100
	}

	epsStr := "N/A"
	if fin.EPS != 0 {
		epsStr = fmt.Sprintf("%.0f원", fin.EPS)
	}

	type row struct{ label, value string }
	rows := []row{
		{"회사명", fin.CompanyName},
		{"회계 기간", fin.FiscalPeriod},
		{"통화", currency},
		{"", ""},
		{"매출액", fmtAmount(fin.Revenue, unit)},
		{"영업이익", fmtAmount(fin.OperatingProfit, unit)},
		{"당기순이익", fmtAmount(fin.NetIncome, unit)},
		{"자산총계", fmtAmount(fin.TotalAssets, unit)},
		{"자본총계", fmtAmount(fin.TotalEquity, unit)},
		{"부채총계", fmtAmount(fin.TotalLiabilities, unit)},
		{"영업활동CF", fmtAmount(fin.OperatingCashflow, unit)},
		{"EPS", epsStr},
		{"", ""},
		{"영업이익률", fmt.Sprintf("%.1f%%", opMargin)},
		{"순이익률", fmt.Sprintf("%.1f%%", netMargin)},
		{"부채비율", fmt.Sprintf("%.1f%%", debtRatio)},
		{"ROE", fmt.Sprintf("%.1f%%", roe)},
	}

	// Column width (Korean-aware)
	w1 := 10
	for _, r := range rows {
		if v := visWidth(r.label); v > w1 {
			w1 = v
		}
	}
	tableW := w1 + 34

	divider := strings.Repeat("─", tableW)

	if NoColor {
		fmt.Println("\n[핵심 재무지표]")
		fmt.Println(strings.Repeat("-", tableW))
		for _, r := range rows {
			if r.label == "" {
				fmt.Println(strings.Repeat("-", tableW))
				continue
			}
			fmt.Printf("  %s  %s\n", padRight(r.label, w1), r.value)
		}
		fmt.Println(strings.Repeat("-", tableW))
	} else {
		fmt.Println()
		fmt.Println("  " + c(styleBold, "핵심 재무지표"))
		fmt.Println("  " + c(styleDim, divider))
		for _, r := range rows {
			if r.label == "" {
				fmt.Println("  " + c(styleDim, divider))
				continue
			}
			fmt.Printf("  %s  %s\n", c(styleCyan, padRight(r.label, w1)), r.value)
		}
		fmt.Println("  " + c(styleDim, divider))
		fmt.Println()
	}
}

// PrintAnalysis prints the AI analysis text.
func PrintAnalysis(text string) {
	divider := strings.Repeat("─", 60)
	if NoColor {
		fmt.Println("\n[AI 분석 — Solar Pro 3]")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println(text)
		fmt.Println(strings.Repeat("-", 60))
	} else {
		fmt.Println("  " + c(styleBold, c(styleYellow, "AI 분석 — Solar Pro 3")))
		fmt.Println("  " + c(styleDim, divider))
		for _, line := range strings.Split(text, "\n") {
			fmt.Println("  " + line)
		}
		fmt.Println("  " + c(styleDim, divider))
		fmt.Println()
	}
}

// PrintParse prints document parse markdown output.
func PrintParse(markdown string) {
	if NoColor {
		fmt.Println(markdown)
		return
	}
	fmt.Println()
	fmt.Println(c(styleBold, "  파싱 결과"))
	fmt.Println("  " + c(styleDim, strings.Repeat("─", 60)))
	fmt.Println(markdown)
	fmt.Println("  " + c(styleDim, strings.Repeat("─", 60)))
}

// PrintSearchHeader prints the search table header.
func PrintSearchHeader(query string) {
	if NoColor {
		fmt.Printf("검색 결과: \"%s\"\n", query)
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("  %-12s %-10s %s\n", "기업코드", "종목코드", "회사명")
		fmt.Println(strings.Repeat("-", 50))
	} else {
		fmt.Println()
		fmt.Printf("  %s\n", c(styleBold, fmt.Sprintf("검색 결과: \"%s\"", query)))
		fmt.Println("  " + c(styleDim, strings.Repeat("─", 50)))
		fmt.Printf("  %s  %s  %s\n",
			c(styleDim, padRight("기업코드", 12)),
			c(styleDim, padRight("종목코드", 8)),
			c(styleDim, "회사명"),
		)
		fmt.Println("  " + c(styleDim, strings.Repeat("─", 50)))
	}
}

// PrintSearchRow prints one company row.
func PrintSearchRow(corpCode, stockCode, corpName string) {
	sk := stockCode
	if strings.TrimSpace(sk) == "" {
		sk = "-"
	}
	if NoColor {
		fmt.Printf("  %-12s %-10s %s\n", corpCode, sk, corpName)
	} else {
		fmt.Printf("  %s  %s  %s\n",
			c(styleDim, padRight(corpCode, 12)),
			c(styleCyan, padRight(sk, 8)),
			c(styleBold, corpName),
		)
	}
}

// PrintFilingHeader prints the filing list header.
func PrintFilingHeader(corpName string) {
	if NoColor {
		fmt.Printf("\n  %s 최근 공시\n", corpName)
		fmt.Printf("  %-12s %-14s %s\n", "접수일", "접수번호", "보고서명")
		fmt.Println("  " + strings.Repeat("-", 60))
	} else {
		fmt.Printf("\n  %s\n", c(styleBold, c(styleCyan, corpName)+" 최근 공시"))
		fmt.Printf("  %s  %s  %s\n",
			c(styleDim, padRight("접수일", 10)),
			c(styleDim, padRight("접수번호", 14)),
			c(styleDim, "보고서명"),
		)
		fmt.Println("  " + c(styleDim, strings.Repeat("─", 60)))
	}
}

// PrintFilingRow prints one filing row.
func PrintFilingRow(rceptDt, rceptNo, reportNm string) {
	if NoColor {
		fmt.Printf("  %-12s %-14s %s\n", rceptDt, rceptNo, reportNm)
	} else {
		fmt.Printf("  %s  %s  %s\n",
			padRight(rceptDt, 10),
			c(styleDim, padRight(rceptNo, 14)),
			reportNm,
		)
	}
}

// Tip prints a dim hint message.
func Tip(msg string) {
	if NoColor {
		fmt.Println(msg)
	} else {
		fmt.Println(c(styleDim, msg))
	}
}

// fmtAmountPlain returns a scaled amount string without the unit suffix in parentheses.
func fmtAmountPlain(val float64, unit string) string {
	if val == 0 {
		return "N/A"
	}
	sign := ""
	if val < 0 {
		sign = "-"
	}
	abs := math.Abs(val)
	switch {
	case abs >= 1e12:
		return sign + fmt.Sprintf("%.1f조", abs/1e12)
	case abs >= 1e8:
		return sign + fmt.Sprintf("%.0f억", abs/1e8)
	default:
		return sign + fmt.Sprintf("%.0f%s", abs, unit)
	}
}

// PrintAgentReport prints a compact one-block summary for AI agent pipelines.
func PrintAgentReport(corpName, stockCode, analysis string, fin *model.Financials) {
	unit := fin.Unit
	if unit == "" {
		unit = "원"
	}
	opMargin, netMargin, debtRatio, roe := 0.0, 0.0, 0.0, 0.0
	if fin.Revenue != 0 {
		opMargin = fin.OperatingProfit / fin.Revenue * 100
		netMargin = fin.NetIncome / fin.Revenue * 100
	}
	if fin.TotalEquity != 0 {
		debtRatio = fin.TotalLiabilities / fin.TotalEquity * 100
		roe = fin.NetIncome / fin.TotalEquity * 100
	}
	ticker := ""
	if strings.TrimSpace(stockCode) != "" {
		ticker = " | " + stockCode
	}
	fmt.Printf("[%s%s | %s]\n", corpName, ticker, fin.FiscalPeriod)
	fmt.Printf("매출 %s | 영업이익 %s (%.1f%%) | 순이익 %s (%.1f%%)\n",
		fmtAmountPlain(fin.Revenue, unit),
		fmtAmountPlain(fin.OperatingProfit, unit), opMargin,
		fmtAmountPlain(fin.NetIncome, unit), netMargin,
	)
	line := fmt.Sprintf("자산 %s | 자본 %s | 부채비율 %.1f%% | ROE %.1f%%",
		fmtAmountPlain(fin.TotalAssets, unit),
		fmtAmountPlain(fin.TotalEquity, unit),
		debtRatio, roe,
	)
	if fin.EPS != 0 {
		line += fmt.Sprintf(" | EPS %.0f원", fin.EPS)
	}
	fmt.Println(line)
	if fin.OperatingCashflow != 0 {
		fmt.Printf("영업CF %s\n", fmtAmountPlain(fin.OperatingCashflow, unit))
	}
	if analysis != "" {
		fmt.Println()
		fmt.Println("[AI 분석]")
		fmt.Println(analysis)
	}
}

// CompareEntry holds one company's data for side-by-side comparison.
type CompareEntry struct {
	CorpName  string
	StockCode string
	Fin       *model.Financials
}

// PrintCompare prints a side-by-side comparison table for two companies.
func PrintCompare(entries []CompareEntry, asJSON bool) {
	if asJSON {
		type jsonEntry struct {
			CorpName   string            `json:"corp_name"`
			StockCode  string            `json:"stock_code"`
			Financials *model.Financials `json:"financials"`
		}
		var out []jsonEntry
		for _, e := range entries {
			out = append(out, jsonEntry{e.CorpName, e.StockCode, e.Fin})
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			PrintError("JSON 직렬화 실패: " + err.Error())
			return
		}
		fmt.Println(string(data))
		return
	}

	if len(entries) < 2 || entries[0].Fin == nil || entries[1].Fin == nil {
		PrintError("비교 데이터가 불완전합니다")
		return
	}

	// Derived ratios per entry
	type ratios struct{ opMargin, netMargin, debtRatio, roe float64 }
	calc := func(fin *model.Financials) ratios {
		r := ratios{}
		if fin.Revenue != 0 {
			r.opMargin = fin.OperatingProfit / fin.Revenue * 100
			r.netMargin = fin.NetIncome / fin.Revenue * 100
		}
		if fin.TotalEquity != 0 {
			r.debtRatio = fin.TotalLiabilities / fin.TotalEquity * 100
			r.roe = fin.NetIncome / fin.TotalEquity * 100
		}
		return r
	}

	r0 := calc(entries[0].Fin)
	r1 := calc(entries[1].Fin)

	unit0 := entries[0].Fin.Unit
	if unit0 == "" {
		unit0 = "원"
	}
	unit1 := entries[1].Fin.Unit
	if unit1 == "" {
		unit1 = "원"
	}

	epsStr := func(fin *model.Financials) string {
		if fin.EPS == 0 {
			return "N/A"
		}
		return fmt.Sprintf("%.0f원", fin.EPS)
	}

	type row struct {
		label string
		v0    string
		v1    string
	}
	rows := []row{
		{"회계 기간", entries[0].Fin.FiscalPeriod, entries[1].Fin.FiscalPeriod},
		{"", "", ""},
		{"매출액", fmtAmount(entries[0].Fin.Revenue, unit0), fmtAmount(entries[1].Fin.Revenue, unit1)},
		{"영업이익", fmtAmount(entries[0].Fin.OperatingProfit, unit0), fmtAmount(entries[1].Fin.OperatingProfit, unit1)},
		{"당기순이익", fmtAmount(entries[0].Fin.NetIncome, unit0), fmtAmount(entries[1].Fin.NetIncome, unit1)},
		{"자산총계", fmtAmount(entries[0].Fin.TotalAssets, unit0), fmtAmount(entries[1].Fin.TotalAssets, unit1)},
		{"자본총계", fmtAmount(entries[0].Fin.TotalEquity, unit0), fmtAmount(entries[1].Fin.TotalEquity, unit1)},
		{"부채총계", fmtAmount(entries[0].Fin.TotalLiabilities, unit0), fmtAmount(entries[1].Fin.TotalLiabilities, unit1)},
		{"영업활동CF", fmtAmount(entries[0].Fin.OperatingCashflow, unit0), fmtAmount(entries[1].Fin.OperatingCashflow, unit1)},
		{"EPS", epsStr(entries[0].Fin), epsStr(entries[1].Fin)},
		{"", "", ""},
		{"영업이익률", fmt.Sprintf("%.1f%%", r0.opMargin), fmt.Sprintf("%.1f%%", r1.opMargin)},
		{"순이익률", fmt.Sprintf("%.1f%%", r0.netMargin), fmt.Sprintf("%.1f%%", r1.netMargin)},
		{"부채비율", fmt.Sprintf("%.1f%%", r0.debtRatio), fmt.Sprintf("%.1f%%", r1.debtRatio)},
		{"ROE", fmt.Sprintf("%.1f%%", r0.roe), fmt.Sprintf("%.1f%%", r1.roe)},
	}

	// Dynamic column widths
	wLabel := 10
	for _, r := range rows {
		if v := visWidth(r.label); v > wLabel {
			wLabel = v
		}
	}
	wVal := 20
	for _, r := range rows {
		if v := visWidth(r.v0); v+2 > wVal {
			wVal = v + 2
		}
		if v := visWidth(r.v1); v+2 > wVal {
			wVal = v + 2
		}
	}
	// Clamp to avoid excessively wide columns
	if wVal > 28 {
		wVal = 28
	}

	totalW := wLabel + wVal*2 + 6
	divider := strings.Repeat("─", totalW)

	header0 := entries[0].CorpName
	header1 := entries[1].CorpName

	if NoColor {
		fmt.Println(strings.Repeat("=", totalW))
		fmt.Printf("  %s  %s  %s\n", padRight("항목", wLabel), padRight(header0, wVal), header1)
		fmt.Println(strings.Repeat("-", totalW))
		for _, r := range rows {
			if r.label == "" {
				fmt.Println(strings.Repeat("-", totalW))
				continue
			}
			fmt.Printf("  %s  %s  %s\n", padRight(r.label, wLabel), padRight(r.v0, wVal), r.v1)
		}
		fmt.Println(strings.Repeat("=", totalW))
	} else {
		fmt.Println()
		fmt.Printf("  %s  %s  %s\n",
			c(styleDim, padRight("항목", wLabel)),
			c(styleBold, padRight(header0, wVal)),
			c(styleBold, header1),
		)
		fmt.Println("  " + c(styleDim, divider))
		for _, r := range rows {
			if r.label == "" {
				fmt.Println("  " + c(styleDim, divider))
				continue
			}
			fmt.Printf("  %s  %s  %s\n",
				c(styleCyan, padRight(r.label, wLabel)),
				padRight(r.v0, wVal),
				r.v1,
			)
		}
		fmt.Println("  " + c(styleDim, divider))
		fmt.Println()
	}
}
