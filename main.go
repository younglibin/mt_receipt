package main

import (
	"BX_MT_Project/service"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "receipt":
		runReceiptCommand(os.Args[2:])
	case "settlement":
		runSettlementCommand(os.Args[2:])
	default:
		printUsage()
		os.Exit(2)
	}
}

func runReceiptCommand(args []string) {
	// 1. 确保 output 目录存在
	outDir := "output"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("创建 output 目录失败: %v", err)
	}

	// 2. 默认文件名带 yyyyMMdd 前缀，并放到 output 目录下
	prefix := time.Now().Format("20060102_150405")
	defaultResult := filepath.Join(outDir, prefix+"_result.xlsx")
	defaultReceipt := filepath.Join(outDir, prefix+"_receipt_filled.xlsx")

	fs := flag.NewFlagSet("receipt", flag.ExitOnError)
	receiptPath := fs.String("receipt", "", "receipt 文件路径 (包含 C:采购订单号, K:未付款金额, 目标 P 列)")
	mtPath := fs.String("mt", "", "MT 文件路径 (包含 M:PO单号, L:预估费用, G:执行设计师)")
	outPath := fs.String("out", defaultResult, "输出的结果文件路径 (默认: output/yyyyMMdd_result.xlsx)")
	receiptOutPath := fs.String("receipt-out", defaultReceipt, "写回设计师后的 receipt 输出路径 (默认: output/yyyyMMdd_receipt_filled.xlsx)")
	receiptSheet := fs.String("receipt-sheet", "", "receipt 工作表名 (留空则取第一个工作表)")
	mtSheet := fs.String("mt-sheet", "", "MT 工作表名 (留空则取第一个工作表)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("解析 receipt 命令参数失败: %v", err)
	}

	designerConfigPath := "config/designers.json"

	// 3. 若未传参，则尝试读取 File 目录下的默认文件
	if *receiptPath == "" {
		*receiptPath = service.FindExisting([]string{"File/receipt.xlsx", "File/receipt.xls"})
	}
	if *mtPath == "" {
		*mtPath = service.FindExisting([]string{"File/MT.xlsx", "File/MT.xls"})
	}

	if *receiptPath == "" || *mtPath == "" {
		fmt.Println("用法: go run . receipt [-receipt <receipt.xlsx|receipt.xls>] [-mt <MT.xlsx|MT.xls>] [-out <路径>] [-receipt-out <路径>] [-receipt-sheet 名称] [-mt-sheet 名称]")
		os.Exit(2)
	}
	if _, err := os.Stat(*receiptPath); err != nil {
		log.Fatalf("找不到 receipt 文件: %v", err)
	}
	if _, err := os.Stat(*mtPath); err != nil {
		log.Fatalf("找不到 MT 文件: %v", err)
	}

	// ========== 新增：文件格式校验 ==========
	log.Println("开始校验文件格式...")

	// 校验 MT 文件表头
	templateMTPath := service.FindExisting([]string{"template/MT.xlsx", "template/MT.xls"})
	if templateMTPath == "" {
		log.Fatalf("找不到模板 MT 文件")
	}
	if err := service.ValidateMTHeader(*mtPath, templateMTPath); err != nil {
		log.Fatalf("MT 文件表头校验失败: %v", err)
	}
	log.Println("MT 文件表头校验通过")

	// 校验 receipt 文件表头
	templateReceiptPath := service.FindExisting([]string{"template/receipt.xlsx", "template/receipt.xls"})
	if templateReceiptPath == "" {
		log.Fatalf("找不到模板 receipt 文件")
	}
	if err := service.ValidateReceiptHeader(*receiptPath, templateReceiptPath); err != nil {
		log.Fatalf("receipt 文件表头校验失败: %v", err)
	}
	log.Println("receipt 文件表头校验通过")
	// ========== 校验结束 ==========

	// 4. 业务逻辑
	receiptSheetUsed, receiptRows, err := service.LoadRowsGeneric(*receiptPath, *receiptSheet)
	if err != nil {
		log.Fatalf("读取 receipt 失败: %v", err)
	}
	poToSum := service.SumUnpaidByPO(receiptRows)
	log.Printf("在工作表 %q 中统计到 %d 个 PO 的未付款金额汇总", receiptSheetUsed, len(poToSum))

	mtSheetUsed, mtRows, err := service.LoadRowsGeneric(*mtPath, *mtSheet)
	if err != nil {
		log.Fatalf("读取 MT 失败: %v", err)
	}
	header, matchedRows, matchedDiffs, poToDesigner := service.FilterMTRowsByPO(mtRows, poToSum)
	log.Printf("在工作表 %q 中匹配到 %d 行写入结果", mtSheetUsed, len(matchedRows))

	designerConfig, err := service.LoadDesignerConfig(designerConfigPath)
	if err != nil {
		log.Fatalf("读取设计师配置失败: %v", err)
	}
	designerRecordMap := service.BuildDesignerRecordMap(designerConfig.Records)

	if err := service.WriteResultXLSX(*outPath, header, matchedRows, matchedDiffs, designerRecordMap); err != nil {
		log.Fatalf("写入结果文件失败: %v", err)
	}
	log.Printf("已生成结果文件: %s", *outPath)

	if err := service.FillReceiptDesignerFromRows(receiptRows, receiptSheetUsed, *receiptOutPath, poToDesigner); err != nil {
		log.Fatalf("写回设计师到 receipt 失败: %v", err)
	}
	log.Printf("已写回 receipt 文件到: %s", *receiptOutPath)

	// 4) 以 receipt_filled 为准，按设计师汇总未付款，写到第 2 个 Sheet
	if err := service.SummarizeDesignerUnpaid(*receiptOutPath); err != nil {
		log.Fatalf("按设计师汇总未付款失败: %v", err)
	}
	log.Printf("已生成设计师汇总 Sheet")
}

