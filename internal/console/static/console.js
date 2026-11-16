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
// Refresh feedback: spin the icon, show a top progress bar, fade content in.
(function(){
 function bar(){let b=document.getElementById('page-progress');if(!b){b=document.createElement('div');b.id='page-progress';b.setAttribute('aria-hidden','true');document.body.appendChild(b);}return b;}
 function start(){const b=bar();b.classList.remove('done');void b.offsetWidth;b.classList.add('run');}
 function stop(){const b=document.getElementById('page-progress');if(b){b.classList.add('done');b.classList.remove('run');}}
 document.addEventListener('click',function(e){
  const a=e.target.closest('[data-refresh]');
  if(!a||e.metaKey||e.ctrlKey||e.shiftKey||e.button!==0)return;
  if(a.classList.contains('is-refreshing')){e.preventDefault();return;}
  a.classList.add('is-refreshing');a.setAttribute('aria-busy','true');
  const l=a.querySelector('.refresh-label');if(l)l.textContent='Refreshing…';
  start();
 });
 document.addEventListener('htmx:beforeRequest',function(e){
  if(!e.detail.elt||e.detail.elt.id!=='live-status')return;
  const d=document.querySelector('#live-status .status-dot');if(d)d.classList.add('pulsing');
  start();
 });
 document.addEventListener('htmx:afterSettle',function(){stop();const c=document.getElementById('content');if(c){c.classList.remove('fade-in');void c.offsetWidth;c.classList.add('fade-in');}});
 document.addEventListener('htmx:responseError',stop);
 window.addEventListener('pageshow',function(){
  stop();
  document.querySelectorAll('[data-refresh].is-refreshing').forEach(function(a){a.classList.remove('is-refreshing');a.removeAttribute('aria-busy');const l=a.querySelector('.refresh-label');if(l)l.textContent='Refresh';});
  const c=document.getElementById('content');if(c)c.classList.add('fade-in');
 });
})();
