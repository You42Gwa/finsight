// Package dart implements DART OpenAPI calls:
// corp code search, filing list, XBRL financial statements.
package dart

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/You42Gwa/finsight/internal/model"
)

const dartBase = "https://opendart.fss.or.kr/api"
const cacheTTL = 24 * time.Hour

var (
	cacheDir  string
	cacheFile string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	cacheDir = filepath.Join(home, ".finsight")
	cacheFile = filepath.Join(cacheDir, "corp_codes.json")
}

// CorpCode is a DART company entry.
type CorpCode struct {
	CorpCode   string `xml:"corp_code"   json:"corp_code"`
	CorpName   string `xml:"corp_name"   json:"corp_name"`
	StockCode  string `xml:"stock_code"  json:"stock_code"`
	ModifyDate string `xml:"modify_date" json:"modify_date"`
}

type corpResult struct {
	XMLName xml.Name   `xml:"result"`
	List    []CorpCode `xml:"list"`
}

// Filing is a DART disclosure entry.
type Filing struct {
	RceptNo  string `json:"rcept_no"`
	CorpName string `json:"corp_name"`
	CorpCode string `json:"corp_code"`
	ReportNm string `json:"report_nm"`
	RceptDt  string `json:"rcept_dt"`
	PblntfTy string `json:"pblntf_ty"`
}

// FinStmt is one row from the DART XBRL financial statement API.
type FinStmt struct {
	BsnsYear     string `json:"bsns_year"`
	ReprtCode    string `json:"reprt_code"`
	AccountID    string `json:"account_id"`
	ThstrmAmount string `json:"thstrm_amount"`
	FrmtrmAmount string `json:"frmtrm_amount"`
}

// ReprtInfo holds DART report type code and label.
type ReprtInfo struct{ Code, Label string }

// ReprtCodes maps user-facing type names to DART codes.
var ReprtCodes = map[string]ReprtInfo{
	"annual": {"11011", "사업보고서"},
	"half":   {"11012", "반기보고서"},
	"q1":     {"11013", "1분기보고서"},
	"q3":     {"11014", "3분기보고서"},
}

// FsDivCodes maps Korean labels to DART fs_div values.
var FsDivCodes = map[string]string{
	"연결": "CFS",
	"별도": "OFS",
}

// accountMap maps financial fields to ordered lists of XBRL account IDs.
// Exact-prefix matches (containing "_") are tried first; plain keywords fall back to substring.
var accountMap = map[string][]string{
	"revenue": {
		"ifrs-full_Revenue",
		"dart_Revenue",
		"SalesRevenue",
	},
	"operating_profit": {
		"dart_OperatingIncomeLoss",
		"ifrs-full_OperatingProfit",
		"ProfitLossFromOperatingActivities",
	},
	"net_income": {
		"ifrs-full_ProfitLoss",
		"dart_NetIncome",
		"NetIncomeLoss",
	},
	"total_assets": {
		"ifrs-full_Assets",
		"dart_Assets",
	},
	"total_equity": {
		"ifrs-full_Equity",
		"dart_Equity",
	},
	"total_liabilities": {
		"ifrs-full_Liabilities",
		"dart_Liabilities",
	},
	"operating_cashflow": {
		"ifrs-full_CashFlowsFromUsedInOperatingActivities",
		"ifrs-full_CashFlowsFromOperatingActivities",
		"dart_CashFlowsFromOperatingActivities",
		"CashFlowsFromUsedInOperatingActivities",
	},
}

// DART(opendart.fss.or.kr) serves TLS_RSA_WITH_AES_128_GCM_SHA256 (no forward secrecy),
// which Go 1.18+ removed from its default cipher list. Add it back explicitly.
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{ //nolint:gosec
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
		},
	},
}

func doGet(apiKey, endpoint string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(dartBase + endpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("crtfc_key", apiKey)
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	resp, err := httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func downloadCorpCodes(apiKey string) ([]CorpCode, error) {
	data, err := doGet(apiKey, "/corpCode.xml", nil)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, "CORPCODE.xml") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			xmlData, err := io.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			var result corpResult
			if err = xml.Unmarshal(xmlData, &result); err != nil {
				return nil, err
			}
			return result.List, nil
		}
	}
	return nil, fmt.Errorf("CORPCODE.xml not found in ZIP")
}

// GetCorpCodes returns cached corp codes (24h TTL). force=true re-downloads.
func GetCorpCodes(apiKey string, force bool) ([]CorpCode, error) {
	if !force {
		if info, err := os.Stat(cacheFile); err == nil {
			if time.Since(info.ModTime()) < cacheTTL {
				raw, err := os.ReadFile(cacheFile)
				if err == nil {
					var corps []CorpCode
					if json.Unmarshal(raw, &corps) == nil {
						return corps, nil
					}
				}
			}
		}
	}
	corps, err := downloadCorpCodes(apiKey)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	if raw, err := json.Marshal(corps); err == nil {
		_ = os.WriteFile(cacheFile, raw, 0o644)
	}
	return corps, nil
}

