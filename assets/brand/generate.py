import math, sys
# Regenerates every logo file in this folder: python3 assets/brand/generate.py assets/brand
# Needs fontTools and the Inter font (SIL OFL) installed.
OUT = sys.argv[1] if len(sys.argv) > 1 else "."

NAVY, BLUE, CYAN, GRAY = "#0F2A56", "#0D6EFB", "#22D3EE", "#94A3B8"

def crescent(cx, cy, r, a0, a1, w0, w1, n=64):
    """A stroke along a circle arc whose width goes from w0 to w1 (eased).
    Angles in degrees, SVG orientation (0 = +x, 90 = down)."""
    outer, inner = [], []
    for i in range(n + 1):
        t = i / n
        a = math.radians(a0 + (a1 - a0) * t)
        e = t * t * (3 - 2 * t)
        w = w0 + (w1 - w0) * e
        outer.append((cx + (r + w / 2) * math.cos(a), cy + (r + w / 2) * math.sin(a)))
        inner.append((cx + (r - w / 2) * math.cos(a), cy + (r - w / 2) * math.sin(a)))
    pts = outer + inner[::-1]
    return "M" + " L".join(f"{x:.1f} {y:.1f}" for x, y in pts) + " Z"

def defs(prefix, light):
    # light = True for the coloured mark on a pale background
    return f'''<defs>
<linearGradient id="{prefix}scr" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#2FA2F5"/><stop offset="1" stop-color="{BLUE}"/></linearGradient>
<linearGradient id="{prefix}file" x1="0" y1="0" x2="0.4" y2="1"><stop offset="0" stop-color="{CYAN}"/><stop offset="1" stop-color="{BLUE}"/></linearGradient>
<linearGradient id="{prefix}arcT" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="{BLUE}"/><stop offset="1" stop-color="{CYAN}"/></linearGradient>
<linearGradient id="{prefix}arcB" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="{CYAN if not light else '#1E8BF7'}"/><stop offset="1" stop-color="{BLUE}"/></linearGradient>
<linearGradient id="{prefix}tile" x1="1" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#1CCBF2"/><stop offset=".5" stop-color="{BLUE}"/><stop offset="1" stop-color="#0838B8"/></linearGradient>
</defs>'''

def mark(prefix, mode):
    """mode: 'color' (navy frames, blue screens), 'white' (for the app tile), 'mono:#hex'."""
    if mode == "color":
        frame, screen, file_, fold, arcT, arcB = NAVY, f"url(#{prefix}scr)", f"url(#{prefix}file)", "#8BE6F8", f"url(#{prefix}arcT)", f"url(#{prefix}arcB)"
        fw = None
    elif mode == "dark":
        frame, screen, file_, fold, arcT, arcB = "#DCE7F7", f"url(#{prefix}scr)", f"url(#{prefix}file)", "#8BE6F8", f"url(#{prefix}arcT)", f"url(#{prefix}arcB)"
        fw = None
    elif mode == "white":
        frame, screen, file_, fold, arcT, arcB = "#FFFFFF", "none", "#5FE3F8", "#E6FBFF", "#8BEAFB", "#B9F2FD"
        fw = 13
    else:
        c = mode.split(":")[1]
        frame, screen, file_, fold, arcT, arcB = c, "none", c, "none", c, c
        fw = 13
    arcs = (f'<path d="{crescent(258, 262, 160, 214, 318, 6, 34)}" fill="{arcT}"/>'
            f'<path d="{crescent(258, 262, 150, 136, 38, 34, 6)}" fill="{arcB}"/>')
    if fw:  # outlined devices (white / mono)
        monitor = (f'<g transform="skewY(4) translate(0 -8)">'
                   f'<rect x="{38+fw/2}" y="{176+fw/2}" width="{150-fw}" height="{112-fw}" rx="14" fill="none" stroke="{frame}" stroke-width="{fw}"/>'
                   f'<rect x="102" y="288" width="22" height="30" fill="{frame}"/><rect x="72" y="314" width="82" height="13" rx="6.5" fill="{frame}"/></g>')
        phone = (f'<g transform="skewY(-4) translate(0 26)">'
                 f'<rect x="{384+fw/2}" y="{168+fw/2}" width="{82-fw}" height="{148-fw}" rx="16" fill="none" stroke="{frame}" stroke-width="{fw}"/>'
                 f'<rect x="411" y="185" width="28" height="7" rx="3.5" fill="{frame}"/><circle cx="425" cy="298" r="7" fill="{frame}"/></g>')
    else:
        monitor = (f'<g transform="skewY(4) translate(0 -8)">'
                   f'<rect x="38" y="176" width="150" height="112" rx="16" fill="{frame}"/>'
                   f'<rect x="51" y="189" width="124" height="86" rx="6" fill="{screen}"/>'
                   f'<rect x="102" y="288" width="22" height="30" fill="{frame}"/><rect x="72" y="314" width="82" height="14" rx="7" fill="{frame}"/></g>')
        phone = (f'<g transform="skewY(-4) translate(0 26)">'
                 f'<rect x="384" y="168" width="82" height="148" rx="16" fill="{frame}"/>'
                 f'<rect x="394" y="196" width="62" height="88" rx="6" fill="{screen}"/>'
                 f'<rect x="411" y="179" width="28" height="7" rx="3.5" fill="#FFFFFF" opacity=".9"/><circle cx="425" cy="299" r="7" fill="#FFFFFF"/></g>')
    fold_path = f'<path d="M298 186 v26 a6 6 0 0 0 6 6 h22 z" fill="{fold}"/>' if fold != "none" else ""
    file_g = (f'<g transform="skewY(10) translate(0 -46)">'
              f'<path d="M232 186 h66 l28 32 v92 a10 10 0 0 1 -10 10 h-84 a10 10 0 0 1 -10 -10 v-114 a10 10 0 0 1 10 -10 z" fill="{file_}"/>'
              f'{fold_path}</g>')
    return arcs + monitor + file_g + phone

