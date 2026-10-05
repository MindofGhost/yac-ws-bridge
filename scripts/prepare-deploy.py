#!/usr/bin/env python3
"""Create local configs once; never rotate an existing deployment's secret."""
from pathlib import Path
import json
import secrets

root = Path(__file__).resolve().parent.parent
out = root / 'deploy'
out.mkdir(exist_ok=True)
out.chmod(0o700)
files = [out / f'{role}.config.yaml' for role in ('adapter', 'helper')] + [out / 'cloud.env']
if any(p.exists() for p in files):
    if not all(p.exists() for p in files):
        raise SystemExit('Incomplete deployment: restore missing configs from your backup; secret was not rotated.')
    print('Existing deployment kept; configs and secret were not changed.')
else:
    token = secrets.token_urlsafe(32)
    for role in ('adapter', 'helper'):
        p = out / f'{role}.config.yaml'
        with p.open('x') as f:
            f.write((out / f'{role}.config.example.yaml').read_text().replace('YOUR_AUTH_TOKEN', token))
        p.chmod(0o644 if role == 'adapter' else 0o600)
    endpoints = json.loads((out / 'endpoints.json').read_text())
    p = out / 'cloud.env'
    with p.open('x') as f:
        f.write(f'AUTH_TOKEN={token}\nHTTP_URL=https://YOUR_PUBLIC_HOST{endpoints["http"]}\n')
    p.chmod(0o600)
    print('Created adapter/helper configs and cloud.env with a shared random secret.')
