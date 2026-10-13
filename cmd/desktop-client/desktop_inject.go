package main

// desktopShellJS is injected into every text/html response served by the
// desktop gateway (see gateway.go) on Windows. It:
//   - opens external http(s) links in the system browser instead of the
//     webview (plain clicks and window.open),
//   - routes <a download href="blob:..."> clicks through the gateway's
//     /__desktop__/save-blob endpoint so files land where the user chooses
//     via a native "Save as" dialog, with a completion toast offering
//     "open file" / "show in folder",
//   - suppresses overscroll bounce.
//
// It must not rely on Wails bindings: the app pages live on the gateway's
// real HTTP origin, where window.go/window.runtime do not exist. Native
// helpers are reached through same-origin /__desktop__/* endpoints.
//
// Large payloads (over maxBridgePayloadBytes, declared in app.go) fall back
// to the webview's default download behaviour.
const desktopShellJS = `(function(){
if(window.__zhishuDesktopShell)return;
window.__zhishuDesktopShell=true;

document.documentElement.style.overscrollBehavior='none';
if(document.body){document.body.style.overscrollBehavior='none';}

// ── external links → system browser ──
function openExternal(url){
  try{ fetch('/__desktop__/open-external?url='+encodeURIComponent(url), {cache:'no-store'}); }catch(e){}
}
document.addEventListener('click',function(e){
  var el=e.target;
  while(el&&el.tagName!=='A')el=el.parentElement;
  if(!el||!el.href)return;
  var href=el.href;
  if(href.indexOf('http://')===0||href.indexOf('https://')===0){
    e.preventDefault();
    e.stopPropagation();
    openExternal(href);
  }
},true);

var origOpen=window.open;
window.open=function(url){
  if(url&&(typeof url==='string')&&(url.indexOf('http://')===0||url.indexOf('https://')===0)){
    openExternal(url);
    return null;
  }
  return origOpen.apply(window,arguments);
};

// ── download interception → native save dialog via gateway ──
var MAX_BYTES=48*1024*1024;
function toB64(buf){
  var u=new Uint8Array(buf),s='',CH=0x8000;
  for(var i=0;i<u.length;i+=CH){s+=String.fromCharCode.apply(null,u.subarray(i,i+CH));}
  return btoa(s);
}
function saveBlob(anchor,fallback){
  var name=anchor.getAttribute('download')||'download';
  fetch(anchor.getAttribute('href'))
    .then(function(r){return r.arrayBuffer();})
    .then(function(buf){
      if(buf.byteLength>MAX_BYTES){fallback();return;}
      return fetch('/__desktop__/save-blob',{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({name:name,dataB64:toB64(buf)})
      }).then(function(r){return r.json();}).then(function(j){
        if(j&&j.ok&&j.path){showSavedToast(name,j.path);}else{fallback();}
      });
    })
    .catch(function(){fallback();});
}

document.addEventListener('click',function(e){
  var el=e.target;
  if(!el||!el.closest)return;
  var anchor=el.closest('a[download]');
  if(!anchor)return;
  if(anchor.dataset&&anchor.dataset.zhishuFallback){delete anchor.dataset.zhishuFallback;return;}
  var href=anchor.getAttribute('href')||'';
  var name=anchor.getAttribute('download')||'';
  if(!name||href.indexOf('blob:')!==0)return;
  e.preventDefault();
  e.stopPropagation();
  saveBlob(anchor,function(){
    anchor.dataset.zhishuFallback='1';
    anchor.click();
  });
},true);

// ── saved-file toast ──
function ensureToastHost(){
  var host=document.getElementById('__zhishu_toast_host');
  if(!host){
    host=document.createElement('div');
    host.id='__zhishu_toast_host';
    host.style.cssText='position:fixed;right:18px;bottom:18px;z-index:2147483647;display:flex;flex-direction:column;gap:10px;pointer-events:none;';
    (document.body||document.documentElement).appendChild(host);
  }
  return host;
}
function showSavedToast(name,path){
  try{
    var host=ensureToastHost();
    var card=document.createElement('div');
    card.style.cssText='pointer-events:auto;min-width:260px;max-width:360px;background:#ffffff;border:1px solid #dfe8e2;'+
      'border-left:4px solid #21a35a;border-radius:10px;box-shadow:0 8px 28px rgba(24,64,43,.18);padding:12px 14px;'+
      'font-family:"Segoe UI","Microsoft YaHei",system-ui,sans-serif;color:#1f2d27;';
    var title=document.createElement('div');
    title.textContent='已保存：'+name;
    title.style.cssText='font-size:13px;font-weight:600;margin-bottom:2px;word-break:break-all;';
    var sub=document.createElement('div');
    sub.textContent=path;
    sub.style.cssText='font-size:11.5px;color:#6b7f74;word-break:break-all;margin-bottom:9px;';
    var btns=document.createElement('div');
    btns.style.cssText='display:flex;gap:8px;';
    function mkBtn(label,endpoint){
      var b=document.createElement('button');
      b.textContent=label;
      b.style.cssText='flex:1;height:28px;border-radius:6px;border:1px solid #dfe8e2;background:#fff;color:#1f2d27;'+
        'font-size:12px;cursor:pointer;';
      b.onmouseenter=function(){b.style.borderColor='#21a35a';b.style.color='#178a49';};
      b.onmouseleave=function(){b.style.borderColor='#dfe8e2';b.style.color='#1f2d27';};
      b.onclick=function(){
        fetch(endpoint+'?path='+encodeURIComponent(path),{cache:'no-store'});
        card.remove();
      };
      return b;
    }
    btns.appendChild(mkBtn('打开文件','/__desktop__/open'));
    btns.appendChild(mkBtn('打开所在文件夹','/__desktop__/reveal'));
    var close=document.createElement('div');
    close.textContent='×';
    close.style.cssText='position:absolute;top:4px;right:8px;color:#9ab0a4;cursor:pointer;font-size:14px;';
    close.onclick=function(){card.remove();};
    card.style.position='relative';
    card.appendChild(title);card.appendChild(sub);card.appendChild(btns);card.appendChild(close);
    host.appendChild(card);
    setTimeout(function(){card.remove();},15000);
  }catch(e){}
}
})();`
