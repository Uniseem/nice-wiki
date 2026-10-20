# 推送

1. 只在用户明确要求时推送，不强推。
2. 推送前在本地跑 `make build`、`make check` 和 `make privacy`；提交里不得出现真实邮箱、密钥、本机路径或访客数据，作者必须是 GitHub 代用邮箱。
3. 双推：`git push origin HEAD && git push gitlab HEAD`，然后确认两个远端的 SHA 一致。
4. 不写推送记录文件，git log 就是记录。
