# Turns the frames a recording saved into the frames of the board demo GIF: for each scene in scenes.json it takes the
# frames named <epoch-ms>-<scene>.png, steps through them to fill the scene's seconds, paints over the "Not logged in"
# notice Claude Code shows when it signs in through a gateway (as the sandbox does), scales to the width, and adds the
# scene's caption under the frame. Then ffmpeg makes the GIF:
#
#   demo.sh assemble FRAMES_DIR OUT_DIR
#   ffmpeg -framerate 8 -i OUT_DIR/%04d.png -vf "split[a][b];[a]palettegen=max_colors=96:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" board-demo.gif
#
# A development step for remaking the GIF, not shipped. It needs Pillow and numpy, which nothing else here uses.
import glob
import json
import os
import sys

import numpy as np
from PIL import Image, ImageDraw, ImageFont

CAPTION_BAR = 40
FONT_PATHS = [
    '/usr/share/fonts/adwaita-sans-fonts/AdwaitaSans-Regular.ttf',
    '/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf',
    '/usr/share/fonts/dejavu-sans-fonts/DejaVuSans.ttf',
]


def load_font(size):
    for path in FONT_PATHS:
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    return ImageFont.load_default()


def mask(img):
    """Paints over the gateway's "Not logged in" notice (pink text above Claude Code's input box)."""
    a = np.asarray(img.convert('RGB')).astype(int)
    h, w = a.shape[:2]
    if h < 800 or w < 1280:
        return img.convert('RGB'), False
    r = a[683:700, 1030:1250]
    pink = ((r[..., 0] > 200) & (r[..., 1] < 150) & (r[..., 2] > 100)).sum()
    if pink > 30:
        a[678:704, 1018:1268] = a[704, 1100].copy()
        return Image.fromarray(a.astype('uint8')), True
    return img.convert('RGB'), False


def frames_by_scene(frames_dir):
    scenes = {}
    for path in sorted(glob.glob(os.path.join(frames_dir, '*-*.png'))):
        stem = os.path.basename(path)[:-4]
        ms, _, scene = stem.partition('-')
        if ms.isdigit() and scene:
            scenes.setdefault(scene, []).append((int(ms), path))
    return {name: [p for _, p in sorted(items)] for name, items in scenes.items()}


def pick(paths, count):
    """count frames spread evenly over paths (repeating when there are fewer)."""
    if count <= 1 or len(paths) == 1:
        return [paths[-1]] * max(count, 1)
    return [paths[round(i * (len(paths) - 1) / (count - 1))] for i in range(count)]


def main(frames_dir, out_dir, scenes_file):
    spec = json.load(open(scenes_file))
    fps, width, hold = spec.get('fps', 8), spec.get('width', 960), spec.get('hold', 1.0)
    found = frames_by_scene(frames_dir)
    missing = [s['name'] for s in spec['scenes'] if s['name'] not in found]
    if missing:
        sys.exit('assemble.py: no frames for the scene(s) %s in %s' % (', '.join(missing), frames_dir))
    os.makedirs(out_dir, exist_ok=True)
    for old in glob.glob(os.path.join(out_dir, '*.png')):
        os.remove(old)
    font = load_font(22)
    n = masked = 0
    for scene in spec['scenes']:
        total = max(1, round(scene['seconds'] * fps))
        still = min(total - 1, round(scene.get('hold', hold) * fps))
        chosen = pick(found[scene['name']], total - still)
        chosen += [chosen[-1]] * still
        for path in chosen:
            img, was_masked = mask(Image.open(path))
            masked += was_masked
            height = round(img.height * width / img.width)
            img = img.resize((width, height), Image.LANCZOS)
            canvas = Image.new('RGB', (width, height + CAPTION_BAR), (11, 11, 11))
            canvas.paste(img, (0, 0))
            draw = ImageDraw.Draw(canvas)
            text_width = draw.textlength(scene['caption'], font=font)
            draw.text(((width - text_width) / 2, height + 6), scene['caption'], font=font, fill=(235, 235, 235))
            canvas.save(os.path.join(out_dir, '%04d.png' % n))
            n += 1
    print('frames', n, 'masked', masked)


if __name__ == '__main__':
    if len(sys.argv) not in (3, 4):
        sys.exit('usage: assemble.py FRAMES_DIR OUT_DIR [SCENES_JSON]')
    main(sys.argv[1], sys.argv[2], sys.argv[3] if len(sys.argv) == 4 else os.path.join(os.path.dirname(os.path.abspath(__file__)), 'scenes.json'))
