# Copyright 2026 The casbin Authors. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import io
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest


INSTALLER = Path(__file__).with_name("install.sh").resolve()
SERVER = "casbin-gateway"
DESKTOP = "casbin-gateway-desktop"


@unittest.skipUnless(sys.platform == "linux", "requires the Linux dynamic loader")
class InstallTest(unittest.TestCase):
    def setUp(self):
        fixture = tempfile.TemporaryDirectory(prefix="gateway-install-")
        self.addCleanup(fixture.cleanup)
        self.root = Path(fixture.name)
        self.shims = self.root / "shims"
        self.install_dir = self.root / "gateway data"
        self.bin_dir = self.root / "bin"
        self.events = self.root / "events"
        self.shims.mkdir()
        fixture_home = self.root / "home"
        fixture_home.mkdir()

        self.env = {
            "HOME": str(fixture_home),
            "PATH": os.pathsep.join([str(self.shims), os.environ["PATH"]]),
            "SHELL": "/bin/false",
            "INSTALL_DIR": str(self.install_dir),
            "BIN_DIR": str(self.bin_dir),
            "NO_START": "1",
            "NO_AUTOSTART": "1",
            "NO_SHORTCUT": "",
            "SERVER_ONLY": "",
            "INSTALL_TEST_OS": "Linux",
            "INSTALL_TEST_EVENTS": str(self.events),
        }
        self.command("curl", """#!/bin/sh
printf 'download\n' >> "$INSTALL_TEST_EVENTS"
while [ "$#" -gt 1 ]; do
    if [ "$1" = "-o" ]; then
        exec cp "$INSTALL_TEST_ARCHIVE" "$2"
    fi
    shift
done
exit 98
""")
        self.command("uname", """#!/bin/sh
case "$1" in
    -s) printf '%s\n' "$INSTALL_TEST_OS" ;;
    -m) printf 'x86_64\n' ;;
    *) exit 98 ;;
esac
""")
        for action in ("shortcut", "autostart"):
            self.command(action, f"""#!/bin/sh
printf '{action} %s\n' "$1" >> "$INSTALL_TEST_EVENTS"
""")
        self.command("quit", """#!/bin/sh
printf 'quit\n' >> "$INSTALL_TEST_EVENTS"
exit 2
""")
        self.launcher = Path("/usr/bin/env").read_bytes()

    def command(self, name, contents):
        command = self.shims / name
        command.write_text(contents)
        command.chmod(0o755)

    def archive(self, *, launcher=None, omit=()):
        files = {
            SERVER: b'#!/bin/sh\nprintf "%s: %s\\n" "$PWD" "$1"\n',
            DESKTOP: self.launcher if launcher is None else launcher,
            "LICENSE": b"fixture license\n",
            "NOTICE": b"fixture notice\n",
            "DISCLAIMER": b"fixture disclaimer\n",
        }
        archive = self.root / "nightly.tar.gz"
        with tarfile.open(archive, "w:gz") as package:
            for name, contents in files.items():
                if name not in omit:
                    entry = tarfile.TarInfo(f"nightly/{name}")
                    entry.size = len(contents)
                    package.addfile(entry, io.BytesIO(contents))
        return archive

    def run_installer(self, archive, **options):
        return subprocess.run(
            ["bash", str(INSTALLER)], cwd=self.root,
            env={**self.env, "INSTALL_TEST_ARCHIVE": str(archive), **options},
            text=True, capture_output=True, timeout=20,
        )

    def existing_installation(self, *, desktop=True):
        self.install_dir.mkdir()
        self.bin_dir.mkdir()
        files = {
            self.install_dir / SERVER: "existing server",
            self.install_dir / "database-marker": "existing data",
            self.bin_dir / SERVER: "existing CLI wrapper",
        }
        if desktop:
            files[self.install_dir / DESKTOP] = "existing launcher"
        for path, contents in files.items():
            path.write_text(contents)
        return files

    def assert_events(self, *events):
        actual = self.events.read_text().splitlines() if self.events.exists() else []
        self.assertEqual(actual, list(events))

    def assert_failure(self, result, diagnostic, original_files=()):
        self.assertEqual(result.returncode, 1, result)
        self.assertIn(diagnostic, result.stderr)
        self.assertNotIn("is installed", result.stdout)
        self.assertFalse(list(self.install_dir.glob(".install.*")))
        for path, contents in dict(original_files).items():
            self.assertEqual(path.read_text(), contents)

    def test_missing_shared_library_preserves_an_existing_installation(self):
        original = self.existing_installation()
        self.assertIn(b"libc.so.6\0", self.launcher)
        unusable = self.launcher.replace(b"libc.so.6\0", b"libx.so.9\0")
        result = self.run_installer(self.archive(launcher=unusable))
        self.assert_failure(result, "libx.so.9 => not found", original)
        self.assert_events("download")

    def test_missing_webkit_prints_package_names_even_when_ldd_exits_zero(self):
        self.command("ldd", """#!/bin/sh
printf 'libwebkit2gtk-4.1.so.0 => not found\n'
""")
        result = self.run_installer(self.archive())
        self.assert_failure(result, "webkit2gtk-4.1 on Arch Linux")
        self.assertIn("libwebkit2gtk-4.1-0 on Debian or Ubuntu", result.stderr)
        self.assertIn("SERVER_ONLY=1", result.stderr)
        self.assertFalse((self.install_dir / SERVER).exists())

    def test_incomplete_archive_preserves_an_existing_installation(self):
        original = self.existing_installation()
        result = self.run_installer(self.archive(omit=("NOTICE",)))
        self.assert_failure(result, "the downloaded archive is missing NOTICE", original)
        self.assert_events("download")

    def test_healthy_linux_install_creates_shortcut_and_autostart(self):
        result = self.run_installer(self.archive(), NO_AUTOSTART="")
        self.assertEqual(result.returncode, 0, result)
        self.assertIn("Casbin Gateway is installed", result.stdout)
        self.assertTrue(os.access(self.install_dir / DESKTOP, os.X_OK))
        self.assertTrue(os.access(self.bin_dir / SERVER, os.X_OK))
        self.assert_events("download", "shortcut on", "autostart on")

    def test_darwin_bypasses_ldd_and_honors_no_shortcut(self):
        self.command("ldd", "#!/bin/sh\nexit 97\n")
        result = self.run_installer(self.archive(), INSTALL_TEST_OS="Darwin", NO_SHORTCUT="1")
        self.assertEqual(result.returncode, 0, result)
        self.assert_events("download", "shortcut off")

    def test_server_only_install_and_wrapper(self):
        self.command("ldd", "#!/bin/sh\nexit 97\n")
        result = self.run_installer(
            self.archive(omit=(DESKTOP,)), SERVER_ONLY="1", NO_START="", NO_AUTOSTART="",
        )
        self.assertEqual(result.returncode, 0, result)
        self.assertIn("Casbin Gateway server is installed", result.stdout)
        self.assertFalse((self.install_dir / DESKTOP).exists())
        self.assert_events("download")
        wrapper = subprocess.run(
            [self.bin_dir / SERVER, "status"], cwd=self.root, env=self.env,
            text=True, capture_output=True, check=True, timeout=20,
        )
        self.assertEqual(wrapper.stdout.strip(), f"{self.install_dir}: status")

    def test_server_only_update_preserves_data(self):
        original = self.existing_installation(desktop=False)
        marker = self.install_dir / "database-marker"
        result = self.run_installer(self.archive(omit=(DESKTOP,)), SERVER_ONLY="1")
        self.assertEqual(result.returncode, 0, result)
        self.assertEqual(marker.read_text(), original[marker])
        self.assertTrue(os.access(self.install_dir / SERVER, os.X_OK))
        self.assert_events("download")

    def test_server_only_refuses_to_replace_a_desktop_installation(self):
        original = self.existing_installation()
        result = self.run_installer(self.archive(omit=(DESKTOP,)), SERVER_ONLY="1")
        self.assert_failure(result, "SERVER_ONLY needs an install directory without a desktop launcher", original)
        self.assert_events()


if __name__ == "__main__":
    unittest.main()
