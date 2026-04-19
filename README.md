# finsight

**한국 재무공시 분석 CLI** — DART OpenAPI + Upstage AI를 연결하는 Go 단일 바이너리

DART(전자공시시스템)에서 XBRL 재무제표를 자동 수집하거나, 로컬 PDF를 AI로 파싱하여 핵심 재무지표를 추출하고 Solar Pro 3의 심층 분석을 제공합니다.

```bash
finsight report --ticker 005930
```

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
finsight — DART: 005930
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  삼성전자  종목코드: 005930  corp_code: 00126380

  핵심 재무지표
  ──────────────────────────────────────────────────────────
  회사명        삼성전자
  회계 기간     2025년 사업보고서
  통화          KRW
  ──────────────────────────────────────────────────────────
  매출액        333.6조 (원)
  영업이익      43.6조 (원)
  당기순이익    45.2조 (원)
  자산총계      566.9조 (원)
  자본총계      436.3조 (원)
  부채총계      130.6조 (원)
  영업활동CF    85.3조 (원)
  EPS           6,605원
  ──────────────────────────────────────────────────────────
  영업이익률    13.1%
  순이익률      13.6%
  부채비율      29.9%
  ROE           10.4%
  ──────────────────────────────────────────────────────────

  AI 분석 — Solar Pro 3
  ──────────────────────────────────────────────────────────
  영업이익률·순이익률 모두 13% 수준의 높은 마진 유지.
  영업현금흐름이 순이익보다 40조 더 높아 현금 창출력 탁월.
  부채비율 23%로 재무구조 건전, 반도체 가격 사이클이 핵심 변수...
```

---

## 목차

- [기술 스택](#기술-스택)
- [데이터 파이프라인](#데이터-파이프라인)
- [설치](#설치)
- [커맨드 레퍼런스](#커맨드-레퍼런스)
- [출력 형식](#출력-형식)
- [실전 활용 패턴](#실전-활용-패턴)
- [Claude Code 연동](#claude-code--ai-agent-연동)
- [프로젝트 구조](#프로젝트-구조)

---

## 기술 스택

### 언어 & 런타임

| 항목 | 버전/세부 |
|------|----------|
| **Go** | 1.22+ — 단일 바이너리 크로스 컴파일, 의존성 최소화 |
| **cobra** | v1.8 — 서브커맨드 기반 CLI 프레임워크 |
| **fatih/color** | v1.17 — ANSI 컬러 출력 (CJK 너비 보정 포함) |
| **joho/godotenv** | v1.5 — `.env` 파일 자동 로드 |

### 외부 API

| API | 용도 | 인증 |
|-----|------|------|
| **DART OpenAPI** | XBRL 재무제표, 기업 코드, 공시 목록 | `DART_API_KEY` |
| **Upstage Document Parse** | PDF → Markdown (표 구조 보존, 차트 인식) | `UPSTAGE_API_KEY` |
| **Upstage Information Extract** | 비정형 문서 → 구조화 JSON (재무지표 10종) | `UPSTAGE_API_KEY` |
| **Upstage Document Classify** | 사업/반기/분기/감사보고서 자동 판별 | `UPSTAGE_API_KEY` |
| **Upstage Solar Pro 3** | `reasoning_effort=high` 재무 심층 분석 | `UPSTAGE_API_KEY` |

---

## 데이터 파이프라인

```
[DART 경로 — 권장]  finsight report --ticker 005930
  1. DART corp code 조회 (로컬 캐시 24h TTL)
  2. DART XBRL fnlttSinglAcntAll API → FinStmt 파싱
  3. Solar Pro 3 (reasoning=high) → 분석 텍스트
  ※ Upstage API 호출 없음 → API 비용 최소

[PDF 경로]  finsight report ./report.pdf
  1. Upstage document-classify → 문서 유형 판별
  2. Upstage information-extract → model.Financials JSON
  3. Solar Pro 3 (reasoning=high) → 분석 텍스트
