# Prints one Playwright step script with its placeholders filled in, ready to pass to the browser tool as `code`.
#
#   demo.sh step NAME FRAMES_DIR EDITOR_URL
#
# @FRAMES@ and @URL@ are the two arguments; @COMMAND@ and @ARGS@ are the first word and the rest of the scene's prompt
# in scenes.json (as JavaScript string literals), for the scene the script is named after. The Python standard library only.
import json
import os
import sys

here = os.path.dirname(os.path.abspath(__file__))
if len(sys.argv) != 4:
    sys.exit('usage: step.py NAME FRAMES_DIR EDITOR_URL')
name, frames, url = sys.argv[1:4]
path = os.path.join(here, 'playwright', name + '.js')
if not os.path.exists(path):
    steps = sorted(f[:-3] for f in os.listdir(os.path.join(here, 'playwright')) if f.endswith('.js'))
    sys.exit('step.py: no step "%s" (steps: %s)' % (name, ', '.join(steps)))
text = open(path).read()
spec = json.load(open(os.path.join(here, 'scenes.json')))
prompt = next((s.get('prompt', '') for s in spec['scenes'] if s['name'] == name), '')
command, _, args = prompt.partition(' ')
for key, value in (('@FRAMES@', frames), ('@URL@', url), ('@COMMAND@', json.dumps(command)), ('@ARGS@', json.dumps(args))):
    text = text.replace(key, value)
sys.stdout.write(text)
