#!/usr/bin/env python3
"""Keep absolute Debian symlinks inside the exported cross-compilation sysroot."""
import os
import pathlib
import sys

root = pathlib.Path(sys.argv[1]).resolve()
for directory, directories, files in os.walk(root, followlinks=False):
    for name in directories + files:
        link = pathlib.Path(directory) / name
        if not link.is_symlink():
            continue
        target = os.readlink(link)
        if target.startswith('/'):
            relative = os.path.relpath(root / target.lstrip('/'), link.parent)
            link.unlink()
            link.symlink_to(relative)
