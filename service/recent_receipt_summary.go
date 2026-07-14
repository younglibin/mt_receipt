package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const designerSummarySheetName = "设计师汇总"

type DesignerUnpaidSummary struct {
	DesignerName string  `json:"designer_name"`
	UnpaidAmount float64 `json:"unpaid_amount"`
}

type RecentReceiptSummary struct {
	OutputDir  string                  `json:"output_dir"`
	FilePath   string                  `json:"file_path"`
	FileName   string                  `json:"file_name"`
	ModifiedAt string                  `json:"modified_at"`
	Designers  []DesignerUnpaidSummary `json:"designers"`
	Total      float64                 `json:"total"`
}

func GetLatestReceiptSummary(outputDir string) (*RecentReceiptSummary, error) {
	if strings.TrimSpace(outputDir) == "" {
		outputDir = GetRuntimePathConfig().WebFileOutput
	}
	if strings.TrimSpace(outputDir) == "" {
		outputDir = "output"
	}

	latestFile, info, err := findLatestReceiptFilledFile(outputDir)
	if err != nil {
		return nil, err
	}

	_, rows, err := LoadRowsGeneric(latestFile, designerSummarySheetName)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 的 %s 失败: %w", latestFile, designerSummarySheetName, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("文件 %s 中未找到有效的设计师汇总数据", latestFile)
	}

	designers := make([]DesignerUnpaidSummary, 0, len(rows)-1)
	var total float64
	for _, row := range rows[1:] {
		name := strings.TrimSpace(safeCol(row, 0))
		amountText := strings.TrimSpace(strings.ReplaceAll(safeCol(row, 1), ",", ""))
		if name == "" && amountText == "" {
			continue
		}

		amount, err := strconv.ParseFloat(amountText, 64)
		if err != nil {
			return nil, fmt.Errorf("解析设计师汇总金额失败，文件: %s, 设计师: %s, 金额: %s", latestFile, name, amountText)
		}

		if name == "总计" {
			total = amount
			continue
		}

		designers = append(designers, DesignerUnpaidSummary{
			DesignerName: name,
			UnpaidAmount: amount,
		})
	}

	if len(designers) == 0 {
		return nil, fmt.Errorf("文件 %s 中未找到设计师汇总明细", latestFile)
	}
	if total == 0 {
		for _, item := range designers {
			total += item.UnpaidAmount
		}
	}

	sort.Slice(designers, func(i, j int) bool {
		return designers[i].DesignerName < designers[j].DesignerName
	})

	return &RecentReceiptSummary{
		OutputDir:  outputDir,
		FilePath:   latestFile,
		FileName:   filepath.Base(latestFile),
		ModifiedAt: info.ModTime().Format(time.RFC3339),
		Designers:  designers,
		Total:      total,
	}, nil
}

func findLatestReceiptFilledFile(outputDir string) (string, os.FileInfo, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return "", nil, fmt.Errorf("读取输出目录失败: %s, err: %w", outputDir, err)
	}

	var latestPath string
	var latestInfo os.FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.Contains(strings.ToLower(name), "receipt") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".xlsx") && !strings.HasSuffix(strings.ToLower(name), ".xls") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return "", nil, fmt.Errorf("读取文件信息失败: %s, err: %w", name, err)
		}
		if latestInfo == nil || info.ModTime().After(latestInfo.ModTime()) {
			latestInfo = info
			latestPath = filepath.Join(outputDir, name)
		}
	}

	if latestInfo == nil {
		return "", nil, fmt.Errorf("目录 %s 下未找到 receipt 输出文件", outputDir)
	}

	return latestPath, latestInfo, nil
}
