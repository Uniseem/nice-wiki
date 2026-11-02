#!/usr/bin/env python3
"""生成内置外观“低三角形”的三角面素材到 libs/platform/theme/builtin/low-poly/assets（外观包不能带 SVG，只能放位图）。
随机数种子固定，改完配色或形状重跑即可。用法：python3 scripts/gen_low_poly_theme.py（需要 Pillow）"""
import math, os, random
from PIL import Image, ImageDraw

OUT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "libs", "platform", "theme", "builtin", "low-poly", "assets")
SUPERSAMPLE = 2  # 先画大再缩小，三角形的边才不带锯齿
SUN = (-0.55, 0.62, -0.56)  # 光从左上方照过来

# 每种配色的取值；background 和 footer 要和 theme.json、theme.css 里的同名颜色一致。
SCHEMES = {
    "light": {
        "background": "#eef2f3", "facet": "#e0e8ea",
        "sky": ["#a9cbd6", "#cfe0e2", "#eee8dc", "#f9dcc0"],
        "land": ["#0e3a46", "#17606b", "#3f9188", "#8cc4ae"], "snow": "#f6f8f3",
        "footer": "#10323b", "ridge": ["#1b5660", "#2c7a7c", "#5aa899"], "ridge_far": ["#6ea8a6", "#8fc0b6", "#b4d6c8"],
    },
    "dark": {
        "background": "#0f1a20", "facet": "#131f26",
        "sky": ["#070f18", "#0e1e2c", "#1b3142", "#37445a"],
        "land": ["#050f15", "#0d2a34", "#1c5058", "#37787a"], "snow": "#b9d3d6",
        "footer": "#091217", "ridge": ["#0f2a33", "#174049", "#25606a"], "ridge_far": ["#142a33", "#1b3640", "#26474f"],
    },
}
TRIM = ["#0e3a46", "#17606b", "#3f9188", "#8cc4ae", "#f1a26a", "#e4572e", "#c2410c"]


def rgb(value):
    return tuple(int(value[i:i + 2], 16) for i in (1, 3, 5))


def mix(a, b, t):
    return tuple(round(x + (y - x) * t) for x, y in zip(a, b))


def ramp(stops, t):
    t = max(0.0, min(1.0, t)) * (len(stops) - 1)
    index = min(int(t), len(stops) - 2)
    return mix(rgb(stops[index]), rgb(stops[index + 1]), t - index)


def smooth(low, high, value):
    t = max(0.0, min(1.0, (value - low) / (high - low)))
    return t * t * (3 - 2 * t)


class Noise:
    """一维周期值噪声。"""

    def __init__(self, seed, period):
        rng = random.Random(seed)
        self.values = [rng.random() for _ in range(period)]

    def __call__(self, x):
        n = len(self.values)
        i = math.floor(x)
        t = x - i
        t = t * t * (3 - 2 * t)
        return self.values[i % n] * (1 - t) + self.values[(i + 1) % n] * t


def fill(draw, points, color):
    scaled = [(x * SUPERSAMPLE, y * SUPERSAMPLE) for x, y in points]
    # 边线同色，相邻三角形之间不露缝。
    draw.polygon(scaled, fill=color, outline=color)


def facets(width, height, cell, jitter, seed):
    """抖动网格切成三角形（每格沿较短的对角线切开），网格在两个方向都首尾相接。返回 (三个顶点, 随机数)。"""
    rng = random.Random(seed)
    cols, rows = round(width / cell), round(height / cell)
    cw, ch = width / cols, height / rows
    offsets = {(i, j): (rng.uniform(-jitter, jitter) * cw, rng.uniform(-jitter, jitter) * ch) for i in range(cols) for j in range(rows)}

    def vertex(i, j):
        dx, dy = offsets[i % cols, j % rows]
        return (i * cw + dx, j * ch + dy)

    out = []
    for i in range(cols):
        for j in range(rows):
            a, b, c, d = vertex(i, j), vertex(i + 1, j), vertex(i + 1, j + 1), vertex(i, j + 1)
            if math.dist(a, c) < math.dist(b, d):
                pair = ((a, b, c), (a, c, d))
            else:
                pair = ((a, b, d), (b, c, d))
            out += [(tri, rng.random()) for tri in pair]
    return out


