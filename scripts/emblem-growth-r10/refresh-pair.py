"""Bind rebuilt resources, then refresh the explicitly committed LocalFeatures pair."""
from pathlib import Path
import argparse
import hashlib
import json
import re
import shutil

ROOT = Path(__file__).resolve().parents[2]
SRC = ROOT / 'client-patchs/local-features/src'
PAIR = ROOT / 'client-patchs/r7-nonraid-update'


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def source_digest(path):
    raw = path.read_bytes()
    if path.suffix.lower() in {'.h', '.c', '.cpp', '.def', '.hex', '.json', '.txt', '.md'}:
        raw = raw.replace(b'\r\n', b'\n')
    return hashlib.sha256(raw).hexdigest()


def resources(directory):
    data = json.loads((directory / 'paired-resources.json').read_text('utf-8'))
    for target, expected in data['files'].items():
        path = directory / target if target == 'Script.inner.pvf' else directory / 'client' / target
        if digest(path) != expected:
            raise ValueError('Resource pair changed: ' + target)
    for row in data['image_packs']:
        if digest(directory / 'client' / row['target']) != row['sha256']:
            raise ValueError('Image pack changed: ' + row['target'])
    return data


def bindings():
    raw = (SRC / 'native_inventory_binding.h').read_text('utf-8')
    match = re.search(r'pvfSHA256\[32\]=\{([^}]+)\}', raw)
    outer = bytes(int(v.strip(), 16) for v in match[1].split(',')).hex()
    inner = re.search(r'char source\[\]="([a-f0-9]{64})"', (SRC / 'native_symbol_effects_binding.h').read_text('utf-8'))[1]
    return {'Script.pvf': outer, 'Script.inner.pvf': inner}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--resources', type=Path, required=True)
    parser.add_argument('--bind', action='store_true', help='Update the two resource identities before compiling.')
    parser.add_argument('--refresh-payload', action='store_true', help='After compiling, copy DLL and refresh its source/payload identities.')
    parser.add_argument('--additional-target', action='append', default=[], help='Also refresh a freshly rebuilt inherited DLL by its manifest target.')
    args = parser.parse_args()
    if args.bind and args.refresh_payload:
        parser.error('Bind, compile, then refresh in separate steps.')
    if args.additional_target and not args.refresh_payload:
        parser.error('--additional-target requires --refresh-payload')
    data = resources(args.resources.resolve())
    if args.bind:
        path = SRC / 'native_inventory_binding.h'
        text = path.read_text('utf-8')
        values = ','.join('0x' + data['files']['Script.pvf'][i:i+2] for i in range(0, 64, 2))
        text, count = re.subn(r'(pvfSHA256\[32\]=\{)[^}]+', lambda m: m[1] + values, text)
        if count != 1:
            raise ValueError('Inventory binding declaration changed')
        path.write_text(text, 'utf-8')
        path = SRC / 'native_symbol_effects_binding.h'
        text, count = re.subn(r'(char source\[\]=")[a-f0-9]{64}', lambda m: m[1] + data['files']['Script.inner.pvf'], path.read_text('utf-8'))
        if count != 1:
            raise ValueError('Effects binding declaration changed')
        path.write_text(text, 'utf-8')
    if bindings() != {k: data['files'][k] for k in bindings()}:
        raise ValueError('DLL source is not bound to these resources; bind and compile first')
    if args.refresh_payload:
        path = PAIR / 'paired-manifest.json'
        paired = json.loads(path.read_text('utf-8'))
        copies = []
        for target in ['.115us-mods/LocalFeatures.dll', *args.additional_target]:
            row = next(v for v in paired['files'] if v['target'] == target)
            source = ROOT / row['source_directory']
            dll = source.parent / 'dist' / Path(target).name
            if dll.stat().st_mtime_ns < max(p.stat().st_mtime_ns for p in source.rglob('*') if p.is_file()):
                raise ValueError('DLL is older than source; compile first: ' + target)
            copies.append((dll, PAIR / 'payload/client' / target))
            row.update(size=dll.stat().st_size, sha256=digest(dll), built_from_current_source=True)
            dependencies = set(row['source_files_sha256'])
            dependencies.update(p.relative_to(ROOT).as_posix() for p in source.rglob('*') if p.is_file())
            row['source_files_sha256'] = {p: source_digest(ROOT / p) for p in sorted(dependencies)}
        # Earlier R7 builds recorded the checkout's newline form. Normalize only
        # after proving that either the exact bytes or their LF form still match.
        for row in paired['files']:
            for name, expected in row.get('source_files_sha256', {}).items():
                source_path = ROOT / name
                normalized = source_digest(source_path)
                if expected not in {digest(source_path), normalized}:
                    raise ValueError('Inherited source changed; rebuild its target: ' + name)
                row['source_files_sha256'][name] = normalized
        paired['source_hash_normalization'] = 'Text source files use LF; binary payload fingerprints use exact bytes.'
        paired['release'] = 'r7-nonraid-with-emblem-growth-r10-candidate'
        paired['required_resources'] = {'files': data['files'], 'image_packs': data['image_packs'], 'manual_ui_accepted': False}
        for dll, destination in copies:
            shutil.copyfile(dll, destination)
        path.write_text(json.dumps(paired, ensure_ascii=False, indent=2) + '\n', 'utf-8')
    print('PASS resource binding' + (' and committed payload refreshed' if args.refresh_payload else ''))


if __name__ == '__main__':
    main()
