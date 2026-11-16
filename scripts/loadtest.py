"""Read/write workload for existing test students; optional real sandbox checks.

Identities are read from an external JSON file, not command arguments.
Never point this script at student accounts: it updates exercise 1 drafts.
"""
import argparse
import concurrent.futures
import http.cookiejar
import json
import statistics
import time
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://localhost:3000")
    parser.add_argument("--accounts", required=True, help='JSON: [{"full_name":"Test Student","group_name":"TEST","code":"optional"}, ...]')
    parser.add_argument("--login-delay", type=float, default=6.1, help="Seconds between logins; respects 10 attempts/min/IP")
    parser.add_argument("--users", type=int, default=30)
    parser.add_argument("--rounds", type=int, default=5)
    parser.add_argument("--exercise", type=int, default=1)
    parser.add_argument("--execute", action="store_true", help="Also submit one real run per user")
    args = parser.parse_args()
    with open(args.accounts, encoding="utf-8") as source:
        accounts = json.load(source)[:args.users]
    if len(accounts) < args.users:
        raise SystemExit("Not enough existing test accounts")
    clients = []
    for index, account in enumerate(accounts):
        if index:
            time.sleep(args.login_delay)
        opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        request(opener, args.url + "/api/auth/login", account)
        clients.append(opener)

    def workload(opener):
        latencies = []
        for _ in range(args.rounds):
            start = time.monotonic()
            request(opener, args.url + "/api/catalog")
            latencies.append((time.monotonic() - start) * 1000)
            request(opener, f"{args.url}/api/exercises/{args.exercise}/draft", {"code": "print(42)\n"}, "PUT")
        if args.execute:
            start = time.monotonic()
            job = request(opener, f"{args.url}/api/exercises/{args.exercise}/submit", {"code": "print(42)", "mode": "run", "stdin": ""})
            deadline = time.monotonic() + 180
            while time.monotonic() < deadline:
                result = request(opener, f"{args.url}/api/submissions/{job['id']}")
                if result["status"] not in ("queued", "running"):
                    if result["status"] != "executed":
                        raise RuntimeError(f"Execution failed: {result['report']}")
                    break
                time.sleep(0.5)
            else:
                raise TimeoutError("Queue did not finish in 180 seconds")
            print(f"Execution including queue: {(time.monotonic() - start):.2f} s")
        return latencies

    started = time.monotonic()
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.users) as pool:
        samples = [v for group in pool.map(workload, clients) for v in group]
    samples.sort()
    print(json.dumps({"users": args.users, "requests": len(samples), "catalog_p50_ms": round(statistics.median(samples), 2), "catalog_p95_ms": round(samples[int((len(samples) - 1) * 0.95)], 2), "elapsed_s": round(time.monotonic() - started, 2)}, indent=2))


def request(opener, url, body=None, method=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers={"Content-Type": "application/json"})
    with opener.open(req, timeout=30) as response:
        return json.load(response)


if __name__ == "__main__":
    main()
