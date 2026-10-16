#!/usr/bin/env python3
"""Control only the repository's isolated QEMU guest through its QMP socket."""
import argparse
import json
from pathlib import Path
import socket
import stat
import time

VM = Path(__file__).resolve().parent / 'vm'
p = argparse.ArgumentParser()
p.add_argument('action', choices=['text', 'secret', 'key', 'screen', 'quit'])
p.add_argument('value', nargs='?', default='')
a = p.parse_args()
s = socket.socket(socket.AF_UNIX)
s.connect(str(VM / 'qmp.sock'))
f = s.makefile('rwb', buffering=0)
f.readline()

def execute(command, arguments=None):
    message = {'execute': command}
    if arguments is not None:
        message['arguments'] = arguments
    f.write(json.dumps(message).encode() + b'\n')
    while True:
        result = json.loads(f.readline())
        if 'error' in result:
            raise RuntimeError(result['error'])
        if 'return' in result:
            return result['return']

execute('qmp_capabilities')
if a.action == 'screen':
    execute('screendump', {'filename': str(VM / 'screen.png'), 'format': 'png'})
elif a.action == 'quit':
    execute('quit')
elif a.action == 'key':
    execute('human-monitor-command', {'command-line': 'sendkey ' + a.value + ' 10'})
    time.sleep(0.08)
else:
    if a.action == 'secret':
        credential = VM / 'credential.txt'
        if stat.S_IMODE(credential.stat().st_mode) != 0o600:
            raise RuntimeError('Test credential must have mode 0600')
        a.value = credential.read_text().rstrip('\n')
    keys = {' ': 'spc', '\n': 'ret', '-': 'minus', '_': 'shift-minus', '=': 'equal',
            '+': 'shift-equal', '/': 'slash', ':': 'shift-semicolon', ';': 'semicolon',
            '.': 'dot', ',': 'comma', '>': 'shift-dot', '<': 'shift-comma',
            "'": 'apostrophe', '"': 'shift-apostrophe', '|': 'shift-backslash',
            '\\': 'backslash', '$': 'shift-4', '(': 'shift-9', ')': 'shift-0',
            '*': 'shift-8', '&': 'shift-7', '?': 'shift-slash', '!': 'shift-1',
            '@': 'shift-2', '#': 'shift-3', '%': 'shift-5', '^': 'shift-6',
            '[': 'bracket_left', ']': 'bracket_right',
            '{': 'shift-bracket_left', '}': 'shift-bracket_right',
            '`': 'grave_accent', '~': 'shift-grave_accent'}
    for char in a.value:
        key = keys.get(char, 'shift-' + char.lower() if char.isupper() else char)
        execute('human-monitor-command', {'command-line': 'sendkey ' + key + ' 10'})
        time.sleep(0.05)
