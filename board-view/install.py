"""Pack board-view into a .vsix and install it with the VSCodium server's own CLI.

Local only, never published. Run: python3 -I <kanbanzai>/board-view/install.py
The .vsix records this checkout's bin/kanbanzai (bin.json), so the board runs the same core as the plugins; the
`kanbanzai.command` setting overrides it.
  --vsix-only PATH   write the .vsix there and do not install it
"""

import argparse
import glob
import json
import os
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path
from xml.sax.saxutils import escape, quoteattr

HERE = Path(__file__).resolve().parent
FILES = ["package.json", "extension.js", "render.js", "README.md"]

CONTENT_TYPES = (
    '<?xml version="1.0" encoding="utf-8"?>\n'
    '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
    '<Default Extension=".json" ContentType="application/json"/>'
    '<Default Extension=".js" ContentType="application/javascript"/>'
    '<Default Extension=".md" ContentType="text/markdown"/>'
    '<Default Extension=".vsixmanifest" ContentType="text/xml"/>'
    '<Default Extension=".xml" ContentType="text/xml"/>'
    "</Types>"
)


def manifest_xml(pkg):
    engine = pkg.get("engines", {}).get("vscode", "*")
    kinds = ",".join(pkg.get("extensionKind", ["workspace"]))
    return f"""<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011" xmlns:d="http://schemas.microsoft.com/developer/vsx-schema-design/2011">
  <Metadata>
    <Identity Language="en-US" Id={quoteattr(pkg["name"])} Version={quoteattr(pkg["version"])} Publisher={quoteattr(pkg["publisher"])} />
    <DisplayName>{escape(pkg.get("displayName", pkg["name"]))}</DisplayName>
    <Description xml:space="preserve">{escape(pkg.get("description", ""))}</Description>
    <Categories>{escape(",".join(pkg.get("categories", ["Other"])))}</Categories>
    <GalleryFlags>Public</GalleryFlags>
    <Properties>
      <Property Id="Microsoft.VisualStudio.Code.Engine" Value={quoteattr(engine)} />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionDependencies" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionPack" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionKind" Value={quoteattr(kinds)} />
      <Property Id="Microsoft.VisualStudio.Code.LocalizedLanguages" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.EnabledApiProposals" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExecutesCode" Value="true" />
    </Properties>
  </Metadata>
  <Installation>
    <InstallationTarget Id="Microsoft.VisualStudio.Code"/>
  </Installation>
  <Dependencies/>
  <Assets>
    <Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true" />
    <Asset Type="Microsoft.VisualStudio.Services.Content.Details" Path="extension/README.md" Addressable="true" />
  </Assets>
</PackageManifest>
"""


def build_vsix(dest):
    pkg = json.loads((HERE / "package.json").read_text(encoding="utf-8"))
    dest = Path(dest)
    dest.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(dest, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("extension.vsixmanifest", manifest_xml(pkg))
        z.writestr("[Content_Types].xml", CONTENT_TYPES)
        for name in FILES:
            z.write(HERE / name, f"extension/{name}")
        z.writestr("extension/bin.json", json.dumps({"command": str(HERE.parent / "bin" / "kanbanzai")}) + "\n")
    return pkg


def newest(paths):
    paths = [p for p in paths if os.path.exists(p)]
    return max(paths, key=os.path.getmtime) if paths else None


def find_codium():
    home = os.path.expanduser("~")
    return newest(glob.glob(f"{home}/.vscodium-server/bin/*/bin/remote-cli/codium"))


def find_ipc_socket():
    return newest(glob.glob("/run/user/*/vscode-ipc-*.sock") + glob.glob("/tmp/vscode-ipc-*.sock"))


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--vsix-only", metavar="PATH", help="write the .vsix here and do not install it")
    args = ap.parse_args(argv)

    if args.vsix_only:
        pkg = build_vsix(args.vsix_only)
        print(f"wrote {args.vsix_only} ({pkg['publisher']}.{pkg['name']} {pkg['version']}); not installed")
        return 0

    codium = find_codium()
    if not codium:
        print("error: no ~/.vscodium-server/bin/*/bin/remote-cli/codium found; open this folder in VSCodium first",
              file=sys.stderr)
        return 1

    env = dict(os.environ)
    if not env.get("VSCODE_IPC_HOOK_CLI"):
        sock = find_ipc_socket()
        if not sock:
            print("error: VSCODE_IPC_HOOK_CLI is not set and no vscode-ipc-*.sock was found in /run/user or /tmp; "
                  "run this from a VSCodium terminal", file=sys.stderr)
            return 1
        env["VSCODE_IPC_HOOK_CLI"] = sock
        print(f"VSCODE_IPC_HOOK_CLI was not set; using {sock}")

    with tempfile.TemporaryDirectory(prefix="board-view-") as tmp:
        pkg = json.loads((HERE / "package.json").read_text(encoding="utf-8"))
        vsix = os.path.join(tmp, f"{pkg['name']}-{pkg['version']}.vsix")
        build_vsix(vsix)
        print(f"built {vsix}")
        print(f"running {codium} --install-extension --force")
        result = subprocess.run([codium, "--install-extension", vsix, "--force"], env=env, check=False)
    if result.returncode != 0:
        print(f"error: codium exited with {result.returncode}", file=sys.stderr)
        return result.returncode
    print(f"installed {pkg['publisher']}.{pkg['name']} {pkg['version']}; run Developer: Reload Window")
    return 0


if __name__ == "__main__":
    sys.exit(main())
