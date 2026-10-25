# 原生 Agent

只存内存中的在途帧；完成的原生 checkpoint 由 qa 保存，schemaVersion=1。重启后在途问答按既有失败收尾，不自动重新调用模型。
