import { loadAdminLanguage } from '@ui/i18n';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './styles.css';

// 先载入界面语言的译文，再加载后台：各模块里预先算好的文字和格式器都要用到它。
void loadAdminLanguage().then(async () => {
  const { App } = await import('./App');
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
});
