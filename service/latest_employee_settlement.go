package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LatestEmployeeSettlementResult struct {
	EmployeeName       string              `json:"employee_name"`
	DesignType         string              `json:"design_type"`
	SettlementCompany  string              `json:"settlement_company"`
	SettlementFilePath string              `json:"settlement_file_path"`
	SettlementFileName string              `json:"settlement_file_name"`
	SheetName          string              `json:"sheet_name"`
	ModifiedAt         string              `json:"modified_at"`
	Headers            []string            `json:"headers"`
	Rows               []map[string]string `json:"rows"`
}

func GetLatestEmployeeSettlement(employeeName, outputDir, designerConfigPath string) (*LatestEmployeeSettlementResult, error) {
	name := strings.TrimSpace(employeeName)
	if name == "" {
		return nil, fmt.Errorf("employee_name 不能为空")
	}

	if strings.TrimSpace(outputDir) == "" {
		outputDir = GetRuntimePathConfig().WebFileOutput
	}
	if strings.TrimSpace(outputDir) == "" {
		outputDir = "output"
	}
	if strings.TrimSpace(designerConfigPath) == "" {
		designerConfigPath = "config/designers.json"
	}

	cfg, err := LoadDesignerConfig(designerConfigPath)
	if err != nil {
		return nil, fmt.Errorf("读取设计师配置失败: %w", err)
	}
	recordMap := BuildDesignerRecordMap(cfg.Records)
	record, ok := FindDesignerRecord(name, recordMap)
	if !ok {
		return nil, fmt.Errorf("designers.json 中未找到员工: %s", name)
	}

	settlementFilePath, info, err := findLatestSettlementFile(outputDir)
	if err != nil {
		return nil, err
	}

	sheetName, rows, err := loadSettlementCompanyRows(settlementFilePath, record.SettlementCompany)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("结算文件 %s 的 sheet %s 没有数据", settlementFilePath, sheetName)
	}

	headers := rows[0]
	matchedRows := make([]map[string]string, 0)
	for _, row := range rows[1:] {
		if strings.TrimSpace(safeCol(row, 5)) != name {
			continue
		}

		rowMap := make(map[string]string, len(headers))
		for i, header := range headers {
			key := strings.TrimSpace(header)
			if key == "" {
				key = fmt.Sprintf("column_%d", i+1)
			}
			rowMap[key] = safeCol(row, i)
		}
		matchedRows = append(matchedRows, rowMap)
	}

	if len(matchedRows) == 0 {
		return nil, fmt.Errorf("最近一次结算文件中未找到员工 %s 的结算记录", name)
	}

	return &LatestEmployeeSettlementResult{
		EmployeeName:       record.Name,
		DesignType:         record.DesignType,
		SettlementCompany:  record.SettlementCompany,
		SettlementFilePath: settlementFilePath,
		SettlementFileName: filepath.Base(settlementFilePath),
		SheetName:          sheetName,
		ModifiedAt:         info.ModTime().Format(time.RFC3339),
		Headers:            headers,
		Rows:               matchedRows,
	}, nil
}

func findLatestSettlementFile(outputDir string) (string, os.FileInfo, error) {
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

		name := strings.ToLower(entry.Name())
		if !strings.Contains(name, "settlement") || !strings.HasSuffix(name, ".xlsx") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return "", nil, fmt.Errorf("读取文件信息失败: %s, err: %w", entry.Name(), err)
		}
		if latestInfo == nil || info.ModTime().After(latestInfo.ModTime()) {
			latestInfo = info
			latestPath = filepath.Join(outputDir, entry.Name())
		}
	}

	if latestInfo == nil {
		return "", nil, fmt.Errorf("目录 %s 下未找到 settlement 输出文件", outputDir)
	}

	return latestPath, latestInfo, nil
}

func loadSettlementCompanyRows(filePath, settlementCompany string) (string, [][]string, error) {
	sheetName, rows, err := LoadRowsGeneric(filePath, settlementCompany)
	if err == nil {
		return sheetName, rows, nil
	}

	sanitizedSheetName := sanitizeSheetName(settlementCompany)
	if sanitizedSheetName != settlementCompany {
		sheetName, rows, retryErr := LoadRowsGeneric(filePath, sanitizedSheetName)
		if retryErr == nil {
			return sheetName, rows, nil
		}
	}

	return "", nil, fmt.Errorf("在结算文件 %s 中未找到结算公司 sheet: %s", filePath, settlementCompany)
}
