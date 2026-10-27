// ops 服务可执行入口：同一进程内运行 jobs、analytics、traffic-guard 组件；
// 各组件独立装配、监听和配置，进程模板由 libs/platform/lifecycle.RunServices 统一实现。
package main

import (
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	analyticscomponent "nice-wiki/modules/analytics/component"
	jobscomponent "nice-wiki/modules/jobs/component"
	trafficguardcomponent "nice-wiki/modules/traffic-guard/component"
)

func main() {
	os.Exit(lifecycle.RunServices(os.Args[1:], []platform.ServiceSpec{
		jobscomponent.Spec(),
		analyticscomponent.Spec(),
		trafficguardcomponent.Spec(),
	}))
}
