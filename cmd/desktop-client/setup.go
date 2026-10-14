package main

import (
	"fmt"
	"net/http"
)

// desktopSetupPath is served by both origins (asset server on wails:// and
// the gateway on 127.0.0.1) so the server address can be (re)configured from
// the menu at any time. The page auto-detects which transport is available:
// Wails bindings on the wails:// origin, /__desktop__/rpc on the gateway.
const clientSetupPath = "/__client_setup__"

func serveClientSetup(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, clientSetupPageHTML)
}

const clientSetupPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>智枢文渊 · 服务器设置</title>
<style>
  :root { --green: #21a35a; --green-dark: #178a49; --text: #1f2d27; --muted: #6b7f74; --line: #dfe8e2; }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  html, body { height: 100%; }
  body {
    font-family: "Segoe UI", "Microsoft YaHei", system-ui, sans-serif;
    background: linear-gradient(150deg, #eef7f1 0%, #e4f1e9 40%, #dcebe2 100%);
    color: var(--text); display: flex; align-items: center; justify-content: center; overflow: auto;
  }
  .card { width: 540px; max-width: calc(100vw - 48px); background: rgba(255,255,255,.94); border: 1px solid var(--line);
          border-radius: 14px; box-shadow: 0 18px 50px rgba(24,64,43,.14); padding: 34px 38px 28px; }
  .logo { display: flex; align-items: center; gap: 12px; margin-bottom: 6px; }
  .logo-badge { width: 40px; height: 40px; border-radius: 10px; background: linear-gradient(135deg, var(--green), #3ecf7c);
                display: flex; align-items: center; justify-content: center; color: #fff; font-weight: 700; font-size: 20px; }
  h1 { font-size: 21px; font-weight: 600; }
  .sub { color: var(--muted); font-size: 13px; margin: 12px 0 22px; line-height: 1.6; }
  label { display: block; font-size: 13px; color: var(--muted); margin-bottom: 7px; }
  .row { display: flex; gap: 10px; }
  input[type=text] { flex: 1; height: 42px; border: 1.5px solid var(--line); border-radius: 9px; padding: 0 13px;
                     font-size: 14px; outline: none; color: var(--text); }
  input[type=text]:focus { border-color: var(--green); box-shadow: 0 0 0 3px rgba(33,163,90,.10); }
  button { height: 42px; border-radius: 9px; border: 1.5px solid var(--line); background: #fff; color: var(--text);
           font-size: 13.5px; padding: 0 15px; cursor: pointer; font-family: inherit; }
  button:hover { border-color: #b7d9c4; }
  button:disabled { opacity: .55; cursor: not-allowed; }
  #test-msg { min-height: 18px; font-size: 12.5px; margin-top: 8px; color: var(--muted); line-height: 1.5; }
  #test-msg.ok { color: var(--green-dark); }
  #test-msg.err { color: #c2410c; }
  .actions { display: flex; gap: 12px; margin-top: 22px; }
  .primary { flex: 1; height: 44px; border: none; border-radius: 10px; background: linear-gradient(135deg, var(--green), #2eb968);
             color: #fff; font-size: 15px; font-weight: 600; }
  .primary:hover { filter: brightness(1.05); }
  .hint { color: var(--muted); font-size: 12px; line-height: 1.7; margin-top: 18px; border-top: 1px dashed var(--line); padding-top: 12px; }
  .restarting { text-align: center; padding: 46px 0 36px; display: none; }
  .restarting .big { font-size: 40px; margin-bottom: 14px; }
  .restarting p { color: var(--muted); font-size: 13.5px; line-height: 1.8; }
</style>
</head>
<body>
<div class="card" id="main">
  <div class="logo"><div class="logo-badge">枢</div><h1>智枢文渊客户端 · 服务器设置</h1></div>
  <div class="sub">填入智枢系统服务器的地址并保存，客户端将直接连接该服务器使用。账号、知识库与配置均保存在服务器上，与网页端完全一致。</div>

  <label for="server-url">服务器地址（平时浏览器访问系统使用的地址）</label>
  <div class="row">
    <input type="text" id="server-url" placeholder="http://192.168.1.100" spellcheck="false">
    <button id="btn-test">测试连接</button>
  </div>
  <div id="test-msg"></div>

  <div class="actions">
    <button class="primary" id="btn-save" disabled>保存并连接</button>
  </div>

  <div class="hint">连接前请确认：服务器上的智述系统已启动，且本电脑能访问该地址与端口（服务器需开放防火墙端口）。登录、注册、账号权限均由服务器统一管理。</div>
</div>

<div class="card restarting" id="done">
  <div class="big">✅</div>
  <p id="done-text">设置已保存，正在连接服务器…</p>
</div>

<script>
(function(){
  var urlInput = document.getElementById('server-url');
  var testMsg = document.getElementById('test-msg');
  var btnTest = document.getElementById('btn-test');
  var btnSave = document.getElementById('btn-save');
  var app = window.go && window.go.main ? window.go.main.App : null;

  function desktopCall(fn, args){
    if (app) return app[fn].apply(null, args);
    return fetch('/__desktop__/rpc', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ fn: fn, args: args })
    }).then(function(r){
      var ct = (r.headers.get('Content-Type') || '');
      if (ct.indexOf('application/json') === -1) {
        throw '桌面通道未就绪，请关闭窗口后重新打开应用';
      }
      return r.json();
    }).then(function(j){
      if (!j.ok) throw (j.error || 'rpc failed');
      return j.result;
    });
  }

  function refreshSave(){
    btnSave.disabled = !(urlInput.value.trim().length > 0 && testMsg.classList.contains('ok'));
  }
  urlInput.addEventListener('input', function(){
    testMsg.className = ''; testMsg.textContent = '';
    refreshSave();
  });
  urlInput.addEventListener('keydown', function(e){ if (e.key === 'Enter') btnTest.click(); });

  btnTest.addEventListener('click', function(){
    var raw = urlInput.value.trim();
    testMsg.className = ''; testMsg.textContent = '正在测试连接…';
    btnTest.disabled = true;
    desktopCall('TestServerURL', [raw]).then(function(msg){
      if (msg) { testMsg.className = 'err'; testMsg.textContent = '✗ ' + msg; }
      else {
        testMsg.className = 'ok';
        testMsg.textContent = '✓ 连接成功，该服务器上的智述系统可以访问。';
      }
      btnTest.disabled = false;
      refreshSave();
    }).catch(function(e){
      testMsg.className = 'err'; testMsg.textContent = '✗ ' + e;
      btnTest.disabled = false;
    });
  });

  btnSave.addEventListener('click', function(){
    var raw = urlInput.value.trim();
    btnSave.disabled = true;
    desktopCall('ApplyClientSetup', [raw]).then(function(err){
      if (err) { alert(err); btnSave.disabled = false; return; }
      document.getElementById('main').style.display = 'none';
      document.getElementById('done').style.display = 'block';
    }).catch(function(e){
      alert('保存失败：' + e);
      btnSave.disabled = false;
    });
  });

  if (!app && !(window.fetch)) {
    testMsg.className = 'err';
    testMsg.textContent = '桌面桥接未就绪，请关闭窗口后重新打开应用。';
    return;
  }
  desktopCall('GetClientSetupState', []).then(function(s){
    if (s && s.serverURL) {
      urlInput.value = s.serverURL;
      refreshSave();
    }
  }).catch(function(){});
})();
</script>
</body>
</html>
`
