# Turns the frames a recording session saved into docs/media/board-demo.gif's frames: it picks the frames of each
# scene, paints over the sandbox's "Not logged in" notice, scales to 960 wide and adds a caption under each scene.
# A development step for remaking the GIF, not shipped: it needs Pillow and numpy, which nothing else here uses.
#
#   python3 -I tools/demo-gif/assemble.py FRAMES_DIR OUT_DIR
#   ffmpeg -framerate 8 -i OUT_DIR/%04d.png -vf "split[a][b];[a]palettegen=max_colors=96:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" docs/media/board-demo.gif
#
# The scene table below holds frame numbers of the one recording the committed GIF was cut from; a remake changes
# them to its own (a contact sheet from ffmpeg's tile filter shows which frame is which).
import os, sys, glob
import numpy as np
from PIL import Image, ImageDraw, ImageFont

FR = sys.argv[1]
OUT = sys.argv[2]
files = sorted(glob.glob(FR + '/*.png'))

# (first frame, last frame, take every Nth, extra copies of the last to hold on it, the scene's caption)
SEGMENTS = [
    (50, 140, 2, 8, "Init, then Apply: set a project up from the status bar"),
    (185, 215, 3, 0, "Commit the setup, then start Claude Code in the terminal"),
    (375, 430, 2, 0, "/kanbanzai:triage turns a request into a card in Todo"),
    (430, 500, 4, 0, "/kanbanzai:triage turns a request into a card in Todo"),
    (500, 540, 8, 4, "/kanbanzai:triage turns a request into a card in Todo"),
    (600, 700, 5, 0, "/kanbanzai:breakdown splits a big requirement into linked cards"),
    (700, 760, 4, 6, "A session waiting for your answer lights up the board"),
    (760, 1060, 12, 0, "/kanbanzai:breakdown splits a big requirement into linked cards"),
    (1075, 1170, 8, 6, "/kanbanzai:breakdown splits a big requirement into linked cards"),
    (1175, 1500, 8, 6, "/kanbanzai:do claims a card and works it; the board follows"),
    (1560, 1700, 5, 6, "Every card is a Markdown brief: click one to open it"),
    (1735, 1755, 4, 14, "Cards are Markdown files in your repository"),
]

os.makedirs(OUT, exist_ok=True)
for f in glob.glob(OUT + '/*.png'):
    os.remove(f)

def mask(img):
    a = np.asarray(img.convert('RGB')).astype(int)
    r = a[683:700, 1030:1250]
    pink = ((r[..., 0] > 200) & (r[..., 1] < 150) & (r[..., 2] > 100)).sum()
    if pink > 30:
        bg = tuple(a[704, 1100])
        a[678:704, 1018:1268] = bg
        return Image.fromarray(a.astype('uint8')), True
    return img.convert('RGB'), False

n = 0
masked = 0
font = ImageFont.truetype('/usr/share/fonts/adwaita-sans-fonts/AdwaitaSans-Regular.ttf', 22)
for first, last, step, hold, caption in SEGMENTS:
    idx = list(range(first, last + 1, step))
    idx += [idx[-1]] * hold
    for i in idx:
        img, m = mask(Image.open(files[i]))
        masked += m
        img = img.resize((960, 600), Image.LANCZOS)
        canvas = Image.new('RGB', (960, 640), (11, 11, 11))
        canvas.paste(img, (0, 0))
        d = ImageDraw.Draw(canvas)
        w = d.textlength(caption, font=font)
        d.text(((960 - w) / 2, 606), caption, font=font, fill=(235, 235, 235))
        img = canvas
        img.save(f'{OUT}/{n:04d}.png')
        n += 1
print('frames', n, 'masked', masked)
