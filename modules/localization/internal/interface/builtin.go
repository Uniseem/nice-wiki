package interfacecatalog

import (
	"context"
	"database/sql"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/language"
)

// builtinStrings 是公开页和问答小部件里固定出现的界面文字（中文原文即字典 key）。
// public-site 用同一批原文做替换，所以这里的 key 必须与页面上实际出现的文字一致。
var builtinStrings = []string{
	"当前语言的译文尚未完成，部分内容显示旧译文或原文。", "复制失败，请打开 Markdown 版本",
	"打开本页目录", "打开文档树", "本页目录", "文档树", "相邻文档", "面包屑",
	"AI 未启用", "AI 问答", "亮 / 暗", "复制为 Markdown", "Markdown 版本", "已复制",
	"上一篇", "下一篇", "首页", "源代码", "视频播放器", "文档", "语言",
	"新对话", "发送", "关闭", "问问这个站点的文档…", "来源：",
	"这次没有回答成功（", "），请稍后再试。", "回答连接失败（", "请求失败（",
	"人机验证没有完成", "人机验证无法在此浏览器运行",
	"跳到正文", "复制代码", "停止", "有帮助", "没帮助", "感谢反馈", "思考中…", "回答由 AI 生成，可能有误，请以文档原文为准。",
	"浅色", "深色", "跟随系统", "向这个站点的文档提问，回答会附上来源。", "公开问答暂未开放。", "需要启用 JavaScript 才能使用 AI 问答。", "开始阅读",
	"页面不存在", "地址可能有误，或这篇文档还没有发布。", "页面已删除", "这篇文档已经被删除。", "页面暂时无法访问", "请稍后刷新重试。", "回到首页",
}

// installSource 在给定事务里把固定界面源目录写入 interface_sources（已存在的不动）。
// 目录为空时由 SourceCatalog 自动补装；站点没初始化时本域拿不到已核本站，所以不在启动时装。
func installSource(ctx context.Context, tx *sql.Tx) error {
	for _, text := range builtinStrings {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO interface_sources(key,value_text,source_hash,placeholders) VALUES(?,?,?,'[]')`, text, text, platform.HashBytes([]byte(text))); err != nil {
			return language.StorageError(err)
		}
	}
	return nil
}
