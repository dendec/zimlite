"""Keep upstream m17n modules relocatable beside the PortMaster executable."""
from pathlib import Path
import sys
root = Path(sys.argv[1])
p = root / 'src/input.c'
s = p.read_text()
needle = '  module_file = alloca (strlen (M17N_MODULE_DIR)'
assert needle in s
s = s.replace(needle, '  const char *module_dir = getenv ("M17N_INPUT_MODULE_DIR");\n  if (!module_dir) module_dir = M17N_MODULE_DIR;\n  module_file = alloca (strlen (module_dir)')
s = s.replace('M17N_MODULE_DIR, MSYMBOL_NAME (module), DLOPEN_SHLIB_EXT);', 'module_dir, MSYMBOL_NAME (module), DLOPEN_SHLIB_EXT);')
p.write_text(s)
