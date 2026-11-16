"""Container-only Python executor. The web API never executes Python on its host."""
import contextlib
import io
import json
import os
import subprocess
import sys
import time

MAX_OUTPUT = 65536


class VirtualFiles:
    """Text-only, in-memory files: student open() never touches the host filesystem."""
    def __init__(self, files, denied):
        self.files = dict(files)
        self.denied = set(denied)
        self.handles = []

    def open(self, filename, mode="r", buffering=-1, encoding=None, errors=None, newline=None, **kwargs):
        if not isinstance(filename, str) and hasattr(filename, "__fspath__"):
            filename = filename.__fspath__()
        if not isinstance(filename, str) or not filename or len(filename) > 100 or filename in (".", "..") or any(c not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-" for c in filename):
            raise PermissionError("Разрешены только относительные имена учебных файлов")
        if filename in self.denied:
            raise PermissionError("Нет доступа к учебному файлу")
        if mode not in ("r", "w", "a", "r+", "w+", "a+", "x", "x+"):
            raise ValueError("Учебная среда поддерживает текстовые режимы r, w, a, x и +")
        if encoding and encoding.lower().replace("-", "") not in ("utf8", "utf8sig"):
            raise ValueError("Используйте UTF-8")
        if mode.startswith("r") and filename not in self.files:
            raise FileNotFoundError(f"Файл {filename} не найден")
        if mode.startswith("x") and filename in self.files:
            raise FileExistsError(filename)
        if filename not in self.files and len(self.files) >= 16:
            raise OSError("Не более 16 учебных файлов")
        if mode[0] in "wx": self.files[filename] = ""
        if mode.startswith("a"): self.files.setdefault(filename, "")
        owner = self
        writable = mode[0] in "wax" or "+" in mode

        class Handle(io.StringIO):
            def __init__(self):
                super().__init__(owner.files.get(filename, ""), newline=newline)
                self.name, self.mode = filename, mode
                if mode.startswith("a"): self.seek(0, 2)

            def write(self, value):
                if not writable: raise io.UnsupportedOperation("not writable")
                if mode.startswith("a"): self.seek(0, 2)
                if max(len(self.getvalue()), self.tell()+len(value)) > 32768:
                    raise OSError("Учебный файл превышает 32 КБ")
                previous=self.getvalue()
                position=self.tell()
                candidate=previous[:position] + ("\0" * max(0,position-len(previous))) + value + previous[position+len(value):]
                if len(candidate.encode("utf-8")) > 32768:
                    raise OSError("Учебный файл превышает 32 КБ")
                result = super().write(value)
                self.flush()
                return result

            def writelines(self, lines):
                for line in lines: self.write(line)

            def truncate(self, size=None):
                if not writable: raise io.UnsupportedOperation("not writable")
                if size is not None and size>32768: raise OSError("Учебный файл превышает 32 КБ")
                result=super().truncate(size); self.flush(); return result

            def read(self, *args):
                if mode[0] != "r" and "+" not in mode: raise io.UnsupportedOperation("not readable")
                return super().read(*args)

            def readline(self, *args):
                if mode[0] != "r" and "+" not in mode: raise io.UnsupportedOperation("not readable")
                return super().readline(*args)

            def flush(self):
                if not self.closed and writable:
                    value=self.getvalue()
                    total=sum(len(v.encode("utf-8")) for k,v in owner.files.items() if k!=filename)+len(value.encode("utf-8"))
                    if total>65536: raise OSError("Учебные файлы превышают 64 КБ")
                    owner.files[filename]=value

            def close(self):
                self.flush(); super().close()

        handle=Handle(); self.handles.append(handle)
        return handle


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
    import builtins
    import types

    allowed = {
        "math", "random", "re", "statistics", "collections", "itertools", "functools",
        "string", "decimal", "fractions", "datetime", "json", "heapq", "bisect", "operator", "array",
    }
    memory = int(payload["memory_mb"]) * 1024 * 1024
    resource.setrlimit(resource.RLIMIT_AS, (memory, memory))
    resource.setrlimit(resource.RLIMIT_FSIZE, (0, 0))
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    code = compile(payload["code"], "main.py", "exec")
    virtual = VirtualFiles(payload.get("files", {}), payload.get("denied_files", []))
    student_builtins = dict(vars(builtins))
    student_builtins["open"] = virtual.open
    original_import = builtins.__import__

    def file_exists(name):
        if not isinstance(name,str): name=str(name)
        if "/" in name or "\\" in name or name in (".",".."):
            raise PermissionError("Разрешены только имена учебных файлов")
        return name in virtual.files

    class VirtualPath:
        def __init__(self,name): self.name=str(name)
        def __str__(self): return self.name
        def __fspath__(self): return self.name
        def exists(self): return file_exists(self.name)
        def is_file(self): return self.exists()
        def open(self,*args,**kwargs): return virtual.open(self.name,*args,**kwargs)
        def read_text(self,encoding="utf-8",errors=None):
            with self.open(encoding=encoding) as handle: return handle.read()
        def write_text(self,text,encoding="utf-8",errors=None):
            with self.open("w",encoding=encoding) as handle: return handle.write(text)

    class SafeModule(types.ModuleType):
        def __getattr__(self,name):
            if name.startswith("__"): raise AttributeError(name)
            raise PermissionError("Операция не поддерживается учебной файловой системой")

    virtual_os=SafeModule("os")
    virtual_path=SafeModule("os.path")
    virtual_path.exists=file_exists
    virtual_path.isfile=file_exists
    virtual_os.path=virtual_path
    virtual_pathlib=SafeModule("pathlib")
    virtual_pathlib.Path=VirtualPath

    def student_import(name,globals=None,locals=None,fromlist=(),level=0):
        if name=="os": return virtual_os
        if name=="os.path": return virtual_path if fromlist else virtual_os
        if name=="pathlib": return virtual_pathlib
        return original_import(name,globals,locals,fromlist,level)

    student_builtins["__import__"]=student_import

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
            exec(code, {"__name__": "__main__", "__builtins__": student_builtins})
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
    for handle in virtual.handles:
        if not handle.closed: handle.close()
    return {"stdout": stdout.getvalue(), "stderr": stderr.getvalue(), "exit_code": exit_code, "output_limit": output_limit, "files": virtual.files}


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
        child_payload = {"code": payload["code"], "stdin": run["stdin"], "memory_mb": payload["memory_mb"], "files": run.get("files",{}), "denied_files":run.get("denied_files",[])}
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
