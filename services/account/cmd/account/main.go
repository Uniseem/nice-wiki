// account 服务可执行入口：同一进程内运行 site-config、identity、portal 组件；
// 各组件独立装配、监听和配置，进程模板由 libs/platform/lifecycle.RunServices 统一实现。
package main

import (
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	identitycomponent "nice-wiki/modules/identity/component"
	portalcomponent "nice-wiki/modules/portal/component"
	siteconfigcomponent "nice-wiki/modules/site-config/component"
)

func main() {
	os.Exit(lifecycle.RunServices(os.Args[1:], []platform.ServiceSpec{
		siteconfigcomponent.Spec(),
		identitycomponent.Spec(),
		portalcomponent.Spec(),
	}))
}
