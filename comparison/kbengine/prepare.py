"""Create runtime assets from the pinned upstream template, then apply overrides."""
from pathlib import Path
import shutil
import sys

here = Path(__file__).resolve().parent
source = Path(sys.argv[1]).resolve()
target = here.parent / "runtime/kbengine-assets"
if target.exists():
    raise SystemExit("runtime assets already exist; refusing to overwrite")
shutil.copytree(str(source / "kbe/res/sdk_templates/server/python_assets"), str(target))
for path in (here / "overrides").rglob("*"):
    if path.is_file():
        dest = target / path.relative_to(here / "overrides")
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(str(path), str(dest))
print(target)
