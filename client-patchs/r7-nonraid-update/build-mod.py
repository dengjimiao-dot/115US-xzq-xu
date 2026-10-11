"""Package the committed paired DLLs through schema-2 modkit; never install here."""
from pathlib import Path
import argparse, hashlib, json, zipfile

ROOT = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--client', type=Path, help='Inspect existing target files to select add/replace operations (read-only).')
    parser.add_argument('--out', type=Path, default=ROOT / 'dist/r7-nonraid-client.mod.zip')
    args = parser.parse_args()
    paired = json.loads((ROOT / 'paired-manifest.json').read_text('utf-8'))
    ops = []
    for row in paired['files']:
        target = row['target']
        payload = ROOT / 'payload/client' / target
        data = payload.read_bytes()
        if len(data) != row['size'] or hashlib.sha256(data).hexdigest() != row['sha256']:
            raise ValueError('Paired payload changed: ' + target)
        exists = args.client is not None and (args.client / target).is_file()
        # A clean game already contains dinput8.dll; the other files can be new.
        kind = 'file.replace' if exists or (args.client is None and target == 'dinput8.dll') else 'file.add'
        ops.append(dict(kind=kind, target=target, source='client/' + target,
                        size=row['size'], sha256=row['sha256']))
    manifest = dict(schema=2, id='qol.r7-nonraid-client', version='0.1.0-upstream-candidate',
                    name='R7 nonraid paired client DLLs', author='Deemo',
                    permissions=['client.file.write'], layers={'client': {'ops': ops}})
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(args.out, 'w', zipfile.ZIP_DEFLATED) as archive:
        archive.writestr('mod.json', json.dumps(manifest, indent=2) + '\n')
        archive.write(ROOT / 'paired-manifest.json', 'evidence/paired-manifest.json')
        for row in paired['files']:
            archive.write(ROOT / 'payload/client' / row['target'], 'client/' + row['target'])
    print(args.out.resolve())


if __name__ == '__main__':
    main()
