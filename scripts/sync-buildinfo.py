"""Copies the code hylad runs into internal/buildinfo/hyladSource.

An embed pattern reaches only files of the package's own directory, and
the code is the module's, so package buildinfo embeds a copy of it. The
copy is the files as they are: the fingerprint of a build is of the code
it was made of.

The copy is no package of this module: go list names it ./hyladSource,
not ./..., and go vet and go test take the same list.
"""

import json
import os
import shutil
import subprocess
import sys

OUT = "internal/buildinfo"
DIR = "_hyladSource"


def main():
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    os.chdir(root)
    if not os.path.isfile("go.mod"):
        sys.exit("sync-buildinfo: not a module")

    out = subprocess.run(
        [
            "go", "list", "-e", "-deps",
            "-json=ImportPath,Dir,Module,GoFiles,CgoFiles,EmbedFiles",
            "./internal/daemon",
        ],
        check=True,
        capture_output=True,
    ).stdout.decode()

    dst_root = os.path.join(OUT, DIR)
    shutil.rmtree(dst_root, ignore_errors=True)
    os.makedirs(dst_root)

    dec = json.JSONDecoder()
    i = 0
    while i < len(out):
        while i < len(out) and out[i] in " \n\t":
            i += 1
        if i >= len(out):
            break
        p, i = dec.raw_decode(out, i)
        if not (p.get("Module") or {}).get("Main"):
            continue
        rel = os.path.relpath(p["Dir"], root)
        files = (p.get("GoFiles") or []) + (p.get("CgoFiles") or [])
        files += p.get("EmbedFiles") or []
        for f in files:
            src = os.path.join(p["Dir"], f)
            if not os.path.isfile(src):
                continue
            dst = os.path.join(dst_root, rel, f)
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            shutil.copyfile(src, dst)


if __name__ == "__main__":
    main()
