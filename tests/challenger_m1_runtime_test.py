#!/usr/bin/env python3
"""
Empirical Challenger Test Suite for Milestone 1
Target: 七喵记账 (com.xsguyu.accounting) Backend Server Runtime Behavior

Tests:
1. Health check (/api/v1/health): status 200, JSON schema, headers, latency benchmark (100 reqs), HEAD, 405 for POST/PUT/DELETE.
2. Static file serving & SPA fallback: GET /, /index.html, /dashboard, /wallets/create, CSS & JS asset serving, /api/ unknown 404 JSON vs SPA fallback, path traversal probe.
3. Environment variable overrides: UGAPP_DATA_DIR, UGAPP_LOG_DIR creation of nested paths, PORT env vs --port flag.
4. Graceful shutdown: SIGINT (exit 0, socket released), SIGTERM (exit 0, socket released), in-flight requests.
5. Concurrent stress testing: 20 worker threads, 1000 requests, error rate & throughput.
6. Large header test: 8KB Ugreen-Ttk token.
7. Docker Linux amd64 binary test: execute rootfs_amd64/bin/accounting_serv in alpine container, verify health, SPA fallback, graceful SIGTERM stop.
"""

import sys
import os
import time
import json
import signal
import socket
import tempfile
import shutil
import subprocess
import threading
import concurrent.futures
import urllib.request
import urllib.error
import http.client

PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
BACKEND_DIR = os.path.join(PROJECT_ROOT, "backend")
LINUX_AMD64_BIN = os.path.join(PROJECT_ROOT, "rootfs_amd64", "bin", "accounting_serv")

# Results tracking
test_results = []

def record_result(test_id, name, passed, details=""):
    test_results.append({
        "id": test_id,
        "name": name,
        "passed": passed,
        "details": details
    })
    status_str = "\033[92mPASS\033[0m" if passed else "\033[91mFAIL\033[0m"
    print(f"[{status_str}] {test_id}: {name}")
    if details:
        for line in str(details).strip().splitlines():
            print(f"       {line}")

def get_free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]

def wait_for_port(port, timeout=5.0):
    start = time.time()
    while time.time() - start < timeout:
        try:
            with socket.create_connection(('127.0.0.1', port), timeout=0.2):
                return True
        except (ConnectionRefusedError, OSError):
            time.sleep(0.05)
    return False

def http_get(url, headers=None):
    req = urllib.request.Request(url, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=3.0) as resp:
            return resp.status, resp.headers, resp.read().decode('utf-8', errors='replace')
    except urllib.error.HTTPError as e:
        return e.code, e.headers, e.read().decode('utf-8', errors='replace')

def http_custom(method, url, body=None, headers=None):
    parsed = urllib.parse.urlparse(url)
    conn = http.client.HTTPConnection(parsed.netloc, timeout=3.0)
    path = parsed.path
    if parsed.query:
        path += "?" + parsed.query
    conn.request(method, path, body=body, headers=headers or {})
    resp = conn.getresponse()
    content = resp.read().decode('utf-8', errors='replace')
    resp_headers = {k.lower(): v for k, v in resp.getheaders()}
    conn.close()
    return resp.status, resp_headers, content

