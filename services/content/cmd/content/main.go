// content 服务可执行入口：同一进程内运行 content、media、localization、public-site 组件；
// 各组件独立装配、监听和配置，进程模板由 libs/platform/lifecycle.RunServices 统一实现。
package main

import (
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	contentcomponent "nice-wiki/modules/content/component"
	localizationcomponent "nice-wiki/modules/localization/component"
	mediacomponent "nice-wiki/modules/media/component"
	publicsitecomponent "nice-wiki/modules/public-site/component"
)

func main() {
	os.Exit(lifecycle.RunServices(os.Args[1:], []platform.ServiceSpec{
		contentcomponent.Spec(),
		mediacomponent.Spec(),
		localizationcomponent.Spec(),
		publicsitecomponent.Spec(),
	}))
}
