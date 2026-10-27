import { Archive } from '@gravity-ui/icons';
import { Button } from '@heroui/react';
import { api, FileButton, navigate, Notice, Panel, SelectInput, useAction, t } from '@ui';
import { useState } from 'react';

type Format = 'auto' | 'markdown' | 'gitbook' | 'notion' | 'nicewiki';
type Result = { format: string; documents: number; images: number; warnings: string[]; failures: string[] };

const formatOptions: { value: Format; label: string; description: string }[] = [
  { value: 'auto', label: t('自动识别'), description: t('按压缩包里的文件判断是哪种格式') },
  { value: 'markdown', label: t('Markdown 目录'), description: t('一个文件夹的 .md 文件，子文件夹成为子文档') },
  { value: 'gitbook', label: 'GitBook', description: t('带 SUMMARY.md 的 GitBook 仓库，目录顺序照 SUMMARY.md') },
  { value: 'notion', label: 'Notion', description: t('Notion 的“导出为 Markdown & CSV”压缩包') },
  { value: 'nicewiki', label: t('本站导出包'), description: t('本站“导出与备份”里下载的压缩包，含图片和视频') },
];

const formatName: Record<string, string> = { markdown: 'Markdown', gitbook: 'GitBook', notion: 'Notion', nicewiki: t('本站导出包') };

export function ImportPanel() {
  const [format, setFormat] = useState<Format>('auto');
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<Result | null>(null);
  const { busy, run } = useAction();
  const start = async () => {
    if (!file) return;
    setResult(null);
    const bytes = await file.arrayBuffer();
    const done = await run(() => api<Result>(`/v1/site-import${format === 'auto' ? '' : `?format=${format}`}`, { method: 'POST', body: new Blob([bytes], { type: 'application/octet-stream' }) }));
    if (done) setResult(done);
  };
  return (
    <div className="flex flex-col gap-6">
      <Panel title={t('导入文档')} description={t('选一个 .zip 压缩包（16MB 以内）。导入的文档都是草稿，不会自动发布；检查没问题后在“文档”里发布。')}>
        <div className="flex flex-col gap-4">
          <SelectInput label={t('压缩包格式')} value={format} onChange={setFormat} options={formatOptions} />
          <div className="flex flex-wrap items-center gap-3">
            <FileButton accept=".zip,application/zip" onFile={setFile}><Archive />{file ? t('换一个文件') : t('选择压缩包')}</FileButton>
            {file && <span className="text-sm text-muted">{file.name}</span>}
            <Button isDisabled={!file} isPending={busy} onPress={() => void start()}>{t('开始导入')}</Button>
          </div>
          <Notice status="default" title={t('导入前请知道')}>
            {t('图片会一并导入到资源库；Markdown 里的 HTML 会被清洗成纯文字；站内链接按相对路径对应到导入的文档，找不到的链接会去掉。同一压缩包重复导入会产生重复文档。')}
          </Notice>
        </div>
      </Panel>
      {result && (
        <Panel title={t('导入结果')} description={t('格式：{0}', formatName[result.format] ?? result.format)}>
          <div className="flex flex-col gap-3">
            <Notice status={result.failures.length ? 'warning' : 'success'} title={t('已导入 {0} 篇文档、{1} 个文件', result.documents, result.images)}>
              <button className="text-accent underline" type="button" onClick={() => navigate('/admin/content')}>{t('去文档管理查看')}</button>
            </Notice>
            {result.failures.length > 0 && <Notice status="danger" title={t('有 {0} 项没有成功', result.failures.length)}><ul className="list-disc ps-5">{result.failures.map((line) => <li key={line}>{line}</li>)}</ul></Notice>}
            {result.warnings.length > 0 && <Notice status="warning" title={t('提示（{0}）', result.warnings.length)}><ul className="list-disc ps-5">{result.warnings.map((line) => <li key={line}>{line}</li>)}</ul></Notice>}
          </div>
        </Panel>
      )}
    </div>
  );
}

