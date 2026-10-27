#!/usr/bin/env python3
"""本地开发环境：生成密钥与配置到 .local/dev（不进 git），编译并启动/停止全部服务。

用法：python3 scripts/dev.py up [服务...] | down | reset（清空数据）| status | logs <服务>
"""
import json, os, shutil, signal, socket, subprocess, sys, time, glob

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# NW_DEV_DIR / NW_DEV_PORT_OFFSET 让同一份脚本在别处（如 NL 测试机）跑隔离实例，不碰默认的 .local/dev。
DEV = os.environ.get("NW_DEV_DIR") or os.path.join(ROOT, ".local", "dev")
PORT_OFFSET = int(os.environ.get("NW_DEV_PORT_OFFSET", "0"))
# PostgreSQL 连接串，如 postgres://用户:口令@127.0.0.1:5432/库?sslmode=disable；开发时用 docker 起一个 pgvector 实例。
DATABASE_URL = os.environ.get("NW_DEV_DATABASE_URL", "")
PORTS = {c: p + PORT_OFFSET for c, p in {"portal": 8080, "public-site": 8081, "identity": 8101, "site-config": 8102, "content": 8103,
         "media": 8104, "model-gateway": 8105, "knowledge": 8106, "qa": 8107, "localization": 8108,
         "memory": 8109, "traffic-guard": 8110, "jobs": 8111, "analytics": 8112, "agent": 8114}.items()}
# 进程 → 组件：宿主进程（services/<进程>）在同一进程里运行多个组件，组件各自监听、各自配置；
# portal 随 account 进程、knowledge 随 ai 进程运行（保持 edge、account、content、ai、search、ops 六个服务）。
HOSTS = {"account": ["site-config", "identity", "portal"], "content": ["content", "media", "localization", "public-site"],
         "ai": ["model-gateway", "qa", "agent", "memory", "knowledge"], "ops": ["jobs", "analytics", "traffic-guard"]}
# Rust 检索服务（rust/search）：独立进程，没有 cargo 或设置了 NW_DEV_NO_SEARCH 时不启动，知识库退回进程内检索。
SEARCH_PORT = 8120 + PORT_OFFSET
USE_SEARCH = shutil.which("cargo") is not None and not os.environ.get("NW_DEV_NO_SEARCH")
DEFAULT_UP = (["search"] if USE_SEARCH else []) + ["account", "ops", "content", "ai"]
LISTEN_PORTS = {**PORTS, "search": SEARCH_PORT}


def components(proc):
    return ["search"] if proc == "search" else HOSTS.get(proc, [proc])


def workdir(proc):
    if proc == "search":
        return os.path.join(ROOT, "rust")
    return os.path.join(ROOT, "services" if proc in HOSTS else "modules", proc)


def command(proc):
    if proc == "search":
        return [path("bin", "search"), path("config", "search.json")]
    if proc in HOSTS:
        return [path("bin", proc), "--config-dir", path("config")]
    return [path("bin", proc), "--config", path("config", proc + ".json")]


def path(*parts):
    return os.path.join(DEV, *parts)


def geoip():
    """离线 IP 国家库（DB-IP Lite Country，CC BY 4.0，“IP Geolocation by DB-IP”，按月更新）：没有就下载当月版本，下载失败时不提供国家信号。"""
    target = path("geoip", "dbip-country-lite.mmdb")
    if os.path.exists(target):
        return target
    if os.environ.get("NW_DEV_NO_GEOIP"):
        return None
    os.makedirs(path("geoip"), exist_ok=True)
    import datetime, gzip, urllib.request
    today = datetime.date.today()
    for month in (today, today.replace(day=1) - datetime.timedelta(days=1)):
        try:
            # 下载站拒绝 Python 默认的 User-Agent，需带一个明确的。
            request = urllib.request.Request(f"https://download.db-ip.com/free/dbip-country-lite-{month:%Y-%m}.mmdb.gz", headers={"User-Agent": "nice-wiki-dev/1"})
            with urllib.request.urlopen(request, timeout=120) as response:
                data = gzip.decompress(response.read())
        except Exception:
            continue
        with open(target + ".part", "wb") as f:
            f.write(data)
        os.replace(target + ".part", target)
        return target
    print("离线 IP 国家库下载失败，语言建议不带国家信号")
    return None


