#!/usr/bin/env bash
# Test the configuration builder across engines:
#   1. Node logic tests (V8 — Chrome/Edge engine) via node --test
#   2. In-page #selftest under headless Chrome (Blink — Chrome & Edge)
#   3. In-page #selftest under headless Firefox (Gecko), verified by banner colour
#
# Each browser stage is skipped (not failed) if that browser isn't installed, so
# CI can run the subset it has. Run from anywhere; paths are resolved here.
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HTML="$DIR/configurator.html"
URL="file://$HTML#selftest"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
rc=0

step() { printf "\n\033[1m== %s ==\033[0m\n" "$1"; }

step "1/3 Node logic tests (V8)"
if command -v node >/dev/null 2>&1; then
  node --test "$DIR/configurator.test.mjs" || rc=1
else
  echo "SKIP: node not found"
fi

chrome_bin=""
for c in google-chrome google-chrome-stable chromium chromium-browser; do
  command -v "$c" >/dev/null 2>&1 && { chrome_bin="$c"; break; }
done
step "2/3 Headless Chrome / Edge (Blink)"
if [ -n "$chrome_bin" ]; then
  dom="$("$chrome_bin" --headless=new --no-sandbox --disable-gpu --virtual-time-budget=5000 --dump-dom "$URL" 2>/dev/null)"
  res="$(printf '%s' "$dom" | grep -oE 'SELFTEST (PASS|FAIL)[^<]*' | head -1)"
  echo "${res:-<no result>}"
  case "$res" in *PASS*) : ;; *) echo "Chrome self-test did not pass"; rc=1 ;; esac
else
  echo "SKIP: no Chromium/Chrome found"
fi

step "3/3 Headless Firefox (Gecko)"
if command -v firefox >/dev/null 2>&1 && python3 -c "import PIL" >/dev/null 2>&1; then
  shot="$TMP/ff.png"
  firefox --headless --window-size=1200,900 --screenshot "$shot" "$URL" >/dev/null 2>&1
  if [ -f "$shot" ]; then
    python3 - "$shot" <<'PY' || rc=1
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert("RGB"); w, h = im.size
xs = range(10, w-10, max(1, (w-20)//40)); ys = range(6, 16)
r=g=b=n=0
for y in ys:
  for x in xs:
    pr,pg,pb = im.getpixel((x,y)); r+=pr; g+=pg; b+=pb; n+=1
r//=n; g//=n; b//=n
if g>r and g>80: print(f"FIREFOX SELFTEST PASS (banner rgb {r},{g},{b})")
elif r>g and r>120: print(f"FIREFOX SELFTEST FAIL (banner rgb {r},{g},{b})"); sys.exit(1)
else: print(f"FIREFOX SELFTEST INCONCLUSIVE (rgb {r},{g},{b}) — JS may not have run"); sys.exit(1)
PY
  else
    echo "SKIP: Firefox screenshot not produced"
  fi
else
  echo "SKIP: firefox or python3-PIL not available"
fi

printf "\n"
[ "$rc" -eq 0 ] && echo "browser tests: OK" || echo "browser tests: FAILURES"
exit "$rc"
