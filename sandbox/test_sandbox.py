"""Real container integration tests. Build nexcode-sandbox:local first."""
import json
import os
import shutil
import subprocess
import unittest


@unittest.skipUnless(shutil.which("docker"), "Docker is required")
class SandboxTests(unittest.TestCase):
    def run_code(self, code, stdin="", timeout=1000, memory=64, files=None, denied_files=None):
        payload = {"code": code, "runs": [{"stdin": stdin, "timeout_ms": timeout, "files":files or {}, "denied_files":denied_files or []}], "memory_mb": memory}
        image=os.environ.get("NEXCODE_SANDBOX_IMAGE","nexcode-sandbox:local")
        cmd = ["docker", "run", "--rm", "--pull=never", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges:true", "--pids-limit=32", "--memory=384m", "--memory-swap=384m", "--cpus=1", "--user=65534:65534", "-i", image]
        result = subprocess.run(cmd, input=json.dumps(payload), text=True, encoding="utf-8", capture_output=True, timeout=15, check=True)
        return json.loads(result.stdout)

    def test_unicode_and_lint(self):
        out = self.run_code('import math\nname = input()\nprint(f"Привет, {name}!")', "Анна\n")
        self.assertEqual(out["results"][0]["stdout"], "Привет, Анна!\n")
        self.assertTrue(any(d["code"] == "F401" for d in out["lint"]))

    def test_timeout(self):
        self.assertTrue(self.run_code("while True: pass", timeout=200)["results"][0]["timed_out"])

    def test_files_network_and_processes(self):
        for code in ['print(open("/etc/passwd").read())', 'import socket\nsocket.socket()', 'import os\nos.system("id")']:
            with self.subTest(code=code):
                result = self.run_code(code)["results"][0]
                self.assertNotEqual(result["exit_code"], 0)
                self.assertIn("PermissionError", result["stderr"])

    def test_output_and_memory_limits(self):
        self.assertTrue(self.run_code('print("x" * 70000)')["results"][0]["output_limit"])
        self.assertNotEqual(self.run_code('x = bytearray(200 * 1024 * 1024)')["results"][0]["exit_code"], 0)

    def test_syntax_error(self):
        result = self.run_code('if :')["results"][0]
        self.assertNotEqual(result["exit_code"], 0)
        self.assertIn("SyntaxError", result["stderr"])

    def test_virtual_read_write_append_and_path_checks(self):
        code = ('from pathlib import Path\nimport os.path\n'
                'print(Path("input.txt").exists(), os.path.exists("missing.txt"))\n'
                'with open("report.txt", "w", encoding="utf-8") as out:\n'
                '    out.write(Path("input.txt").read_text())\n'
                'with open("report.txt", "a", encoding="utf-8") as out:\n'
                '    out.write("Готово\\n")\n')
        result=self.run_code(code,files={"input.txt":"Отчёт\n"})["results"][0]
        self.assertEqual(result["exit_code"],0,result["stderr"])
        self.assertEqual(result["stdout"],"True False\n")
        self.assertEqual(result["files"]["report.txt"],"Отчёт\nГотово\n")

    def test_virtual_files_missing_denied_and_host_escape(self):
        for code, files, denied, expected in [
            ('open("missing.txt").read()',{},[],"FileNotFoundError"),
            ('open("protected.txt").read()',{"protected.txt":"private"},["protected.txt"],"PermissionError"),
            ('open("../escape.txt", "w")',{},[],"PermissionError"),
            ('from pathlib import Path\nPath("/etc/passwd").read_text()',{},[],"PermissionError"),
            ('import os\nos.system("id")',{},[],"PermissionError"),
        ]:
            with self.subTest(code=code):
                result=self.run_code(code,files=files,denied_files=denied)["results"][0]
                self.assertNotEqual(result["exit_code"],0)
                self.assertIn(expected,result["stderr"])

    def test_virtual_files_reset_between_tests(self):
        first=self.run_code('open("saved.txt", "w").write("one")')["results"][0]
        self.assertEqual(first["files"]["saved.txt"],"one")
        second=self.run_code('open("saved.txt").read()')["results"][0]
        self.assertIn("FileNotFoundError",second["stderr"])


if __name__ == "__main__":
    unittest.main()
