#!/bin/sh
# Assembles the GitHub Pages site into $1 (default: dist/site).
# The screenshots and logos are copied from the repository so they exist once.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-"$root/dist/site"}
rm -rf "$out"
mkdir -p "$out/img" "$out/fonts"
cp "$root/site/index.html" "$root/site/style.css" "$root/site/site.js" "$root/site/og.png" "$out/"
cp "$root/site/fonts/"*.woff "$out/fonts/"
cp "$root/site/fonts/OFL.txt" "$out/fonts/"
cp "$root/docs/screenshots/"*.png "$out/img/"
cp "$root/assets/brand/logo-plain.svg" "$root/assets/brand/logo-plain-dark.svg" \
   "$root/assets/brand/icon-small.svg" "$root/assets/brand/icon-1024.png" "$out/img/"
echo "site: $out"
