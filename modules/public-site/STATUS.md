# 公开交付当前进度

更新：2026-10-15。第四轮支援仅补首页公开链接解析producer契约，阶段仍设计；本域无实际go.mod，所有业务函数、文件与生成物仍planned，业务实现/验收/部署均未开始。

- 分支codex/public-site-link-contract-r1，精确基线b56430efe56b724d6efdbe8bf3ae39f15fc6803c，独占modules/public-site；接续时工作树干净。
- 新增POST /v1/public-links:resolve及binding ResolvePublishedLink，登记于既有internal/links.go；输入稳定documentId/显式language/可选块ID，封闭true/false结果和精确reason，site-config纯服务限定，无新产品行为或实际文件。
- 已确认启用语言缺译保留该语言URL、旧译/原文显示，resolvedLanguage等请求language；content公开树required canonicalPath已由根在合成快照补齐，不能由裁剪后导航树拼缺失祖先。最终公开/路径水位与未知依赖均失败关闭，禁止readPublicHome递归。
- 本轮合同/生成物已通过VPS协议检查，准确合同交付SHA待本轮提交后回填；未本机生成/编译/测试，未新增测试、样例或临时业务程序，未运行真实接口/模型/服务，未push或部署。数量以机器台账和生成目录为准。
- VPS证据：2026-10-15 第四轮协议冻结快照在隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8通过make docs/gen和完整make check（858文件/1105函数/379操作/2253类型），回传生成物0冲突。根14个现有Go模块readonly build/vet/gofmt同时通过；public-site尚无go.mod或业务源码，本域仅协议/生成物检查，不构成自身Go业务编译、实现或行为验收。pure-null在Go降级副本保留nullable+enum:[null]+x-go-type:any，未放宽3.1/TS源约束。未新增或运行业务测试、真实接口/模型、服务、镜像或部署。
- 生成/结构检查只能验证协议设计，本域无Go module，不能据根14个既有模块通过把本域标implemented/verified。

下一项：按交接核对本域启动与真实handler装配的既定合同，只有根分派后实施；本轮交付后冻结。
