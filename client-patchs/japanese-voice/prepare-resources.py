"""Restore two large NPKs from their verified Git LFS parts, without touching a client."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import tempfile


def digest(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def basename(name: str) -> str:
    if not name or Path(name).name != name or "/" in name or "\\" in name:
        raise ValueError(f"Unsafe resource name: {name!r}")
    return name


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, help="Optional separate directory for assembly verification")
    args = parser.parse_args()
    package = Path(__file__).resolve().parent
    payload = package / "payload" / "SoundPacks"
    manifest = json.loads((package / "manifest.json").read_text("utf-8"))
    originals = {row["name"]: row for row in manifest["files"]}
    output = args.out.resolve() if args.out else payload
    split_files = manifest.get("storage", {}).get("split_files", [])
    # Validate every source before creating any output. LFS pointers fail the size check.
    for row in split_files:
        expected = originals[basename(row["name"])]
        if sum(part["size"] for part in row["parts"]) != expected["patched_size"]:
            raise ValueError("Resource part sizes disagree with the manifest")
        for part in row["parts"]:
            source = payload / basename(part["name"])
            if not source.is_file() or source.stat().st_size != part["size"] or digest(source) != part["sha256"]:
                raise ValueError(f"Missing or corrupt LFS part: {source}; run git lfs pull first")
    output.mkdir(parents=True, exist_ok=True)
    for row in split_files:
        expected = originals[row["name"]]
        target = output / row["name"]
        if target.exists():
            if target.stat().st_size != expected["patched_size"] or digest(target) != expected["patched_sha256"]:
                raise ValueError(f"Refusing to overwrite an unexpected resource: {target}")
            print(f"Already verified: {target.name}")
            continue
        handle, temporary_name = tempfile.mkstemp(prefix=".jp-voice-assemble-", dir=output)
        temporary = Path(temporary_name)
        try:
            combined = hashlib.sha256()
            with os.fdopen(handle, "wb") as destination:
                for part in row["parts"]:
                    with (payload / part["name"]).open("rb") as source:
                        while block := source.read(1024 * 1024):
                            destination.write(block)
                            combined.update(block)
                destination.flush()
                os.fsync(destination.fileno())
            if temporary.stat().st_size != expected["patched_size"] or combined.hexdigest() != expected["patched_sha256"]:
                raise ValueError(f"Reassembled hash mismatch: {target.name}")
            # Publish only if absent; never replace another file, including in a race.
            os.link(temporary, target)
            print(f"Reassembled and verified: {target.name}")
        finally:
            temporary.unlink(missing_ok=True)
    print("Resource preparation complete; no game client was modified.")


if __name__ == "__main__":
    main()
