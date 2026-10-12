# CrowdSec

> 开源的入侵检测和防护系统，带社区共享的恶意 IP 黑名单。

| 项目 | 信息 |
|---|---|
| 方向 | 防滥用 |
| 仓库 | crowdsecurity/crowdsec |
| 许可证 | MIT |
| 主要语言 | Go |
| Star | 15.2k |
| 最近更新 | 2026-10 |

## 它是什么

分析日志和 HTTP 请求识别攻击，通过"修复组件"（bouncer，可装在反向代理上）拦截，并共享社区维护的实时黑名单。

## 和我们相关的部分

- 检测规则在它的 Hub 上共享。
- 拦截动作由修复组件执行，可以装在 Nginx 等反向代理上。

## 借鉴什么

- **作为可选的部署加固**写进部署文档：在反向代理层用 CrowdSec 拦截已知的恶意 IP，和程序内部的防刷流互补。程序本身不集成。

## 注意

- 会多运行一个服务，属于运维层面的选择，不是必需的。

## 资料

- [仓库](https://github.com/crowdsecurity/crowdsec)
- [修复组件文档](https://docs.crowdsec.net/u/bouncers/intro)
