// Package model defines shared data types used across packages.
package model

// Financials holds extracted financial metrics from a disclosure document.
type Financials struct {
	CompanyName       string  `json:"company_name"`
	FiscalPeriod      string  `json:"fiscal_period"`
	Revenue           float64 `json:"revenue"`
	OperatingProfit   float64 `json:"operating_profit"`
	NetIncome         float64 `json:"net_income"`
	TotalAssets       float64 `json:"total_assets"`
	TotalEquity       float64 `json:"total_equity"`
	TotalLiabilities  float64 `json:"total_liabilities"`
	OperatingCashflow float64 `json:"operating_cashflow"`
	EPS               float64 `json:"eps"`
	Currency          string  `json:"currency"`
	Unit              string  `json:"unit"`
}
