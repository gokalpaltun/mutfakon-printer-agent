package main

// The agent's own tiny page (http://127.0.0.1:9187): lists printers with a test-print button, so setup
// on the kasa needs no typing.
const testPageHTML = `<!doctype html>
<html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Mutfak On Yazıcı Yardımcısı</title>
<style>
body{font-family:system-ui,sans-serif;margin:0;padding:24px;background:#f5f5f0;color:#1a1a1a}
h1{font-size:22px;margin:0 0 4px}.sub{color:#666;margin:0 0 20px}
.card{background:#fff;border-radius:14px;padding:16px 18px;margin-bottom:12px;display:flex;align-items:center;gap:14px}
.card b{font-size:18px}.card small{color:#666;display:block}.grow{flex:1}
button{font-size:16px;font-weight:700;border:0;border-radius:999px;padding:12px 22px;background:#1a7f4b;color:#fff;cursor:pointer}
button:disabled{opacity:.5}.msg{font-weight:600;margin-left:8px}.ok{color:#1a7f4b}.err{color:#c0392b}
</style></head><body>
<h1>Mutfak On Yazıcı Yardımcısı</h1>
<p class="sub">Sürüm <span id="v"></span> · çalışıyor</p>
<div id="list">Yazıcılar yükleniyor…</div>
<script>
const list = document.getElementById('list');
fetch('/health').then(r => r.json()).then(h => { document.getElementById('v').textContent = h.version; });
fetch('/printers').then(r => r.json()).then(d => {
  if (!d.printers || !d.printers.length) { list.textContent = 'Yazıcı bulunamadı.'; return; }
  list.innerHTML = '';
  d.printers.forEach(p => {
    const card = document.createElement('div'); card.className = 'card';
    const info = document.createElement('div'); info.className = 'grow';
    const name = document.createElement('b'); name.textContent = p.name;
    const meta = document.createElement('small'); meta.textContent = [p.port, p.driver, p.isDefault ? 'varsayılan' : ''].filter(Boolean).join(' · ');
    info.append(name, meta);
    const msg = document.createElement('span'); msg.className = 'msg';
    const mk = (label, path) => {
    const btn = document.createElement('button'); btn.textContent = label;
    btn.onclick = async () => {
      btn.disabled = true; msg.textContent = ''; msg.className = 'msg';
      try {
        const r = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ printer: p.name }) });
        const j = await r.json();
        if (j.ok) { msg.textContent = 'Gönderildi'; msg.className = 'msg ok'; } else { msg.textContent = (j.error || 'Hata') + ': ' + (j.message || ''); msg.className = 'msg err'; }
      } catch (e) { msg.textContent = 'Ulaşılamadı'; msg.className = 'msg err'; }
      btn.disabled = false;
    };
    return btn; };
    card.append(info, msg, mk('Test fişi bas', '/test-print'), mk('Zor test fişi', '/test-print-hard')); list.append(card);
  });
}).catch(() => { list.textContent = 'Yazıcılar okunamadı.'; });
</script></body></html>`
