package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// 本地 OCR：把截图识别成文字，完全离线，不出 NAS。
//
// 引擎抽象成 OCRProvider：今天用 RapidOCR（PP-OCRv6 模型，见
// RapidOCRProvider），以后想换 Tesseract / 完整版 PaddleOCR /
// chineseocr_lite 时，只需再实现一个 Provider，不用改记账业务代码。
//
// 当前实现调 ppu-paddle-ocr v6.7.1（单文件命令行程序，内含 ONNX Runtime，
// PP-OCRv6 small 模型）——实测三张模拟账单（微信/支付宝/淘宝）金额、日期、
// 商户全部认对，单张约 5 秒。程序 + 模型由 build.sh 打进 .upk，
// 运行时不联网、不下载模型。
// ---------------------------------------------------------------------------

// OCRResult 是一次本地识别的结果。
type OCRResult struct {
	Text       string  // 识别出的全部文字，按行拼接
	Confidence float64 // 平均置信度（0~1），仅供参考
}

// OCRProvider 是本地 OCR 引擎的接口。
type OCRProvider interface {
	// Available 报告引擎是否就绪（程序 + 模型文件都在）。
	Available() bool
	// Recognize 把一张图片（JPEG/PNG 等）的字节认成文字。
	Recognize(ctx context.Context, imageBytes []byte) (OCRResult, error)
}

// RapidOCR 模型文件名（PP-OCRv6 small）。
const (
	rapidOCRDetModel  = "PP-OCRv6_small_det.ort"
	rapidOCRRecModel  = "PP-OCRv6_small_rec.ort"
	rapidOCRDictModel = "ppocrv6_dict.txt"
)

// RapidOCRProvider 用 ppu-paddle-ocr 命令行程序做识别。
type RapidOCRProvider struct {
	bin  string // ppu-paddle-ocr 可执行文件
	det  string // detection 模型
	rec  string // recognition 模型
	dict string // 字符表
}

// NewRapidOCRProvider 构造一个 RapidOCRProvider。路径为空表示未配置。
func NewRapidOCRProvider(bin, det, rec, dict string) *RapidOCRProvider {
	return &RapidOCRProvider{bin: bin, det: det, rec: rec, dict: dict}
}

// BinaryPath 返回 OCR 程序路径（供启动日志用）。
func (p *RapidOCRProvider) BinaryPath() string {
	if p == nil {
		return ""
	}
	return p.bin
}

// Available 检查程序可执行且三个模型文件都在。
func (p *RapidOCRProvider) Available() bool {
	if p == nil || p.bin == "" {
		return false
	}
	if st, err := os.Stat(p.bin); err != nil || st.Mode()&0111 == 0 {
		return false
	}
	for _, f := range []string{p.det, p.rec, p.dict} {
		if f == "" {
			return false
		}
		if _, err := os.Stat(f); err != nil {
			return false
		}
	}
	return true
}

// rapidOCRTimeout 是单张图片识别的最长等待（模型加载约 1~2 秒，识别约 3 秒）。
const rapidOCRTimeout = 60 * time.Second