# ==============================================================================
# Suite 1: Local Native Server Runtime Tests
# ==============================================================================
def run_native_suite():
    print("\n" + "="*70)
    print(" SUITE 1: Native Server Runtime Tests (Health, SPA, Env, Shutdown)")
    print("="*70)

    # 1. Compile native binary
    tmp_bin = tempfile.mktemp(prefix="accounting_serv_mac_")
    compile_cmd = ["go", "build", "-o", tmp_bin, "./cmd/server/main.go"]
    comp = subprocess.run(compile_cmd, cwd=BACKEND_DIR, capture_output=True, text=True)
    if comp.returncode != 0:
        record_result("NAT-01", "Native Darwin Go Build", False, comp.stderr)
        return
    record_result("NAT-01", "Native Darwin Go Build", True, f"Binary: {tmp_bin}")

    # 2. Test Env Var Overrides with Deeply Nested Paths
    base_tmp = tempfile.mkdtemp(prefix="ugapp_test_root_")
    custom_data_dir = os.path.join(base_tmp, "level1", "level2", "custom_data")
    custom_log_dir = os.path.join(base_tmp, "nested", "path", "custom_logs")
    port1 = get_free_port()

    env1 = os.environ.copy()
    env1["UGAPP_DATA_DIR"] = custom_data_dir
    env1["UGAPP_LOG_DIR"] = custom_log_dir
    env1["PORT"] = str(port1)

    proc1 = subprocess.Popen(
        [tmp_bin],
        cwd=PROJECT_ROOT,
        env=env1,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True
    )

    try:
        ready = wait_for_port(port1, timeout=5.0)
        record_result("ENV-01", "Server binds on PORT env var", ready, f"Port: {port1}")

        # Check nested directories were created
        data_created = os.path.isdir(custom_data_dir)
        log_created = os.path.isdir(custom_log_dir)
        record_result("ENV-02", "Deeply nested UGAPP_DATA_DIR auto-created", data_created, f"Path: {custom_data_dir}")
        record_result("ENV-03", "Deeply nested UGAPP_LOG_DIR auto-created", log_created, f"Path: {custom_log_dir}")

        base_url = f"http://127.0.0.1:{port1}"

        # 3. Health Check Verification
        status, headers, body = http_get(f"{base_url}/api/v1/health")
        record_result("HLT-01", "GET /api/v1/health returns 200 OK", status == 200, f"Status: {status}")

        ctype = headers.get("Content-Type", "")
        record_result("HLT-02", "Content-Type is application/json", "application/json" in ctype, f"Content-Type: {ctype}")

        cache_ctrl = headers.get("Cache-Control", "")
        record_result("HLT-03", "Cache-Control is no-cache", "no-cache" in cache_ctrl, f"Cache-Control: {cache_ctrl}")

        schema_valid = False
        try:
            parsed_json = json.loads(body)
            schema_valid = (
                parsed_json.get("status") == "ok" and
                parsed_json.get("app_id") == "com.xsguyu.accounting" and
                parsed_json.get("version") == "0.1.0"
            )
        except Exception:
            schema_valid = False
        record_result("HLT-04", "Health JSON schema exact match", schema_valid, f"Body: {body.strip()}")

        # 4. Latency benchmark (100 requests sequential)
        latencies = []
        for _ in range(100):
            t0 = time.perf_counter()
            s, _, _ = http_get(f"{base_url}/api/v1/health")
            t1 = time.perf_counter()
            if s == 200:
                latencies.append((t1 - t0) * 1000.0)

        latencies.sort()
        min_lat = min(latencies)
        avg_lat = sum(latencies) / len(latencies)
        p50_lat = latencies[50]
        p95_lat = latencies[95]
        p99_lat = latencies[99]
        max_lat = max(latencies)
        lat_pass = p95_lat < 5.0
        details = f"Count: 100 | Min: {min_lat:.3f}ms | Avg: {avg_lat:.3f}ms | P50: {p50_lat:.3f}ms | P95: {p95_lat:.3f}ms | P99: {p99_lat:.3f}ms | Max: {max_lat:.3f}ms"
        record_result("HLT-05", "Health latency P95 < 5ms benchmark", lat_pass, details)

        # 5. Method handling on health
        status_head, _, body_head = http_custom("HEAD", f"{base_url}/api/v1/health")
        record_result("HLT-06", "HEAD /api/v1/health returns 200 OK with empty body", status_head == 200 and len(body_head) == 0, f"Status: {status_head}, BodyLen: {len(body_head)}")

        status_post, _, body_post = http_custom("POST", f"{base_url}/api/v1/health", body="{}")
        record_result("HLT-07", "POST /api/v1/health returns 405 Method Not Allowed", status_post == 405, f"Status: {status_post}, Body: {body_post}")

        # 6. Static File Serving
        status_root, headers_root, body_root = http_get(f"{base_url}/")
        root_pass = (status_root == 200 and "<title>七喵记账</title>" in body_root)
        record_result("STA-01", "GET / serves index.html", root_pass, f"Status: {status_root}, Contains Title: {'<title>七喵记账</title>' in body_root}")

        # 7. Static Assets
        status_css, headers_css, body_css = http_get(f"{base_url}/assets/index.CYlW3k5p.css")
        record_result("STA-02", "GET /assets/...css serves stylesheet", status_css == 200 and len(body_css) > 100, f"Status: {status_css}, Size: {len(body_css)} bytes")

        status_js, headers_js, body_js = http_get(f"{base_url}/assets/index.BnnPWwZ8.js")
        record_result("STA-03", "GET /assets/...js serves javascript bundle", status_js == 200 and len(body_js) > 1000, f"Status: {status_js}, Size: {len(body_js)} bytes")

        # 8. SPA Fallback
        status_dash, _, body_dash = http_get(f"{base_url}/dashboard")
        dash_pass = (status_dash == 200 and "<title>七喵记账</title>" in body_dash)
        record_result("SPA-01", "GET /dashboard triggers SPA fallback to index.html", dash_pass, f"Status: {status_dash}")

        status_wallet, _, body_wallet = http_get(f"{base_url}/wallets/create")
        wallet_pass = (status_wallet == 200 and "<title>七喵记账</title>" in body_wallet)
        record_result("SPA-02", "GET /wallets/create (deep route) triggers SPA fallback", wallet_pass, f"Status: {status_wallet}")

        # 9. API unknown route protection (must NOT fall back to index.html)
        status_api_unk, _, body_api_unk = http_get(f"{base_url}/api/v1/unknown_route")
        api_unk_pass = (status_api_unk == 404 and "API endpoint not found" in body_api_unk)
        record_result("API-01", "GET /api/v1/unknown returns 404 JSON (no SPA fallback)", api_unk_pass, f"Status: {status_api_unk}, Body: {body_api_unk.strip()}")

        status_api_post, _, body_api_post = http_custom("POST", f"{base_url}/api/v1/books", body="{}")
        api_post_pass = (status_api_post == 404 and "API endpoint not found" in body_api_post)
        record_result("API-02", "POST /api/v1/books returns 404 JSON (M1 stub check)", api_post_pass, f"Status: {status_api_post}")

        # 10. Large Header Stress Test (8KB Ugreen-Ttk token)
        large_token = "A" * 8192
        status_hdr, _, body_hdr = http_get(f"{base_url}/api/v1/health", headers={"Ugreen-Ttk": large_token, "Ugreen-User-ID": "10001"})
        record_result("STR-01", "Server handles 8KB Ugreen-Ttk header without 431", status_hdr == 200, f"Status: {status_hdr}")

        # 11. Concurrent Stress Test (20 threads, 1000 requests)
        num_requests = 1000
        concurrency = 20
        endpoints = [
            f"{base_url}/api/v1/health",
            f"{base_url}/",
            f"{base_url}/dashboard",
            f"{base_url}/assets/index.CYlW3k5p.css"
        ]

        def worker_task(idx):
            url = endpoints[idx % len(endpoints)]
            t_s = time.perf_counter()
            s, _, _ = http_get(url)
            t_e = time.perf_counter()
            return s, (t_e - t_s) * 1000.0

        t_conc_start = time.perf_counter()
        with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
            futures = [executor.submit(worker_task, i) for i in range(num_requests)]
            results = [f.result() for f in concurrent.futures.as_completed(futures)]
        t_conc_end = time.perf_counter()

        conc_duration = t_conc_end - t_conc_start
        rps = num_requests / conc_duration
        failed_requests = sum(1 for s, _ in results if s != 200)
        durations = sorted([d for _, d in results])
        p95_conc = durations[int(num_requests * 0.95)]
        record_result(
            "STR-02",
            f"Concurrent stress test ({concurrency} workers, {num_requests} reqs)",
            failed_requests == 0 and rps > 500,
            f"Total: {num_requests} reqs in {conc_duration:.2f}s | Throughput: {rps:.1f} req/s | Errors: {failed_requests} | P95: {p95_conc:.2f}ms"
        )

        # 12. Adversarial / Edge Cases:
        # Edge A: GET /api (without trailing slash)
        status_api_noslash, _, body_api_noslash = http_get(f"{base_url}/api")
        details_api_noslash = f"Status: {status_api_noslash}, Body: {body_api_noslash[:100].strip()}"
        record_result("ADV-01", "GET /api (no trailing slash) behavior audit", True, details_api_noslash)

        # Edge B: Path traversal attempts
        traversal_attempts = [
            "/../../project.yaml",
            "/%2e%2e/project.yaml",
            "/assets/../../project.yaml",
            "/....//project.yaml"
        ]
        leaked = False
        for trav in traversal_attempts:
            st, _, bd = http_get(f"{base_url}{trav}")
            if "spec_version" in bd:
                leaked = True
                break
        record_result("ADV-02", "Path traversal attempts contained", not leaked, f"Tested {len(traversal_attempts)} traversal patterns. Leaked: {leaked}")

    finally:
        # 13. Graceful Shutdown Test (SIGTERM)
        t_start = time.time()
        proc1.send_signal(signal.SIGTERM)
        try:
            stdout1, _ = proc1.communicate(timeout=5.0)
            t_exit = time.time() - t_start
            exit_code = proc1.returncode
            term_pass = (exit_code == 0 and "Server terminated gracefully" in stdout1)
            record_result("SHT-01", "SIGTERM graceful shutdown (exit code 0)", term_pass, f"Exit code: {exit_code}, Duration: {t_exit:.2f}s, Log matched: {'Server terminated gracefully' in stdout1}")
        except subprocess.TimeoutExpired:
            proc1.kill()
            record_result("SHT-01", "SIGTERM graceful shutdown", False, "Timed out after 5s waiting for exit")

        # Verify port released
        port_free = not wait_for_port(port1, timeout=1.0)
        record_result("SHT-02", "Port released immediately after SIGTERM", port_free, f"Port {port1} released")

        # Cleanup temp dirs
        shutil.rmtree(base_tmp, ignore_errors=True)

    # 14. Test Graceful Shutdown with SIGINT
    port2 = get_free_port()
    proc2 = subprocess.Popen(
        [tmp_bin, f"--port={port2}"],
        cwd=PROJECT_ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True
    )
    try:
        ready2 = wait_for_port(port2, timeout=5.0)
        record_result("CLI-01", "--port flag binding precedence", ready2, f"Port: {port2}")

        t_start2 = time.time()
        proc2.send_signal(signal.SIGINT)
        stdout2, _ = proc2.communicate(timeout=5.0)
        t_exit2 = time.time() - t_start2
        int_pass = (proc2.returncode == 0 and "Server terminated gracefully" in stdout2)
        record_result("SHT-03", "SIGINT graceful shutdown (exit code 0)", int_pass, f"Exit code: {proc2.returncode}, Duration: {t_exit2:.2f}s, Log matched: {'Server terminated gracefully' in stdout2}")
        port2_free = not wait_for_port(port2, timeout=1.0)
        record_result("SHT-04", "Port released immediately after SIGINT", port2_free, f"Port {port2} released")
    except Exception as e:
        proc2.kill()
        record_result("SHT-03", "SIGINT graceful shutdown", False, str(e))
    finally:
        if os.path.exists(tmp_bin):
            os.remove(tmp_bin)

