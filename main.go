package main

import (
	"BX_MT_Project/service"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	mcpProtocolVersion = "2025-03-26"
	mcpServerName      = "bx-mt-project"
	mcpServerVersion   = "1.0.0"
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
	case "web":
		runWebCommand(os.Args[2:])
	default:
		printUsage()
		os.Exit(2)
	}
}

func runReceiptCommand(args []string) {
	// 1. 确保 output 目录存在
	outDir := service.GetRuntimePathConfig().WebFileOutput
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
		*receiptPath = service.FindExisting(buildDefaultInputCandidates("receipt.xlsx", "receipt.xls"))
	}
	if *mtPath == "" {
		*mtPath = service.FindExisting(buildDefaultInputCandidates("MT.xlsx", "MT.xls"))
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
	outDir := service.GetRuntimePathConfig().WebFileOutput
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
		*settlementPath = service.FindExisting(buildDefaultInputCandidates("result.xlsx"))
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

func runWebCommand(args []string) {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	addr := fs.String("addr", ":8080", "Web 服务监听地址")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("解析 web 命令参数失败: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndexPage)
	mux.HandleFunc("/api/receipt", handleReceiptAPI)
	mux.HandleFunc("/api/settlement", handleSettlementAPI)
	mux.HandleFunc("/api/recent-receipt-summary", handleRecentReceiptSummaryAPI)
	mux.HandleFunc("/mcp", handleMCPAPI)

	log.Printf("页面已启动: http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("启动 Web 服务失败: %v", err)
	}
}

func serveIndexPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func handleReceiptAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Message: "只支持 POST 请求"})
		return
	}
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: fmt.Sprintf("解析上传文件失败: %v", err)})
		return
	}

	mtPath, err := saveUploadedFile(r, "mtFile")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: fmt.Sprintf("保存 MT 文件失败: %v", err)})
		return
	}
	receiptPath, err := saveUploadedFile(r, "receiptFile")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: fmt.Sprintf("保存 receipt 文件失败: %v", err)})
		return
	}

	output, err := executeSubcommand("receipt", "-mt", mtPath, "-receipt", receiptPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Message: fmt.Sprintf("receipt 执行失败: %v", err), Output: output})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: "receipt 执行完成", Output: output})
}

func handleSettlementAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Message: "只支持 POST 请求"})
		return
	}
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: fmt.Sprintf("解析上传文件失败: %v", err)})
		return
	}

	resultPath, err := saveUploadedFile(r, "resultFile")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: fmt.Sprintf("保存 result 文件失败: %v", err)})
		return
	}

	output, err := executeSubcommand("settlement", "-input", resultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Message: fmt.Sprintf("settlement 执行失败: %v", err), Output: output})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: "settlement 执行完成", Output: output})
}

func handleRecentReceiptSummaryAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Message: "只支持 GET 请求"})
		return
	}

	outputDir := strings.TrimSpace(r.URL.Query().Get("output_dir"))
	summary, err := service.GetLatestReceiptSummary(outputDir)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		OK:      true,
		Message: "最近 receipt 汇总查询成功",
		Data:    summary,
	})
}

func handleMCPAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "MCP endpoint only supports POST", http.StatusMethodNotAllowed)
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMCPError(w, nil, -32700, fmt.Sprintf("解析 MCP 请求失败: %v", err))
		return
	}
	if req.JSONRPC != "2.0" {
		writeMCPError(w, req.ID, -32600, "只支持 JSON-RPC 2.0")
		return
	}

	switch req.Method {
	case "initialize":
		writeMCPResult(w, req.ID, buildInitializeResult())
	case "notifications/initialized":
		writeMCPNotificationAck(w)
	case "ping":
		writeMCPResult(w, req.ID, map[string]any{})
	case "tools/list":
		writeMCPResult(w, req.ID, map[string]any{
			"tools":      buildMCPTools(),
			"nextCursor": "",
		})
	case "tools/call":
		result, err := handleMCPToolCall(req.Params)
		if err != nil {
			writeMCPError(w, req.ID, -32602, err.Error())
			return
		}
		writeMCPResult(w, req.ID, result)
	default:
		writeMCPError(w, req.ID, -32601, fmt.Sprintf("不支持的方法: %s", req.Method))
	}
}

