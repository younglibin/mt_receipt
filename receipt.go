package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// 填充receipt文件的设计师信息并排序
func fillReceiptDesignerFromRows(rows [][]string, sheetName, outPath string, poToDesigner map[string]string) error {
	if poToDesigner == nil {
		poToDesigner = make(map[string]string)
	}
	if len(rows) == 0 {
		return nil
	}

	// 1. 以原 receipt 为模板打开
	tmpl, err := excelize.OpenFile(outPath)
	if err != nil {
		tmpl = excelize.NewFile()
	}
	sheet := tmpl.GetSheetName(0) // 始终第一表
	if sheet == "" {
		sheet = "Sheet1"
	}

	// 2. 分离标题行和数据行
	var headerRow []string
	var dataRows [][]string

	if len(rows) > 0 {
		headerRow = rows[0] // 第一行为标题行
		dataRows = rows[1:] // 其余为数据行
	}

	// 3. 为数据行补设计师并排序
	// L列：品类经理，通常是第12列，索引为11
	const colLIdx = 11
	// P列：执行设计师，通常是第16列，索引为15
	const colPIdx = 15

	type rowInfo struct {
		cells       []string
		po          string // C 列
		categoryMgr string // L 列：品类经理
		designer    string // P 列：执行设计师
	}
	list := make([]rowInfo, 0, len(dataRows))
	for _, r := range dataRows {
		po := safeCol(r, colCIdx)
		categoryMgr := safeCol(r, colLIdx) // 获取 L 列品类经理
		designer := ""
		if d, ok := poToDesigner[strings.ToUpper(strings.TrimSpace(po))]; ok {
			designer = d
		}
		list = append(list, rowInfo{
			cells:       r,
			po:          po,
			categoryMgr: categoryMgr,
			designer:    designer,
		})
	}

	// 4. 排序数据行：先按 L 列品类经理名字，再按 P 列执行设计师名字
	sort.Slice(list, func(i, j int) bool {
		// 先按品类经理名字排序
		if list[i].categoryMgr != list[j].categoryMgr {
			return list[i].categoryMgr < list[j].categoryMgr
		}
		// 品类经理相同的情况下，按执行设计师名字排序
		return list[i].designer < list[j].designer
	})

	// 5. 写回数据：先写标题行，再写排序后的数据行
	rowNum := 1

	// 5.1 写标题行
	for col, val := range headerRow {
		cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
		_ = tmpl.SetCellValue(sheet, cell, val)
	}
	rowNum++

	// 5.2 写排序后的数据行
	for _, item := range list {
		newRow := make([]string, max(len(item.cells), colPIdx+1))
		copy(newRow, item.cells)
		newRow[colPIdx] = item.designer // 覆盖 P 列执行设计师
		for col, val := range newRow {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
			_ = tmpl.SetCellValue(sheet, cell, val)
		}
		rowNum++
	}

	return tmpl.SaveAs(outPath)
}

// 在 receipt_filled 文件中按设计师汇总未付款，写到第 2 个 Sheet
func summarizeDesignerUnpaid(xlsxPath string) error {
	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		return err
	}
	defer f.Save()

	// 1. 始终用第一个工作表
	dataSheet := f.GetSheetName(0)
	rows, err := f.GetRows(dataSheet)
	if err != nil {
		return err
	}
	if len(rows) < 2 { // 只有表头
		return nil
	}

	// 2. 按设计师累加未付款金额
	type summary struct {
		designer string
		total    float64
	}
	m := make(map[string]float64)
	var grandTotal float64       // 总金额变量
	for _, r := range rows[1:] { // 跳过表头
		designer := safeCol(r, colPIdx) // P 列
		amtStr := safeCol(r, colKIdx)   // K 列
		amt, _ := strconv.ParseFloat(strings.TrimSpace(amtStr), 64)
		m[designer] += amt
		grandTotal += amt // 累加总金额
	}

	// 3. 转切片并排序
	list := make([]summary, 0, len(m))
	for d, t := range m {
		list = append(list, summary{designer: d, total: t})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].designer < list[j].designer })

	// 4. 写到第 2 个 Sheet（没有就新建）
	sumSheet := "设计师汇总"
	idx, _ := f.GetSheetIndex(sumSheet)
	if idx < 0 {
		idx, _ = f.NewSheet(sumSheet)
	}
	f.SetActiveSheet(idx)

	// 5. 写表头
	_ = f.SetCellValue(sumSheet, "A1", "执行设计师")
	_ = f.SetCellValue(sumSheet, "B1", "未付款金额合计")

	// 6. 写数据
	for rowNum, item := range list {
		_ = f.SetCellValue(sumSheet, fmt.Sprintf("A%d", rowNum+2), item.designer)
		_ = f.SetCellValue(sumSheet, fmt.Sprintf("B%d", rowNum+2), item.total)
	}

	// 7. 写总计
	totalRowNum := len(list) + 3 // 总计行位于数据行下方一行（空一行分隔）
	_ = f.SetCellValue(sumSheet, fmt.Sprintf("A%d", totalRowNum), "总计")
	_ = f.SetCellValue(sumSheet, fmt.Sprintf("B%d", totalRowNum), grandTotal)

	return f.Save()
}
