#!/usr/bin/env python3
"""Generate deterministic test fixtures for Seven Cats Accounting E2E Suite."""
import os
import sys
import struct
import zlib

FIXTURES_DIR = os.path.dirname(os.path.abspath(__file__))

def create_valid_png(path, width=256, height=256):
    """Create a minimal valid 256x256 PNG file (< 100KB)."""
    def chunk(chunk_type, data):
        c = chunk_type + data
        crc = zlib.crc32(c) & 0xffffffff
        return struct.pack('>I', len(data)) + c + struct.pack('>I', crc)

    header = b'\x89PNG\r\n\x1a\n'
    ihdr = chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 2, 0, 0, 0)) # RGB
    # Raw pixel rows: filter type 0 + width * 3 bytes (deep turquoise: 14, 140, 127)
    row = b'\x00' + (bytes([14, 140, 127]) * width)
    raw_data = row * height
    compressed = zlib.compress(raw_data, 9)
    idat = chunk(b'IDAT', compressed)
    iend = chunk(b'IEND', b'')
    png_bytes = header + ihdr + idat + iend
    with open(path, 'wb') as f:
        f.write(png_bytes)
    print(f"Generated {path} ({len(png_bytes)} bytes)")

def create_valid_jpg(path):
    """Create a minimal valid JPEG file for receipt attachment testing."""
    # Minimal 1x1 JPEG bytes
    jpg_bytes = bytes([
        0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
        0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
        0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
        0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
        0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
        0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
        0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
        0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
        0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
        0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
        0x09, 0x0A, 0x0B, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F,
        0x00, 0xBF, 0x80, 0xFF, 0xD9
    ])
    with open(path, 'wb') as f:
        f.write(jpg_bytes)
    print(f"Generated {path} ({len(jpg_bytes)} bytes)")

def create_wechat_csv(path):
    """Create WeChat Pay statement CSV with UTF-8 BOM."""
    bom = b'\xef\xbb\xbf'
    content = """微信支付账单明细,,,,,,,,
微信昵称：[Alice],,,,,,,,
起始时间：[2026-09-01 00:00:00] 终止时间：[2026-09-30 23:59:59],,,,,,,,
导出类型：[全部],,,,,,,,
导出时间：[2026-10-01 10:00:00],,,,,,,,
----------------------微信支付账单明细列表--------------------,,,,,,,,
交易时间,交易类型,交易对方,商品,收/支,金额(元),支付方式,当前状态,交易单号,商户单号,备注
2026-09-02 08:30:00,商户消费,美团外卖,美团订单-早餐包子,支出,¥18.50,微信零钱,支付成功,wx10001,sh10001,/
2026-09-03 12:15:00,商户消费,星巴克咖啡,拿铁咖啡大杯,支出,¥38.00,微信零钱,支付成功,wx10002,sh10002,提神
2026-09-04 18:40:00,商户消费,滴滴出行,快车行程费,支出,¥26.80,微信零钱,支付成功,wx10003,sh10003,/
2026-09-05 19:20:00,商户消费,海底捞火锅,四人餐消费,支出,¥368.00,微信零钱,支付成功,wx10004,sh10004,家庭聚餐
2026-09-06 09:10:00,商户消费,喜茶,多肉葡萄,支出,¥19.00,微信零钱,支付成功,wx10005,sh10005,/
2026-09-07 14:00:00,微信红包,微信好友,好友转账红包,收入,¥88.00,/,已存入零钱,wx10006,sh10006,生日祝福
2026-09-08 20:10:00,商户消费,永辉超市,日用生鲜杂货,支出,¥125.60,微信零钱,支付成功,wx10007,sh10007,/
2026-09-10 11:30:00,商户消费,麦当劳,麦辣鸡腿堡套餐,支出,¥32.50,微信零钱,支付成功,wx10008,sh10008,/
2026-09-12 16:45:00,商户消费,瑞幸咖啡,生椰拿铁,支出,¥16.00,微信零钱,支付成功,wx10009,sh10009,/
2026-09-14 21:00:00,商户消费,朴朴超市,夜宵零食水果,支出,¥64.20,微信零钱,支付成功,wx10010,sh10010,/
2026-09-15 08:20:00,商户消费,地铁出行,地铁单程票,支出,¥5.00,微信零钱,支付成功,wx10011,sh10011,/
2026-09-18 13:00:00,商户消费,肯德基,疯狂星期四,支出,¥49.90,微信零钱,支付成功,wx10012,sh10012,/
2026-09-20 17:30:00,商户消费,屈臣氏,日常洗护用品,支出,¥89.00,微信零钱,支付成功,wx10013,sh10013,/
2026-09-22 19:15:00,商户消费,便利蜂,便当饮料,支出,¥24.50,微信零钱,支付成功,wx10014,sh10014,/
2026-09-25 10:00:00,转账,公司薪资账户,9月预支劳务费,收入,¥2500.00,/,已存入零钱,wx10015,sh10015,劳务报酬
"""
    with open(path, 'wb') as f:
        f.write(bom + content.strip().encode('utf-8'))
    print(f"Generated {path} (WeChat UTF-8 BOM)")

