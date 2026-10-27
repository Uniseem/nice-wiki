# 长期记忆、任务映射与记忆助手需求

维护人工可审计记忆条目、任务映射、语言范围、版本化注入预算和只产生提议的记忆助手；批准后才应用记忆或术语变更。

## M11-R01

原始人工记忆是本服务权威数据；其他服务仅请求MemoryContext，不读文件/表、不缓存无版本正文。

验收规格：契约验证行为：原始人工记忆是本服务权威数据；其他服务仅请求MemoryContext，不读文件/表、不缓存无版本正文。；实现阶段才可执行验证，不在设计阶段宣称通过。

## M11-R02

每个context固定entry revisions、mapping revision、语言fallback、renderer version；versionHash为规范排序manifest与正文SHA256。

验收规格：契约验证行为：每个context固定entry revisions、mapping revision、语言fallback、renderer version；versionHash为规范排序manifest与正文SHA256。；实现阶段才可执行验证，不在设计阶段宣称通过。

## M11-R03

单条默认1536UTF8bytes可单独调整，单任务注入总量默认5120UTF8bytes；人工写条目或映射前投影全部受影响任务，任何超预算拒绝保存并指出taskId，禁止静默丢条目。

验收规格：契约验证行为：单条默认1536UTF8bytes可单独调整，单任务注入总量默认5120UTF8bytes；人工写条目或映射前投影全部受影响任务，任何超预算拒绝保存并指出taskId，禁止静默丢条目。；实现阶段才可执行验证，不在设计阶段宣称通过。

## M11-R04

助手只生成proposal，不写active entry或glossary；批准与拒绝必须由人工memory.manage，助手服务身份不能自批准。

验收规格：契约验证行为：助手只生成proposal，不写active entry或glossary；批准与拒绝必须由人工memory.manage，助手服务身份不能自批准。；实现阶段才可执行验证，不在设计阶段宣称通过。

## M11-R05

术语提议批准产生发送localization批准命令的outbox；localization终态事件异步更新proposal，不同步回调resolveContext。

验收规格：契约验证行为：术语提议批准产生发送localization批准命令的outbox；localization终态事件异步更新proposal，不同步回调resolveContext。；实现阶段才可执行验证，不在设计阶段宣称通过。

## M11-R06

人工版本变更后旧助手结果只能superseded；expectedEntryVersion/expectedMappingRevision CAS禁止覆盖新编辑。

验收规格：契约验证行为：人工版本变更后旧助手结果只能superseded；expectedEntryVersion/expectedMappingRevision CAS禁止覆盖新编辑。；实现阶段才可执行验证，不在设计阶段宣称通过。

## M11-R07

记忆不保存供应方密钥、费用余额、匿名IP或完整访客问答；导入条目必须人工确认正文来源。

验收规格：契约验证行为：记忆不保存供应方密钥、费用余额、匿名IP或完整访客问答；导入条目必须人工确认正文来源。；实现阶段才可执行验证，不在设计阶段宣称通过。

## M11-R08

entry正文权威为本域Markdown文件含frontmatter，数据库仅索引；readonly条目助手只能建议人手动修改。

验收规格：契约验证行为：entry正文权威为本域Markdown文件含frontmatter，数据库仅索引；readonly条目助手只能建议人手动修改。；实现阶段才可执行验证，不在设计阶段宣称通过。

所有用户可观察错误均通过公共Problem；本阶段仅设计。
