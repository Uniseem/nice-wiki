#!/usr/bin/env python3
"""给某个域建立最小的后台界面工程（Vue 模块，由 portal 壳挂载）。
用法：python3 scripts/new_ui.py <域> <页面组件名>，例如 new_ui.py media LibraryPage"""
import json, os, shutil, subprocess, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
module, page = sys.argv[1], sys.argv[2]
web = os.path.join(ROOT, "modules", module, "web")
os.makedirs(os.path.join(web, "src"), exist_ok=True)
template = os.path.join(ROOT, "modules", "site-config", "web")
for name in ("tsconfig.json", "vite.config.ts"):
    shutil.copy(os.path.join(template, name), os.path.join(web, name))
for name in ("client.ts", "style.css"):
    shutil.copy(os.path.join(template, "src", name), os.path.join(web, "src", name))
package = json.load(open(os.path.join(template, "package.json")))
package["name"] = f"@nice-wiki/{module}-ui"
json.dump(package, open(os.path.join(web, "package.json"), "w"), indent=2)
open(os.path.join(web, "package.json"), "a").write("\n")
main = open(os.path.join(template, "src", "main.ts")).read().replace("'site-config'", f"'{module}'").replace("SitePage", page)
open(os.path.join(web, "src", "main.ts"), "w").write(main)
subprocess.check_call(["pnpm", "install"], cwd=web)
print("已建立", web, "；请写 src/%s.vue，并在 scripts/dev.py 的 WEB_MODULES、scripts/gen_deploy.py 的 WEB_MODULES、portal/web/src/Shell.vue 的导航里登记" % page)
