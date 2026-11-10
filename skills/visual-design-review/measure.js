// Visual design review, step 4: measure what can be measured on the rendered page.
// Run it inside the page (Playwright page.evaluate, the CDP Runtime.evaluate, or the browser console)
// and read the JSON it returns. It needs no library and changes nothing on the page.
(() => {
  const rgba = (s) => {
    const m = /rgba?\(([^)]+)\)/.exec(s || '');
    if (!m) return null;
    const [r, g, b, a = 1] = m[1].split(/[ ,/]+/).filter(Boolean).map(Number);
    return { r, g, b, a };
  };
  const lum = ({ r, g, b }) => {
    const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; };
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
  };
  const over = (top, under) => ({
    r: top.r * top.a + under.r * (1 - top.a),
    g: top.g * top.a + under.g * (1 - top.a),
    b: top.b * top.a + under.b * (1 - top.a),
    a: 1,
  });
  // The background a text element sits on: its own and its ancestors' colours, blended from the page down.
  const ground = (el) => {
    const layers = [];
    for (let e = el; e; e = e.parentElement) {
      const c = rgba(getComputedStyle(e).backgroundColor);
      if (c && c.a > 0) { layers.push(c); if (c.a >= 1) break; }
    }
    let base = { r: 255, g: 255, b: 255, a: 1 };
    for (let i = layers.length - 1; i >= 0; i--) base = over(layers[i], base);
    return base;
  };
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const name = (el) => el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (el.classList.length ? '.' + [...el.classList].slice(0, 2).join('.') : '');
  const seen = (el) => { const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && +s.opacity > 0.05; };

  const phone = innerWidth < 500;
  const lowContrast = [], tiny = [], monoProse = [], families = new Map(), sizes = new Map();
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  const done = new Set();
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const el = n.parentElement;
    const text = n.textContent.replace(/\s+/g, ' ').trim();
    if (!el || !text || done.has(el) || !seen(el)) continue;
    done.add(el);
    const s = getComputedStyle(el);
    const px = parseFloat(s.fontSize), bold = +s.fontWeight >= 700;
    const fam = s.fontFamily.split(',')[0].replace(/["']/g, '').trim();
    families.set(fam, (families.get(fam) || 0) + 1);
    sizes.set(px, (sizes.get(px) || 0) + 1);
    const fg = rgba(s.color); if (!fg) continue;
    const bg = ground(el);
    const cr = ratio(over(fg, bg), bg);
    const large = px >= 24 || (bold && px >= 18.66);
    const need = large ? 3 : 4.5;
    const off = el.closest('[disabled],[aria-disabled="true"]');
    if (cr < need && !off) lowContrast.push({ el: name(el), text: text.slice(0, 60), ratio: +cr.toFixed(2), need, px });
    if (px < 11) tiny.push({ el: name(el), text: text.slice(0, 40), px });
    if (/mono|courier|consol|menlo/i.test(s.fontFamily) && text.length > 60 && text.split(' ').length > 9) monoProse.push({ el: name(el), text: text.slice(0, 60) });
  }
  const smallTargets = [];
  if (phone) {
    for (const el of document.querySelectorAll('a[href], button, [role=button], input, select, textarea, summary')) {
      if (!seen(el)) continue;
      const r = el.getBoundingClientRect();
      if (r.width < 44 || r.height < 44) smallTargets.push({ el: name(el), text: (el.textContent || el.value || '').trim().slice(0, 30), w: Math.round(r.width), h: Math.round(r.height) });
    }
  }
  // Boxes that clip or scroll their content sideways. A page-level sideways-scroll check misses these:
  // a table in its own scroll box, or a label cut by overflow hidden, reads as cut off on a screenshot.
  const clipped = [];
  for (const el of document.querySelectorAll('body *')) {
    if (el.scrollWidth <= el.clientWidth + 1 || !seen(el)) continue;
    const ox = getComputedStyle(el).overflowX;
    if (ox === 'visible') continue;
    clipped.push({ el: name(el), overflowX: ox, scrollWidth: el.scrollWidth, clientWidth: el.clientWidth, text: (el.textContent || '').trim().slice(0, 40) });
  }
  const de = document.documentElement;
  return JSON.stringify({
    width: innerWidth,
    sidewaysScroll: de.scrollWidth > innerWidth || document.body.scrollWidth > innerWidth,
    lowContrast: lowContrast.sort((a, b) => a.ratio - b.ratio).slice(0, 15), lowContrastCount: lowContrast.length,
    tinyText: tiny.slice(0, 10), tinyTextCount: tiny.length,
    monoProse: monoProse.slice(0, 5),
    families: [...families].sort((a, b) => b[1] - a[1]),
    fontSizes: [...sizes].sort((a, b) => a[0] - b[0]),
    smallTapTargets: smallTargets.slice(0, 15), smallTapTargetCount: smallTargets.length,
    clippedBoxes: clipped.slice(0, 15), clippedBoxCount: clipped.length,
  });
})();
