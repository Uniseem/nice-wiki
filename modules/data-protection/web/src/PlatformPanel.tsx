import { Mono, Notice, Panel, t } from '@ui';

// 全平台备份：数据库与文件卷由部署目录里的脚本备份（不在网页里操作，避免备份期间依赖被备份的服务本身）。
export function PlatformPanel() {
  return (
    <div className="flex flex-col gap-6">
      <Panel title={t('整个平台的备份')} description={t('平台的数据库、正文文件、媒体文件和密钥由部署目录里的脚本备份和恢复；用户各自站点的内容备份在“导出与备份”里。')}>
        <ol className="flex list-decimal flex-col gap-2 ps-5 text-sm leading-6 text-foreground">
          <li>{t('在服务器的部署目录运行')} <Mono>deploy/backup.sh</Mono>{t('：导出数据库，增量复制正文和媒体（第二次起只复制新增的），其余文件卷和密钥打成压缩包，放到 backups 目录。')}</li>
          <li>{t('设置')} <Mono>BACKUP_PASSPHRASE_FILE</Mono>{t('可以用口令加密数据库导出和密钥包；设置')} <Mono>BACKUP_REMOTE</Mono>{t('（rclone 目标，支持 S3、WebDAV 和各家网盘）会在备份后自动同步到异地。')}</li>
          <li>{t('恢复时先停服务，再运行')} <Mono>deploy/restore.sh {'<'}{t('备份目录')}{'>'} {'<'}{t('时间戳')}{'>'}</Mono>{t('，完成后重新启动。')}</li>
        </ol>
      </Panel>
      <Notice status="warning" title={t('密钥要单独保管')}>{t('密钥包里包含主密钥：它能解开数据库里加密保存的第三方登录凭证、用户自带的 AI 密钥和备份凭据。请设置口令加密，把备份放在受控的存储里，口令和备份分开保管。')}</Notice>
    </div>
  );
}
