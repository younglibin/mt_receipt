package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// 1. 确保 output 目录存在
	outDir := "output"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("创建 output 目录失败: %v", err)
	}

	// 2. 默认文件名带 yyyyMMdd 前缀，并放到 output 目录下
	prefix := time.Now().Format("20060102_150405")
	defaultResult := filepath.Join(outDir, prefix+"_result.xlsx")
	defaultReceipt := filepath.Join(outDir, prefix+"_receipt_filled.xlsx")

	receiptPath := flag.String("receipt", "", "receipt 文件路径 (包含 C:采购订单号, K:未付款金额, 目标 P 列)")
	mtPath := flag.String("mt", "", "MT 文件路径 (包含 M:PO单号, L:预估费用, G:执行设计师)")
	outPath := flag.String("out", defaultResult, "输出的结果文件路径 (默认: output/yyyyMMdd_result.xlsx)")
	receiptOutPath := flag.String("receipt-out", defaultReceipt, "写回设计师后的 receipt 输出路径 (默认: output/yyyyMMdd_receipt_filled.xlsx)")
	receiptSheet := flag.String("receipt-sheet", "", "receipt 工作表名 (留空则取第一个工作表)")
	mtSheet := flag.String("mt-sheet", "", "MT 工作表名 (留空则取第一个工作表)")
	flag.Parse()

	// 3. 若未传参，则尝试读取 File 目录下的默认文件
	if *receiptPath == "" {
		*receiptPath = findExisting([]string{"File/receipt.xlsx", "File/receipt.xls"})
	}
	if *mtPath == "" {
		*mtPath = findExisting([]string{"File/MT.xlsx", "File/MT.xls"})
	}

	if *receiptPath == "" || *mtPath == "" {
		fmt.Println("用法: go run . [-receipt <receipt.xlsx|receipt.xls>] [-mt <MT.xlsx|MT.xls>] [-out <路径>] [-receipt-out <路径>] [-receipt-sheet 名称] [-mt-sheet 名称]")
		os.Exit(2)
	}
	if _, err := os.Stat(*receiptPath); err != nil {
		log.Fatalf("找不到 receipt 文件: %v", err)
	}
	if _, err := os.Stat(*mtPath); err != nil {
		log.Fatalf("找不到 MT 文件: %v", err)
	}

	// 4. 业务逻辑
	receiptSheetUsed, receiptRows, err := loadRowsGeneric(*receiptPath, *receiptSheet)
	if err != nil {
		log.Fatalf("读取 receipt 失败: %v", err)
	}
	poToSum := sumUnpaidByPO(receiptRows)
	log.Printf("在工作表 %q 中统计到 %d 个 PO 的未付款金额汇总", receiptSheetUsed, len(poToSum))

	mtSheetUsed, mtRows, err := loadRowsGeneric(*mtPath, *mtSheet)
	if err != nil {
		log.Fatalf("读取 MT 失败: %v", err)
	}
	header, matchedRows, matchedDiffs, poToDesigner := filterMTRowsByPO(mtRows, poToSum)
	log.Printf("在工作表 %q 中匹配到 %d 行写入结果", mtSheetUsed, len(matchedRows))

	if err := writeResultXLSX(*outPath, header, matchedRows, matchedDiffs); err != nil {
		log.Fatalf("写入结果文件失败: %v", err)
	}
	log.Printf("已生成结果文件: %s", *outPath)

	if err := fillReceiptDesignerFromRows(receiptRows, receiptSheetUsed, *receiptOutPath, poToDesigner); err != nil {
		log.Fatalf("写回设计师到 receipt 失败: %v", err)
	}
	log.Printf("已写回 receipt 文件到: %s", *receiptOutPath)

	// 4) 以 receipt_filled 为准，按设计师汇总未付款，写到第 2 个 Sheet
	if err := summarizeDesignerUnpaid(*receiptOutPath); err != nil {
		log.Fatalf("按设计师汇总未付款失败: %v", err)
	}
	log.Printf("已生成设计师汇总 Sheet")
}
