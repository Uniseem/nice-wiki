// 构建时按语言拆出的后台译文（见 i18n.ts）。
declare module 'virtual:admin-messages/*' {
  const messages: Record<string, string>;
  export default messages;
}
