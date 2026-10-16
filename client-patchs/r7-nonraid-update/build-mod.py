"""Package the committed paired DLLs through schema-2 modkit; never install here."""
from pathlib import Path
import argparse, hashlib, json, zipfile

ROOT = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--client', type=Path, help='Inspect existing target files to select add/replace operations (read-only).')
    parser.add_argument('--resources', type=Path, help='Verified output of scripts/emblem-growth-r10/build-paired-resources.py.')
    parser.add_argument('--out', type=Path, default=ROOT / 'dist/r7-nonraid-client.mod.zip')
    args = parser.parse_args()
    paired = json.loads((ROOT / 'paired-manifest.json').read_text('utf-8'))
    ops = []
    sources = {}
    for row in paired['files']:
        target = row['target']
        payload = ROOT / 'payload/client' / target
        data = payload.read_bytes()
        if len(data) != row['size'] or hashlib.sha256(data).hexdigest() != row['sha256']:
            raise ValueError('Paired payload changed: ' + target)
        for name, expected in row.get('source_files_sha256', {}).items():
            raw = (ROOT.parents[1] / name).read_bytes()
            if Path(name).suffix.lower() in {'.h', '.c', '.cpp', '.def', '.hex', '.json', '.txt', '.md'}:
                raw = raw.replace(b'\r\n', b'\n')
            if hashlib.sha256(raw).hexdigest() != expected:
                raise ValueError('Paired source changed; rebuild and refresh: ' + name)
        exists = args.client is not None and (args.client / target).is_file()
        # A clean game already contains dinput8.dll; the other files can be new.
        kind = 'file.replace' if exists or (args.client is None and target == 'dinput8.dll') else 'file.add'
        ops.append(dict(kind=kind, target=target, source='client/' + target,
                        size=row['size'], sha256=row['sha256']))
        sources['client/' + target] = payload
    required = paired.get('required_resources')
    if required:
        if args.client is None:
            parser.error('Paired resources require --client to select safe add/replace operations.')
        if args.resources is None:
            parser.error('This LocalFeatures DLL requires --resources; a DLL-only package is incomplete.')
        dll = (ROOT / 'payload/client/.115us-mods/LocalFeatures.dll').read_bytes()
        if bytes.fromhex(required['files']['Script.pvf']) not in dll or required['files']['Script.inner.pvf'].encode('ascii') not in dll:
            raise ValueError('LocalFeatures was not compiled with the required resource identities')
        for target, expected in required['files'].items():
            payload = args.resources / target if target == 'Script.inner.pvf' else args.resources / 'client' / target
            data = payload.read_bytes()
            if hashlib.sha256(data).hexdigest() != expected:
                raise ValueError('Required resource changed: ' + target)
            if target == 'Script.inner.pvf':
                # Not a modkit server-hook layer: select the paired profile explicitly.
                sources['paired-server/' + target] = payload
                continue
            exists = args.client is not None and (args.client / target).is_file()
            kind = 'file.replace' if exists or args.client is None else 'file.add'
            ops.append(dict(kind=kind, target=target, source='client/' + target, size=len(data), sha256=expected))
            sources['client/' + target] = payload
        for row in required['image_packs']:
            target = row['target']; payload = args.resources / 'client' / target; data = payload.read_bytes()
            if hashlib.sha256(data).hexdigest() != row['sha256']:
                raise ValueError('Required image pack changed: ' + target)
            exists = args.client is not None and (args.client / target).is_file()
            kind = 'file.replace' if exists else 'file.add'
            ops.append(dict(kind=kind, target=target, source='client/' + target, size=len(data), sha256=row['sha256']))
            sources['client/' + target] = payload
    manifest = dict(schema=2, id='qol.r7-nonraid-client', version='0.2.0-emblem-r10-candidate' if required else '0.1.0-upstream-candidate',
                    name='R7 nonraid paired client DLLs', author='Deemo',
                    permissions=['client.file.write'], layers={'client': {'ops': ops}})
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(args.out, 'w', zipfile.ZIP_DEFLATED) as archive:
        archive.writestr('mod.json', json.dumps(manifest, indent=2) + '\n')
        archive.write(ROOT / 'paired-manifest.json', 'evidence/paired-manifest.json')
        for target, payload in sources.items():
            archive.write(payload, target)
        archive.write(ROOT / 'README.md', 'README.md')
    print(args.out.resolve())


if __name__ == '__main__':
    main()
