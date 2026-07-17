package service

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var settlementOutputHeaders = []string{
	"需求方事业部",
	"需求ID",
	"PO单号",
	"需求名称",
	"设计类型",
	"执行设计师",
	"结算公司金额",
	"实际金额",
	"开票日期",
	"结算公司",
}

func BuildSettlementOutputPath(inputPath string) string {
	prefix := time.Now().Format("20060102_150405")
	outDir := GetRuntimePathConfig().WebFileOutput
	if outDir == "" {
		outDir = "output"
	}
	return filepath.Join(outDir, prefix+"_settlement.xlsx")
}

func GenerateSettlementSheets(inputPath, outputPath string) error {
	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	sourceSheet := f.GetSheetName(0)
	if sourceSheet == "" {
		return fmt.Errorf("未找到可用工作表")
	}

	rows, err := f.GetRows(sourceSheet)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("工作表 %q 没有数据", sourceSheet)
	}

	headerIndex := buildHeaderIndex(rows[0])
	requiredSourceHeaders := []string{
		"需求方事业部",
		"需求ID",
		"PO单号",
		"需求名称",
		"设计类型",
		"执行设计师",
		"实际金额",
		"开票日期",
		"结算公司",
	}
	for _, header := range requiredSourceHeaders {
		if _, ok := headerIndex[header]; !ok {
			return fmt.Errorf("工作表 %q 缺少列: %s", sourceSheet, header)
		}
	}

	groupedRows := make(map[string][][]string)
	for _, row := range rows[1:] {
		settlementCompany := strings.TrimSpace(safeCol(row, headerIndex["结算公司"]))
		if settlementCompany == "" {
			settlementCompany = "未配置结算公司"
		}
		groupedRows[settlementCompany] = append(groupedRows[settlementCompany], row)
	}

	if len(groupedRows) == 0 {
		return fmt.Errorf("工作表 %q 没有可拆分的数据行", sourceSheet)
	}

	companyNames := make([]string, 0, len(groupedRows))
	for companyName := range groupedRows {
		companyNames = append(companyNames, companyName)
	}
	sort.Strings(companyNames)

	usedSheetNames := map[string]struct{}{
		sourceSheet: {},
	}
	for _, companyName := range companyNames {
		sheetName := uniqueSheetName(companyName, usedSheetNames)
		usedSheetNames[sheetName] = struct{}{}

		if index, err := f.NewSheet(sheetName); err == nil {
			f.SetActiveSheet(index)
		} else {
			return err
		}

		if err := writeSettlementSheet(f, sheetName, groupedRows[companyName], headerIndex); err != nil {
			return err
		}
	}

	return f.SaveAs(outputPath)
}

func buildHeaderIndex(headerRow []string) map[string]int {
	index := make(map[string]int, len(headerRow))
	for i, header := range headerRow {
		index[strings.TrimSpace(header)] = i
	}
	return index
}

func writeSettlementSheet(f *excelize.File, sheetName string, sourceRows [][]string, headerIndex map[string]int) error {
	for col, header := range settlementOutputHeaders {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			return err
		}
	}

	groupedByDesigner := make(map[string][][]string)
	for _, row := range sourceRows {
		designer := strings.TrimSpace(safeCol(row, headerIndex["执行设计师"]))
		groupedByDesigner[designer] = append(groupedByDesigner[designer], row)
	}

	designerNames := make([]string, 0, len(groupedByDesigner))
	for designer := range groupedByDesigner {
		designerNames = append(designerNames, designer)
	}
	sort.Strings(designerNames)

	rowNum := 2
	for idx, designer := range designerNames {
		rows := groupedByDesigner[designer]
		for _, sourceRow := range rows {
			outputRow := buildSettlementOutputRow(sourceRow, headerIndex)
			for col, value := range outputRow {
				cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
				if err := f.SetCellValue(sheetName, cell, value); err != nil {
					return err
				}
			}
			rowNum++
		}

		if idx < len(designerNames)-1 {
			rowNum += 2
		}
	}

	return nil
}

func buildSettlementOutputRow(sourceRow []string, headerIndex map[string]int) []string {
	return []string{
		safeCol(sourceRow, headerIndex["需求方事业部"]),
		safeCol(sourceRow, headerIndex["需求ID"]),
		safeCol(sourceRow, headerIndex["PO单号"]),
		safeCol(sourceRow, headerIndex["需求名称"]),
		safeCol(sourceRow, headerIndex["设计类型"]),
		safeCol(sourceRow, headerIndex["执行设计师"]),
		"",
		safeCol(sourceRow, headerIndex["实际金额"]),
		safeCol(sourceRow, headerIndex["开票日期"]),
		safeCol(sourceRow, headerIndex["结算公司"]),
	}
}

func uniqueSheetName(rawName string, used map[string]struct{}) string {
	name := sanitizeSheetName(strings.TrimSpace(rawName))
	if name == "" {
		name = "未命名"
	}
	if _, ok := used[name]; !ok {
		return name
	}

	for i := 1; ; i++ {
		suffix := fmt.Sprintf("_%d", i)
		trimmed := name
		if len([]rune(trimmed))+len([]rune(suffix)) > 31 {
			trimmed = truncateSheetName(trimmed, 31-len([]rune(suffix)))
		}
		candidate := trimmed + suffix
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func sanitizeSheetName(name string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", "?", "_", "*", "_", ":", "_", "[", "_", "]", "_")
	name = replacer.Replace(name)
	return truncateSheetName(name, 31)
}

func truncateSheetName(name string, maxLen int) string {
	runes := []rune(name)
	if len(runes) <= maxLen {
		return name
	}
	return string(runes[:maxLen])
}
