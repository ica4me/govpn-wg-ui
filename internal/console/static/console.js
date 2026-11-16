'use strict';
document.addEventListener('click', function(e) {
 const trigger=e.target.closest('[popovertarget]');
 if(trigger){
  const menu=document.getElementById(trigger.getAttribute('popovertarget'));
  if(menu){const r=trigger.getBoundingClientRect();menu.style.left=Math.max(8,Math.min(r.right-240,window.innerWidth-248))+'px';menu.style.top=Math.max(8,Math.min(r.bottom+4,window.innerHeight-210))+'px';}
 }
 const item=e.target.closest('.action-popover a');
 if(item) item.closest('.action-popover').hidePopover();
});
window.addEventListener('resize',function(){document.querySelectorAll('.action-popover:popover-open').forEach(m=>m.hidePopover());});
document.addEventListener('click', function(e) { if(e.target.closest('[data-menu]')) document.getElementById('sidebar').classList.toggle('open'); });
document.addEventListener('submit', function(e) { const f=e.target; if(f.dataset.confirm && !window.confirm(f.dataset.confirm)) { e.preventDefault(); return; } const b=f.querySelector('button[type=submit]'); if(b) { b.disabled=true; b.textContent='Working…'; } });
document.addEventListener('htmx:beforeSwap',function(e){if(e.detail.xhr.status===401){window.location='/login';}});
