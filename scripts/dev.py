#!/usr/bin/env python3
"""本地开发环境：生成密钥与配置到 .local/dev（不进 git），编译并启动/停止全部服务。

用法：python3 scripts/dev.py up [服务...] | down | reset（清空数据）| status | logs <服务>
"""
import json, os, signal, socket, subprocess, sys, time, glob

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DEV = os.path.join(ROOT, ".local", "dev")
PORTS = {"portal": 8080, "public-site": 8081, "identity": 8101, "site-config": 8102, "content": 8103,
         "media": 8104, "model-gateway": 8105, "knowledge": 8106, "qa": 8107, "localization": 8108,
         "memory": 8109, "traffic-guard": 8110, "jobs": 8111, "analytics": 8112, "data-protection": 8113}
DEFAULT_UP = ["site-config", "identity", "jobs", "traffic-guard", "media", "localization", "content", "public-site", "model-gateway", "knowledge", "memory", "analytics", "data-protection", "qa", "portal"]


def path(*parts):
    return os.path.join(DEV, *parts)


def setup():
    os.makedirs(path("bin"), exist_ok=True)
    os.makedirs(path("logs"), exist_ok=True)
    services = [{"slug": s, "secretsDir": path("secrets", s)} for s in PORTS]
    os.makedirs(path("config"), exist_ok=True)
    json.dump({"version": 1, "services": services}, open(path("config", "services.json"), "w"))
    subprocess.check_call(["go", "run", "./cmd/nw-keys", "--services", path("config", "services.json"),
                           "--out", path("config", "trust-anchors.json")], cwd=os.path.join(ROOT, "libs/platform"))
    for slug in PORTS:
        sec = path("secrets", slug)
        pem = sorted(glob.glob(os.path.join(sec, "signing-*.pem")))[-1]
        key_id = os.path.basename(pem)[len("signing-"):-len(".pem")]
        json.dump({"keyId": key_id, "privateKeyRef": pem}, open(os.path.join(sec, "signing-key.json"), "w"))
        cfg = {"module": slug, "listen": f"127.0.0.1:{PORTS[slug]}", "dataRoot": path("data", slug),
               "trustAnchorRef": path("config", "trust-anchors.json"),
               "signingKeyRef": os.path.join(sec, "signing-key.json"),
               "dependencies": {s: f"http://127.0.0.1:{p}" for s, p in PORTS.items() if s != slug},
               "allowLocalHTTP": True}
        if slug in ("model-gateway", "data-protection"):
            master = os.path.join(sec, "master-key")
            if not os.path.exists(master):
                with open(master, "wb") as f:
                    f.write(os.urandom(32))
                os.chmod(master, 0o600)
            cfg["masterKeyRef"] = master
        if slug == "analytics":
            cfg["reportingTimeZone"] = "Asia/Shanghai"
        if slug == "site-config":
            cfg["publicBaseURL"] = f"http://127.0.0.1:{PORTS['public-site']}"
        if slug in ("identity", "portal"):
            cfg["publicBaseURL"] = f"http://127.0.0.1:{PORTS['portal']}"
        if slug == "portal":
            dele = sorted(glob.glob(os.path.join(sec, "delegation-*.pem")))[-1]
            did = os.path.basename(dele)[len("delegation-"):-len(".pem")]
            json.dump({"keyId": did, "privateKeyRef": dele}, open(os.path.join(sec, "delegation-key.json"), "w"))
            cfg["delegationKeyRef"] = os.path.join(sec, "delegation-key.json")
            cfg["trustedProxyCIDRs"] = []
        os.makedirs(cfg["dataRoot"], exist_ok=True, mode=0o700)
        json.dump(cfg, open(path("config", f"{slug}.json"), "w"), indent=2)


WEB_MODULES = ["content", "site-config", "identity", "media", "localization", "model-gateway", "traffic-guard", "jobs", "analytics", "data-protection"]


def build(slugs):
    for s in slugs:
        subprocess.check_call(["go", "build", "-o", path("bin", s), f"./cmd/{s}"], cwd=os.path.join(ROOT, "modules", s))


def port_open(port):
    with socket.socket() as sock:
        sock.settimeout(0.2)
        return sock.connect_ex(("127.0.0.1", port)) == 0


def quiet(cmd, cwd):
    result = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    if result.returncode:
        sys.exit(f"命令失败 {' '.join(cmd)} @ {cwd}\n{result.stdout}{result.stderr}")


def build_web():
    """后台前端：壳放在 web/ 根，各域界面放在 web/modules/<域>/。"""
    web = path("web")
    subprocess.run(["rm", "-rf", web])
    os.makedirs(os.path.join(web, "modules"))
    shell = os.path.join(ROOT, "modules", "portal", "web")
    quiet(["pnpm", "install", "--frozen-lockfile"], shell)
    quiet(["pnpm", "exec", "vite", "build", "--outDir", web, "--emptyOutDir"], shell)
    for m in WEB_MODULES:
        d = os.path.join(ROOT, "modules", m, "web")
        quiet(["pnpm", "install", "--frozen-lockfile"], d)
        quiet(["pnpm", "exec", "vite", "build", "--outDir", os.path.join(web, "modules", m), "--emptyOutDir"], d)


def pidfile(s):
    return path("run", s + ".pid")


def up(slugs):
    setup()
    build(slugs)
    if "portal" in slugs:
        build_web()
    os.makedirs(path("run"), exist_ok=True)
    for s in slugs:
        stop(s)
        log = open(path("logs", s + ".log"), "w")
        p = subprocess.Popen([path("bin", s), "--config", path("config", s + ".json")], stdout=log, stderr=log, start_new_session=True, cwd=os.path.join(ROOT, "modules", s), env={**os.environ, "NW_DEBUG": "1", "NW_DEBUG_STACK": "1", "NW_WEB_DIR": path("web")})
        open(pidfile(s), "w").write(str(p.pid))
        for _ in range(100):
            time.sleep(0.1)
            if p.poll() is not None or port_open(PORTS[s]):
                break
        print(("运行中 " if p.poll() is None else "已退出 ") + s, f"(:{PORTS[s]})")


def stop(slug):
    """停掉该服务已有的进程（若有），避免新进程因端口占用而悄悄失败。"""
    f = pidfile(slug)
    if os.path.exists(f):
        try:
            os.killpg(int(open(f).read()), signal.SIGTERM)
        except (ProcessLookupError, ValueError):
            pass
        os.remove(f)
        for _ in range(50):
            if not port_open(PORTS[slug]):
                break
            time.sleep(0.1)


def down():
    for f in glob.glob(path("run", "*.pid")):
        try:
            os.killpg(int(open(f).read()), signal.SIGTERM)
        except (ProcessLookupError, ValueError):
            pass
        os.remove(f)
    subprocess.run(["pkill", "-f", path("bin") + "/"])


def status():
    for s in PORTS:
        f = pidfile(s)
        alive = False
        if os.path.exists(f):
            try:
                os.kill(int(open(f).read()), 0)
                alive = True
            except (ProcessLookupError, ValueError):
                pass
        print(("运行中 " if alive else "未运行 ") + s)


if __name__ == "__main__":
    cmd = sys.argv[1] if len(sys.argv) > 1 else "status"
    if cmd == "up":
        up(sys.argv[2:] or DEFAULT_UP)
    elif cmd == "down":
        down()
    elif cmd == "reset":
        down()
        subprocess.run(["rm", "-rf", path("data")])
    elif cmd == "logs":
        print(open(path("logs", sys.argv[2] + ".log")).read())
    else:
        status()