def create_alipay_csv(path):
    """Create Alipay statement CSV encoded in GBK."""
    content = """支付宝交易记录明细查询
账号:[alice@nas.local]
起始时间:[2026-09-01 00:00:00] 终止时间:[2026-09-30 23:59:59]
------------------------------------------------------------------------------------
交易时间,交易分类,交易对方,对方账号,商品说明,收/支,金额,收/付款方式,交易状态,交易订单号,商家订单号,备注
2026-09-01 09:00:00,日用百货,中国移动通信,10086,手机话费充值,支出,100.00,余额宝,交易成功,ali20001,sh20001,充值
2026-09-02 12:30:00,餐饮美食,美团外卖,meituan@corp,午餐商务套餐,支出,45.00,余额宝,交易成功,ali20002,sh20002,/
2026-09-03 14:20:00,日用百货,山姆会员店,sams@walmart,高品质牛肉牛排,支出,358.00,余额宝,交易成功,ali20003,sh20003,居家生鲜
2026-09-04 17:50:00,交通出行,滴滴出行,didi@travel,晚高峰通勤,支出,35.50,余额宝,交易成功,ali20004,sh20004,/
2026-09-05 20:00:00,数码电器,Apple Store,apple@pay,iPad保护壳,支出,299.00,招商储蓄卡,交易成功,ali20005,sh20005,/
2026-09-06 11:00:00,日用百货,淘宝网商家,tb@shop,猫粮与猫砂套装,支出,188.00,余额宝,交易成功,ali20006,sh20006,七喵用品
2026-09-08 15:30:00,餐饮美食,星巴克咖啡,starbucks@pay,秋季特饮,支出,42.00,余额宝,交易成功,ali20007,sh20007,/
2026-09-10 10:00:00,金融理财,天弘余额宝基金,th@fund,理财收益分红,收入,12.80,/,交易成功,ali20008,sh20008,收益
2026-09-11 18:10:00,日用百货,盒马鲜生,freshippo@corp,海鲜三文鱼,支出,168.00,余额宝,交易成功,ali20009,sh20009,/
2026-09-13 19:40:00,文化休闲,猫眼电影,maoyan@cinema,周末电影票双人,支出,96.00,余额宝,交易成功,ali20010,sh20010,/
2026-09-15 08:30:00,交通出行,中国铁路12306,rail@12306,高铁二等座车票,支出,245.50,招商储蓄卡,交易成功,ali20011,sh20011,出差
2026-09-16 12:45:00,餐饮美食,肯德基,kfc@china,工作日套餐,支出,38.50,余额宝,交易成功,ali20012,sh20012,/
2026-09-17 16:00:00,通讯物流,顺丰速运,sf@express,同城特快运费,支出,23.00,余额宝,交易成功,ali20013,sh20013,/
2026-09-19 21:15:00,医药健康,同仁堂大药房,trt@pharm,家庭常备维生素,支出,78.00,余额宝,交易成功,ali20014,sh20014,/
2026-09-21 09:20:00,日用百货,京东商城,jd@mall,抽纸整箱装,支出,69.90,招商储蓄卡,交易成功,ali20015,sh20015,/
2026-09-23 14:00:00,日用百货,网易严选,yanxuan@corp,床上四件套,支出,219.00,余额宝,交易成功,ali20016,sh20016,/
2026-09-24 17:30:00,交通出行,青桔单车,qingju@bike,单车月卡充值,支出,15.00,余额宝,交易成功,ali20017,sh20017,/
2026-09-26 19:50:00,餐饮美食,必胜客披萨,pizzahut@corp,晚餐聚会,支出,158.00,余额宝,交易成功,ali20018,sh20018,/
2026-09-28 11:20:00,日用百货,屈臣氏,watsons@corp,护手霜防晒霜,支出,112.00,余额宝,交易成功,ali20019,sh20019,/
2026-09-30 18:00:00,转账代扣,水电燃气公用事业,util@gov,9月燃气费用,支出,65.00,招商储蓄卡,交易成功,ali20020,sh20020,/
"""
    with open(path, 'wb') as f:
        f.write(content.strip().encode('gbk'))
    print(f"Generated {path} (Alipay GBK)")

def create_corrupted_snapshot(path):
    """Create a corrupt snapshot file."""
    corrupted_data = b"NOT_A_VALID_TAR_GZ_OR_SQLITE_FILE_CONTENT_XYZ_1234567890\xFF\xFE\x00\x01" * 10
    with open(path, 'wb') as f:
        f.write(corrupted_data)
    print(f"Generated {path} ({len(corrupted_data)} bytes)")

if __name__ == '__main__':
    os.makedirs(FIXTURES_DIR, exist_ok=True)
    create_valid_png(os.path.join(FIXTURES_DIR, 'valid_icon.png'))
    create_valid_jpg(os.path.join(FIXTURES_DIR, 'test_receipt.jpg'))
    create_wechat_csv(os.path.join(FIXTURES_DIR, 'wechat_sample_utf8_bom.csv'))
    create_alipay_csv(os.path.join(FIXTURES_DIR, 'alipay_sample_gbk.csv'))
    create_corrupted_snapshot(os.path.join(FIXTURES_DIR, 'corrupted_snapshot.ugledger'))
    print("All fixtures generated successfully.")
