package publicsite

// qaIslandScript 是文档页右下角的问答面板：只用 textContent 渲染，回答与引用来自同源接口，脚本失败不影响阅读。
const qaIslandScript = `(function(){
var root=document.querySelector('.ai-island');if(!root)return;
var css=document.createElement('style');
css.textContent='.ai-island{position:fixed;right:1.2rem;bottom:1.2rem;z-index:50;font:14px/1.5 system-ui}.ai-fab{border:0;border-radius:999px;padding:.7rem 1.1rem;background:#2563eb;color:#fff;cursor:pointer;box-shadow:0 4px 14px #0004}.ai-panel{display:none;flex-direction:column;width:min(24rem,calc(100vw - 2rem));height:min(32rem,70vh);background:Canvas;color:CanvasText;border:1px solid #8886;border-radius:12px;box-shadow:0 8px 30px #0005;overflow:hidden}.ai-island[data-open] .ai-panel{display:flex}.ai-island[data-open] .ai-fab{display:none}.ai-head{display:flex;gap:.5rem;align-items:center;padding:.6rem .8rem;border-bottom:1px solid #8885}.ai-head b{margin-right:auto}.ai-head button{border:0;background:none;color:inherit;cursor:pointer}.ai-log{flex:1;overflow:auto;padding:.8rem;display:flex;flex-direction:column;gap:.6rem}.ai-msg{white-space:pre-wrap;overflow-wrap:anywhere;padding:.5rem .7rem;border-radius:8px;max-width:92%}.ai-q{align-self:flex-end;background:#2563eb;color:#fff}.ai-a{align-self:flex-start;background:#8882}.ai-err{align-self:flex-start;color:#dc2626}.ai-src{font-size:12px;margin-top:.4rem}.ai-src a{display:block}.ai-form{display:flex;gap:.4rem;padding:.6rem;border-top:1px solid #8885}.ai-form input{flex:1;min-width:0;padding:.5rem;border:1px solid #8886;border-radius:6px;background:Field;color:FieldText}.ai-form button{border:0;border-radius:6px;padding:.5rem .8rem;background:#2563eb;color:#fff;cursor:pointer}.ai-form button:disabled{opacity:.5}';
document.head.appendChild(css);
root.innerHTML='<button class="ai-fab" type="button">AI 问答</button><div class="ai-panel"><div class="ai-head"><b>AI 问答</b><button type="button" data-new>新对话</button><button type="button" data-close aria-label="关闭">×</button></div><div class="ai-log"></div><form class="ai-form"><input type="text" maxlength="500" placeholder="问问这个站点的文档…" required><button>发送</button></form></div>';
var log=root.querySelector('.ai-log'),form=root.querySelector('form'),input=form.querySelector('input'),send=form.querySelector('button');
var store=window.sessionStorage;var convKey='nw-qa-conv';
function key(){var a=new Uint8Array(16);crypto.getRandomValues(a);return Array.prototype.map.call(a,function(b){return('0'+b.toString(16)).slice(-2)}).join('')}
function add(cls,text){var d=document.createElement('div');d.className='ai-msg '+cls;d.textContent=text;log.appendChild(d);log.scrollTop=log.scrollHeight;return d}
function api(method,path,body){var k=body&&body.idempotencyKey;var h={'Content-Type':'application/json'};if(method!=='GET'&&k)h['Idempotency-Key']=k;
return fetch(path,{method:method,headers:h,body:body?JSON.stringify(body):undefined}).then(function(r){return r.text().then(function(t){var j;try{j=JSON.parse(t)}catch(e){j={}}if(!r.ok)throw new Error(j.detail||('请求失败（'+r.status+'）'));return j})})}
function conversation(){var id=store.getItem(convKey);if(id)return Promise.resolve(id);
return api('POST','/v1/conversations',{language:document.documentElement.lang||'zh-Hans',idempotencyKey:key()}).then(function(r){store.setItem(convKey,r.conversation.conversationId);return r.conversation.conversationId})}
function follow(turn,box){var last=0,sources=[],text='';
function run(){return fetch(turn.streamPath+'?afterSequence='+last+'&limit=100').then(function(r){if(!r.ok||!r.body)throw new Error('回答连接失败（'+r.status+'）');var rd=r.body.getReader(),dec=new TextDecoder(),buf='',ended=false;
function pump(){return rd.read().then(function(x){if(x.done)return ended;buf+=dec.decode(x.value,{stream:true});var parts=buf.split('\n\n');buf=parts.pop();parts.forEach(function(p){var ev='',data='',id='';p.split('\n').forEach(function(l){if(l.indexOf('event:')===0)ev=l.slice(6).trim();else if(l.indexOf('data:')===0)data+=l.slice(5).trim();else if(l.indexOf('id:')===0)id=l.slice(3).trim()});if(id)last=parseInt(id,10)||last;if(!data)return;var j;try{j=JSON.parse(data)}catch(e){return}
if(ev==='delta'&&j.delta){text+=j.delta;box.textContent=text;log.scrollTop=log.scrollHeight}
else if(ev==='citation'&&j.citation){sources.push(j.citation)}
else if(ev==='failed'){box.className='ai-msg ai-err';box.textContent='这次没有回答成功（'+(j.code||'错误')+'），请稍后再试。';ended=true}
else if(ev==='cancelled'){ended=true}
else if(ev==='completed'){ended=true}});return pump()})}
return pump()}).then(function(ended){if(!ended)return run();showSources(box,sources)})}
return run()}
function showSources(box,sources){if(!sources.length)return;var seen={},list=document.createElement('div');list.className='ai-src';sources.forEach(function(c){var k=(c.href||'').split('#')[0];if(!k||seen[k])return;seen[k]=1;var a=document.createElement('a');a.href=c.href;a.textContent='来源：'+(c.title||c.href);list.appendChild(a)});box.appendChild(list)}
form.addEventListener('submit',function(e){e.preventDefault();var q=input.value.trim();if(!q)return;input.value='';send.disabled=true;add('ai-q',q);var box=add('ai-a','…');
conversation().then(function(id){return api('POST','/v1/turns',{conversationId:id,question:q,idempotencyKey:key()})}).then(function(r){box.textContent='';return follow(r,box)}).catch(function(err){box.className='ai-msg ai-err';box.textContent=err.message}).then(function(){send.disabled=false;input.focus()})});
root.querySelector('.ai-fab').onclick=function(){root.setAttribute('data-open','');input.focus()};
document.querySelectorAll('[data-ask]').forEach(function(a){a.addEventListener('click',function(e){e.preventDefault();root.setAttribute('data-open','');input.focus()})});
root.querySelector('[data-close]').onclick=function(){root.removeAttribute('data-open')};
root.querySelector('[data-new]').onclick=function(){store.removeItem(convKey);log.textContent=''};
})();`