def setup():
    if not DATABASE_URL:
        sys.exit("请先设置 NW_DEV_DATABASE_URL（PostgreSQL 连接串）")
    os.makedirs(path("bin"), exist_ok=True)
    os.makedirs(path("logs"), exist_ok=True)
    services = [{"slug": s, "secretsDir": path("secrets", s)} for s in PORTS]
    os.makedirs(path("config"), exist_ok=True)
    json.dump({"version": 1, "services": services}, open(path("config", "services.json"), "w"))
    subprocess.check_call(["go", "run", "./cmd/nw-keys", "--services", path("config", "services.json"),
                           "--out", path("config", "trust-anchors.json"), "--budget-dir", path("secrets", "budget")], cwd=os.path.join(ROOT, "libs/platform"))
    budget_pem = sorted(glob.glob(path("secrets", "budget", "budget-*.pem")))[-1]
    budget_id = os.path.basename(budget_pem)[len("budget-"):-len(".pem")]
    json.dump({"keyId": budget_id, "privateKeyRef": budget_pem}, open(path("secrets", "budget", "budget-key.json"), "w"))
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
        if slug in ("model-gateway", "identity", "content"):
            master = os.path.join(sec, "master-key")
            if not os.path.exists(master):
                with open(master, "wb") as f:
                    f.write(os.urandom(32))
                os.chmod(master, 0o600)
            cfg["masterKeyRef"] = master
        if slug in ("portal", "traffic-guard"):
            cfg["budgetKeyRef"] = path("secrets", "budget", "budget-key.json")
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
            countries = geoip()
            if countries:
                cfg["geoipCountryDBRef"] = countries
        # 全部组件共用一个 PostgreSQL（各组件在自己的 schema 里）；连接串含口令，只写进私有文件，配置里放文件路径。
        dsn_file = os.path.join(sec, "database-url")
        with open(dsn_file, "w") as f:
            f.write(DATABASE_URL)
        os.chmod(dsn_file, 0o600)
        cfg["databaseRef"] = dsn_file
        os.makedirs(cfg["dataRoot"], exist_ok=True, mode=0o700)
        if slug == "knowledge" and USE_SEARCH:
            cfg["dependencies"]["search"] = f"http://127.0.0.1:{SEARCH_PORT}"
            cfg["searchSecretRef"] = path("secrets", "search", "secret")
        json.dump(cfg, open(path("config", f"{slug}.json"), "w"), indent=2)
    if USE_SEARCH:
        # 检索服务：与 knowledge 共用一个 HMAC 密钥；索引放自己的目录，向量放同一个 PostgreSQL 的 search 模式。
        os.makedirs(path("secrets", "search"), exist_ok=True, mode=0o700)
        secret = path("secrets", "search", "secret")
        if not os.path.exists(secret):
            with open(secret, "w") as f:
                f.write(os.urandom(32).hex())
            os.chmod(secret, 0o600)
        dsn_file = path("secrets", "search", "database-url")
        with open(dsn_file, "w") as f:
            f.write(DATABASE_URL)
        os.chmod(dsn_file, 0o600)
        os.makedirs(path("data", "search"), exist_ok=True, mode=0o700)
        json.dump({"listen": f"127.0.0.1:{SEARCH_PORT}", "dataDir": path("data", "search", "index"), "secretFile": secret, "databaseUrlFile": dsn_file},
                  open(path("config", "search.json"), "w"), indent=2)


def build(procs):
    for s in procs:
        if s == "search":
            subprocess.check_call(["cargo", "build", "--release", "-p", "nw-search"], cwd=workdir(s))
            target = os.environ.get("CARGO_TARGET_DIR") or os.path.join(workdir(s), "target")
            shutil.copy(os.path.join(target, "release", "nw-search"), path("bin", "search"))
            continue
        subprocess.check_call(["go", "build", "-o", path("bin", s), f"./cmd/{s}"], cwd=workdir(s))


def port_open(port):
    with socket.socket() as sock:
        sock.settimeout(0.2)
        return sock.connect_ex(("127.0.0.1", port)) == 0


def quiet(cmd, cwd, env=None):
    result = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, env={**os.environ, **(env or {})})
    if result.returncode:
        sys.exit(f"命令失败 {' '.join(cmd)} @ {cwd}\n{result.stdout}{result.stderr}")


def build_web():
    """后台单页应用与公开页样式/交互岛：仓库根一个 pnpm 工程，产物放 .local/dev/web（site/ 是公开页部分）。
    NW_SKIP_WEB_BUILD=1 且已有产物时跳过（例如直接用 GitLab CI 产出的 web-dist）。"""
    web = path("web")
    if os.environ.get("NW_SKIP_WEB_BUILD") and os.path.exists(os.path.join(web, "index.html")):
        return
    quiet(["pnpm", "install", "--frozen-lockfile"], ROOT)
    quiet(["pnpm", "build"], ROOT, {"NW_WEB_OUT": web, "NW_AGENT_OUT": path("agent", "worker.mjs")})


