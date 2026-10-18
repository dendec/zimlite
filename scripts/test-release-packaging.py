"""Exercise the actual ARM64 Servo packaging recipe with fixture artifacts."""
from pathlib import Path
import subprocess
import tempfile
import unittest
import zipfile


class ServoPackagingTest(unittest.TestCase):
    def test_arm64_zip_contains_application_and_servo(self):
        source = (Path(__file__).resolve().parent.parent / "Makefile").read_text()
        target = source.split("dist-arm64-servo: dist-arm64 servo-runtime-arm64\n", 1)[1].split("\n\n", 1)[0]
        with tempfile.TemporaryDirectory(prefix="zimlite-packaging-test-") as tmp:
            root = Path(tmp)
            (root / "dist/zimlite").mkdir(parents=True)
            (root / "dist/servo-runtime").mkdir()
            (root / "dist/zimlite/zimlite").write_text("app fixture")
            (root / "dist/servo-runtime/run.sh").write_text("helper fixture")
            archive = root / "dist/zimlite-linux-arm64.zip"
            with zipfile.ZipFile(archive, "w") as output:
                output.writestr("zimlite", "app fixture")
                output.writestr("obsolete", "must disappear")
            (root / "Review.mk").write_text(
                ".PHONY: dist-arm64 servo-runtime-arm64\n"
                "dist-arm64:\nservo-runtime-arm64:\n"
                "dist-arm64-servo: dist-arm64 servo-runtime-arm64\n" + target + "\n"
            )
            subprocess.run(["make", "-f", "Review.mk", "dist-arm64-servo"], cwd=root, check=True, capture_output=True)
            with zipfile.ZipFile(archive) as output:
                self.assertEqual(output.read("zimlite"), b"app fixture")
                self.assertEqual(output.read("servo/run.sh"), b"helper fixture")
                self.assertNotIn("obsolete", output.namelist())


if __name__ == "__main__":
    unittest.main()
