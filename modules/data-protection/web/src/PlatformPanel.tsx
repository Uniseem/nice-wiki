import { Mono, Notice, Panel, t } from '@ui';

// 全平台备份：数据库与文件卷由部署目录里的脚本备份（不在网页里操作，避免备份期间依赖被备份的服务本身）。
export function PlatformPanel() {
  return (
    <div className="flex flex-col gap-6">
      <Panel title={t('整个平台的备份')} description={t('平台的数据库、正文文件、媒体文件和密钥由部署目录里的脚本备份和恢复；用户各自站点的内容备份在“导出与备份”里。')}>
        <ol className="flex list-decimal flex-col gap-2 ps-5 text-sm leading-6 text-foreground">
          <li>{t('在服务器的部署目录运行')} <Mono>deploy/backup.sh</Mono>{t('：导出整个数据库，再把正文、媒体和密钥打成一个压缩包，放到 backups 目录。')}</li>
          <li>{t('把压缩包同步到异地：可以用 rclone 同步到 S3、WebDAV 或网盘，例如')} <Mono>rclone copy backups remote:nicewiki</Mono>{t('。')}</li>
          <li>{t('恢复时先停服务，再运行')} <Mono>deploy/restore.sh {'<'}{t('压缩包路径')}{'>'}</Mono>{t('，完成后重新启动。')}</li>
        </ol>
      </Panel>
      <Notice status="warning" title={t('密钥要单独保管')}>{t('备份包里包含主密钥：它能解开数据库里加密保存的第三方登录凭证、用户自带的 AI 密钥和备份凭据。压缩包请放在受控的存储里，不要公开。')}</Notice>
    </div>
  );
}