def render(width, height, paint, mode="RGB"):
    image = Image.new(mode, (width * SUPERSAMPLE, height * SUPERSAMPLE), (0, 0, 0, 0) if mode == "RGBA" else (0, 0, 0))
    paint(ImageDraw.Draw(image))
    return image.resize((width, height), Image.LANCZOS)


def save(image, name, **options):
    path = os.path.join(OUT, name)
    image.save(path, "WEBP", method=6, **options)
    print(f"{name}: {image.width}x{image.height} {os.path.getsize(path) // 1024}KiB")


def tile(scheme):
    """正文背后的底纹：低对比、四边无缝，按 640px 平铺。"""
    size = 960
    base, facet = rgb(scheme["background"]), rgb(scheme["facet"])

    def paint(draw):
        for tri, r in facets(size, size, 96, 0.36, 11):
            for ox in (-size, 0, size):
                for oy in (-size, 0, size):
                    moved = [(x + ox, y + oy) for x, y in tri]
                    if max(x for x, _ in moved) >= 0 and min(x for x, _ in moved) <= size and max(y for _, y in moved) >= 0 and min(y for _, y in moved) <= size:
                        fill(draw, moved, mix(base, facet, r))
    return render(size, size, paint)


def shade(a, b, c):
    """三维三角形的朝向光照，0 背光到 1 正对太阳。"""
    u = [b[i] - a[i] for i in range(3)]
    v = [c[i] - a[i] for i in range(3)]
    n = (u[1] * v[2] - u[2] * v[1], u[2] * v[0] - u[0] * v[2], u[0] * v[1] - u[1] * v[0])
    length = math.hypot(*n) or 1
    if n[1] < 0:
        n = tuple(-x for x in n)
    return max(0.0, min(1.0, sum(x * s for x, s in zip(n, SUN)) / length * 0.5 + 0.5))


def peaks(scheme):
    """首页山景：天空三角面，下面是由远及近画的群山，两侧高、中间低，把站名和入口让出来。"""
    width, height = 2400, 1350
    horizon = height * 0.60
    rows, cols = 22, 44
    rng = random.Random(23)
    jitter = {(i, j): (rng.uniform(-0.3, 0.3), rng.uniform(-0.3, 0.3), rng.random()) for i in range(rows + 1) for j in range(cols + 2)}
    wide, fine = Noise(5, 64), Noise(6, 64)
    fog_color = rgb(scheme["sky"][2])

    def lift(u, t):
        """u 是横向 0–1，t 是由远（0）到近（1）的深度；返回山高（像素）。"""
        phase = t * 11.3
        ridge = 1 - abs(2 * wide(u * 6 + phase) - 1)
        detail = 1 - abs(2 * fine(u * 17 + phase * 1.7) - 1)
        sides = 0.18 + 0.82 * smooth(0.14, 0.48, abs(u - 0.5))
        return (ridge ** 1.3 * 0.86 + detail * 0.14) * sides * (height * 0.5) * (1 - t) ** 1.5

    def point(i, j):
        t = i / rows
        dx, dy, _ = jitter[i, j]
        u = (j - 0.5 + dx) / cols
        return u, t, width * u, horizon + (height - horizon) * 1.08 * max(0.0, t + dy / rows) ** 1.3, lift(u, t)

    def paint(draw):
        cell = 118
        for tri, r in facets(width, round(horizon * 1.3), cell, 0.36, 31):
            # 网格抖动后上沿和左右会缺角：整体上移一格，横向把首尾相接的那一圈也画上。
            tri = [(x, y - cell) for x, y in tri]
            color = mix(ramp(scheme["sky"], sum(y for _, y in tri) / 3 / horizon), rgb(scheme["sky"][1]), r * 0.16)
            for ox in (-width, 0, width):
                fill(draw, [(x + ox, y) for x, y in tri], color)
        for i in range(rows):
            t = (i + 0.5) / rows
            for j in range(cols + 1):
                a, b, c, d = point(i, j), point(i, j + 1), point(i + 1, j + 1), point(i + 1, j)
                for tri in ((a, b, c), (a, c, d)) if jitter[i, j][2] < 0.5 else ((a, b, d), (b, c, d)):
                    # 三维坐标：横向、高度、纵深；纵深拉开后坡面的明暗才分得开。
                    light = shade(*[(p[0] * 7, p[4] / height * 2.6, p[1] * 4) for p in tri])
                    top = max(p[4] for p in tri) / (height * 0.5)
                    light = smooth(0.3, 0.86, light)
                    # 越近越暗，山脚沉下去，亮面留给远处的山。
                    color = ramp(scheme["land"], light * (1 - t * 0.55))
                    color = mix(color, rgb(scheme["snow"]), smooth(0.36, 0.56, top) * (0.3 + 0.7 * light))
                    fill(draw, [(p[2], p[3] - p[4]) for p in tri], mix(color, fog_color, (1 - t) ** 4 * 0.5))
    return render(width, height, paint)


