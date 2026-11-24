#!/bin/sh
# Downloads the Maia-3 ONNX models that opposition uses for move predictions.
#
#   tools/fetch-models.sh            the 79M model (the most accurate, 312 MB)
#   tools/fetch-models.sh 5m 23m     other sizes: 5m (21 MB), 23m (91 MB), 79m
#
# The files are a community ONNX conversion of the official Maia-3 weights
# (https://huggingface.co/bqrio/maia3-onnx, AGPL-3.0), pinned to one revision
# and checked against known checksums. The tests in internal/maia compare
# their output with the official implementation.
set -eu

REVISION=923df9a2e9396b54a09b168bb858ebbd5e5b76bc
DIR=${MODELS_DIR:-models}

checksum_of() {
  case "$1" in
    5m)  echo 02944645ab2454f628aa814887e5d4bdcd1b4f58b1aed6cfdbd1bbfbc55b7e4a ;;
    23m) echo b0469eda3eba7b934de4c7356768f0dc4fd640aa45ae5df031cd1efd6b63545d ;;
    79m) echo fbdf7d32b41184c1fb6f352ed901c4cf1b60e93a71b6708c5e9e4c7ca0805c04 ;;
    *)   echo "unknown model size: $1 (use 5m, 23m or 79m)" >&2; exit 2 ;;
  esac
}

sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    sha256sum "$1" | cut -d' ' -f1
  fi
}

[ $# -gt 0 ] || set -- 79m
mkdir -p "$DIR"

for size in "$@"; do
  want=$(checksum_of "$size")
  file="$DIR/maia3-$size.fp32.onnx"
  if [ -f "$file" ] && [ "$(sha256_of "$file")" = "$want" ]; then
    echo "$file is already there"
    continue
  fi
  echo "downloading $file"
  curl -L --fail --progress-bar -o "$file.part" \
    "https://huggingface.co/bqrio/maia3-onnx/resolve/$REVISION/maia3-$size.fp32.onnx"
  got=$(sha256_of "$file.part")
  if [ "$got" != "$want" ]; then
    rm -f "$file.part"
    echo "checksum mismatch for $size: got $got, want $want" >&2
    exit 1
  fi
  mv "$file.part" "$file"
done
