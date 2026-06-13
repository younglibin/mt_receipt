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
	case "web":
		runWebCommand(os.Args[2:])
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

func saveUploadedFile(r *http.Request, fieldName string) (string, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if mkdirErr := os.MkdirAll("uploads", 0755); mkdirErr != nil {
		return "", mkdirErr
	}
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102_150405_000000000"), fieldName, ext)
	path, err := filepath.Abs(filepath.Join("uploads", fileName))
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
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func printUsage() {
	fmt.Println("用法:")
	fmt.Println("  go run . receipt [-receipt <receipt.xlsx|receipt.xls>] [-mt <MT.xlsx|MT.xls>] [-out <路径>] [-receipt-out <路径>] [-receipt-sheet 名称] [-mt-sheet 名称]")
	fmt.Println("  go run . settlement [-input <input.xlsx>] [-out <输出路径>]  (默认读取 File/result.xlsx，输出到 output/yyyyMMdd_HHmmss_settlement.xlsx)")
	fmt.Println("  go run . web [-addr :8080]  (启动本地页面)")
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
