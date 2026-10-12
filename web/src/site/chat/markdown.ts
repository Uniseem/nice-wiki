const escape=(value:string)=>value.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;')
export function renderAnswer(value:string):string{return escape(value).replace(/\[块\s+([A-Za-z0-9_-]+)\]/g,'<a class="nw-citation" href="#b-$1">[$1]</a>').replaceAll('\n','<br>')}
