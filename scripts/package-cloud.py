#!/usr/bin/env python3
from pathlib import Path
from zipfile import ZipFile, ZIP_DEFLATED

out = Path(__file__).resolve().parent.parent / 'deploy/dist'
with ZipFile(out / 'bridge-function.zip', 'w', compression=ZIP_DEFLATED, compresslevel=9) as archive:
    for name in ('index.js', 'package.json'):
        archive.write(out / 'cloud' / name, name)
print('Created deploy/dist/bridge-function.zip (index.js and package.json only)')