def pidfile(s):
    return path("run", s + ".pid")


def proc_env():
    return {**os.environ, "NW_DEBUG": "1", "NW_DEBUG_STACK": "1", "NW_DEV_NO_DISK_WATERMARK": "1", "NW_WEB_DIR": path("web"), "NW_ROUTES_FILE": os.path.join(ROOT, "modules", "portal", "config", "routes.json"), "NW_ZCODE_WORKER": path("agent", "worker.mjs")}


def up(procs):
    setup()
    build(procs)
    if "account" in procs or "ai" in procs:
        build_web()
    os.makedirs(path("run"), exist_ok=True)
    for s in procs:
        stop(s)
        log = open(path("logs", s + ".log"), "w")
        p = subprocess.Popen(command(s), stdout=log, stderr=log, start_new_session=True, cwd=workdir(s), env=proc_env())
        open(pidfile(s), "w").write(str(p.pid))
        for _ in range(100):
            time.sleep(0.1)
            if p.poll() is not None or all(port_open(LISTEN_PORTS[c]) for c in components(s)):
                break
        print(("运行中 " if p.poll() is None else "已退出 ") + s, " ".join(f"{c}:{LISTEN_PORTS[c]}" for c in components(s)))
    if not os.path.exists(path("run", "watch.pid")):
        subprocess.Popen([sys.executable, os.path.abspath(__file__), "watch", *procs], start_new_session=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def stop(proc):
    """停掉该进程已有的实例（若有），避免新进程因端口占用而悄悄失败。"""
    f = pidfile(proc)
    if os.path.exists(f):
        try:
            os.killpg(int(open(f).read()), signal.SIGTERM)
        except (ProcessLookupError, ValueError, PermissionError):
            pass
        os.remove(f)
        for _ in range(50):
            if not any(port_open(LISTEN_PORTS[c]) for c in components(proc)):
                break
            time.sleep(0.1)


def start_one(s):
    log = open(path("logs", s + ".log"), "a")
    p = subprocess.Popen(command(s), stdout=log, stderr=log, start_new_session=True, cwd=workdir(s), env=proc_env())
    open(pidfile(s), "w").write(str(p.pid))
    return p


def alive(slug):
    f = pidfile(slug)
    if not os.path.exists(f):
        return False
    try:
        os.kill(int(open(f).read()), 0)
        return True
    except (ProcessLookupError, ValueError, PermissionError):
        return False


def watch(slugs):
    """模拟部署时的进程守护（restart: unless-stopped）：服务因依赖未就绪或站点未初始化而退出时，隔几秒重新拉起。"""
    open(path("run", "watch.pid"), "w").write(str(os.getpid()))
    children = {}  # 自己拉起的子进程要 poll() 才会回收，否则退出后成僵尸，kill(pid, 0) 仍成功
    while True:
        for s in slugs:
            child = children.get(s)
            if child is not None and child.poll() is not None:
                children.pop(s)
                if os.path.exists(pidfile(s)) and open(pidfile(s)).read() == str(child.pid):
                    os.remove(pidfile(s))
            if os.path.exists(pidfile(s)) and not alive(s):
                children[s] = start_one(s)
        time.sleep(3)


def down():
    wp = path("run", "watch.pid")
    if os.path.exists(wp):
        try:
            os.kill(int(open(wp).read()), signal.SIGTERM)
        except (ProcessLookupError, ValueError, PermissionError):
            pass
        os.remove(wp)
    for f in glob.glob(path("run", "*.pid")):
        try:
            os.killpg(int(open(f).read()), signal.SIGTERM)
        except (ProcessLookupError, ValueError, PermissionError):
            pass
        os.remove(f)
    subprocess.run(["pkill", "-f", path("bin") + "/"])


def status():
    for s in DEFAULT_UP:
        f = pidfile(s)
        alive = False
        if os.path.exists(f):
            try:
                os.kill(int(open(f).read()), 0)
                alive = True
            except (ProcessLookupError, ValueError, PermissionError):
                pass
        print(("运行中 " if alive else "未运行 ") + s)


if __name__ == "__main__":
    cmd = sys.argv[1] if len(sys.argv) > 1 else "status"
    if cmd == "up":
        up(sys.argv[2:] or DEFAULT_UP)
    elif cmd == "down":
        down()
    elif cmd == "watch":
        watch(sys.argv[2:])
    elif cmd == "reset":
        down()
        subprocess.run(["rm", "-rf", path("data")])
    elif cmd == "logs":
        print(open(path("logs", sys.argv[2] + ".log")).read())
    else:
        status()
