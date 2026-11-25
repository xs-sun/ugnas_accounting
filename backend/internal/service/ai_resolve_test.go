package service

import (
	"context"
	"testing"
)

// v0.4.0 实测反馈（2026-11-25，Justin 拍天猫钢化膜账单）：
// 模型返回的一级/二级对不上（餐饮/手机配件）、钱包花呗被错认成微信。
// resolveCategoryMatch 必须保证一级二级成对、且都是账本里真实存在的分类；
// resolveWalletMatch 必须认出花呗/信用卡这类付款方式原文。
func TestResolveCategoryMatch(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()

	cases := []struct {
		name      string
		catHint   string
		subHint   string
		wantCat   string
		wantSub   string
		wantCatID bool
	}{
		// 模型自造的二级分类名：手机配件账本里没有，应映射到购物/数码家电。
		{"alias sub maps to real pair", "餐饮", "手机配件", "购物", "数码家电", true},
		// 真实的二级分类名优先于给错的一级：数码家电属于购物，不是餐饮。
		{"real sub wins over wrong parent", "餐饮", "数码家电", "购物", "数码家电", true},
		// 正常成对：原样通过。
		{"matching pair passes through", "餐饮", "咖啡饮品", "餐饮", "咖啡饮品", true},
		// 二级不属于该一级：丢掉二级，不造“餐饮/手机配件”这种错配。
		{"sub not under parent is dropped", "餐饮", "不存在的二级", "餐饮", "", true},
		// 只有二级能对上：由二级反推一级。
		{"sub alone determines parent", "不存在的一级", "午餐", "餐饮", "午餐", true},
		// 没有任何提示：沿用第一个一级分类。
		{"no hints falls back to first parent", "", "", "餐饮", "", true},
		// 提示存在但一个都对不上：如实返回提示名、不选分类，不要默认成餐饮。
		{"unmatched hints return hint without id", "不存在的一级", "不存在的二级", "不存在的一级", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			catID, catName, subName, _, _, _ := aiSvc.resolveCategoryMatch(ctx, bookA, "expense", tc.catHint, tc.subHint)
			if catName != tc.wantCat {
				t.Errorf("category = %q, want %q", catName, tc.wantCat)
			}
			if subName != tc.wantSub {
				t.Errorf("sub_category = %q, want %q", subName, tc.wantSub)
			}
			if tc.wantCatID && catID == "" {
				t.Errorf("expected a category id, got empty")
			}
			if !tc.wantCatID && catID != "" {
				t.Errorf("expected no category id, got %q", catID)
			}
		})
	}
}

func TestResolveWalletMatch(t *testing.T) {
	aiSvc, bookA, _, _, _ := setupAgentTest(t)
	ctx := context.Background()

	wallets, err := aiSvc.walletRepo.ListWallets(ctx, bookA, false)
	if err != nil || len(wallets) == 0 {
		t.Fatalf("ListWallets: %v %d", err, len(wallets))
	}
	byName := make(map[string]string)
	for _, w := range wallets {
		byName[w.Name] = w.ID
	}
	alipayID, ok := byName["支付宝"]
	if !ok {
		t.Fatal("preset wallet 支付宝 not found")
	}
	bankID, ok := byName["招商银行卡"]
	if !ok {
		t.Fatal("preset wallet 招商银行卡 not found")
	}
	wechatID, ok := byName["微信支付"]
	if !ok {
		t.Fatal("preset wallet 微信支付 not found")
	}

	cases := []struct {
		name     string
		hint     string
		wantID   string
		wantName string
	}{
		// 花呗是支付宝：以前不认花呗，提示对不上就默认落到微信。
		{"huabei maps to alipay", "花呗", alipayID, "支付宝"},
		{"alipay maps to alipay", "支付宝", alipayID, "支付宝"},
		// 信用卡/储蓄卡是银行卡：以前只认“银行卡/储蓄卡”两个词。
		{"credit card maps to bank card", "光大银行信用卡(8626)", bankID, "招商银行卡"},
		{"debit card maps to bank card", "平安银行储蓄卡(4021)", bankID, "招商银行卡"},
		{"wechat maps to wechat", "微信支付", wechatID, "微信支付"},
		// 提示对不上任何账户：用账本的第一个账户，不再默认微信。
		{"unmatched hint falls back to first wallet", "不存在的提示", wallets[0].ID, wallets[0].Name},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotName := aiSvc.resolveWalletMatch(ctx, bookA, tc.hint)
			if gotID != tc.wantID || gotName != tc.wantName {
				t.Errorf("resolveWalletMatch(%q) = (%q, %q), want (%q, %q)", tc.hint, gotID, gotName, tc.wantID, tc.wantName)
			}
		})
	}
}
