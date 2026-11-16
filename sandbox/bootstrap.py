"""Container-only Python executor. The web API never executes Python on its host."""
import contextlib
import io
import json
import os
import subprocess
import sys
import time

MAX_OUTPUT = 65536


class OutputLimit(Exception):
    pass


class BoundedText(io.StringIO):
    def write(self, text):
        if self.tell() + len(text) > MAX_OUTPUT:
            raise OutputLimit("Превышен лимит вывода (64 КБ)")
        return super().write(text)


def child(payload):
    # Load the supported educational standard library before installing the audit hook.
    import math, random, re, statistics, collections, itertools, functools, string
    import decimal, fractions, datetime, heapq, bisect, operator, array
    import resource

    allowed = {
        "math", "random", "re", "statistics", "collections", "itertools", "functools",
        "string", "decimal", "fractions", "datetime", "json", "heapq", "bisect", "operator", "array",
    }
    memory = int(payload["memory_mb"]) * 1024 * 1024
    resource.setrlimit(resource.RLIMIT_AS, (memory, memory))
    resource.setrlimit(resource.RLIMIT_FSIZE, (0, 0))
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    code = compile(payload["code"], "main.py", "exec")

    def audit(event, args):
        if event == "import" and args[0].split(".")[0] not in allowed:
            raise PermissionError("Этот модуль недоступен в учебной песочнице")
        if event == "open" or event.startswith(("os.", "socket.", "subprocess.", "ctypes.", "resource.", "shutil.", "pty.", "mmap.")):
            raise PermissionError("Доступ к файлам, сети и системным процессам запрещён")
        if event in {"sys.settrace", "sys.setprofile", "sys.addaudithook", "sys._getframe", "builtins.breakpoint"}:
            raise PermissionError("Операция недоступна в песочнице")

    stdout, stderr = BoundedText(), BoundedText()
    sys.stdin = io.StringIO(payload["stdin"])
    exit_code, output_limit = 0, False
    # The hook belongs to this child process only and cannot be removed by student code.
    sys.addaudithook(audit)
    with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        try:
            exec(code, {"__name__": "__main__", "__builtins__": __builtins__})
        except OutputLimit:
            output_limit, exit_code = True, 1
        except SystemExit as exc:
            exit_code = exc.code if isinstance(exc.code, int) else (0 if exc.code is None else 1)
        except BaseException as exc:
            exit_code = 1
            # No traceback file reads after the audit hook.
            try:
                stderr.write(f"{type(exc).__name__}: {exc}\n")
            except OutputLimit:
                output_limit = True
    return {"stdout": stdout.getvalue(), "stderr": stderr.getvalue(), "exit_code": exit_code, "output_limit": output_limit}


def execute(payload):
    lint = subprocess.run(
        ["ruff", "check", "--isolated", "--select", "E,F,W", "--output-format", "json", "--no-cache", "--stdin-filename", "main.py", "-"],
        input=payload["code"], text=True, capture_output=True, timeout=5,
    )
    if lint.returncode not in (0, 1):
        raise RuntimeError("Ruff analysis failed")
    diagnostics = json.loads(lint.stdout)
    results = []
    for run in payload["runs"]:
        start = time.monotonic()
        result = {"stdout": "", "stderr": "", "exit_code": 1, "timed_out": False, "output_limit": False}
        child_payload = {"code": payload["code"], "stdin": run["stdin"], "memory_mb": payload["memory_mb"]}
        try:
            process = subprocess.run(
                [sys.executable, "-I", "-B", __file__, "--child"],
                input=json.dumps(child_payload), text=True, capture_output=True,
                timeout=run["timeout_ms"] / 1000,
                env={"PATH": os.environ.get("PATH", ""), "PYTHONIOENCODING": "utf-8"},
            )
            if process.returncode == 0:
                result.update(json.loads(process.stdout))
            else:
                result["stderr"] = process.stderr[-4096:] or "Процесс завершён: превышены ресурсы или произошла ошибка"
        except subprocess.TimeoutExpired:
            result.update(timed_out=True, stderr="Превышено время исполнения")
        result["duration_ms"] = round((time.monotonic() - start) * 1000, 2)
        results.append(result)
    return {"results": results, "lint": diagnostics}


if __name__ == "__main__":
    data = json.load(sys.stdin)
    try:
        response = child(data) if "--child" in sys.argv else execute(data)
        sys.stdout.write(json.dumps(response, ensure_ascii=False))
    except BaseException as exc:
        # Compilation errors are regular execution failures, not broken runner responses.
        if "--child" in sys.argv:
            sys.stdout.write(json.dumps({"stdout": "", "stderr": f"{type(exc).__name__}: {exc}", "exit_code": 1, "output_limit": False}))
        else:
            raise
