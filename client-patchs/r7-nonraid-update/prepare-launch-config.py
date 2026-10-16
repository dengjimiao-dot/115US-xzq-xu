"""Plan or initialize missing robot launch configuration; preserve existing values."""
from pathlib import Path
import argparse
import json
import os
import uuid


def prepare(root, apply=False):
    here = Path(__file__).resolve().parent
    manifest = root / 'release-manifest.json'
    original = manifest.read_bytes() if manifest.exists() else None
    data = json.loads(original.decode('utf-8-sig')) if original else {'schema': 1, 'edition': 'local'}
    defaults = json.loads((here / 'templates/release-manifest.fragment.json').read_text('utf-8'))
    features = data.setdefault('local_features', {})
    robots = features.setdefault('robots', {})
    for key, value in defaults['local_features']['robots'].items():
        robots.setdefault(key, value)
    template = root / 'server/work/dfo-lan/configs/robots-template.json'
    planned = {'manifest': str(manifest), 'template': str(template),
               'create_template': not template.exists(), 'apply': apply}
    if apply:
        template.parent.mkdir(parents=True, exist_ok=True)
        if not template.exists():
            # Exclusive creation never replaces a user's prepared template.
            with template.open('xb') as stream:
                stream.write((here / 'templates/robots-template.json').read_bytes())
        if original:
            backup = manifest.with_name(manifest.name + '.' + uuid.uuid4().hex + '.backup')
            backup.write_bytes(original)
            planned['backup'] = str(backup)
        temp = manifest.with_name(manifest.name + '.' + uuid.uuid4().hex + '.tmp')
        try:
            temp.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
            os.replace(temp, manifest)
        finally:
            temp.unlink(missing_ok=True)
    return planned


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--apply', action='store_true', help='Write missing settings after reviewing the default plan.')
    args = parser.parse_args()
    print(json.dumps(prepare(args.root.resolve(), args.apply), ensure_ascii=False, indent=2))
