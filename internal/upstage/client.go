// Package upstage implements Upstage API calls:
// document-parse, information-extract, document-classify, solar-pro3 chat.
package upstage

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/finsight-cli/finsight/internal/model"
)

const baseURL = "https://api.upstage.ai/v1"

// ParseResult holds document-parse API output.
type ParseResult struct {
	Elements []struct {
		Category string `json:"category"`
		Content  struct {
			Markdown string `json:"markdown"`
		} `json:"content"`
	} `json:"elements"`
	Content struct {
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"content"`
	Usage struct {
		Pages int `json:"pages"`
	} `json:"usage"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

var httpClient = &http.Client{Timeout: 120 * time.Second}

func doPost(apiKey, endpoint string, body io.Reader, contentType string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, baseURL+endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := httpClient.Do(req)
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

func fileToBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func mimeType(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".tiff", ".tif":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}

// documentMessage wraps a file (PDF or image) as a base64 image_url message.
func documentMessage(filePath string) (map[string]any, error) {
	b64, err := fileToBase64(filePath)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"role": "user",
		"content": []map[string]any{
			{
				"type": "image_url",
				"image_url": map[string]string{
					"url": "data:" + mimeType(filePath) + ";base64," + b64,
				},
			},
		},
	}, nil
}

// ParseDocument calls document-parse and returns the parsed result.
func ParseDocument(apiKey, filePath, mode string) (*ParseResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", "document-parse")
	_ = w.WriteField("mode", mode)
	_ = w.WriteField("output_formats", `["markdown","text"]`)
	_ = w.WriteField("chart_recognition", "true")
	_ = w.WriteField("merge_multipage_tables", "true")
	part, err := w.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(part, f); err != nil {
		return nil, err
	}
	w.Close()

	data, err := doPost(apiKey, "/document-digitization", &buf, w.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var result ParseResult
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

var financialSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"company_name":       map[string]any{"type": "string", "description": "회사명 (법인명)"},
		"fiscal_period":      map[string]any{"type": "string", "description": "회계 기간 (예: 2024년 3분기)"},
		"revenue":            map[string]any{"type": "number", "description": "매출액 (백만원). 없으면 0"},
		"operating_profit":   map[string]any{"type": "number", "description": "영업이익 (백만원). 손실이면 음수. 없으면 0"},
		"net_income":         map[string]any{"type": "number", "description": "당기순이익 (백만원). 손실이면 음수. 없으면 0"},
		"total_assets":       map[string]any{"type": "number", "description": "자산총계 (백만원). 없으면 0"},
		"total_equity":       map[string]any{"type": "number", "description": "자본총계 (백만원). 없으면 0"},
		"total_liabilities":  map[string]any{"type": "number", "description": "부채총계 (백만원). 없으면 0"},
		"operating_cashflow": map[string]any{"type": "number", "description": "영업활동 현금흐름 (백만원). 없으면 0"},
		"eps":                map[string]any{"type": "number", "description": "주당순이익 EPS (원). 없으면 0"},
		"currency":           map[string]any{"type": "string", "description": "통화 단위 (KRW, USD 등). 기본값 KRW"},
		"unit":               map[string]any{"type": "string", "description": "금액 단위 (백만원, 천원, 원 등)"},
	},
}

// ExtractFinancials calls information-extract and returns structured financials.
func ExtractFinancials(apiKey, filePath string) (*model.Financials, error) {
	msg, err := documentMessage(filePath)
	if err != nil {
		return nil, err
	}
	reqBody := map[string]any{
		"model":    "information-extract",
		"messages": []any{msg},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "financial_extraction",
				"schema": financialSchema,
			},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	data, err := doPost(apiKey, "/information-extraction", bytes.NewReader(bodyBytes), "application/json")
	if err != nil {
		return nil, err
	}
	var cr chatResponse
	if err = json.Unmarshal(data, &cr); err != nil {
		return nil, err
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("빈 응답")
	}
	var fin model.Financials
	if err = json.Unmarshal([]byte(cr.Choices[0].Message.Content), &fin); err != nil {
		return nil, err
	}
	return &fin, nil
}

var docClasses = []map[string]string{
	{"const": "annual_report", "description": "사업보고서 — 연간 전체 사업 내용 및 재무제표 포함"},
	{"const": "half_report", "description": "반기보고서 — 상반기 재무성과 및 사업 현황"},
	{"const": "quarter_report", "description": "분기보고서 — 분기별 재무성과 (1분기 또는 3분기)"},
	{"const": "audit_report", "description": "감사보고서 — 외부감사인의 재무제표 검토 의견"},
	{"const": "prospectus", "description": "투자설명서 또는 증권신고서"},
	{"const": "others", "description": "기타 문서"},
}

// ClassifyDocument returns document type and confidence score.
func ClassifyDocument(apiKey, filePath string) (string, float64, error) {
	msg, err := documentMessage(filePath)
	if err != nil {
		return "", 0, err
	}
	oneOf := make([]any, len(docClasses))
	for i, c := range docClasses {
		oneOf[i] = c
	}
	reqBody := map[string]any{
		"model":    "document-classify",
		"messages": []any{msg},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name": "document-classify",
				"schema": map[string]any{
					"type":  "string",
					"oneOf": oneOf,
				},
			},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, err
	}
	data, err := doPost(apiKey, "/document-classification", bytes.NewReader(bodyBytes), "application/json")
	if err != nil {
		return "", 0, err
	}
	var cr chatResponse
	if err = json.Unmarshal(data, &cr); err != nil {
		return "", 0, err
	}
	if len(cr.Choices) == 0 {
		return "", 0, fmt.Errorf("빈 응답")
	}
	docType := strings.TrimSpace(cr.Choices[0].Message.Content)
	var confidence float64
	if len(cr.Choices[0].Message.ToolCalls) > 0 {
		var args struct {
			DocumentType struct {
				ConfidenceScore float64 `json:"confidence_score"`
			} `json:"document_type"`
		}
		_ = json.Unmarshal(cr.Choices[0].Message.ToolCalls[0].Function.Arguments, &args)
		confidence = args.DocumentType.ConfidenceScore
	}
	return docType, confidence, nil
}

const analysisSysPrompt = `당신은 한국 주식시장 전문 재무 분석가입니다.
제공된 재무 데이터를 바탕으로 간결하고 실용적인 분석을 제공하세요.

분석 시 다음을 포함하세요:
1. 수익성 평가 (영업이익률, 순이익률)
2. 안정성 평가 (부채비율, 자본 건전성)
3. 현금흐름 평가
4. 주요 긍정 신호와 주의 요인
5. 다음 분기 주목 지표

응답은 한국어로, 투자자가 이해하기 쉽게 작성하세요.`

// AnalyzeFinancials sends financials to Solar Pro 3 (reasoning=high) and returns analysis.
func AnalyzeFinancials(apiKey string, fin *model.Financials, docType string) (string, error) {
	finJSON, err := json.MarshalIndent(fin, "", "  ")
	if err != nil {
		return "", err
	}
	reqBody := map[string]any{
		"model": "solar-pro3",
		"messages": []map[string]string{
			{"role": "system", "content": analysisSysPrompt},
			{"role": "user", "content": fmt.Sprintf("문서 유형: %s\n\n재무 데이터:\n%s", docType, string(finJSON))},
		},
		"reasoning_effort": "high",
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	data, err := doPost(apiKey, "/chat/completions", bytes.NewReader(bodyBytes), "application/json")
	if err != nil {
		return "", err
	}
	var cr chatResponse
	if err = json.Unmarshal(data, &cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("빈 응답")
	}
	return cr.Choices[0].Message.Content, nil
}
