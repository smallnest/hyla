// Small, dependency-free behaviour for the Hyla site.
(() => {
  const body = document.body;
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)');

  // Reveal as the page scrolls. Without JS, everything stays visible.
  const reveal = () => {
    const items = [...document.querySelectorAll('[data-reveal]')];
    if (!items.length) return;
    if (!('IntersectionObserver' in window) || reduce.matches) {
      items.forEach((el) => el.classList.add('is-in'));
      return;
    }
    body.classList.add('reveal-ready');
    const io = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          entry.target.classList.add('is-in');
          io.unobserve(entry.target);
        }
      },
      { rootMargin: '0px 0px -8% 0px', threshold: 0.08 }
    );
    items.forEach((el) => io.observe(el));
  };

  // The menu on a phone.
  const menu = () => {
    const button = document.querySelector('.nav-toggle');
    const nav = document.getElementById('nav');
    if (!button || !nav) return;
    const close = () => {
      nav.classList.remove('is-open');
      button.setAttribute('aria-expanded', 'false');
      button.setAttribute('aria-label', '打开菜单');
    };
    button.addEventListener('click', () => {
      const open = !nav.classList.contains('is-open');
      nav.classList.toggle('is-open', open);
      button.setAttribute('aria-expanded', String(open));
      button.setAttribute('aria-label', open ? '关闭菜单' : '打开菜单');
    });
    nav.addEventListener('click', (event) => {
      if (event.target.closest('a')) close();
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') close();
    });
    document.addEventListener('pointerdown', (event) => {
      if (!nav.classList.contains('is-open')) return;
      if (nav.contains(event.target) || button.contains(event.target)) return;
      close();
    });
    window.addEventListener('resize', () => {
      if (window.innerWidth > 820) close();
    });
  };

  // Copy the commands, with a check for a moment.
  const copy = () => {
    const buttons = [...document.querySelectorAll('.copy[data-copy]')];
    for (const button of buttons) {
      const original = button.innerHTML;
      button.addEventListener('click', async () => {
        const text = button.dataset.copy || '';
        try {
          await navigator.clipboard.writeText(text);
        } catch {
          const area = document.createElement('textarea');
          area.value = text;
          area.style.position = 'fixed';
          area.style.opacity = '0';
          document.body.appendChild(area);
          area.select();
          try {
            document.execCommand('copy');
          } catch {
            /* the command is in the page either way */
          }
          area.remove();
        }
        button.classList.add('is-copied');
        button.innerHTML = '<svg class="icon" aria-hidden="true"><use href="#i-check"/></svg>';
        button.setAttribute('aria-label', '已复制');
        window.setTimeout(() => {
          button.classList.remove('is-copied');
          button.innerHTML = original;
          button.setAttribute('aria-label', '复制命令');
        }, 1600);
      });
    }
  };

  // The frog hops when pressed.
  const frog = () => {
    const el = document.querySelector('.frog');
    if (!el) return;
    el.addEventListener('click', () => {
      el.classList.remove('is-hopping');
      void el.offsetWidth;
      el.classList.add('is-hopping');
    });
    el.addEventListener('animationend', () => el.classList.remove('is-hopping'));
  };

  // A quiet way back up.
  const top = () => {
    const button = document.querySelector('.to-top');
    if (!button) return;
    const onScroll = () => {
      button.hidden = window.scrollY < 900;
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    button.addEventListener('click', () => {
      window.scrollTo({ top: 0, behavior: reduce.matches ? 'auto' : 'smooth' });
    });
  };

  reveal();
  menu();
  copy();
  frog();
  top();
})();
