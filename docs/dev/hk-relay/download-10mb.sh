#!/bin/bash
# Pull ~10MB through the phase-1 proxy (40 x 272538-byte JS).
set -euo pipefail
url='http://127.0.0.1/assets/vendor-misc-D2hb4tDs.js'
total=0
start=$(date +%s%N)
for _ in $(seq 1 40); do
	n=$(curl -sS -o /dev/null -m 30 -w '%{size_download}' -H 'Host: 191.40.32.186' "$url")
	total=$((total + n))
done
end=$(date +%s%N)
ms=$(( (end - start) / 1000000 ))
echo "downloaded_bytes=$total elapsed_ms=$ms"
test "$total" -ge 10000000
