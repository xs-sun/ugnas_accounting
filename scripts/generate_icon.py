#!/usr/bin/env python3
"""
七喵记账 (com.xsguyu.accounting) 应用图标生成脚本
优先使用官方设计图 cat7-logo.png 作为母图，输出合规的 256x256 RGBA PNG 图标。
"""

import os
from PIL import Image

def generate_app_icon(output_path: str):
    root_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    logo_candidates = [
        os.path.join(root_dir, "cat7-logo.png"),
        os.path.join(root_dir, "cat7-logo-backup.png"),
    ]

    source_logo = None
    for cand in logo_candidates:
        if os.path.exists(cand):
            source_logo = cand
            break

    os.makedirs(os.path.dirname(os.path.abspath(output_path)), exist_ok=True)

    if source_logo:
        print(f"Using source logo: {source_logo}")
        img = Image.open(source_logo)
        if img.mode != "RGBA":
            img = img.convert("RGBA")
        
        # 高保真 Lanczos 采样至 256x256
        img_256 = img.resize((256, 256), Image.Resampling.LANCZOS)
        img_256.save(output_path, "PNG", optimize=True)
        file_size = os.path.getsize(output_path)
        print(f"Generated {output_path} from {os.path.basename(source_logo)}: 256x256, {file_size} bytes ({file_size / 1024:.2f} KB)")
        return

    # Fallback if no source logo is found
    print("Warning: cat7-logo.png not found, falling back to procedural generation...")
    from PIL import ImageDraw
    canvas_size = 1024
    img = Image.new("RGBA", (canvas_size, canvas_size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    pad = 64
    radius = 210
    draw.rounded_rectangle([pad, pad, canvas_size - pad, canvas_size - pad], radius=radius, fill="#0E8C7F")
    img_256 = img.resize((256, 256), Image.Resampling.LANCZOS)
    img_256.save(output_path, "PNG", optimize=True)

if __name__ == "__main__":
    generate_app_icon("rootfs_common/icon.png")
