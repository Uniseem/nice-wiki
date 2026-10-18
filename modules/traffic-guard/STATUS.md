# 流量风控当前进度

更新：2026-10-18。用户要求持续并行实现，目标全仓至少75%，并及时经根GitLab检查及推送。独占分支codex/traffic-guard-20261018-bulk，基线0a69a5a，首批源码e8d9215。

首批16个既有主要函数与11个必要参考助手已有源码，全部blocked（待统一GitLab），不提前implemented/verified；原未改登录17函数与存储函数保留既有证据，accepted仍为0。已完成策略可信站点读取/首次0→1与后续CAS、同事务原安全回执/outbox、AI禁令/撤销/分页、三层桶（第二批补齐网络）、真实匿名风险信号、FIFO排队与显式取消、到期及保留清理。

ReportQuestionSignals只收已有前置风险，决策严格allow/deny/challenge；similarityScore/intervalMs可选，缺失保持未知/SQL NULL，不按0观测计数或增加信任。主体与网络仅取原guard签名scope，不收问题正文或原IP。

完整BuildApplication仍失败关闭，运行写窗口/独立预算key/其他HTTP/后台和恢复回执尚缺。下一批继续使用根新增平台统一许可/通过证明签名和30日网络预算scope，不在本域复制签名。

本机仅阅读和Git差异审阅，未运行Go/gofmt/生成/编译/SQL行为验收/服务/真实接口。未新增测试、fixture、mock、样例或临时验证程序。

第二批源码5b44ea5新增10个原有主业务函数及8个必要参考助手，仍blocked待GitLab：匿名主体、原根准入、并发分配、消费签名、续租释放、原签名/在线验证和HTTP入口。三层频率原子扣减，网络次数字段兼容默认600。先分配空token，只有原根已消费QA可签模型凭证；免费提示永不签模型token。继续挑战与事件，不等完整服务才交CI。