// RefreshCache forces a re-download and returns the total count.
func RefreshCache(apiKey string) (int, error) {
	corps, err := GetCorpCodes(apiKey, true)
	if err != nil {
		return 0, err
	}
	return len(corps), nil
}

// ClearCache removes the local corp code cache file.
func ClearCache() { _ = os.Remove(cacheFile) }

// CacheStatus returns existence, count, mod time, and file size.
func CacheStatus() (exists bool, count int, modTime time.Time, size int64) {
	info, err := os.Stat(cacheFile)
	if err != nil {
		return false, 0, time.Time{}, 0
	}
	raw, err := os.ReadFile(cacheFile)
	if err != nil {
		return true, 0, info.ModTime(), info.Size()
	}
	var corps []CorpCode
	_ = json.Unmarshal(raw, &corps)
	return true, len(corps), info.ModTime(), info.Size()
}

// SearchByName returns companies whose name contains the given substring.
// Listed companies (with stock code) are sorted first.
func SearchByName(apiKey, name string) ([]CorpCode, error) {
	corps, err := GetCorpCodes(apiKey, false)
	if err != nil {
		return nil, err
	}
	nameLower := strings.ToLower(name)
	var listed, unlisted []CorpCode
	for _, c := range corps {
		if strings.Contains(strings.ToLower(c.CorpName), nameLower) {
			if strings.TrimSpace(c.StockCode) != "" {
				listed = append(listed, c)
			} else {
				unlisted = append(unlisted, c)
			}
		}
	}
	return append(listed, unlisted...), nil
}

// SearchByStockCode finds a company by its 6-digit stock code.
func SearchByStockCode(apiKey, code string) (*CorpCode, error) {
	padded := fmt.Sprintf("%06d", mustAtoi(code))
	corps, err := GetCorpCodes(apiKey, false)
	if err != nil {
		return nil, err
	}
	for _, c := range corps {
		if strings.TrimSpace(c.StockCode) == padded {
			return &c, nil
		}
	}
	return nil, nil
}

// FindCompany searches by stock code (all digits) or company name.
// corpAliases maps common Korean display names to DART-registered corp names.
var corpAliases = map[string]string{
	"네이버":    "NAVER",
	"카카오모빌리티": "카카오모빌리티",
	"엔씨":     "엔씨소프트",
	"넥슨":     "NEXON",
	"크래프톤":   "크래프톤",
	"배민":     "우아한형제들",
}

func FindCompany(apiKey, query string) (*CorpCode, error) {
	if isDigits(query) {
		return SearchByStockCode(apiKey, query)
	}

	// alias 먼저 확인
	if mapped, ok := corpAliases[query]; ok {
		query = mapped
	}

	results, err := SearchByName(apiKey, query)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	// 정확 일치 우선
	for _, c := range results {
		if strings.EqualFold(c.CorpName, query) {
			return &c, nil
		}
	}

	// 상장사(종목코드 있는 곳) 첫 번째 선택
	for _, c := range results {
		if strings.TrimSpace(c.StockCode) != "" {
			return &c, nil
		}
	}

	// 상장사가 없으면 에러 — 비상장 법인으로 재무데이터 조회 시 DART 오류 방지
	return nil, fmt.Errorf(`"%s"에 해당하는 상장 기업을 찾을 수 없습니다. 종목코드로 다시 시도해보세요 (예: finsight search %s)`, query, query)
}

func isDigits(s string) bool {
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

// mustAtoi converts a digit-only string to int; returns 0 on parse failure.
func mustAtoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// GetFilings returns recent filings sorted by date (newest first).
func GetFilings(apiKey, corpCode, pblntfTy, bgnDe string, pageCount int) ([]Filing, error) {
	var all []Filing
	for _, ty := range strings.Split(pblntfTy, ",") {
		ty = strings.TrimSpace(ty)
		data, err := doGet(apiKey, "/list.json", map[string]string{
			"corp_code":  corpCode,
			"pblntf_ty":  ty,
			"bgn_de":     bgnDe,
			"page_count": strconv.Itoa(pageCount),
		})
		if err != nil {
			continue
		}
		var resp struct {
			Status string   `json:"status"`
			List   []Filing `json:"list"`
		}
		if json.Unmarshal(data, &resp) == nil && resp.Status == "000" {
			all = append(all, resp.List...)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].RceptDt > all[j].RceptDt })
	return all, nil
}

