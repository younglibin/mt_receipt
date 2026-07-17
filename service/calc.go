package service

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

func normalizePO(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\x00' {
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}

	return strings.TrimSpace(b.String())
}

func getCell(row []string, idx int) string {
	if idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

func parseMoney(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg = true
		s = strings.TrimPrefix(strings.TrimSuffix(s, ")"), "(")
	}
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "￥", "")
	s = strings.ReplaceAll(s, "¥", "")
	s = strings.ReplaceAll(strings.ToUpper(s), "RMB", "")
	s = strings.ReplaceAll(strings.ToUpper(s), "CNY", "")
	s = strings.TrimSpace(s)

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		re := regexp.MustCompile(`[^0-9.\-]+`)
		clean := re.ReplaceAllString(s, "")
		v, err = strconv.ParseFloat(clean, 64)
		if err != nil {
			return 0, fmt.Errorf("无法解析金额 %q: %w", s, err)
		}
	}
	if neg {
		v = -v
	}
	return v, nil
}

// 在 receipt 中按 C 列分组，对 K 列求和
func SumUnpaidByPO(receiptRows [][]string) map[string]float64 {
	sums := make(map[string]float64)
	for i, r := range receiptRows {
		if i == 0 { // 跳过表头
			continue
		}
		po := normalizePO(getCell(r, colCIdx))
		if po == "" {
			continue
		}
		amtStr := getCell(r, colKIdx)
		if amtStr == "" {
			continue
		}
		amt, err := parseMoney(amtStr)
		if err != nil {
			log.Printf("行 %d: 解析未付款金额失败(%q): %v", i+1, amtStr, err)
			continue
		}
		sums[po] += amt
	}
	return sums
}

// 在 MT 中匹配 PO，返回：表头、匹配行、对应差值、PO->设计师映射
func FilterMTRowsByPO(mtRows [][]string, sums map[string]float64) (
	header []string,
	selected [][]string, // 匹配到的原始行
	selectedDiff []float64, // 对应的差值（AA - 预估费用）
	poToDesigner map[string]string,
) {
	if len(mtRows) == 0 {
		return nil, nil, nil, map[string]string{}
	}
	header = mtRows[0]
	var (
		rows        [][]string
		diffList    []float64
		designerMap = make(map[string]string)
	)
	for i := 1; i < len(mtRows); i++ {
		r := mtRows[i]
		po := normalizePO(getCell(r, colMIdx))
		if po == "" {
			continue
		}
		if aa, ok := sums[po]; ok {
			estStr := getCell(r, colLIdx)
			est, _ := parseMoney(estStr)
			diff := aa - est
			log.Printf("匹配 PO=%s: 预估费用=%.2f, 累加未付款(AA)=%.2f, 差值=%.2f", po, est, aa, diff)

			rows = append(rows, r)
			diffList = append(diffList, diff)
			if d := getCell(r, colGIdx); d != "" {
				designerMap[po] = d
			}
		}
	}
	return header, rows, diffList, designerMap
}
