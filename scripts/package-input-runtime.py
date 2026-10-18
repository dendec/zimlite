"""Collect only the headless input/shaping runtime and its non-system libraries."""
from pathlib import Path
import shutil, subprocess, sys, re
triple, dest = sys.argv[1:]
out=Path(dest); (out/'lib').mkdir(parents=True,exist_ok=True)
search=[Path('/opt/input/lib'),Path('/opt/input/lib/m17n/1.0'),Path('/usr/lib')/triple,Path('/lib')/triple,Path('/pc-zim')]
queue=[Path('/tmp/zimlite-input-host'),Path('/tmp/zimlite-search-host'),Path('/tmp/libzimlite-text.so'),Path('/opt/input/lib/m17n/1.0/libmimx-anthy.so')]
system={'libc.so.6','libm.so.6','libpthread.so.0','libdl.so.2','librt.so.1','libstdc++.so.6','libgcc_s.so.1','ld-linux-aarch64.so.1','ld-linux-x86-64.so.2'}
seen=set()
while queue:
 p=queue.pop(0)
 if p.name in seen: continue
 seen.add(p.name)
 target=out/p.name if p.name.startswith('zimlite-') else out/'lib'/p.name
 shutil.copyfile(p.resolve(),target);target.chmod(0o755)
 info=subprocess.check_output([triple+'-readelf','-d',str(p)],text=True)
 for name in re.findall(r'\(NEEDED\).*?\[(.*?)\]',info):
  if name in system or name in seen: continue
  match=next((d/name for d in search if (d/name).exists()),None)
  if match is None:raise RuntimeError('Missing runtime dependency '+name)
  queue.append(match)
shutil.copytree('/opt/input/share/m17n',out/'db',dirs_exist_ok=True)
shutil.copytree('/usr/share/anthy',out/'anthy',dirs_exist_ok=True)
fonts=out/'fonts'
if fonts.exists():shutil.rmtree(fonts)
fonts.mkdir()
shutil.copyfile('/tmp/unifont.otf',fonts/'unifont.otf')
# Distribution notices accompany statically or dynamically shipped third-party data.
(out/'licenses').mkdir(exist_ok=True)
for pkg in ['m17n-db','libanthy-dev','anthy-common','libharfbuzz0b','libfribidi0','libfreetype6','libxml2','libicu67','zlib1g','libpng16-16','libbrotli1','liblzma5','libthai0','libdatrie1']:
 p=Path('/usr/share/doc')/pkg/'copyright'
 if p.exists():shutil.copyfile(p,out/'licenses'/(pkg+'.txt'))
shutil.copyfile('/tmp/m17n/COPYING',out/'licenses/m17n-lib-LGPL.txt')
