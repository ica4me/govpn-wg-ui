'use strict';
document.addEventListener('click', function(e) { if(e.target.closest('[data-menu]')) document.getElementById('sidebar').classList.toggle('open'); });
document.addEventListener('submit', function(e) { const f=e.target; if(f.dataset.confirm && !window.confirm(f.dataset.confirm)) { e.preventDefault(); return; } const b=f.querySelector('button[type=submit]'); if(b) { b.disabled=true; b.textContent='Working…'; } });
document.addEventListener('htmx:beforeSwap',function(e){if(e.detail.xhr.status===401){window.location='/login';}});