func saveUploadedFile(r *http.Request, fieldName string) (string, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	uploadDir, err := resolveWebUploadDir()
	if err != nil {
		return "", err
	}
	if mkdirErr := os.MkdirAll(uploadDir, 0755); mkdirErr != nil {
		return "", mkdirErr
	}
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102_150405_000000000"), fieldName, ext)
	path, err := filepath.Abs(filepath.Join(uploadDir, fileName))
	if err != nil {
		return "", err
	}
	dst, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, copyErr := io.Copy(dst, file); copyErr != nil {
		return "", copyErr
	}
	return path, nil
}

func buildDefaultInputCandidates(fileNames ...string) []string {
	cfg := service.GetRuntimePathConfig()
	baseDir := cfg.WebFile
	if baseDir == "" {
		baseDir = "File"
	}

	candidates := make([]string, 0, len(fileNames))
	for _, fileName := range fileNames {
		candidates = append(candidates, filepath.Join(baseDir, fileName))
	}
	return candidates
}

func resolveWebUploadDir() (string, error) {
	dir := service.GetRuntimePathConfig().WebFileUploads
	if dir == "" {
		dir = "uploads"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func executeSubcommand(args ...string) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(exePath, args...)
	cmd.Dir = workDir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	return output.String(), err
}

type apiResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeMCPResult(w http.ResponseWriter, id json.RawMessage, result any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeMCPError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcpError{
			Code:    code,
			Message: message,
		},
	})
}

func writeMCPNotificationAck(w http.ResponseWriter) {
	w.WriteHeader(http.StatusAccepted)
}

func buildInitializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{
				"listChanged": false,
			},
		},
		"serverInfo": map[string]any{
			"name":    mcpServerName,
			"version": mcpServerVersion,
		},
		"instructions": "提供 MT/receipt 算账、月度 settlement 拆分、最近一次 receipt 设计师汇总查询，以及按员工名查询最近一次结算明细能力。文件处理类 MCP 工具入参使用容器内可访问的文件路径。",
	}
}

func buildMCPTools() []map[string]any {
	return []map[string]any{
		{
			"name":        "receipt_calculate",
			"description": "根据 MT 文件路径和 receipt 文件路径执行算账，生成 result.xlsx 和 receipt_filled.xlsx。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"mt_file_path": map[string]any{
						"type":        "string",
						"description": "MT 文件路径，支持 .xls / .xlsx",
					},
					"receipt_file_path": map[string]any{
						"type":        "string",
						"description": "receipt 文件路径，支持 .xls / .xlsx",
					},
				},
				"required":             []string{"mt_file_path", "receipt_file_path"},
				"additionalProperties": false,
			},
		},
		{
			"name":        "monthly_settlement",
			"description": "根据 result.xlsx 文件路径执行月度算账，生成 settlement 输出文件。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"result_file_path": map[string]any{
						"type":        "string",
						"description": "result.xlsx 文件路径",
					},
				},
				"required":             []string{"result_file_path"},
				"additionalProperties": false,
			},
		},
		{
			"name":        "latest_receipt_summary",
			"description": "查询输出目录下最近一个 receipt 输出文件中的设计师汇总信息，返回设计师名字、未付款金额和总计。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"output_dir": map[string]any{
						"type":        "string",
						"description": "可选。输出目录路径；留空时默认使用配置中的 web.File.output。",
					},
				},
				"additionalProperties": false,
			},
		},
		{
			"name":        "latest_employee_settlement",
			"description": "输入员工名字，查询该员工最近一次结算信息。先从 designers.json 获取 designType 和 settlementCompany，再从最近的 settlement 文件对应 sheet 中返回该员工的所有结算行数据。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_name": map[string]any{
						"type":        "string",
						"description": "执行设计师名字，必须能在 config/designers.json 中找到。",
					},
					"output_dir": map[string]any{
						"type":        "string",
						"description": "可选。settlement 输出目录；留空时默认使用配置中的 web.File.output。",
					},
				},
				"required":             []string{"employee_name"},
				"additionalProperties": false,
			},
		},
	}
}