```

---

## 설치

### 소스 빌드 (권장 — Go 1.22+)

```bash
git clone https://github.com/finsight-cli/finsight
cd finsight
go build -o finsight .
```

### go install

```bash
go install github.com/finsight-cli/finsight@latest
```

### API 키 설정

```bash
cp .env.example .env
# 편집기로 .env 열어 키 입력
```

또는 셸 환경변수로 직접 설정:

```bash
export UPSTAGE_API_KEY=up_xxxx    # https://console.upstage.ai
export DART_API_KEY=xxxx          # https://opendart.fss.or.kr
```

| 환경변수 | 필수 조건 |
|----------|----------|
| `UPSTAGE_API_KEY` | 항상 필수 |
| `DART_API_KEY` | `search` / `report --ticker` / `report --company` 사용 시 |

### 첫 실행 — DART 기업 코드 캐시 초기화

```bash
finsight cache refresh
# DART 기업 코드 다운로드 중 (수 초 소요) ...
# 갱신 완료: 42,318개 기업 코드
```

캐시는 `~/.finsight/corp_codes.json`에 저장되며 24시간 TTL로 자동 갱신됩니다.

---

## 커맨드 레퍼런스

> `--no-color` 는 모든 커맨드에서 사용할 수 있는 전역 플래그입니다.

---

### `report` — 풀 파이프라인 ★ 핵심 커맨드

DART 자동 조회와 로컬 PDF 분석을 하나의 커맨드로 처리합니다. 입력 유형에 따라 파이프라인을 자동 선택합니다.

#### DART 자동 조회 (권장)

```bash
# 종목코드 또는 회사명으로 조회
finsight report --ticker 005930
finsight report --company 삼성전자

# 보고서 유형 선택 (기본값: annual)
finsight report --ticker 005930 --type annual   # 사업보고서
finsight report --ticker 005930 --type half     # 반기보고서  (6월 말 기준)
finsight report --ticker 005930 --type q1       # 1분기보고서 (3월 말 기준)
finsight report --ticker 005930 --type q3       # 3분기보고서 (9월 말 기준)

# 재무제표 기준 선택 (기본값: 연결)
finsight report --ticker 005930                 # 연결재무제표
finsight report --ticker 005930 --fs 별도       # 별도재무제표

# 출력 형식
finsight report --ticker 005930 --output json   # 구조화 JSON
finsight report --ticker 005930 --no-color      # plain text (파이프·로그용)
```

**보고서 유형 코드 대응:**

| `--type` | DART 코드 | 설명 | 공시 시점 |
|----------|-----------|------|----------|
| `annual` | 11011 | 사업보고서 | 매년 3~4월 |
| `half`   | 11012 | 반기보고서 | 매년 8월 |
| `q1`     | 11013 | 1분기보고서 | 매년 5월 |
| `q3`     | 11014 | 3분기보고서 | 매년 11월 |

#### 로컬 PDF 분석

```bash
finsight report ./samsung_ir.pdf
```

> DART에 없는 해외 IR 자료, 감사보고서 등 로컬 PDF에 사용합니다.  
> `UPSTAGE_API_KEY` 만 있으면 되며 `DART_API_KEY` 는 불필요합니다.

---

### `search` — 기업 검색

DART 기업 코드 캐시를 바탕으로 회사명(부분 일치) 또는 종목코드로 검색합니다. 상장사가 우선 표시됩니다.

```bash
finsight search 삼성전자              # 회사명 부분 검색
finsight search 005930                # 종목코드 검색
finsight search 카카오 --filings      # 기업 + 최근 공시 목록
finsight search 현대 --limit 3        # 결과 수 제한 (기본값: 5)
```

| 플래그 | 기본값 | 설명 |
|--------|--------|------|
| `--filings` | `false` | 최근 공시 목록도 함께 표시 |
| `--limit` | `5` | 최대 표시 기업 수 |

---

### `cache` — DART 기업 코드 캐시 관리

```bash
finsight cache status                 # 캐시 상태 확인 (기본값)
finsight cache refresh                # 강제 갱신 (~4만 기업)
finsight cache clear                  # 캐시 삭제
```

> 서브커맨드를 생략하면 `status`로 동작합니다.

---

### `parse` — PDF → Markdown

`document-parse` API로 PDF를 Markdown으로 변환합니다. 재무제표 표 구조를 보존합니다.

```bash
finsight parse ./report.pdf                        # 기본 (mode=auto)
finsight parse ./report.pdf --tables-only          # 표(table)만 추출
finsight parse ./report.pdf --mode enhanced        # 복잡한 표·차트 처리
finsight parse ./report.pdf --mode standard        # 빠른 처리
```

| 플래그 | 기본값 | 설명 |
|--------|--------|------|
| `--mode` | `auto` | 파싱 모드: `standard` \| `enhanced` \| `auto` |
| `--tables-only` | `false` | 표 요소만 출력 |

---

### `extract` — 재무지표 구조화 추출

`information-extract` API로 PDF에서 핵심 재무지표 10종을 추출합니다.

```bash
finsight extract ./report.pdf                      # 표 형식 출력 (기본값)
finsight extract ./report.pdf --output json        # JSON 출력
```

**추출 항목:** 매출액 · 영업이익 · 당기순이익 · 자산총계 · 자본총계 · 부채총계 · 영업활동CF · EPS · 통화 · 단위

| 플래그 | 기본값 | 설명 |
|--------|--------|------|
| `--output` | `table` | 출력 형식: `table` \| `json` |

---

### `analyze` — AI 심층 분석

문서 분류 → 재무지표 추출 → Solar Pro 3 분석을 순차 실행합니다. `report`의 PDF 경로와 동일한 파이프라인이며, 항상 재무지표 테이블을 함께 출력합니다.

```bash
finsight analyze ./report.pdf
finsight analyze ./report.pdf --skip-classify      # 문서 분류 단계 생략
```

| 플래그 | 기본값 | 설명 |
|--------|--------|------|
| `--skip-classify` | `false` | 문서 분류 단계 건너뜀 |

---

## 출력 형식

| 플래그 | 설명 | 용도 |
|--------|------|------|
| 기본 | 컬러·볼드·정렬 적용 | 터미널 직접 확인 |
| `--no-color` | ANSI 없는 순수 텍스트 (전역 플래그) | 로그 파일·파이프 처리 |
| `--output json` | 구조화 JSON (stdout) | 자동화·데이터 수집 |

---

## 실전 활용 패턴

```bash
# jq로 특정 지표만 추출
finsight report --ticker 005930 --output json | jq '.financials.operating_profit'

