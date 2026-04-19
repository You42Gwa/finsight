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
		data, _ := json.MarshalIndent(fin, "", "  ")
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
