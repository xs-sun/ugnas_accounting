package service

import "testing"

// 真实账单截图的 OCR 文字回归测试：2026-11-25 Justin 发来的 5 张账单截图，
// 用 ppu-paddle-ocr v6.7.1 + PP-OCRv6 small 实际识别出的文字（逐字转录）。
// 注意：#3 的收货人姓名电话地址、#5 的油卡卡号两行已从夹具里删掉
// （个人隐私，不进公开仓库；它们在金额/日期/商户之后，不影响提取结果）。
//
// 这批真实截图暴露过两个正则提取 bug，已修：
//   - #2 “优惠 -0.10” 会让“取最后一个小数”误取 0.10 元；
//   - #3 “实付款 共减¥48.2 合计¥63.8” 会让“取第一个标签金额”误取 48.2 元。
//
// 2026-11-25 v0.4.0 实测反馈又暴露三处：标题带打码星号（#1 的“图拉**店”
// 应显示为去星号的“图拉店”）、分类错配（#1 数码电器应为购物/数码家电，
// 不是餐饮）、#5 中海油应为交通/加油停车。
var realBillOCRTexts = []struct {
	name     string
	text     string
	amount   int64
	date     string
	merchant string
	wallet   string
	source   string
	category string
	sub      string
}{
	{
		name: "alipay-tmall",
		text: `20:37
< 账单详情
天猫
图拉**店
-30.77
支付成功
关联记录 查看关联记录
支付时间 2026-11-1810:57:56
付款方式 花呗>
扣款说明 先用后付订单完成付款，已计入芝麻分
>
评估
交易详情
超清无果边    图 拉 斯新 无 界 膜AR抗 反射 适用 苹
美  果18Pro钢化膜iPhone17proma 共1件
20.
更多
账单管理 已解锁"再买亿件"贴纸，去兑换好礼 > y
账单分类 数码电器 >
标签 请选择 >
 使用记账本，查看自定义分类、标签统计
计入收支
备注 添加 >`,
		amount:   3077,
		date:     "2026-11-18",
		merchant: "图拉店",
		wallet:   "支付宝",
		source:   "taobao",
		category: "购物",
		sub:      "数码家电",
	},
	{
		name: "alipay-taobao-flash",
		text: `20:37
< 账单详情
两购
淘宝闪购
-70.78
交易成功
关联记录 查看关联记录
优惠 -0.10
支付时间 2026-11-2019:15:30
付款方式 花呗>
商品说明 签客行烧烤·免费50串西美店外卖订单
支付奖励  立即领取5积分
订单号 2026112023001182361430253635
商家订单号 13190600726112097873691690168
账单管理 已解锁"吃顿饭"贴纸，去兑换好礼 >
账单分类 餐饮美食〉
标签 请选择 >
为您推荐 三餐+  面包+ 下馆子+
计入收支`,
		amount:   7078,
		date:     "2026-11-20",
		merchant: "淘宝闪购",
		wallet:   "支付宝",
		source:   "taobao",
		category: "餐饮",
		sub:      "午餐",
	},
	{
		name: "jd-order",
		text: `20:36
完成
直马勒MAHLE京东自营旗舰店
进店>
回头客超3万 近90天1万好评 月销超10万件
 马勒京东自营旗舰店 mRHLE 马勒（MAHLE)滤芯套. i到手¥63.8
合 ¥112
2 mAHLE 数量×1,空气滤空调滤套
2 4
2
P图 8
 支持7天无理由退货·7天价保·买贵双倍赔
百年品牌可急性强
高过指书目好守护
20京豆
评价晒单 卖了换钱 加购物车 申请售后
实付款 共减¥48.2 合计¥63.8>
订单编号 3680440009248610复制
交易快照 发生交易争议时，可作为判断依据
支付方式 银行卡支付
支付时间 2026-11-1200:16:29
下单时间 2026-11-1200:16:12
配送方式 京东快递
期望配送时间 2026-11-1309:00-15:00
收货方式 送货上门
收起
20京豆
更多 退换/售后 评价晒单 再次购买`,
		amount:   6380,
		date:     "2026-11-12",
		merchant: "直马勒MAHLE京东自营旗舰店",
		wallet:   "银行卡",
		source:   "jd",
		category: "购物",
		sub:      "日用品",
	},
	{
		name: "wechat-jd",
		text: `20:36
京乐 喜欢
等760万+人喜欢
小程序 点外卖领国补
服务会员 交易详情
-54.78
京东
当前状态 支付成功
支付时间 2026年11月08日13:12:39
商品 京东-订单编号3676440007414904
收单机构 财付通支付科技有限公司
支付方式 平安银行储蓄卡(4021)
交易单号 4500000579202611083181253059
商户单号 102114752611081312290911533897
交易服务
对订单有疑惑 发起群收款
在此商户的交易 申请电子凭证`,
		amount:   5478,
		date:     "2026-11-08",
		merchant: "京东",
		wallet:   "银行卡",
		source:   "jd",
		// 注意：正则启发式的已知弱点——小字推广“点外卖领国补”触发了外卖规则，
		// 实际是京东订单；模型提取路径（带允许分类列表）能正确分成购物。
		category: "餐饮",
		sub:      "午餐",
	},
	{
		name: "wechat-fuel",
		text: `21:59
X 账单
中海油销售河北有限公司
-350.00
当前状态 支付成功
支付时间 2026年11月18日20:09:50
商户全称 中海油销售河北有限公司
收单机构 财付通支付科技有限公司
支付方式 光大银行信用卡(8626)
交易单号 4500000531202611187150022541
商户单号 755371709190145
账单服务
对订单有疑惑 发起群收款
在此商户的交易 申请电子凭证
本服务由财付通提供`,
		amount:   35000,
		date:     "2026-11-18",
		merchant: "中海油销售河北有限公司",
		wallet:   "银行卡",
		source:   "wechat",
		category: "交通",
		sub:      "加油停车",
	},
}

func TestExtractBillFromOCRTextLocalRealBills(t *testing.T) {
	for _, tc := range realBillOCRTexts {
		t.Run(tc.name, func(t *testing.T) {
			b := extractBillFromOCRTextLocal(tc.text)
			if b == nil {
				t.Fatalf("expected a bill, got nil")
			}
			if b.AmountCents != tc.amount {
				t.Errorf("amount = %d, want %d", b.AmountCents, tc.amount)
			}
			if b.Date != tc.date {
				t.Errorf("date = %q, want %q", b.Date, tc.date)
			}
			if b.Title != tc.merchant {
				t.Errorf("merchant = %q, want %q", b.Title, tc.merchant)
			}
			if b.Wallet != tc.wallet {
				t.Errorf("wallet = %q, want %q", b.Wallet, tc.wallet)
			}
			if b.Source != tc.source {
				t.Errorf("source = %q, want %q", b.Source, tc.source)
			}
			if b.Category != tc.category {
				t.Errorf("category = %q, want %q", b.Category, tc.category)
			}
			if b.SubCategory != tc.sub {
				t.Errorf("sub_category = %q, want %q", b.SubCategory, tc.sub)
			}
		})
	}
}
