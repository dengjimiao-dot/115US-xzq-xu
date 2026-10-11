"""Prepare a separate paired server profile; never overwrite a runtime profile or save."""
from pathlib import Path
import argparse
import importlib.util
import json
import shutil


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--resources', type=Path, required=True)
    parser.add_argument('--base-profile', type=Path, required=True, help='Existing launcher profile; all unrelated environment values survive.')
    parser.add_argument('--binary', type=Path, required=True, help='Gateway freshly compiled from this branch.')
    parser.add_argument('--out', type=Path, required=True, help='New empty candidate directory outside the game client.')
    args = parser.parse_args()
    source = args.resources.resolve(); out = args.out.resolve()
    if out.exists() and any(out.iterdir()):
        raise ValueError('Output must be empty')
    if out == source or source in out.parents or out in source.parents:
        raise ValueError('Output overlaps resources')
    if not args.binary.is_file():
        raise ValueError('Compile the paired gateway first')
    spec = importlib.util.spec_from_file_location('pair', Path(__file__).with_name('refresh-pair.py'))
    pair = importlib.util.module_from_spec(spec); spec.loader.exec_module(pair)
    metadata = pair.resources(source)
    profile = json.loads(args.base_profile.read_text('utf-8-sig'))
    profile['binary'] = str(args.binary.resolve())
    environment = profile.setdefault('environment', {})
    environment['DFO_PVF_ARCHIVE'] = str(out / 'Script.inner.pvf')
    environment['DFO_PVF_SHA256'] = metadata['files']['Script.inner.pvf']
    out.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source / 'Script.inner.pvf', out / 'Script.inner.pvf')
    (out / 'profile.json').write_text(json.dumps(profile, ensure_ascii=False, indent=2) + '\n', 'utf-8')
    print(out / 'profile.json')


if __name__ == '__main__':
    main()