func handleMCPToolCall(rawParams json.RawMessage) (map[string]any, error) {
	var params mcpToolCallParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, fmt.Errorf("解析 tools/call 参数失败: %w", err)
	}

	switch params.Name {
	case "receipt_calculate":
		var args receiptMCPArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("解析 receipt_calculate 入参失败: %w", err)
		}
		if args.MTFilePath == "" || args.ReceiptFilePath == "" {
			return buildMCPToolResult("mt_file_path 和 receipt_file_path 不能为空", "", true), nil
		}
		if err := ensureFileExists(args.MTFilePath); err != nil {
			return buildMCPToolResult(err.Error(), "", true), nil
		}
		if err := ensureFileExists(args.ReceiptFilePath); err != nil {
			return buildMCPToolResult(err.Error(), "", true), nil
		}

		output, err := executeSubcommand("receipt", "-mt", args.MTFilePath, "-receipt", args.ReceiptFilePath)
		if err != nil {
			return buildMCPToolResult(fmt.Sprintf("receipt 执行失败: %v", err), output, true), nil
		}
		return buildMCPToolResult("receipt 执行完成", output, false), nil
	case "monthly_settlement":
		var args settlementMCPArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("解析 monthly_settlement 入参失败: %w", err)
		}
		if args.ResultFilePath == "" {
			return buildMCPToolResult("result_file_path 不能为空", "", true), nil
		}
		if err := ensureFileExists(args.ResultFilePath); err != nil {
			return buildMCPToolResult(err.Error(), "", true), nil
		}

		output, err := executeSubcommand("settlement", "-input", args.ResultFilePath)
		if err != nil {
			return buildMCPToolResult(fmt.Sprintf("settlement 执行失败: %v", err), output, true), nil
		}
		return buildMCPToolResult("settlement 执行完成", output, false), nil
	case "latest_receipt_summary":
		var args recentReceiptSummaryMCPArgs
		if len(params.Arguments) > 0 {
			if err := json.Unmarshal(params.Arguments, &args); err != nil {
				return nil, fmt.Errorf("解析 latest_receipt_summary 入参失败: %w", err)
			}
		}

		summary, err := service.GetLatestReceiptSummary(args.OutputDir)
		if err != nil {
			return buildMCPToolResult(err.Error(), "", true), nil
		}
		return buildRecentReceiptSummaryResult(summary), nil
	case "latest_employee_settlement":
		var args latestEmployeeSettlementMCPArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("解析 latest_employee_settlement 入参失败: %w", err)
		}
		result, err := service.GetLatestEmployeeSettlement(args.EmployeeName, args.OutputDir, "config/designers.json")
		if err != nil {
			return buildMCPToolResult(err.Error(), "", true), nil
		}
		return buildLatestEmployeeSettlementResult(result), nil
	default:
		return buildMCPToolResult(fmt.Sprintf("未知工具: %s", params.Name), "", true), nil
	}
}

func buildRecentReceiptSummaryResult(summary *service.RecentReceiptSummary) map[string]any {
	lines := []string{
		fmt.Sprintf("最近 receipt 汇总文件: %s", summary.FilePath),
		fmt.Sprintf("文件时间: %s", summary.ModifiedAt),
		"设计师汇总:",
	}
	for _, item := range summary.Designers {
		lines = append(lines, fmt.Sprintf("- %s: %.2f", item.DesignerName, item.UnpaidAmount))
	}
	lines = append(lines, fmt.Sprintf("总计: %.2f", summary.Total))

	text := strings.Join(lines, "\n")
	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": text,
			},
		},
		"isError": false,
		"structuredContent": map[string]any{
			"ok":      true,
			"message": "最近 receipt 汇总查询成功",
			"summary": summary,
		},
	}
}