# 복수 종목 일괄 수집 → JSONL
for ticker in 005930 000660 035420; do
  finsight report --ticker "$ticker" --no-color --output json >> results.jsonl
done

# 분기별 추이 비교
finsight report --ticker 005930 --type q1     --output json > q1.json
finsight report --ticker 005930 --type half   --output json > half.json
finsight report --ticker 005930 --type q3     --output json > q3.json
finsight report --ticker 005930 --type annual --output json > annual.json

# 연결 vs 별도 비교
finsight report --ticker 005930           --output json > consolidated.json
finsight report --ticker 005930 --fs 별도 --output json > separate.json
```

---

## Claude Code / AI Agent 연동

`.claude/commands/finsight.md` 가 포함되어 있어 Claude Code에서 `/finsight` 슬래시 커맨드로 바로 호출 가능합니다.

```
/finsight 삼성전자 2025년 3분기 재무 분석해줘
```

AI Agent가 파이프로 연동할 때는 `--no-color --output json` 조합을 권장합니다.

---

## 프로젝트 구조

```
finsight/
├── main.go
├── go.mod / go.sum
├── .env.example
├── cmd/                          CLI 커맨드 (cobra)
│   ├── root.go                   환경변수 로드, 전역 플래그 (--no-color)
│   ├── report.go                 풀 파이프라인 (DART + PDF 양 경로)
│   ├── search.go                 기업 검색 + 공시 목록
│   ├── analyze.go                AI 심층 분석 (PDF)
│   ├── extract.go                재무지표 구조화 추출 (PDF)
│   ├── parse.go                  PDF → Markdown 변환
│   └── cache.go                  DART 기업 코드 캐시 관리
├── internal/
│   ├── model/
│   │   └── financials.go         공유 데이터 타입 (Financials)
│   ├── upstage/
│   │   └── client.go             Upstage API 클라이언트
│   │                             (parse / extract / classify / solar-pro3)
│   ├── dart/
│   │   └── client.go             DART OpenAPI 클라이언트
│   │                             (corp code 캐시, XBRL, 공시 목록)
│   └── output/
│       └── printer.go            터미널 출력 (CJK 너비 보정, --no-color)
└── .claude/
    └── commands/finsight.md      Claude Code 슬래시 커맨드 정의
```

---

## 기여 / 개발

```bash
go build -o finsight .    # 빌드
go vet ./...              # 정적 분석
go mod tidy               # 의존성 정리
```

> `go.sum` 은 반드시 커밋에 포함하세요. 없으면 클린 환경에서 빌드가 실패합니다.

---

Powered by [Upstage API](https://console.upstage.ai) · [DART OpenAPI](https://opendart.fss.or.kr)
