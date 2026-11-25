package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"com.xsguyu.accounting/backend/internal/model"
)

const (
	ocrWechatText = "微信支付\n星巴克咖啡\n-38.50\n支付成功\n2026年11月25日 12:30:45\n支付方式\n微信支付 · 零钱\n订单号 4200001234567890\n备注 大杯燕麦拿铁"
	ocrAlipayText = "支付宝\n盒马鲜生\n-86.50\n交易成功\n2026-11-24 19:05:12\n付款方式\n花呗\n订单号 2026112422001234567890\n商品 有机鲜牛奶 全麦吐司 蓝莓"
	ocrTaobaoText = "淘宝\n订单详情\n优衣库官方旗舰店\n男装摇粒绒外套\n实付款 ¥199.00\n2026-11-23 21:18:09\n付款方式 支付宝\n订单编号 1234567890123456789"
)

func TestDetectBillSource(t *testing.T) {
	cases := []struct{ text, want string }{
		{ocrWechatText, "wechat"},
		{ocrAlipayText, "alipay"},
		{ocrTaobaoText, "taobao"},
		{"京东\n商品\n实付 ¥59.00", "jd"},
		{"拼多多\n商品\n实付 ¥12.90", "pdd"},
		{"美团外卖\n商家\n实付 ¥25.50", "food"},
		{"饿了么\n商家\n实付 ¥25.50", "food"},
		{"小票\n合计 ¥38.00", "receipt"},
	}
	for _, c := range cases {
		if got := detectBillSource(c.text); got != c.want {
			t.Errorf("detectBillSource(%q) = %q, want %q", c.text[:12], got, c.want)
		}
	}
}

func TestExtractBillFromOCRTextLocal(t *testing.T) {
	cases := []struct {
		text     string
		amount   int64
		date     string
		merchant string
		wallet   string
		source   string
	}{
		{ocrWechatText, 3850, "2026-11-25", "星巴克咖啡", "微信", "wechat"},
		{ocrAlipayText, 8650, "2026-11-24", "盒马鲜生", "支付宝", "alipay"},
		{ocrTaobaoText, 19900, "2026-11-23", "优衣库官方旗舰店", "支付宝", "taobao"},
	}
	for _, c := range cases {
		b := extractBillFromOCRTextLocal(c.text)
		if b == nil {
			t.Fatalf("extractBillFromOCRTextLocal(%q) = nil", c.text[:12])
		}
		if b.AmountCents != c.amount {
			t.Errorf("amount = %d, want %d (%q)", b.AmountCents, c.amount, c.text[:12])
		}
		if b.Date != c.date {
			t.Errorf("date = %q, want %q (%q)", b.Date, c.date, c.text[:12])
		}
		if b.Title != c.merchant {
			t.Errorf("title = %q, want %q (%q)", b.Title, c.merchant, c.text[:12])
		}
		if b.Wallet != c.wallet {
			t.Errorf("wallet = %q, want %q (%q)", b.Wallet, c.wallet, c.text[:12])
		}
		if b.Source != c.source {
			t.Errorf("source = %q, want %q (%q)", b.Source, c.source, c.text[:12])
		}
	}
	// 没有可信金额 → nil。
	if b := extractBillFromOCRTextLocal("微信支付\n星巴克咖啡\n支付成功\n订单号 123"); b != nil {
		t.Errorf("no-amount text should return nil, got %+v", b)
	}
}

// stubOCRProvider 是测试用的假本地 OCR：直接返回写死的文字。
type stubOCRProvider struct {
	text      string
	err       error
	available bool
}

func (s stubOCRProvider) Available() bool { return s.available }
func (s stubOCRProvider) Recognize(_ context.Context, _ []byte) (OCRResult, error) {
	if s.err != nil {
		return OCRResult{}, s.err
	}
	return OCRResult{Text: s.text, Confidence: 0.99}, nil
}

func saveOCRTestConfig(t *testing.T, aiSvc *AiService, stubURL string) {
	t.Helper()
	cfg := &model.AiConfig{Enabled: true, OcrEnabled: true, Provider: "stub", BaseURL: stubURL, APIKey: "sk-test", Model: "stub-model", DailyLimit: 30}
	if err := aiSvc.settingsRepo.SaveAiConfig(context.Background(), cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}
}