func runSettlementCommand(args []string) {
	outDir := "output"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("创建 output 目录失败: %v", err)
	}

	fs := flag.NewFlagSet("settlement", flag.ExitOnError)
	settlementPath := fs.String("input", "", "结算拆分输入文件路径 (xlsx，留空则默认读取 File/result.xlsx)")
	settlementOutPath := fs.String("out", "", "结算拆分输出文件路径 (默认: output/yyyyMMdd_HHmmss_settlement.xlsx)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("解析 settlement 命令参数失败: %v", err)
	}

	if *settlementPath == "" {
		*settlementPath = service.FindExisting([]string{"File/result.xlsx"})
	}
	if *settlementPath == "" {
		fmt.Println("用法: go run . settlement [-input <input.xlsx>] [-out <输出路径>]")
		os.Exit(2)
	}
	if filepath.Ext(*settlementPath) != ".xlsx" {
		log.Fatalf("结算拆分仅支持 xlsx 文件: %s", *settlementPath)
	}
	if _, err := os.Stat(*settlementPath); err != nil {
		log.Fatalf("找不到结算拆分输入文件: %v", err)
	}
	if *settlementOutPath == "" {
		*settlementOutPath = service.BuildSettlementOutputPath(*settlementPath)
	}

	if err := service.GenerateSettlementSheets(*settlementPath, *settlementOutPath); err != nil {
		log.Fatalf("生成结算拆分文件失败: %v", err)
	}
	log.Printf("已生成结算拆分文件: %s", *settlementOutPath)
}

func printUsage() {
	fmt.Println("用法:")
	fmt.Println("  go run . receipt [-receipt <receipt.xlsx|receipt.xls>] [-mt <MT.xlsx|MT.xls>] [-out <路径>] [-receipt-out <路径>] [-receipt-sheet 名称] [-mt-sheet 名称]")
	fmt.Println("  go run . settlement [-input <input.xlsx>] [-out <输出路径>]  (默认读取 File/result.xlsx，输出到 output/yyyyMMdd_HHmmss_settlement.xlsx)")
}
