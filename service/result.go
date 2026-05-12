package service

import (
	"sort"

	"github.com/xuri/excelize/v2"
)

// 生成 result.xlsx：差异行整红字，W 列写差值，按 E列对接设计师第一个名字和 G列执行设计师排序
func WriteResultXLSX(outPath string, header []string, rows [][]string, diffs []float64) error {
	f := excelize.NewFile()
	sheet := "Sheet1"
	if _, err := f.NewSheet(sheet); err != nil {
		return err
	}

	// 1. 补 W 列表头
	header = append(header, "差值")

	// 2. 写表头
	for col, h := range header {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	// 3. 红色底色样式
	redStyle, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FF0000"}, Pattern: 1},
	})
	if err != nil {
		return err
	}

	// 4. 排序数据：先按 E 列（对接设计师）第一个名字，再按 G 列（执行设计师）
	// E列通常是第5列，索引为4
	const colEIdx = 4
	// G列通常是第7列，索引为6
	const colGIdx = 6

	// 创建一个结构体来存储行数据和对应的差值
	type rowWithDiff struct {
		cells []string
		diff  float64
	}

	// 将数据转换为结构体切片
	rowDiffs := make([]rowWithDiff, 0, len(rows))
	for i, row := range rows {
		diff := 0.0
		if i < len(diffs) {
			diff = diffs[i]
		}
		rowDiffs = append(rowDiffs, rowWithDiff{
			cells: row,
			diff:  diff,
		})
	}

	// 排序：先按 E 列对接设计师第一个名字，再按 G 列执行设计师名字
	sort.Slice(rowDiffs, func(i, j int) bool {
		// 获取 E 列对接设计师，并提取第一个名字
		对接DesignerI := safeCol(rowDiffs[i].cells, colEIdx)
		对接DesignerJ := safeCol(rowDiffs[j].cells, colEIdx)
		firstDesignerI := getFirstDesigner(对接DesignerI)
		firstDesignerJ := getFirstDesigner(对接DesignerJ)

		// 如果对接设计师第一个名字不同，按对接设计师排序
		if firstDesignerI != firstDesignerJ {
			return firstDesignerI < firstDesignerJ
		}

		// 如果对接设计师相同，按 G 列执行设计师名字排序
		执行DesignerI := safeCol(rowDiffs[i].cells, colGIdx)
		执行DesignerJ := safeCol(rowDiffs[j].cells, colGIdx)
		return 执行DesignerI < 执行DesignerJ
	})

	// 5. 写数据 + 差值 + 染色（使用排序后的行）
	for rowIdx, rowDiff := range rowDiffs {
		rowNum := rowIdx + 2
		row := rowDiff.cells
		diff := rowDiff.diff

		// 5.1 先写原始列
		for col, val := range row {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
			_ = f.SetCellValue(sheet, cell, val)
		}
		// 5.2 写 W 列差值
		diffCell, _ := excelize.CoordinatesToCellName(len(header), rowNum)
		_ = f.SetCellValue(sheet, diffCell, diff)

		// 5.3 差值≠0 → 整行染红
		if diff != 0 {
			left, _ := excelize.CoordinatesToCellName(1, rowNum)
			right, _ := excelize.CoordinatesToCellName(len(header), rowNum)
			_ = f.SetCellStyle(sheet, left, right, redStyle)
		}
	}

	return f.SaveAs(outPath)
}
