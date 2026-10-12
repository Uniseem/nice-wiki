# 新增接口

1. 先修改 api/openapi.yaml：路径、operationId、请求、响应、错误和权限。
2. 运行 make gen，禁止手改生成代码。
3. 在 api 权限入口显式声明匿名、登录、功能权限或超管策略，强制改密码仅放行允许的操作。
4. 在领域 handler 实现行为，再接入 shared/api 客户端；运行 make check。新权限测试另获确认。