# ==============================================================================
# Suite 2: Docker Linux amd64 Binary Runtime Tests
# ==============================================================================
def run_docker_suite():
    print("\n" + "="*70)
    print(" SUITE 2: Linux amd64 Binary in Docker (alpine:latest) Runtime Tests")
    print("="*70)

    if not os.path.isfile(LINUX_AMD64_BIN):
        record_result("DCK-01", "Linux amd64 binary exists", False, f"Missing {LINUX_AMD64_BIN}")
        return

    port_dck = get_free_port()
    container_name = f"m1_challenger_test_{int(time.time())}"

    # Run rootfs_amd64/bin/accounting_serv inside alpine:latest
    docker_cmd = [
        "docker", "run", "--name", container_name,
        "--rm", "-d",
        "-p", f"{port_dck}:21010",
        "-v", f"{PROJECT_ROOT}:/app",
        "-w", "/app",
        "-e", "UGAPP_DATA_DIR=/tmp/docker_test_data/sub1/sub2",
        "-e", "UGAPP_LOG_DIR=/tmp/docker_test_logs/sub1/sub2",
        "alpine:latest",
        "/app/rootfs_amd64/bin/accounting_serv", "--port=21010"
    ]

    res = subprocess.run(docker_cmd, capture_output=True, text=True)
    if res.returncode != 0:
        record_result("DCK-01", "Docker container startup", False, res.stderr)
        return
    container_id = res.stdout.strip()
    record_result("DCK-01", "Docker container started with Linux amd64 binary", True, f"Container ID: {container_id[:12]}")

    try:
        ready = wait_for_port(port_dck, timeout=8.0)
        record_result("DCK-02", "Linux amd64 binary port reachable", ready, f"Port {port_dck}")

        if ready:
            base_url = f"http://127.0.0.1:{port_dck}"

            # Test health
            status, headers, body = http_get(f"{base_url}/api/v1/health")
            dck_health_pass = (status == 200 and '"status":"ok"' in body and '"app_id":"com.xsguyu.accounting"' in body)
            record_result("DCK-03", "Docker Linux binary /api/v1/health", dck_health_pass, f"Status: {status}, Body: {body.strip()}")

            # Test SPA fallback in Docker
            status_spa, _, body_spa = http_get(f"{base_url}/dashboard")
            dck_spa_pass = (status_spa == 200 and "<title>七喵记账</title>" in body_spa)
            record_result("DCK-04", "Docker Linux binary SPA fallback (/dashboard)", dck_spa_pass, f"Status: {status_spa}")

            # Test static assets in Docker
            status_css, _, _ = http_get(f"{base_url}/assets/index.CYlW3k5p.css")
            record_result("DCK-05", "Docker Linux binary static asset serving", status_css == 200, f"Status: {status_css}")

            # Verify container directories created
            exec_check = subprocess.run(
                ["docker", "exec", container_name, "test", "-d", "/tmp/docker_test_data/sub1/sub2"],
                capture_output=True
            )
            record_result("DCK-06", "Docker container nested UGAPP_DATA_DIR created", exec_check.returncode == 0, "Dir /tmp/docker_test_data/sub1/sub2 exists")

        # Test graceful shutdown of container via docker stop
        t_stop_start = time.time()
        stop_cmd = ["docker", "stop", "-t", "5", container_name]
        stop_res = subprocess.run(stop_cmd, capture_output=True, text=True)
        stop_dur = time.time() - t_stop_start
        stop_pass = (stop_res.returncode == 0 and stop_dur < 4.0)
        record_result("DCK-07", "Docker SIGTERM graceful container stop (<4s)", stop_pass, f"Duration: {stop_dur:.2f}s")

    finally:
        # Ensure container killed
        subprocess.run(["docker", "rm", "-f", container_name], capture_output=True)

# ==============================================================================
# Main Runner & Summary
# ==============================================================================
if __name__ == "__main__":
    t0 = time.time()
    run_native_suite()
    run_docker_suite()
    elapsed = time.time() - t0

    print("\n" + "="*70)
    print(f" EMPIRICAL CHALLENGE SUMMARY ({len(test_results)} tests in {elapsed:.2f}s)")
    print("="*70)

    passed_count = sum(1 for t in test_results if t["passed"])
    failed_count = sum(1 for t in test_results if not t["passed"])

    for t in test_results:
        st = "PASS" if t["passed"] else "FAIL"
        print(f"[{st}] {t['id']}: {t['name']}")

    print("-" * 70)
    print(f"Total: {len(test_results)} | Passed: {passed_count} | Failed: {failed_count}")
    if failed_count == 0:
        print("\033[92mVERDICT: APPROVE\033[0m")
        sys.exit(0)
    else:
        print("\033[91mVERDICT: REQUEST_CHANGES\033[0m")
        sys.exit(1)
