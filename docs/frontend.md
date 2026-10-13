# 界面装配契约

portal维护后台导航、权限菜单、全局错误与加载外壳；业务域拥有本域页面、状态和API客户端。每域manifest声明moduleId、apiVersion、uiVersion、routeBase、entryAsset、requiredPermissions、capabilities；静态入口只允许同源已注册位置，不接受访客提供脚本URL。

路由以/admin/模块名开头，公开页面由public-site完整HTML输出；问答和语言建议岛通过各域版本化入口加载。模块页面独立构建，API兼容更新只部署该域；公共设计令牌/纯基础组件固定版本，不共享业务Pinia store，不跨域导入内部组件。

后台全部中文；错误、空态、缺能力、冲突、进度和费用确认均有可操作说明。编辑器按content块schema维护稳定ID，界面不能擅增未登记块/属性。共享文档格式是契约，不把每块渲染拆为一次网络调用。

业务模块的implementation清单包含本域UI文件和事件处理函数；没有UI的基础服务可由portal管理页调用其API，但在files.md明确所有权。测试/E2E或新借用UI组件仍须另行授权。