def svg(w, h, body, vb=None):
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{vb or f"0 0 {w} {h}"}" width="{w}" height="{h}">{body}</svg>\n'

def small_mark(prefix, fg, accent):
    """Simplified for 16-32 px: the two arcs and the file only."""
    return (f'<path d="{crescent(256, 256, 150, 205, 325, 14, 62)}" fill="{fg}"/>'
            f'<path d="{crescent(256, 256, 150, 145, 25, 62, 14)}" fill="{fg}"/>'
            f'<g transform="skewY(8) translate(0 -36)"><path d="M196 166 h86 l44 46 v122 a14 14 0 0 1 -14 14 h-116 a14 14 0 0 1 -14 -14 v-154 a14 14 0 0 1 14 -14 z" fill="{accent}"/>'
            f'<path d="M282 166 v34 a12 12 0 0 0 12 12 h32 z" fill="#E6FBFF"/></g>')

files = {}
files["mark.svg"] = svg(512, 512, defs("m", True) + mark("m", "color"), "0 40 512 432")
files["mark-white.svg"] = svg(512, 512, mark("w", "white").replace('"#5FE3F8"', '"#FFFFFF"').replace('"#E6FBFF"', '"#9DEBFA"'), "0 40 512 432")
files["mark-mono.svg"] = svg(512, 512, mark("k", "mono:currentColor"), "0 40 512 432")
tile = f'<rect width="512" height="512" rx="116" fill="url(#tTile)"/>'
icon_body = defs("t", False).replace('id="ttile"', 'id="tTile"') + tile + f'<g transform="translate(46 50) scale(.82)">{mark("t", "white")}</g>'
files["icon.svg"] = svg(512, 512, icon_body)
files["icon-small.svg"] = svg(512, 512, defs("s", False).replace('id="stile"', 'id="sTile"') + '<rect width="512" height="512" rx="116" fill="url(#sTile)"/>' + small_mark("s", "#B9F2FD", "#FFFFFF"))
files["mark-dark.svg"] = svg(512, 512, defs("d", True) + mark("d", "dark"), "0 40 512 432")

# Lockups: mark + "Pasla" (+ tagline), letters outlined from Inter so the
# files render the same everywhere.
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
def text_path(fontfile, text, size, tracking=0.0):
    f = TTFont(fontfile); gs = f.getGlyphSet(); cmap = f.getBestCmap(); upm = f["head"].unitsPerEm
    sc = size / upm; x = 0; parts = []
    for ch in text:
        g = cmap[ord(ch)]; pen = SVGPathPen(gs)
        gs[g].draw(TransformPen(pen, (sc, 0, 0, -sc, x * sc, 0))); parts.append(pen.getCommands())
        x += gs[g].width + tracking * upm
    return " ".join(parts), x * sc
BOLD = "/usr/share/fonts/opentype/inter/Inter-ExtraBold.otf"
MED = "/usr/share/fonts/opentype/inter/Inter-Medium.otf"
word, ww = text_path(BOLD, "Pasla", 250, -0.025)
tag, tw = text_path(MED, "Offline file sharing for your local network", 50, -0.005)
def lockup(prefix, mode, wordc, tagc, with_tag):
    body = defs(prefix, True) + f'<g transform="translate(0 -40)">{mark(prefix, mode)}</g>'
    body += f'<path transform="translate(560 300)" fill="{wordc}" d="{word}"/>'
    w = 560 + max(ww, tw if with_tag else 0) + 20
    if with_tag:
        body += f'<path transform="translate(566 382)" fill="{tagc}" d="{tag}"/>'
    return svg(round(w), 432, body, f"0 0 {w:.0f} 432")
files["logo.svg"] = lockup("a", "color", NAVY, "#5B6B82", True)
files["logo-dark.svg"] = lockup("b", "dark", "#F1F5FB", "#A8B4C6", True)
files["logo-plain.svg"] = lockup("c", "color", NAVY, "", False)
files["logo-plain-dark.svg"] = lockup("e", "dark", "#F1F5FB", "", False)
for k, v in files.items():
    open(f"{OUT}/{k}", "w").write(v)
print("ok", list(files))