def ridge_layer(draw, width, height, cols, layers, peak, base, seed, stops, fade_to, shift):
    wide, fine = Noise(seed, cols), Noise(seed + 1, cols // 2)
    rng = random.Random(seed)
    jitter = {(i, k): rng.uniform(-0.28, 0.28) for i in range(cols) for k in range(layers + 1)}

    def point(i, k):
        # k 从 0（山脊线）到 layers（底边），底边是平的，好接页脚的实色。
        f = k / layers
        crest = (wide((i + shift) * 0.25) * 0.4 + (1 - abs(2 * fine((i + shift) * 0.5) - 1)) * 0.6) * peak
        return (i + jitter[i % cols, k]) * width / cols, height - (crest + base) * (1 - f)

    for k in range(layers):
        for i in range(cols):
            a, b, c, d = point(i, k), point(i + 1, k), point(i + 1, k + 1), point(i, k + 1)
            for index, tri in enumerate(((a, b, c), (a, c, d)) if (i + k) % 2 else ((a, b, d), (b, c, d))):
                r = random.Random(seed * 1000003 + i * 1009 + k * 13 + index).random()
                # 迎光的左坡亮、背光的右坡暗，越靠近底边越融进页脚底色。
                slope = (a[1] - b[1]) / (height or 1) * 6
                color = ramp(stops, 0.5 + slope + (r - 0.5) * 0.55)
                color = mix(color, fade_to, smooth(0.45, 1.0, (k + 1) / layers))
                color = (*color, 255)
                for ox in (-width, 0, width):
                    fill(draw, [(x + ox, y) for x, y in tri], color)


def ridge(scheme):
    """页脚上沿的山脊：横向无缝，透明天空；最下面一排正好是页脚底色。"""
    width, height = 3200, 320
    footer = rgb(scheme["footer"])

    def paint(draw):
        ridge_layer(draw, width, height, 56, 3, 270, 30, 41, scheme["ridge_far"], rgb(scheme["ridge_far"][0]), 0)
        ridge_layer(draw, width, height, 64, 4, 210, 20, 47, scheme["ridge"], footer, 7)
    return render(width, height, paint, "RGBA")


def trim():
    """页头下沿的一条彩色碎面，两种配色共用。"""
    width, height = 1200, 24

    def paint(draw):
        for tri, r in facets(width, height, 24, 0.3, 53):
            x = sum(p[0] for p in tri) / 3
            # 颜色沿横向循环，接缝处对得上。
            t = 0.5 - 0.5 * math.cos(x / width * 2 * math.pi)
            color = ramp(TRIM, t * 0.86 + r * 0.14)
            for ox in (-width, 0, width):
                for oy in (-height, 0, height):
                    fill(draw, [(px + ox, py + oy) for px, py in tri], color)
    return render(width, height, paint)


def main():
    os.makedirs(OUT, exist_ok=True)
    for name, scheme in SCHEMES.items():
        save(tile(scheme), f"facets-{name}.webp", lossless=True)
        save(peaks(scheme), f"peaks-{name}.webp", quality=90)
        save(ridge(scheme), f"ridge-{name}.webp", lossless=True)
    save(trim(), "trim.webp", lossless=True)


if __name__ == "__main__":
    main()
