#!/bin/bash
# 下载 RapidOCR（ppu-paddle-ocr v6.7.1 + PP-OCRv6 small 模型）到 third_party/rapidocr/。
# 供 build.sh 打包（.upk）和本地开发用；CI 里由 .github/workflows/release.yml 调用。
# 校验：安装包 SHA-256 固定（以防上游悄悄换包）；模型走 HuggingFace 官方地址。
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT_DIR/third_party/rapidocr"
mkdir -p "$OUT/models"

VERSION="v6.7.1"
TARBALL_SHA256="ae6fcfab51d62373329a29e2ece876bed2c170a6887dd39471b19af24e49b242"
BASE="https://github.com/PT-Perkasa-Pilar-Utama/ppu-paddle-ocr/releases/download/${VERSION}"
MODELS_BASE="https://huggingface.co/snowfluke/ppu-paddle-ocr-models/resolve/main"
CURL_OPTS=(-fSL --retry 3 --retry-all-errors --retry-delay 10 --connect-timeout 30 --max-time 600 --retry-max-time 1800)

download() { # url, dest
  echo "下载 $1 ..."
  curl "${CURL_OPTS[@]}" -o "$2" "$1"
}

if [ -x "$OUT/ppu-paddle-ocr" ]; then
  echo "RapidOCR 程序已存在，跳过下载：$OUT/ppu-paddle-ocr"
else
  tmp="$OUT/.download-tmp"
  rm -rf "$tmp"
  mkdir -p "$tmp"
  trap 'rm -rf "$tmp"' EXIT
  download "$BASE/ppu-paddle-ocr-linux-x64.tar.gz" "$tmp/ppu.tar.gz"
  echo "$TARBALL_SHA256  $tmp/ppu.tar.gz" | sha256sum -c -
  tar xzf "$tmp/ppu.tar.gz" --no-same-owner -C "$tmp"
  mv "$tmp/ppu-paddle-ocr-linux-x64" "$OUT/ppu-paddle-ocr"
  chmod +x "$OUT/ppu-paddle-ocr"
  rm -rf "$tmp"
  trap - EXIT
fi

for m in detection/ort/PP-OCRv6_small_det.ort recognition/ort/PP-OCRv6_small_rec.ort recognition/ppocrv6_dict.txt; do
  name="$(basename "$m")"
  if [ -f "$OUT/models/$name" ]; then
    echo "模型已存在，跳过：$name"
  else
    download "$MODELS_BASE/$m" "$OUT/models/$name"
  fi
done

echo "验证程序..."
"$OUT/ppu-paddle-ocr" version
ls -la "$OUT" "$OUT/models"
echo "RapidOCR 就绪：$OUT"
