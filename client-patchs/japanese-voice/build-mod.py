"""Create a schema-2 voice mod; inspect the client without installing it."""
from pathlib import Path
import argparse, hashlib, json, re, zipfile

ROOT = Path(__file__).resolve().parent


def sha(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--client', type=Path, required=True)
    parser.add_argument('--out', type=Path, default=ROOT / 'dist/japanese-voice.mod.zip')
    args = parser.parse_args()
    client, output = args.client.resolve(), args.out.resolve()
    if output == client or client in output.parents:
        raise ValueError('Output must be outside the client.')
    if output.exists():
        raise ValueError('Output already exists; choose a new output file.')
    manifest = json.loads((ROOT / 'manifest.json').read_text('utf-8'))
    unknown = {p.name.lower() for p in (client / 'SoundPacks').glob('*.npk')} - {
        name.lower() for name in manifest['baseline_pack_names']}
    if unknown:
        raise ValueError('Other sound patches present: ' + ', '.join(sorted(unknown)))
    ops = []
    for row in manifest['files']:
        name = row['name']
        if not re.fullmatch(r'sounds_[A-Za-z0-9_]+\.npk', name):
            raise ValueError('Invalid resource name: ' + name)
        payload = ROOT / 'payload/SoundPacks' / name
        if payload.stat().st_size != row['patched_size'] or sha(payload) != row['patched_sha256']:
            raise ValueError('Corrupt or missing paired audio payload: ' + name)
        # Require original resources so modkit's recorded backups restore this baseline.
        if sha(client / 'SoundPacks' / name) != row['original_sha256']:
            raise ValueError('Client original audio differs: ' + name)
        target = 'SoundPacks/' + name
        ops.append(dict(kind='file.replace', target=target, source='client/' + target,
                        size=row['patched_size'], sha256=row['patched_sha256']))
    organization = dict(schema=2, id='qol.japanese-voice', version='0.1.0-candidate',
                        name='Japanese voice replacement', author='Deemo',
                        permissions=['client.file.write'], layers={'client': {'ops': ops}})
    output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED, compresslevel=1) as archive:
        archive.writestr('mod.json', json.dumps(organization, indent=2) + '\n')
        archive.write(ROOT / 'manifest.json', 'evidence/manifest.json')
        for row in manifest['files']:
            archive.write(ROOT / 'payload/SoundPacks' / row['name'], 'client/SoundPacks/' + row['name'])
    print(output)


if __name__ == '__main__':
    main()