func TestOCRWithLocalOCRAndModel(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	aiSvc.SetOCRProvider(stubOCRProvider{text: ocrWechatText, available: true})
	// 模型按来源提取：返回微信账单的字段。
	_, srv := newScriptedStubServer(t, textResp(`{"date":"2026-11-25","amount_yuan":38.50,"merchant":"星巴克咖啡","title":"星巴克咖啡","category":"餐饮","sub_category":"咖啡饮品","wallet":"微信支付"}`))
	saveOCRTestConfig(t, aiSvc, srv.URL)

	_, draft, rawText, _, err := aiSvc.OCR(context.Background(), "u1", bookA, "", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "wechat.jpg")
	if err != nil {
		t.Fatalf("OCR: %v", err)
	}
	if draft == nil {
		t.Fatal("draft is nil")
	}
	if draft.Amount != 3850 {
		t.Errorf("draft.Amount = %d, want 3850", draft.Amount)
	}
	if draft.Date != "2026-11-25" {
		t.Errorf("draft.Date = %q, want 2026-11-25", draft.Date)
	}
	if draft.Title != "星巴克咖啡" {
		t.Errorf("draft.Title = %q, want 星巴克咖啡", draft.Title)
	}
	if !strings.Contains(rawText, "星巴克咖啡") {
		t.Errorf("rawText should contain OCR text, got %q", rawText)
	}
}

func TestOCRViewerRawTextFallback(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	// 本地 OCR 认出了文字，但文字里没有可信金额：模型提不出字段，正则也提不出。
	ocrText := "支付宝\n签客行烧烤\n付款方式\n花呗\n订单号 202611023001182361430253635"
	aiSvc.SetOCRProvider(stubOCRProvider{text: ocrText, available: true})
	// 第 1 次调用（模型按文字提字段）：没金额，提失败；
	// 第 2 次调用（远程视觉）：给了账单字段，但没回 raw_text。
	_, srv := newScriptedStubServer(t,
		textResp(`{"amount_yuan":0}`),
		textResp(`{"amount_yuan":55.00,"title":"签客行烧烤","category":"餐饮","sub_category":"午餐","wallet":"花呗"}`),
	)
	saveOCRTestConfig(t, aiSvc, srv.URL)

	_, draft, rawText, _, err := aiSvc.OCR(context.Background(), "u1", bookA, "", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "alipay.jpg")
	if err != nil {
		t.Fatalf("OCR: %v", err)
	}
	if draft == nil {
		t.Fatal("draft is nil")
	}
	if draft.Amount != 5500 {
		t.Errorf("draft.Amount = %d, want 5500", draft.Amount)
	}
	// 查看器原文必须来自本地 OCR，不能因为视觉模型没回 raw_text 就为空。
	if !strings.Contains(rawText, "签客行烧烤") {
		t.Errorf("rawText should fall back to local OCR text, got %q", rawText)
	}
}

func TestOCRWithLocalOCRNoModel(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	aiSvc.SetOCRProvider(stubOCRProvider{text: ocrAlipayText, available: true})
	// 不配模型：只存开关，BaseURL/APIKey 为空，走正则/关键词兜底。
	cfg := &model.AiConfig{Enabled: true, OcrEnabled: true, DailyLimit: 30}
	if err := aiSvc.settingsRepo.SaveAiConfig(context.Background(), cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}

	_, draft, _, _, err := aiSvc.OCR(context.Background(), "u1", bookA, "", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "alipay.jpg")
	if err != nil {
		t.Fatalf("OCR: %v", err)
	}
	if draft == nil {
		t.Fatal("draft is nil")
	}
	if draft.Amount != 8650 {
		t.Errorf("draft.Amount = %d, want 8650", draft.Amount)
	}
	if draft.Date != "2026-11-24" {
		t.Errorf("draft.Date = %q, want 2026-11-24", draft.Date)
	}
	if draft.Title != "盒马鲜生" {
		t.Errorf("draft.Title = %q, want 盒马鲜生", draft.Title)
	}
}

func TestOCRUnrecognizedIsHonestError(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	cfg := &model.AiConfig{Enabled: true, OcrEnabled: true, DailyLimit: 30}
	if err := aiSvc.settingsRepo.SaveAiConfig(context.Background(), cfg); err != nil {
		t.Fatalf("SaveAiConfig: %v", err)
	}

	// 本地 OCR 认不出文字、无远程模型：必须如实报错，不能编一张假小票。
	aiSvc.SetOCRProvider(stubOCRProvider{text: "", available: true})
	_, draft, _, _, err := aiSvc.OCR(context.Background(), "u1", bookA, "", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "blank.jpg")
	if !errors.Is(err, ErrOcrUnrecognized) {
		t.Errorf("err = %v, want ErrOcrUnrecognized", err)
	}
	if draft != nil {
		t.Errorf("draft should be nil on unrecognized image, got %+v", draft)
	}

	// 本地 OCR 压根不可用时也一样。
	aiSvc.SetOCRProvider(nil)
	_, _, _, _, err = aiSvc.OCR(context.Background(), "u1", bookA, "", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "blank.jpg")
	if !errors.Is(err, ErrOcrUnrecognized) {
		t.Errorf("err = %v, want ErrOcrUnrecognized", err)
	}
}

