package transport

import "net/http"

// reportPage 是公开页页脚“举报此站点”指向的独立小页面：不依赖前端框架，匿名可用，直接提交到 /v1/reports。
const reportPage = `<!doctype html>
<html lang="zh-Hans"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>举报 / Report</title>
<style>
:root{color-scheme:light dark;--bg:#fff;--fg:#171717;--muted:#6b7280;--line:#e5e7eb;--accent:#2563eb}
@media (prefers-color-scheme:dark){:root{--bg:#0b0b0c;--fg:#f4f4f5;--muted:#a1a1aa;--line:#27272a;--accent:#60a5fa}}
body{margin:0;background:var(--bg);color:var(--fg);font:16px/1.6 system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:560px;margin:0 auto;padding:32px 16px}
h1{font-size:22px;margin:0 0 8px}p{color:var(--muted);margin:0 0 20px}
label{display:block;margin:14px 0 4px;font-size:14px;font-weight:600}
input,select,textarea,button{width:100%;box-sizing:border-box;font:inherit;color:inherit;background:transparent;border:1px solid var(--line);border-radius:10px;padding:10px 12px}
textarea{min-height:120px;resize:vertical}
button{margin-top:20px;background:var(--accent);border-color:var(--accent);color:#fff;font-weight:600;cursor:pointer}
button:disabled{opacity:.6}
#msg{margin-top:16px;font-size:14px}
</style></head><body><main>
<h1>举报此站点 · Report this site</h1>
<p>违法、钓鱼、侵权、垃圾内容都可以在这里举报。平台会人工处理，不会向被举报方透露你的联系方式。<br>Report illegal, phishing, infringing or spam content. Your contact details are not shared with the site owner.</p>
<form id="f">
<label for="host">站点域名 · Site</label><input id="host" name="host" required maxlength="253">
<label for="url">页面地址（可选）· Page URL (optional)</label><input id="url" name="url" maxlength="1024" placeholder="https://">
<label for="reason">原因 · Reason</label>
<select id="reason" name="reason"><option value="spam">垃圾内容 · Spam</option><option value="phishing">钓鱼或欺诈 · Phishing or fraud</option><option value="illegal">违法内容 · Illegal content</option><option value="copyright">侵权 · Copyright infringement</option><option value="other">其他 · Other</option></select>
<label for="detail">说明 · Details</label><textarea id="detail" name="detail" maxlength="2000"></textarea>
<label for="contact">联系邮箱（可选）· Your email (optional)</label><input id="contact" name="contact" type="email" maxlength="320">
<button id="go" type="submit">提交 · Submit</button>
<div id="msg" role="status"></div>
</form>
<script>
const q=new URLSearchParams(location.search);
document.getElementById('host').value=q.get('host')||location.hostname;
if(q.get('url'))document.getElementById('url').value=q.get('url');
document.getElementById('f').addEventListener('submit',async(e)=>{
  e.preventDefault();
  const go=document.getElementById('go'),msg=document.getElementById('msg');
  go.disabled=true;msg.textContent='';
  const body={host:host.value.trim().toLowerCase(),reason:reason.value};
  if(url.value.trim())body.url=url.value.trim();
  if(detail.value.trim())body.detail=detail.value.trim();
  if(contact.value.trim())body.contact=contact.value.trim();
  const key=Array.from(crypto.getRandomValues(new Uint8Array(16)),b=>b.toString(16).padStart(2,'0')).join('');
  try{
    const r=await fetch('/v1/reports',{method:'POST',headers:{'Content-Type':'application/json','Idempotency-Key':key},body:JSON.stringify(body)});
    if(r.ok){msg.textContent='已收到，谢谢。Received, thank you.';e.target.reset();document.getElementById('host').value=q.get('host')||location.hostname;}
    else if(r.status===429){msg.textContent='提交太频繁，请稍后再试。Too many requests, try again later.';}
    else{msg.textContent='提交失败，请检查填写内容。Could not submit, please check the form.';}
  }catch{msg.textContent='网络错误。Network error.';}
  go.disabled=false;
});
</script></main></body></html>`

func mountReportPage(mux *http.ServeMux) {
	mux.HandleFunc("GET /_report", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		_, _ = w.Write([]byte(reportPage))
	})
}