func buildLatestEmployeeSettlementResult(result *service.LatestEmployeeSettlementResult) map[string]any {
	lines := []string{
		fmt.Sprintf("员工: %s", result.EmployeeName),
		fmt.Sprintf("设计类型: %s", result.DesignType),
		fmt.Sprintf("结算公司: %s", result.SettlementCompany),
		fmt.Sprintf("最近 settlement 文件: %s", result.SettlementFilePath),
		fmt.Sprintf("Sheet: %s", result.SheetName),
		fmt.Sprintf("文件时间: %s", result.ModifiedAt),
		"结算记录:",
	}
	for idx, row := range result.Rows {
		lines = append(lines, fmt.Sprintf("%d. %s", idx+1, formatSettlementRow(row, result.Headers)))
	}

	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": strings.Join(lines, "\n"),
			},
		},
		"isError": false,
		"structuredContent": map[string]any{
			"ok":      true,
			"message": "员工最近一次结算信息查询成功",
			"result":  result,
		},
	}
}

func formatSettlementRow(row map[string]string, headers []string) string {
	parts := make([]string, 0, len(headers))
	for _, header := range headers {
		key := strings.TrimSpace(header)
		if key == "" {
			continue
		}
		value := strings.TrimSpace(row[key])
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, "；")
}

func buildMCPToolResult(message, output string, isError bool) map[string]any {
	text := message
	if output != "" {
		text += "\n\n" + output
	}

	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": text,
			},
		},
		"isError": isError,
		"structuredContent": map[string]any{
			"ok":      !isError,
			"message": message,
			"output":  output,
		},
	}
}

func ensureFileExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("文件不可访问: %s, err: %v", path, err)
	}
	return nil
}

func printUsage() {
	fmt.Println("用法:")
	fmt.Println("  go run . receipt [-receipt <receipt.xlsx|receipt.xls>] [-mt <MT.xlsx|MT.xls>] [-out <路径>] [-receipt-out <路径>] [-receipt-sheet 名称] [-mt-sheet 名称]")
	fmt.Println("  go run . settlement [-input <input.xlsx>] [-out <输出路径>]  (默认读取 File/result.xlsx，输出到 output/yyyyMMdd_HHmmss_settlement.xlsx)")
	fmt.Println("  go run . web [-addr :8080]  (启动本地页面，同时提供 /mcp MCP 接口)")
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type receiptMCPArgs struct {
	MTFilePath      string `json:"mt_file_path"`
	ReceiptFilePath string `json:"receipt_file_path"`
}

type settlementMCPArgs struct {
	ResultFilePath string `json:"result_file_path"`
}

type recentReceiptSummaryMCPArgs struct {
	OutputDir string `json:"output_dir"`
}

type latestEmployeeSettlementMCPArgs struct {
	EmployeeName string `json:"employee_name"`
	OutputDir    string `json:"output_dir"`
}

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>算账工具</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f8fb;
      --card: #ffffff;
      --text: #172033;
      --muted: #667085;
      --primary: #2563eb;
      --primary-dark: #1d4ed8;
      --border: #d9e2ef;
      --success: #0f8f4d;
      --error: #c73636;
    }
    * {
      box-sizing: border-box;
    }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: var(--text);
      background: linear-gradient(135deg, #eef4ff 0%, var(--bg) 46%, #f7f7fb 100%);
    }
    main {
      width: min(960px, calc(100% - 32px));
      margin: 48px auto;
    }
    h1 {
      margin: 0 0 10px;
      font-size: 32px;
      letter-spacing: -0.02em;
    }
    .subtitle {
      margin: 0 0 28px;
      color: var(--muted);
      line-height: 1.6;
    }
    .card {
      margin-bottom: 22px;
      padding: 26px;
      border: 1px solid var(--border);
      border-radius: 18px;
      background: rgba(255, 255, 255, 0.92);
      box-shadow: 0 18px 45px rgba(30, 64, 175, 0.08);
    }
    .card h2 {
      margin: 0 0 18px;
      font-size: 22px;
    }
    .form-grid {
      display: grid;
      grid-template-columns: 1fr 1fr auto;
      gap: 16px;
      align-items: end;
    }
    .form-grid.single {
      grid-template-columns: 1fr auto;
    }
    label {
      display: block;
      margin-bottom: 8px;
      color: #344054;
      font-weight: 600;
    }
    input[type="file"] {
      width: 100%;
      padding: 11px;
      border: 1px solid var(--border);
      border-radius: 12px;
      background: #fff;
      color: var(--text);
    }
    button {
      min-width: 128px;
      height: 45px;
      border: 0;
      border-radius: 12px;
      background: var(--primary);
      color: #fff;
      font-size: 15px;
      font-weight: 700;
      cursor: pointer;
      transition: background 0.2s, transform 0.2s;
    }
    button:hover {
      background: var(--primary-dark);
      transform: translateY(-1px);
    }
    button:disabled {
      cursor: not-allowed;
      opacity: 0.65;
      transform: none;
    }
    .hint {
      margin-top: 14px;
      color: var(--muted);
      font-size: 14px;
      line-height: 1.6;
    }
    .result {
      display: none;
      margin-top: 18px;
      padding: 14px;
      border-radius: 12px;
      background: #f8fafc;
      border: 1px solid var(--border);
      white-space: pre-wrap;
      overflow-x: auto;
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 13px;
      line-height: 1.6;
    }
    .result.success {
      display: block;
      border-color: rgba(15, 143, 77, 0.35);
      color: var(--success);
    }
    .result.error {
      display: block;
      border-color: rgba(199, 54, 54, 0.35);
      color: var(--error);
    }
    @media (max-width: 760px) {
      .form-grid,
      .form-grid.single {
        grid-template-columns: 1fr;
      }
      button {
        width: 100%;
      }
    }
  </style>
