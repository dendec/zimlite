#!/usr/bin/env python3
"""Package only CPU libraries and reject artifacts requiring newer glibc."""
import argparse
import json
import pathlib
import re
import shutil
import subprocess

parser = argparse.ArgumentParser()
parser.add_argument('--helper', required=True)
parser.add_argument('--sysroot', required=True)
parser.add_argument('--output', required=True)
parser.add_argument('--software', action='store_true')
parser.add_argument('--readelf', default='aarch64-linux-gnu-readelf')
args = parser.parse_args()
root = pathlib.Path(args.sysroot)
out = pathlib.Path(args.output)
binary = out / 'aarch64/bin/zimlite-servo-host'
binary.parent.mkdir(parents=True, exist_ok=True)
libs = out / ('aarch64/software' if args.software else 'aarch64/lib')
libs.mkdir(parents=True, exist_ok=True)
shutil.copy2(args.helper, binary)
system = {'libc.so.6', 'libm.so.6', 'libdl.so.2', 'libpthread.so.0', 'librt.so.1', 'ld-linux-aarch64.so.1'}
seen = set()
queue = [binary]
if args.software:
    queue = []
    for name in ('libEGL.so.1', 'libEGL_mesa.so.0', 'libGLESv2.so.2'):
        source = root / 'usr/lib/aarch64-linux-gnu' / name
        target = libs / name
        shutil.copyfile(source.resolve(), target)
        queue.append(target)
    dri = libs / 'dri'
    dri.mkdir(exist_ok=True)
    driver = root / 'usr/lib/aarch64-linux-gnu/dri/swrast_dri.so'
    shutil.copyfile(driver.resolve(), dri / driver.name)
    queue.append(dri / driver.name)
    (libs / 'egl_vendor.json').write_text(json.dumps({'file_format_version': '1.0.0', 'ICD': {'library_path': 'libEGL_mesa.so.0'}}) + '\n')
report = {}
while queue:
    path = queue.pop()
    versions = subprocess.check_output([args.readelf, '--version-info', str(path)], text=True)
    required = sorted(set(re.findall(r'\bGLIBC_(\d+\.\d+)\b', versions)), key=lambda s: tuple(map(int, s.split('.'))))
    if required and tuple(map(int, required[-1].split('.'))) > (2, 30):
        raise SystemExit(f'{path.name} requires GLIBC_{required[-1]}; target maximum is 2.30')
    report[path.name] = required
    dynamic = subprocess.check_output([args.readelf, '-d', str(path)], text=True)
    for name in re.findall(r'\(NEEDED\).*?\[(.*?)\]', dynamic):
        if name in system or name in seen:
            continue
        seen.add(name)
        if not args.software and re.match(r'lib(?:EGL|GL|GLES|gbm|drm|vulkan)', name):
            raise SystemExit(f'GPU library must come from firmware: {name}')
        candidates = [root / 'usr/lib/aarch64-linux-gnu' / name, root / 'lib/aarch64-linux-gnu' / name, root / 'usr/aarch64-linux-gnu/lib' / name]
        source = next((p for p in candidates if p.exists()), None)
        if source is None:
            raise SystemExit(f'missing runtime dependency: {name}')
        target = libs / name
        shutil.copyfile(source.resolve(), target)
        queue.append(target)
(out / ('abi-software.json' if args.software else 'abi.json')).write_text(json.dumps(report, indent=2) + '\n')
(out / 'fonts').mkdir(exist_ok=True)
for p in (root / 'usr/share/fonts').rglob('*'):
    if p.suffix.lower() in {'.ttf', '.otf'}:
        shutil.copy2(p, out / 'fonts' / p.name)
licenses = out / 'licenses/debian'
licenses.mkdir(parents=True, exist_ok=True)
for p in (root / 'usr/share/doc').glob('*/copyright'):
    shutil.copy2(p, licenses / (p.parent.name + '.copyright'))