// GetFinancialStatements fetches XBRL data from DART.
func GetFinancialStatements(apiKey, corpCode, bsnsYear, reprtCode, fsDiv string) ([]FinStmt, error) {
	data, err := doGet(apiKey, "/fnlttSinglAcntAll.json", map[string]string{
		"corp_code":   corpCode,
		"bsns_year":  bsnsYear,
		"reprt_code": reprtCode,
		"fs_div":     fsDiv,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Status  string   `json:"status"`
		Message string   `json:"message"`
		List    []FinStmt `json:"list"`
	}
	if err = json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "000" {
		return nil, fmt.Errorf("DART API 오류: %s", resp.Message)
	}
	return resp.List, nil
}

func safeFloat(s string) float64 {
	s = strings.NewReplacer(",", "", " ", "").Replace(strings.TrimSpace(s))
	if s == "" || s == "-" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func matchAccount(id string, kws []string) bool {
	// Exact match first (for fully-qualified XBRL IDs like "ifrs-full_Revenue").
	for _, kw := range kws {
		if id == kw {
			return true
		}
	}
	// Substring fallback only for plain keywords (no namespace prefix).
	lower := strings.ToLower(id)
	for _, kw := range kws {
		if !strings.Contains(kw, "_") {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return true
			}
		}
	}
	return false
}

// ParseFinancials converts DART XBRL rows into a Financials struct.
func ParseFinancials(stmts []FinStmt, corpName string) *model.Financials {
	fin := &model.Financials{CompanyName: corpName, Currency: "KRW", Unit: "원"}
	if len(stmts) > 0 {
		reprtLabel := ""
		for _, ri := range ReprtCodes {
			if ri.Code == stmts[0].ReprtCode {
				reprtLabel = ri.Label
				break
			}
		}
		if reprtLabel != "" {
			fin.FiscalPeriod = stmts[0].BsnsYear + "년 " + reprtLabel
		} else {
			fin.FiscalPeriod = stmts[0].BsnsYear + "년"
		}
	}
	set := map[string]bool{}
	for _, s := range stmts {
		amount := safeFloat(s.ThstrmAmount)
		if amount == 0 {
			amount = safeFloat(s.FrmtrmAmount)
		}
		for field, kws := range accountMap {
			if !set[field] && matchAccount(s.AccountID, kws) {
				switch field {
				case "revenue":
					fin.Revenue = amount
				case "operating_profit":
					fin.OperatingProfit = amount
				case "net_income":
					fin.NetIncome = amount
				case "total_assets":
					fin.TotalAssets = amount
				case "total_equity":
					fin.TotalEquity = amount
				case "total_liabilities":
					fin.TotalLiabilities = amount
				case "operating_cashflow":
					fin.OperatingCashflow = amount
				}
				set[field] = true
			}
		}
		if !set["eps"] && matchAccount(s.AccountID, []string{"EarningsPerShare", "BasicEarningsLossPerShare"}) {
			fin.EPS = amount
			set["eps"] = true
		}
	}
	return fin
}

// FetchLatestFinancials is a high-level helper that finds the latest report
// and returns parsed financials, year, and report label.
func FetchLatestFinancials(apiKey, corpCode, corpName, reportType, fsDiv string) (*model.Financials, string, string, error) {
	rt, ok := ReprtCodes[reportType]
	if !ok {
		rt = ReprtCodes["annual"]
	}

	pblntfMap := map[string]string{"annual": "A", "half": "B", "q1": "C", "q3": "C"}
	pblntfTy := pblntfMap[reportType]
	if pblntfTy == "" {
		pblntfTy = "A"
	}

	bgnDe := strconv.Itoa(time.Now().Year()-2) + "0101"
	filings, _ := GetFilings(apiKey, corpCode, pblntfTy, bgnDe, 10)

	// bsns_year is the fiscal year, not the filing year.
	// Annual reports for year N are filed in year N+1, so subtract 1.
	// Half/quarterly reports are filed in the same fiscal year.
	yearOffset := 0
	if reportType == "annual" {
		yearOffset = 1
	}
	year := strconv.Itoa(time.Now().Year() - 1 - yearOffset)
	if len(filings) > 0 && len(filings[0].RceptDt) >= 4 {
		rceptYear, _ := strconv.Atoi(filings[0].RceptDt[:4])
		year = strconv.Itoa(rceptYear - yearOffset)
	}

	stmts, err := GetFinancialStatements(apiKey, corpCode, year, rt.Code, fsDiv)
	if (err != nil || len(stmts) == 0) && fsDiv == "CFS" {
		stmts, err = GetFinancialStatements(apiKey, corpCode, year, rt.Code, "OFS")
	}
	if err != nil {
		return nil, year, rt.Label, err
	}
	if len(stmts) == 0 {
		return nil, year, rt.Label, fmt.Errorf("%s %s년 재무제표 데이터가 없습니다", corpName, year)
	}
	fin := ParseFinancials(stmts, corpName)
	return fin, year, rt.Label, nil
}