</head>
<body>
  <main>
    <h1>算账工具</h1>
    <p class="subtitle">选择本地文件后点击按钮执行，服务端会把上传后的文件路径传给现有 receipt 或 settlement 命令。</p>

    <section class="card">
      <h2>第一组：算账</h2>
      <form id="receiptForm" class="form-grid">
        <div>
          <label for="mtFile">MT 文件</label>
          <input id="mtFile" name="mtFile" type="file" accept=".xls,.xlsx" required>
        </div>
        <div>
          <label for="receiptFile">receipt 文件</label>
          <input id="receiptFile" name="receiptFile" type="file" accept=".xls,.xlsx" required>
        </div>
        <button type="submit">执行</button>
      </form>
      <div class="hint">执行命令：receipt -mt &lt;MT文件路径&gt; -receipt &lt;receipt文件路径&gt;</div>
      <pre id="receiptResult" class="result"></pre>
    </section>

    <section class="card">
      <h2>第二组：月度算账</h2>
      <form id="settlementForm" class="form-grid single">
        <div>
          <label for="resultFile">result 文件</label>
          <input id="resultFile" name="resultFile" type="file" accept=".xlsx" required>
        </div>
        <button type="submit">月度算账</button>
      </form>
      <div class="hint">执行命令：settlement -input &lt;result文件路径&gt;</div>
      <pre id="settlementResult" class="result"></pre>
    </section>
  </main>

  <script>
    async function submitForm(form, url, outputElement) {
      const button = form.querySelector("button");
      outputElement.className = "result";
      outputElement.textContent = "执行中...";
      outputElement.style.display = "block";
      button.disabled = true;

      try {
        const response = await fetch(url, {
          method: "POST",
          body: new FormData(form)
        });
        const data = await response.json();
        outputElement.className = "result " + (data.ok ? "success" : "error");
        outputElement.textContent = data.message + (data.output ? "\n\n" + data.output : "");
      } catch (error) {
        outputElement.className = "result error";
        outputElement.textContent = "请求失败: " + error.message;
      } finally {
        button.disabled = false;
      }
    }

    document.getElementById("receiptForm").addEventListener("submit", function(event) {
      event.preventDefault();
      submitForm(event.currentTarget, "/api/receipt", document.getElementById("receiptResult"));
    });

    document.getElementById("settlementForm").addEventListener("submit", function(event) {
      event.preventDefault();
      submitForm(event.currentTarget, "/api/settlement", document.getElementById("settlementResult"));
    });
  </script>
</body>
</html>`
