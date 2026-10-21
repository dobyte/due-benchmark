"""Render the editorial cover locally: pip install pillow numpy.

Fonts default to Windows fonts; override with --latin-font and --cjk-font.
The artwork is decorative, not a visualization of benchmark rankings.
"""
from pathlib import Path
import argparse
import random
import numpy as np
from PIL import Image, ImageDraw, ImageFont, ImageFilter


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--latin-font', default='C:/Windows/Fonts/arialbd.ttf')
    parser.add_argument('--cjk-font', default='C:/Windows/Fonts/msyhbd.ttc')
    args = parser.parse_args()
    w, h = 2400, 1350
    yy, xx = np.mgrid[0:h, 0:w]
    glow = np.exp(-(((xx-1920)/630)**2 + ((yy-620)/590)**2))
    low = np.exp(-(((xx-500)/1100)**2 + ((yy-1250)/500)**2))
    rgb = np.zeros((h, w, 3), dtype=np.uint8)
    for c, (base, bright, blue) in enumerate([(6, 5, 1), (12, 31, 6), (23, 40, 13)]):
        rgb[:, :, c] = np.clip(base + bright*glow + blue*low, 0, 255)
    im = Image.fromarray(rgb).convert('RGBA')
    d = ImageDraw.Draw(im)
    # Quiet technical grid, with a darker title area.
    for x in range(0, w, 100):
        d.line((x, 0, x, h), fill=(15, 30, 42), width=1)
    for y in range(0, h, 100):
        d.line((0, y, w, y), fill=(15, 30, 42), width=1)
    d.polygon([(1390, 0), (2400, 0), (2400, 1350), (1030, 1350)], fill=(10, 23, 34))
    # Converging streaks and fragments suggest pressure, without invented charts.
    rng = random.Random(26)
    beams = Image.new('RGBA', im.size)
    bd = ImageDraw.Draw(beams)
    for i in range(44):
        x = rng.randint(1360, 2600)
        y = rng.randint(-180, 1330)
        length = rng.randint(90, 470)
        color = (27, 144, 170, rng.randint(35, 115))
        bd.line((x, y, x-length*.60, y+length), fill=color, width=rng.choice([1, 2, 3]))
    im = Image.alpha_composite(im, beams)
    # Three dim opposition rails; Due's luminous forward slash is dominant.
    d = ImageDraw.Draw(im)
    for offset in (0, 76, 152):
        d.polygon([(2110+offset, 370), (2131+offset, 370), (1805+offset, 1040), (1784+offset, 1040)], fill=(51, 69, 81))
    shape = [(1565, 1000), (1855, 488), (1690, 488), (2075, 218), (1888, 748), (1828, 624)]
    halo = Image.new('RGBA', im.size)
    hd = ImageDraw.Draw(halo)
    hd.polygon(shape, fill=(16, 231, 221, 180))
    im = Image.alpha_composite(im, halo.filter(ImageFilter.GaussianBlur(62)))
    im = Image.alpha_composite(im, halo.filter(ImageFilter.GaussianBlur(17)))
    d = ImageDraw.Draw(im)
    d.polygon(shape, fill=(73, 246, 225))
    d.polygon([(1565, 1000), (1855, 488), (1828, 624)], fill=(222, 255, 247))
    d.polygon([(1690, 488), (2075, 218), (1855, 488)], fill=(129, 255, 232))
    for i in range(23):
        x, y = rng.randint(1450, 2260), rng.randint(300, 1060)
        if 1690 < x < 2060 and 350 < y < 770:
            continue
        s = rng.randint(3, 8)
        d.polygon([(x, y), (x+s, y-s*2), (x+s*2, y-s*2), (x+s, y)], fill=(64, 183, 184))

    def font(size, cjk=False):
        return ImageFont.truetype(args.cjk_font if cjk else args.latin_font, size)

    def text(pos, value, size, color, cjk=False):
        # Anchor at glyph top to make spacing predictable for Latin and CJK.
        d.text(pos, value, font=font(size, cjk), fill=color, anchor='lt')

    mint, white, muted = '#6BFFE0', '#F1F6F8', '#91A6B6'
    d.rounded_rectangle((156, 132, 168, 174), radius=3, fill=mint)
    text((192, 133), 'DISTRIBUTED TCP BENCHMARK', 31, '#AEC1CD')
    text((158, 245), '向性能边界发起挑战', 43, muted, True)
    # Main brand occupies most of the cover's readable area.
    text((133, 353), 'DUE', 450, white)
    d.rectangle((162, 736, 277, 745), fill=mint)
    text((156, 803), '性能，有多能打？', 104, white, True)
    text((163, 966), '直面 Skynet · Pitaya · KBEngine', 43, '#A4B8C6', True)
    # Small background labels act as challengers rather than equal-size brands.
    text((1835, 103), 'THE CHALLENGERS', 22, '#63818F')
    text((1835, 145), 'Skynet / Pitaya / KBEngine', 25, '#A6BAC5')
    # Footer records real sample counts, with the single-host scope disclosed.
    d.line((160, 1138, 2240, 1138), fill='#29404C', width=2)
    text((162, 1184), '1080 万次请求', 44, mint, True)
    text((598, 1192), '108 个测试点', 31, white, True)
    text((980, 1192), '单机跨进程实测', 31, '#A4B8C6', True)
    text((1905, 1199), 'DUE / 2026.10', 28, '#7893A3')
    dest = Path(__file__).resolve().parent
    final = im.convert('RGB')
    final.save(dest / 'due-benchmark-cover.png', optimize=True)
    # Same composition at a convenient upload size.
    final.resize((1600, 900), Image.Resampling.LANCZOS).save(
        dest / 'due-benchmark-cover-zhihu.jpg', quality=95, subsampling=0, optimize=True)
    print(f'Cover saved to {dest}')


if __name__ == '__main__':
    main()
