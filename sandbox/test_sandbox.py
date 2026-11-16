"""Real container integration tests. Build nexcode-sandbox:local first."""
import json
import shutil
import subprocess
import unittest


@unittest.skipUnless(shutil.which("docker"), "Docker is required")
class SandboxTests(unittest.TestCase):
    def run_code(self, code, stdin="", timeout=1000, memory=64):
        payload = {"code": code, "runs": [{"stdin": stdin, "timeout_ms": timeout}], "memory_mb": memory}
        cmd = ["docker", "run", "--rm", "--pull=never", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges:true", "--pids-limit=32", "--memory=384m", "--memory-swap=384m", "--cpus=1", "--user=65534:65534", "-i", "nexcode-sandbox:local"]
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


if __name__ == "__main__":
    unittest.main()
