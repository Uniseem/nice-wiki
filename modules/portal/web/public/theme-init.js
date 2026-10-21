// 首屏前套上明暗主题（与 HeroUI useTheme 同一个 localStorage 键），避免先白后黑的闪烁。
(function () {
  try {
    var theme = localStorage.getItem('heroui-theme') || 'system';
    if (theme === 'system') theme = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    document.documentElement.classList.add(theme);
    document.documentElement.setAttribute('data-theme', theme);
  } catch (e) { /* 存储不可用时跟随默认浅色 */ }
})();