func TestRapidOCRProviderFakeBinary(t *testing.T) {
	dir := t.TempDir()
	model := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte{}, 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	det, rec, dict := model("PP-OCRv6_small_det.ort"), model("PP-OCRv6_small_rec.ort"), model("ppocrv6_dict.txt")

	// 假程序：输出 --json 格式的识别结果（用 printf，避免 sh 的 echo 转义 \n）。
	bin := filepath.Join(dir, "ppu-paddle-ocr")
	script := "#!/bin/sh\nprintf '%s\\n' '{\"text\":\"微信支付\\n星巴克咖啡\\n-38.50\",\"confidence\":0.99}'\n"
	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	p := NewRapidOCRProvider(bin, det, rec, dict)
	if !p.Available() {
		t.Fatal("Available should be true with fake binary + model files")
	}
	res, err := p.Recognize(context.Background(), []byte{0xFF, 0xD8, 0xFF, 0xE0})
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if !strings.Contains(res.Text, "星巴克咖啡") || !strings.Contains(res.Text, "-38.50") {
		t.Errorf("unexpected text: %q", res.Text)
	}
	if res.Confidence != 0.99 {
		t.Errorf("confidence = %v, want 0.99", res.Confidence)
	}

	// 程序输出为空 → 报错。
	emptyBin := filepath.Join(dir, "empty-ocr")
	if err := os.WriteFile(emptyBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	p2 := NewRapidOCRProvider(emptyBin, det, rec, dict)
	if _, err := p2.Recognize(context.Background(), []byte{0xFF, 0xD8}); err == nil {
		t.Error("empty output should be an error")
	}

	// 程序缺失 → 不可用。
	p3 := NewRapidOCRProvider(filepath.Join(dir, "no-such-bin"), det, rec, dict)
	if p3.Available() {
		t.Error("missing binary should be unavailable")
	}
	if _, err := p3.Recognize(context.Background(), []byte{0xFF}); err == nil {
		t.Error("missing binary should error on Recognize")
	}
}

func TestResolveRapidOCRProviderMissing(t *testing.T) {
	t.Setenv("RAPIDOCR_BIN", "/definitely/not/here/ppu-paddle-ocr")
	t.Setenv("RAPIDOCR_MODELS_DIR", "/definitely/not/here/models")
	if p := ResolveRapidOCRProvider(); p != nil {
		t.Errorf("expected nil provider, got %+v", p)
	}
}

// TestRapidOCRProviderRealBinary 用真实程序 + 真实模型跑一张合成账单图。
// 只在设置 RAPIDOCR_BIN（指向 ppu-paddle-ocr）时跑，CI 默认跳过。
func TestRapidOCRProviderRealBinary(t *testing.T) {
	bin := os.Getenv("RAPIDOCR_BIN")
	modelsDir := os.Getenv("RAPIDOCR_MODELS_DIR")
	img := os.Getenv("RAPIDOCR_TEST_IMAGE")
	if bin == "" || modelsDir == "" || img == "" {
		t.Skip("set RAPIDOCR_BIN, RAPIDOCR_MODELS_DIR and RAPIDOCR_TEST_IMAGE to run the real-binary test")
	}
	imageBytes, err := os.ReadFile(img)
	if err != nil {
		t.Fatalf("read test image: %v", err)
	}
	p := NewRapidOCRProvider(bin,
		filepath.Join(modelsDir, rapidOCRDetModel),
		filepath.Join(modelsDir, rapidOCRRecModel),
		filepath.Join(modelsDir, rapidOCRDictModel))
	if !p.Available() {
		t.Fatalf("real binary/models not available: bin=%s modelsDir=%s", bin, modelsDir)
	}
	res, err := p.Recognize(context.Background(), imageBytes)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if !strings.Contains(res.Text, "-38.50") {
		t.Errorf("real OCR should find -38.50, got:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "星巴克") {
		t.Errorf("real OCR should find 星巴克, got:\n%s", res.Text)
	}
}
