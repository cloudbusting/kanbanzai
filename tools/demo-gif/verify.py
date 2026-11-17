# Checks a take of the board demo GIF and its candidate file; prints one FAIL line per problem and exits 1 if any.
# The Python standard library only.
#
#   demo.sh verify --project DIR --frames DIR --scenes scenes.json --gif FILE --transcripts DIR
#
# project      the demo project: a BUG card in Done with a fix commit, an epic and at least five other cards
# frames       the recording: at least one <ms>-<scene>.png for every scene in scenes.json
# gif          (optional) the candidate: GIF89a, at most --max-bytes (4 MiB), --max-seconds (45) long, --width (960) wide
# transcripts  the Claude Code session's files (the scratch config's projects folder): no email, home path or token
import argparse
import glob
import json
import os
import re
import struct
import subprocess
import sys

ALLOWED_EMAILS = {'noreply@anthropic.com', 'demo@example.com', 'd@example.com'}
EMAIL = re.compile(r'[\w.+-]+@[\w-]+(?:\.[\w-]+)+')
HOME_PATH = re.compile(r'/home/[A-Za-z0-9_.-]+')
TOKEN = re.compile(r'sk-ant-[A-Za-z0-9_-]{8,}|Bearer\s+[A-Za-z0-9._~+/-]{16,}|gh[pousr]_[A-Za-z0-9]{20,}')
# the TUI does not print the skill text Claude Code loads, whose first line names the skill's folder
SKILL_BASE = re.compile(r'Base directory for this skill: \S+')


def gif_facts(path):
    """(signature, width, seconds) of a GIF, by walking its blocks."""
    with open(path, 'rb') as f:
        data = f.read()
    if len(data) < 13:
        return data[:6], 0, 0.0
    signature = data[:6]
    width = struct.unpack('<H', data[6:8])[0]
    flags = data[10]
    pos = 13
    if flags & 0x80:
        pos += 3 * (2 << (flags & 7))
    centis = 0
    while pos < len(data):
        block = data[pos]
        if block == 0x3B:
            break
        if block == 0x21:
            label = data[pos + 1]
            if label == 0xF9 and pos + 5 < len(data):
                centis += struct.unpack('<H', data[pos + 4:pos + 6])[0]
            pos += 2
            while pos < len(data) and data[pos]:
                pos += data[pos] + 1
            pos += 1
        elif block == 0x2C:
            iflags = data[pos + 9]
            pos += 10
            if iflags & 0x80:
                pos += 3 * (2 << (iflags & 7))
            pos += 1
            while pos < len(data) and data[pos]:
                pos += data[pos] + 1
            pos += 1
        else:
            break
    return signature, width, centis / 100.0


def check_project(project, fails):
    try:
        out = subprocess.run(['kanbanzai', 'board', '--json'], cwd=project, capture_output=True, text=True, check=True).stdout
        board = json.loads(out)
    except (OSError, subprocess.CalledProcessError, ValueError) as err:
        fails.append('the demo project\'s board could not be read: %s' % err)
        return
    cards = [c for col in board.get('columns', []) for c in col.get('cards', [])]
    if not any(c.get('id', '').startswith('BUG-') and c.get('stage') == 'done' for c in cards):
        fails.append('no BUG card is in Done on the demo project\'s board')
    if not any(c.get('id', '').startswith('EPIC-') for c in cards):
        fails.append('the demo project has no epic card')
    others = [c for c in cards if not c.get('id', '').startswith(('EPIC-', 'BUG-'))]
    if len(others) < 5:
        fails.append('the demo project has %d cards under the epic, want at least 5' % len(others))
    try:
        log = subprocess.run(['git', 'log', '--format=%s'], cwd=project, capture_output=True, text=True, check=True).stdout
    except (OSError, subprocess.CalledProcessError) as err:
        fails.append('the demo project\'s git log could not be read: %s' % err)
        return
    if not any(line.startswith('BUG-') and 'triaged' not in line for line in log.splitlines()):
        fails.append('the demo project has no fix commit for the BUG card')


def check_frames(frames, scenes_file, fails):
    spec = json.load(open(scenes_file))
    names = [os.path.basename(p) for p in glob.glob(os.path.join(frames, '*.png'))]
    for scene in spec['scenes']:
        if not any(re.fullmatch(r'\d+-' + re.escape(scene['name']) + r'\.png', n) for n in names):
            fails.append('the recording has no frame for the scene "%s"' % scene['name'])


def check_gif(gif, max_bytes, max_seconds, width, fails):
    if not os.path.exists(gif):
        fails.append('the candidate GIF %s does not exist' % gif)
        return
    size = os.path.getsize(gif)
    signature, w, seconds = gif_facts(gif)
    if signature != b'GIF89a':
        fails.append('the candidate is not a GIF89a file')
    if size > max_bytes:
        fails.append('the candidate is %d bytes, over the %d byte budget' % (size, max_bytes))
    if seconds > max_seconds:
        fails.append('the candidate runs %.1f seconds, over the %d second budget' % (seconds, max_seconds))
    if w != width:
        fails.append('the candidate is %d pixels wide, want %d' % (w, width))


def check_transcripts(folder, fails):
    files = [p for p in glob.glob(os.path.join(folder, '**', '*'), recursive=True) if os.path.isfile(p)]
    if not files:
        fails.append('no transcript files under %s to scan' % folder)
        return
    for path in files:
        try:
            text = open(path, encoding='utf-8', errors='replace').read()
        except OSError:
            continue
        text = SKILL_BASE.sub('', text)
        for what, found in (
            ('an email address', [m for m in EMAIL.findall(text) if m.lower() not in ALLOWED_EMAILS]),
            ('a home path', HOME_PATH.findall(text)),
            ('a token', TOKEN.findall(text)),
        ):
            if found:
                fails.append('%s holds %s (%s)' % (os.path.relpath(path, folder), what, found[0][:12] + '...'))


def main():
    ap = argparse.ArgumentParser(description='Check a board demo take and its candidate GIF.')
    ap.add_argument('--project', required=True)
    ap.add_argument('--frames', required=True)
    ap.add_argument('--scenes', required=True)
    ap.add_argument('--gif', help='the candidate; leave out to check the take before the film is cut')
    ap.add_argument('--transcripts', required=True)
    ap.add_argument('--max-bytes', type=int, default=4 * 1024 * 1024)
    ap.add_argument('--max-seconds', type=int, default=45)
    ap.add_argument('--width', type=int, default=960)
    args = ap.parse_args()
    fails = []
    check_project(args.project, fails)
    check_frames(args.frames, args.scenes, fails)
    if args.gif:
        check_gif(args.gif, args.max_bytes, args.max_seconds, args.width, fails)
    check_transcripts(args.transcripts, fails)
    for line in fails:
        print('FAIL:', line)
    if fails:
        sys.exit(1)
    print('verify: the take and the candidate pass')


if __name__ == '__main__':
    main()
