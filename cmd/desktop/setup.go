package main

import (
	"fmt"
	"net/http"
)

// desktopSetupPath is served by the desktop asset handler in every mode so
// the user can (re)configure how the app connects from the menu at any time.
const desktopSetupPath = "/__zhishu_desktop_setup__"

// serveDesktopSetup renders the standalone setup page. It never proxies to a
// backend: all interaction goes through the bound App methods.
func serveDesktopSetup(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, desktopSetupPageHTML)
}

const desktopSetupPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>智述 Lite · 启动设置</title>
<style>
  :root { --green: #21a35a; --green-dark: #178a49; --text: #1f2d27; --muted: #6b7f74; --line: #dfe8e2; }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  html, body { height: 100%; }
  body {
    font-family: "Segoe UI", "Microsoft YaHei", system-ui, sans-serif;
    background: linear-gradient(150deg, #eef7f1 0%, #e4f1e9 40%, #dcebe2 100%);
    color: var(--text); display: flex; align-items: center; justify-content: center; overflow: auto;
  }
  .card { width: 560px; max-width: calc(100vw - 48px); background: rgba(255,255,255,.92); border: 1px solid var(--line);
          border-radius: 14px; box-shadow: 0 18px 50px rgba(24,64,43,.14); padding: 34px 38px 28px; }
  .logo { display: flex; align-items: center; gap: 12px; margin-bottom: 6px; }
  .logo-badge { width: 40px; height: 40px; border-radius: 10px; background: linear-gradient(135deg, var(--green), #3ecf7c);
                display: flex; align-items: center; justify-content: center; color: #fff; font-weight: 700; font-size: 20px; }
  h1 { font-size: 21px; font-weight: 600; }
  .sub { color: var(--muted); font-size: 13px; margin: 10px 0 22px; line-height: 1.6; }
  .modes { display: flex; gap: 14px; margin-bottom: 8px; }
  .mode { flex: 1; border: 1.5px solid var(--line); border-radius: 12px; padding: 16px 16px 14px; cursor: pointer;
          transition: all .15s ease; background: #fff; position: relative; }
  .mode:hover { border-color: #b7d9c4; }
  .mode.active { border-color: var(--green); box-shadow: 0 0 0 3px rgba(33,163,90,.12); }
  .mode .t { font-weight: 600; font-size: 15px; margin-bottom: 6px; display: flex; align-items: center; gap: 8px; }
  .mode .d { color: var(--muted); font-size: 12.5px; line-height: 1.65; }
  .tick { position: absolute; top: 12px; right: 12px; width: 18px; height: 18px; border-radius: 50%;
          background: var(--green); color: #fff; font-size: 12px; display: none; align-items: center; justify-content: center; }
  .mode.active .tick { display: flex; }
  #remote-panel { display: none; margin-top: 16px; }
  #remote-panel.show { display: block; }
  label { display: block; font-size: 13px; color: var(--muted); margin-bottom: 7px; }
  .row { display: flex; gap: 10px; }
  input[type=text] { flex: 1; height: 40px; border: 1.5px solid var(--line); border-radius: 9px; padding: 0 13px;
                     font-size: 14px; outline: none; color: var(--text); }
  input[type=text]:focus { border-color: var(--green); box-shadow: 0 0 0 3px rgba(33,163,90,.10); }
  button { height: 40px; border-radius: 9px; border: 1.5px solid var(--line); background: #fff; color: var(--text);
           font-size: 13.5px; padding: 0 15px; cursor: pointer; font-family: inherit; }
  button:hover { border-color: #b7d9c4; }
  button:disabled { opacity: .55; cursor: not-allowed; }
  #test-msg { min-height: 18px; font-size: 12.5px; margin-top: 8px; color: var(--muted); line-height: 1.5; }
  #test-msg.ok { color: var(--green-dark); }
  #test-msg.err { color: #c2410c; }
  .actions { display: flex; gap: 12px; margin-top: 24px; }
  .primary { flex: 1; height: 44px; border: none; border-radius: 10px; background: linear-gradient(135deg, var(--green), #2eb968);
             color: #fff; font-size: 15px; font-weight: 600; }
  .primary:hover { filter: brightness(1.05); }
  .ghost { display: none; }
  .ghost.show { display: block; }
  .hint { color: var(--muted); font-size: 12px; line-height: 1.7; margin-top: 16px; border-top: 1px dashed var(--line); padding-top: 12px; }
  .restarting { text-align: center; padding: 40px 0 30px; display: none; }
  .restarting .big { font-size: 40px; margin-bottom: 14px; }
  .restarting p { color: var(--muted); font-size: 13.5px; line-height: 1.8; }
</style>
</head>
<body>
<div class="card" id="main">
  <div class="logo"><div class="logo-badge">述</div><h1>智述 Lite · 启动设置</h1></div>
  <div class="sub">选择本应用的运行方式。设置会保存在本机，之后可随时在菜单「服务器设置…」中修改。</div>

  <div class="modes">
    <div class="mode" id="mode-local">
      <div class="tick">✓</div>
      <div class="t">🖥️ 本机模式</div>
      <div class="d">系统服务直接在本电脑内运行，数据保存在本机。适合个人使用，无需服务器。</div>
    </div>
    <div class="mode" id="mode-remote">
      <div class="tick">✓</div>
      <div class="t">🔗 连接服务器</div>
      <div class="d">连接局域网或远程电脑上已部署的智述系统，所有账号、知识库数据都在服务器上。</div>
    </div>
  </div>

  <div id="remote-panel">
    <label for="server-url">服务器地址（例如 http://192.168.1.100:8080）</label>
    <div class="row">
      <input type="text" id="server-url" placeholder="http://192.168.1.100:8080" spellcheck="false">
      <button id="btn-test">测试连接</button>
    </div>
    <div id="test-msg"></div>
  </div>

  <div class="actions">
    <button class="ghost" id="btn-cancel">取消</button>
    <button class="primary" id="btn-save" disabled>保存并启动</button>
  </div>

  <div class="hint">连接服务器前请确认：服务器上的智述系统已启动，且本电脑能访问其地址与端口（服务器需开放防火墙端口）。登录、注册、账号权限均由服务器统一管理。</div>
</div>

<div class="card restarting" id="done">
  <div class="big">✅</div>
  <p id="done-text">设置已保存，正在重新打开智述 Lite…</p>
</div>

<script>
(function(){
  var state = { mode: '', savedMode: '', savedURL: '' };
  var elLocal = document.getElementById('mode-local');
  var elRemote = document.getElementById('mode-remote');
  var panel = document.getElementById('remote-panel');
  var urlInput = document.getElementById('server-url');
  var testMsg = document.getElementById('test-msg');
  var btnTest = document.getElementById('btn-test');
  var btnSave = document.getElementById('btn-save');
  var btnCancel = document.getElementById('btn-cancel');
  // 双通道：wails:// 源上有 Wails 绑定；网关 HTTP 源上走 /__desktop__/rpc。
  function desktopCall(fn, args){
    if (window.go && window.go.main && window.go.main.App){
      return window.go.main.App[fn].apply(null, args);
    }
    return fetch('/__desktop__/rpc', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ fn: fn, args: args })
    }).then(function(r){ return r.json(); }).then(function(j){
      if (!j.ok) throw (j.error || 'rpc failed');
      return j.result;
    });
  }

  function selectMode(m){
    state.mode = m;
    elLocal.classList.toggle('active', m === 'local');
    elRemote.classList.toggle('active', m === 'remote');
    panel.classList.toggle('show', m === 'remote');
    refreshSave();
    if (m === 'remote') urlInput.focus();
  }
  function refreshSave(){
    var ok = state.mode === 'local' || (state.mode === 'remote' && urlInput.value.trim().length > 0 && testMsg.classList.contains('ok'));
    btnSave.disabled = !ok;
  }
  elLocal.addEventListener('click', function(){ selectMode('local'); });
  elRemote.addEventListener('click', function(){ selectMode('remote'); });
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
    var mode = state.mode, raw = urlInput.value.trim();
    btnSave.disabled = true;
    desktopCall('ApplyDesktopSetup', [mode, raw]).then(function(err){
      if (err) {
        alert(err);
        btnSave.disabled = false;
        return;
      }
      document.getElementById('main').style.display = 'none';
      document.getElementById('done').style.display = 'block';
      document.getElementById('done-text').textContent =
        mode === 'local' ? '设置已保存（本机模式），正在重新打开智述 Lite…'
                         : '设置已保存（连接服务器），正在重新打开智述 Lite…';
    }).catch(function(e){
      alert('保存失败：' + e);
      btnSave.disabled = false;
    });
  });

  btnCancel.addEventListener('click', function(){ history.back(); });

  desktopCall('GetDesktopSetupState', []).then(function(s){
    if (s && s.mode) {
      state.savedMode = s.mode; state.savedURL = s.serverURL || '';
      urlInput.value = state.savedURL;
      selectMode(s.mode);
      btnCancel.classList.add('show');
    }
  }).catch(function(){});
})();
</script>
</body>
</html>
`
