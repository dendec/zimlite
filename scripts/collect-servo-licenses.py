#!/usr/bin/env python3
"""Keep locked crate provenance and available license/notice files together."""
import json
import pathlib
import shutil
import subprocess
import sys
out = pathlib.Path(sys.argv[1])
out.mkdir(parents=True, exist_ok=True)
metadata = json.loads(subprocess.check_output(['cargo', 'metadata', '--locked', '--offline', '--format-version=1'], text=True))
records = []
for package in metadata['packages']:
    base = pathlib.Path(package['manifest_path']).parent
    target = out / (package['name'] + '-' + package['version'])
    target.mkdir(exist_ok=True)
    records.append({k: package.get(k) for k in ('name', 'version', 'license', 'repository', 'source')})
    for file in base.iterdir():
        if file.is_file() and file.name.upper().startswith(('LICENSE', 'LICENCE', 'COPYING', 'NOTICE', 'AUTHORS')):
            shutil.copy2(file, target / file.name)
    if package.get('license_file'):
        file = base / package['license_file']
        if file.is_file():
            shutil.copy2(file, target / file.name)
(out / 'crates.json').write_text(json.dumps(records, ensure_ascii=False, indent=2) + '\n')
