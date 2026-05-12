package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
)

// 安全取列，越界返回空串
func safeCol(row []string, idx int) string {
	if idx < len(row) {
		return row[idx]
	}
	return ""
}

// 统一入口：根据扩展名自动选择 xls 或 xlsx 读取
func loadRowsGeneric(path, sheetPref string) (string, [][]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".xls" {
		return loadRowsXLS(path, sheetPref)
	}
	return loadRowsXLSX(path, sheetPref)
}

func loadRowsXLSX(path, sheetPref string) (string, [][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	sheet := sheetPref
	if sheet == "" {
		sheets := f.GetSheetList()
		if len(sheets) > 0 {
			sheet = sheets[0]
		} else {
			sheet = "Sheet1"
		}
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return sheet, nil, err
	}
	return sheet, rows, nil
}

func loadRowsXLS(path, sheetPref string) (string, [][]string, error) {
	wb, err := xls.Open(path, "utf-8")
	if err != nil {
		return "", nil, err
	}

	var sheet *xls.WorkSheet
	var sheetName string

	if sheetPref != "" {
		for i := 0; ; i++ {
			s := wb.GetSheet(i)
			if s == nil {
				break
			}
			if s.Name == sheetPref {
				sheet = s
				sheetName = s.Name
				break
			}
		}
	}
	if sheet == nil {
		s := wb.GetSheet(0)
		if s == nil {
			return "", nil, fmt.Errorf("XLS 工作簿没有可用工作表")
		}
		sheet = s
		sheetName = s.Name
	}

	var rows [][]string
	max := int(sheet.MaxRow)
	for i := 0; i <= max; i++ {
		row := sheet.Row(i)
		if row == nil {
			rows = append(rows, []string{})
			continue
		}
		last := row.LastCol()
		if last <= 0 {
			rows = append(rows, []string{})
			continue
		}
		r := make([]string, last)
		for j := 0; j < last; j++ {
			r[j] = strings.TrimSpace(row.Col(j))
		}
		rows = append(rows, r)
	}
	return sheetName, rows, nil
}

// 提取对接设计师列的第一个名字（处理多个名字的情况）
func getFirstDesigner(designers string) string {
	// 可能的分隔符：逗号、顿号、空格等
	separators := []string{",", "、", " ", ";", "；"}

	for _, sep := range separators {
		if strings.Contains(designers, sep) {
			parts := strings.Split(designers, sep)
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					return trimmed
				}
			}
		}
	}

	// 如果没有分隔符，直接返回
	return strings.TrimSpace(designers)
}

// 在 paths 里找第一个存在的文件，找不到返回空串
func findExisting(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
