"""Build a local Battlesnake CLI with reproducible assignment of start slots."""

import argparse
import json
from pathlib import Path
import shutil
import subprocess
import tempfile


MODULE = "github.com/BattlesnakeOfficial/rules@v1.2.3"


def patch_slots(source):
    replacements = (
        ('\t"path"\n', '\t"path"\n\t"sort"\n'),
        (
            "\tboardState, err := maps.SetupBoard",
            "\tsort.Strings(snakeIds)\n\tboardState, err := maps.SetupBoard",
        ),
        ('\t\t\tid = uuid.New().String()', '\t\t\tid = fmt.Sprintf("slot-%02d", i+1)'),
    )
    code = source.read_text()
    for before, after in replacements:
        if code.count(before) != 1:
            raise RuntimeError("Unexpected upstream CLI source. Check the pinned version.")
        code = code.replace(before, after)
    source.chmod(0o644)
    source.write_text(code)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    output = args.output.resolve()
    if output.exists():
        parser.error(f"{output} already exists")
    details = json.loads(subprocess.check_output(["go", "mod", "download", "-json", MODULE]))
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="battlesnake-cli-") as scratch:
        checkout = Path(scratch) / "rules"
        shutil.copytree(details["Dir"], checkout)
        patch_slots(checkout / "cli/commands/play.go")
        subprocess.run(
            ["go", "build", "-o", str(output), "./cli/battlesnake"],
            cwd=checkout,
            check=True,
        )


if __name__ == "__main__":
    main()
