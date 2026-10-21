// 人机验证（工作量证明）：在 Web Worker 里找一个后缀，使 sha256(nonce + ":" + 后缀) 前若干位为 0。
// 同步实现的 SHA-256 比 crypto.subtle 逐次 await 快得多；不阻塞页面主线程。
const workerSource = 'function H(m){var K=[1116352408,1899447441,3049323471,3921009573,961987163,1508970993,2453635748,2870763221,3624381080,310598401,607225278,1426881987,1925078388,2162078206,2614888103,3248222580,3835390401,4022224774,264347078,604807628,770255983,1249150122,1555081692,1996064986,2554220882,2821834349,2952996808,3210313671,3336571891,3584528711,113926993,338241895,666307205,773529912,1294757372,1396182291,1695183700,1986661051,2177026350,2456956037,2730485921,2820302411,3259730800,3345764771,3516065817,3600352804,4094571909,275423344,430227734,506948616,659060556,883997877,958139571,1322822218,1537002063,1747873779,1955562222,2024104815,2227730452,2361852424,2428436474,2756734187,3204031479,3329325298];var h=[1779033703,3144134277,1013904242,2773480762,1359893119,2600822924,528734635,1541459225];var l=m.length,n=((l+9+63)>>6)<<6,b=new Uint8Array(n);b.set(m);b[l]=128;var dv=new DataView(b.buffer);dv.setUint32(n-4,l*8>>>0);dv.setUint32(n-8,Math.floor(l*8/4294967296));var w=new Uint32Array(64);for(var o=0;o<n;o+=64){for(var i=0;i<16;i++)w[i]=dv.getUint32(o+i*4);for(i=16;i<64;i++){var a=w[i-15],c=w[i-2];w[i]=(w[i-16]+(((a>>>7)|(a<<25))^((a>>>18)|(a<<14))^(a>>>3))+w[i-7]+(((c>>>17)|(c<<15))^((c>>>19)|(c<<13))^(c>>>10)))>>>0}var A=h[0],B=h[1],C=h[2],D=h[3],E=h[4],F=h[5],G=h[6],Hh=h[7];for(i=0;i<64;i++){var t1=(Hh+(((E>>>6)|(E<<26))^((E>>>11)|(E<<21))^((E>>>25)|(E<<7)))+((E&F)^(~E&G))+K[i]+w[i])>>>0,t2=((((A>>>2)|(A<<30))^((A>>>13)|(A<<19))^((A>>>22)|(A<<10)))+((A&B)^(A&C)^(B&C)))>>>0;Hh=G;G=F;F=E;E=(D+t1)>>>0;D=C;C=B;B=A;A=(t1+t2)>>>0}h[0]=(h[0]+A)>>>0;h[1]=(h[1]+B)>>>0;h[2]=(h[2]+C)>>>0;h[3]=(h[3]+D)>>>0;h[4]=(h[4]+E)>>>0;h[5]=(h[5]+F)>>>0;h[6]=(h[6]+G)>>>0;h[7]=(h[7]+Hh)>>>0}return h}function ok(h,bits){var i=0;while(bits>=32){if(h[i++]!==0)return false;bits-=32}return bits===0||(h[i]>>>(32-bits))===0}var al="0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";onmessage=function(e){var nonce=e.data.nonce,bits=e.data.bits,enc=new TextEncoder();for(var len=1;len<=6;len++){var idx=new Array(len).fill(0);for(;;){var s="";for(var j=0;j<len;j++)s+=al[idx[j]];if(ok(H(enc.encode(nonce+":"+s)),bits)){postMessage(s);return}var p=len-1;while(p>=0&&++idx[p]===al.length){idx[p]=0;p--}if(p<0)break}}postMessage(null)}';

export function solveChallenge(nonce: string, bits: number): Promise<string> {
  return new Promise((resolve, reject) => {
    const worker = new Worker(URL.createObjectURL(new Blob([workerSource], { type: 'text/javascript' })));
    worker.onmessage = (event: MessageEvent<string | null>) => {
      worker.terminate();
      if (event.data) resolve(event.data);
      else reject(new Error('challenge-failed'));
    };
    worker.onerror = () => { worker.terminate(); reject(new Error('challenge-unsupported')); };
    worker.postMessage({ nonce, bits });
  });
}
