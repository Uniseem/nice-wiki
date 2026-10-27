// ai 服务可执行入口：同一进程内运行 model-gateway、qa、agent、memory、knowledge 组件；
// 各组件独立装配、监听和配置，进程模板由 libs/platform/lifecycle.RunServices 统一实现。
package main

import (
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	agentcomponent "nice-wiki/modules/agent/component"
	knowledgecomponent "nice-wiki/modules/knowledge/component"
	memorycomponent "nice-wiki/modules/memory/component"
	modelgatewaycomponent "nice-wiki/modules/model-gateway/component"
	qacomponent "nice-wiki/modules/qa/component"
)

func main() {
	// 知识库的 PDF 解析在隔离子进程里做，子进程就是本程序自己，要在读任何配置之前先识别。
	if handled, err := knowledgecomponent.RunParserChild(); handled {
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(lifecycle.RunServices(os.Args[1:], []platform.ServiceSpec{
		modelgatewaycomponent.Spec(),
		qacomponent.Spec(),
		agentcomponent.Spec(),
		memorycomponent.Spec(),
		knowledgecomponent.Spec(),
	}))
}