// Recognize 调 ppu-paddle-ocr 识别一张图片，返回 --json 的 text 字段。
func (p *RapidOCRProvider) Recognize(ctx context.Context, imageBytes []byte) (OCRResult, error) {
	if !p.Available() {
		return OCRResult{}, errors.New("本地 OCR 不可用：未找到 RapidOCR 程序或模型文件")
	}
	if len(imageBytes) == 0 {
		return OCRResult{}, errors.New("图片为空")
	}
	tmp, err := os.CreateTemp("", "rapidocr-*"+sniffImageExt(imageBytes))
	if err != nil {
		return OCRResult{}, fmt.Errorf("创建临时图片失败: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(imageBytes); err != nil {
		_ = tmp.Close()
		return OCRResult{}, fmt.Errorf("写入临时图片失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return OCRResult{}, fmt.Errorf("关闭临时图片失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, rapidOCRTimeout)
	defer cancel()
	args := []string{
		"recognize", tmp.Name(), "--json",
		"--model-detection", p.det,
		"--model-recognition", p.rec,
		"--model-dict", p.dict,
	}
	cmd := exec.CommandContext(ctx, p.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return OCRResult{}, fmt.Errorf("RapidOCR 执行失败: %w（%s）", err, tailString(stderr.String(), 300))
	}
	var parsed struct {
		Text       string  `json:"text"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		// 兼容纯文本输出：直接把 stdout 当识别文字。
		text := strings.TrimSpace(stdout.String())
		if text == "" {
			return OCRResult{}, errors.New("RapidOCR 未识别出文字")
		}
		return OCRResult{Text: text}, nil
	}
	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		return OCRResult{}, errors.New("RapidOCR 未识别出文字")
	}
	return OCRResult{Text: text, Confidence: parsed.Confidence}, nil
}

// sniffImageExt 按文件头判断图片类型，供临时文件取扩展名。
func sniffImageExt(b []byte) string {
	if len(b) >= 4 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4E && b[3] == 0x47 {
		return ".png"
	}
	if len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return ".jpg"
	}
	return ".jpg"
}

func tailString(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ResolveRapidOCRProvider 在当前环境里找 RapidOCR 程序和模型。
// 查找顺序：环境变量 RAPIDOCR_BIN / RAPIDOCR_MODELS_DIR → 程序所在目录 →
// 当前工作目录 → PATH。找不到就返回 nil（本地 OCR 不可用，不报错）。
func ResolveRapidOCRProvider() *RapidOCRProvider {
	bin := strings.TrimSpace(os.Getenv("RAPIDOCR_BIN"))
	if bin == "" {
		bin = findRapidOCRBinary()
	}
	if bin == "" {
		return nil
	}
	modelsDir := strings.TrimSpace(os.Getenv("RAPIDOCR_MODELS_DIR"))
	det, rec, dict := findRapidOCRModels(modelsDir, bin)
	p := NewRapidOCRProvider(bin, det, rec, dict)
	if !p.Available() {
		return nil
	}
	return p
}

func findRapidOCRBinary() string {
	names := []string{"ppu-paddle-ocr", "ppu-paddle-ocr-linux-x64", "rapidocr"}
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd, filepath.Join(cwd, "bin"), filepath.Join(cwd, "third_party", "rapidocr"))
	}
	for _, dir := range dirs {
		for _, name := range names {
			cand := filepath.Join(dir, name)
			if st, err := os.Stat(cand); err == nil && st.Mode()&0111 != 0 {
				return cand
			}
		}
	}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func findRapidOCRModels(modelsDir, bin string) (det, rec, dict string) {
	var dirs []string
	if modelsDir != "" {
		dirs = append(dirs, modelsDir)
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "..", "ocr", "models"))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs,
			filepath.Join(cwd, "ocr", "models"),
			filepath.Join(cwd, "third_party", "rapidocr", "models"),
		)
	}
	for _, dir := range dirs {
		d := filepath.Join(dir, rapidOCRDetModel)
		r := filepath.Join(dir, rapidOCRRecModel)
		k := filepath.Join(dir, rapidOCRDictModel)
		if _, err := os.Stat(d); err == nil {
			if _, err := os.Stat(r); err == nil {
				if _, err := os.Stat(k); err == nil {
					return d, r, k
				}
			}
		}
	}
	return "", "", ""
}

// ---------------------------------------------------------------------------
// 账单来源识别 + 本地字段提取（不调模型时的兜底）。
// ---------------------------------------------------------------------------

// ocrBill 是从 OCR 文字里提取出的一笔账。
type ocrBill struct {
	RawText     string // OCR 原文
	AmountCents int64
	Title       string
	Category    string
	SubCategory string
	Wallet      string
	Date        string // YYYY-MM-DD，可能为空
	Source      string // wechat / alipay / jd / taobao / pdd / food / receipt
}

// billSourceNames 是各来源在提取提示词里的中文名。
var billSourceNames = map[string]string{
	"wechat":  "微信支付账单",
	"alipay":  "支付宝账单",
	"jd":      "京东订单",
	"taobao":  "淘宝/天猫订单",
	"pdd":     "拼多多订单",
	"food":    "外卖订单",
	"receipt": "小票/账单",
}

// detectBillSource 按关键词判断账单来源。先看发单平台（京东/淘宝/拼多多/
// 外卖），再看付款渠道（微信/支付宝），都没有就是通用小票。
func detectBillSource(text string) string {
	switch {
	case strings.Contains(text, "京东"):
		return "jd"
	case strings.Contains(text, "淘宝") || strings.Contains(text, "天猫"):
		return "taobao"
	case strings.Contains(text, "拼多多"):
		return "pdd"
	case strings.Contains(text, "美团") || strings.Contains(text, "饿了么") || strings.Contains(text, "外卖"):
		return "food"
	case strings.Contains(text, "微信支付") || strings.Contains(text, "微信") || strings.Contains(text, "财付通"):
		return "wechat"
	case strings.Contains(text, "支付宝") || strings.Contains(text, "花呗") || strings.Contains(text, "余额宝"):
		return "alipay"
	default:
		return "receipt"
	}
}

var (
	// 合计/实付这类标签后面跟的金额，如 “合计：¥86.50”“实付款 ¥199.00”。
	// 注意：标签和金额之间不能有 ¥ 或数字，否则 “实付款 共减¥48.2 合计¥63.8”
	// 会在“实付”处就匹配到优惠金额 48.2；取最后一个标签金额才是合计。
	billTotalAmountRe = regexp.MustCompile(`(?:合计|总计|实付|实付款|支付金额|应付金额|订单金额|总价|需支付|待支付)[^0-9¥￥\-]{0,6}[-¥￥]?\s*([0-9][0-9,]*\.[0-9]{1,2}|[0-9][0-9,]*)`)
	// 单独成行的金额，如支付宝/微信账单顶部大字 “-70.78”“-350.00”。
	// 比“取最后一个小数”可靠：优惠行 “优惠 -0.10” 不是单独成行，不会被误取。
	billStandaloneAmountRe = regexp.MustCompile(`^-?\s*¥?\s*[0-9][0-9,]*\.[0-9]{1,2}$`)
	// 任意带小数的金额，如 “-38.50”“¥199.00”。
	billDecimalAmountRe = regexp.MustCompile(`-?\s*¥?\s*([0-9][0-9,]*\.[0-9]{1,2})`)
	// 只有 ¥ 标记的整数金额，如 “¥38”。
	billYuanAmountRe = regexp.MustCompile(`[¥￥]\s*([0-9][0-9,]*)`)
	// 2026-11-25 / 2026年11月25日。
	billFullDateRe = regexp.MustCompile(`(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})`)
	// 11月25日（无年份，补当年）。
	billShortDateRe = regexp.MustCompile(`(\d{1,2})[-/月](\d{1,2})`)
	// 状态栏时钟，如 “20:36”。
	billClockRe = regexp.MustCompile(`^\d{1,2}:\d{2}`)
	// 单独成行的金额行（商户名提取时要跳过）。
	billAmountLineRe = regexp.MustCompile(`^-?\s*¥?\s*[0-9][0-9,]*(\.[0-9]{1,2})?$`)
)

// extractBillFromOCRTextLocal 不用模型，只靠正则和关键词从 OCR 文字里提字段。
// 拿不到可信金额时返回 nil（交给远程视觉或如实报错）。
func extractBillFromOCRTextLocal(text string) *ocrBill {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	amt := extractBillAmount(text)
	if amt <= 0 {
		return nil
	}
	b := &ocrBill{
		RawText:     text,
		AmountCents: amt,
		Source:      detectBillSource(text),
		Date:        extractBillDate(text),
		Wallet:      extractBillWallet(text),
	}
	b.Title = extractBillMerchant(text)
	if b.Title == "" {
		b.Title = "账单消费"
	}
	cat, sub := inferCategoryHintFromText(text, false)
	b.Category, b.SubCategory = cat, sub
	return b
}

func extractBillAmount(text string) int64 {
	// 标签金额取最后一个：账单里常先出现优惠（共减¥48.2），合计在后。
	var lastLabel string
	for _, m := range billTotalAmountRe.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			lastLabel = m[1]
		}
	}
	if lastLabel != "" {
		if cents := parseYuanTextToCents(strings.ReplaceAll(lastLabel, ",", "")); cents > 0 {
			return cents
		}
	}
	// 支付宝/微信账单顶部的大字金额单独成行，取第一个（优惠行不是单独成行）。
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !billStandaloneAmountRe.MatchString(line) {
			continue
		}
		num := strings.TrimLeft(line, "-¥￥ ")
		if cents := parseYuanTextToCents(strings.ReplaceAll(num, ",", "")); cents > 0 {
			return cents
		}
	}
	// 总价一般在最后：取最后一个带小数的金额。
	var last string
	for _, m := range billDecimalAmountRe.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			last = m[1]
		}
	}
	if last != "" {
		if cents := parseYuanTextToCents(strings.ReplaceAll(last, ",", "")); cents > 0 {
			return cents
		}
	}
	if m := billYuanAmountRe.FindStringSubmatch(text); len(m) > 1 {
		return parseYuanTextToCents(strings.ReplaceAll(m[1], ",", ""))
	}
	return 0
}

func extractBillDate(text string) string {
	if m := billFullDateRe.FindStringSubmatch(text); len(m) > 3 {
		d := fmt.Sprintf("%04s-%02s-%02s", m[1], m[2], m[3])
		if _, err := time.Parse("2006-01-02", d); err == nil {
			return d
		}
	}
	if m := billShortDateRe.FindStringSubmatch(text); len(m) > 2 {
		d := fmt.Sprintf("%04d-%02s-%02s", time.Now().Year(), m[1], m[2])
		if _, err := time.Parse("2006-01-02", d); err == nil {
			return d
		}
	}
	return ""
}

func extractBillWallet(text string) string {
	switch {
	case strings.Contains(text, "微信支付") || strings.Contains(text, "微信"):
		return "微信"
	case strings.Contains(text, "支付宝") || strings.Contains(text, "花呗") || strings.Contains(text, "余额宝"):
		return "支付宝"
	case strings.Contains(text, "京东白条"):
		return "京东白条"
	case strings.Contains(text, "银行卡") || strings.Contains(text, "储蓄卡") || strings.Contains(text, "信用卡"):
		return "银行卡"
	case strings.Contains(text, "现金"):
		return "现金"
	default:
		return ""
	}
}

var billMerchantSkipRe = regexp.MustCompile(`支付方式|付款方式|订单号|订单编号|交易成功|支付成功|订单详情|账单详情|交易详情|账单|实付|合计|总计|备注|商品|优惠|收单机构|交易单号|商户单号|发起群收款|申请电子凭证|在此商户的交易|完成|收起|进店|关联记录|账单管理|账单服务|交易服务|账单分类|喜欢|小程序|服务会员|更多|全部记录|^\d+$|¥|￥`)

// hasMerchantChars 判断一行是否含有可做商户名的字符（汉字或拉丁字母）。
// 状态栏 “20:36 5” 这类纯数字符号行没有，直接跳过。
func hasMerchantChars(line string) bool {
	for _, r := range line {
		if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// extractBillMerchant 取商户名：第一行不是平台名、不是标签、不是日期金额的行。
// 先跳过平台名找一遍，找不到再放开平台名找一遍——微信账单的商户本身可能就
// 是 “京东”，第一遍会把平台名行跳过，第二遍才能拿到它。
func extractBillMerchant(text string) string {
	if m := extractBillMerchantWith(text, true); m != "" {
		return m
	}
	return extractBillMerchantWith(text, false)
}

func extractBillMerchantWith(text string, skipPlatform bool) string {
	platforms := []string{"微信支付", "支付宝", "淘宝", "天猫", "京东", "拼多多", "美团", "饿了么"}
	minRunes := 3
	if !skipPlatform {
		minRunes = 2
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if utf8.RuneCountInString(line) < minRunes {
			continue
		}
		if strings.HasPrefix(line, "<") || strings.HasPrefix(line, ">") {
			continue
		}
		if billClockRe.MatchString(line) || !hasMerchantChars(line) {
			continue
		}
		if billAmountLineRe.MatchString(line) {
			continue
		}
		if skipPlatform {
			isPlatform := false
			for _, p := range platforms {
				if line == p {
					isPlatform = true
					break
				}
			}
			if isPlatform {
				continue
			}
		}
		if billMerchantSkipRe.MatchString(line) {
			continue
		}
		if billFullDateRe.MatchString(line) || billShortDateRe.MatchString(line) {
			continue
		}
		return line
	}
	return ""
}
