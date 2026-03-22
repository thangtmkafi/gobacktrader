// gobacktrader docs — SPA router
// Pages are <div class="page"> with ids like "page-home", "page-getting-started", etc.
// Navigation uses hash-based routing: #home, #getting-started, #concepts, etc.

(function () {
  'use strict';

  const pages = document.querySelectorAll('.page');
  const navLinks = document.querySelectorAll('[data-page]');

  function navigate(pageName) {
    if (!pageName || pageName === '') pageName = 'home';

    // Hide all pages, show target
    pages.forEach(p => p.classList.remove('active'));
    const target = document.getElementById('page-' + pageName);
    if (target) {
      target.classList.add('active');
    } else {
      document.getElementById('page-home')?.classList.add('active');
    }

    // Update nav active state
    navLinks.forEach(link => {
      link.classList.toggle('active', link.getAttribute('data-page') === pageName);
    });

    // Scroll to top
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  // Handle link clicks
  document.addEventListener('click', function (e) {
    const link = e.target.closest('[data-page]');
    if (link) {
      e.preventDefault();
      const page = link.getAttribute('data-page');
      window.location.hash = page;
    }
  });

  // Handle hash changes
  window.addEventListener('hashchange', function () {
    const hash = window.location.hash.slice(1);
    navigate(hash);
  });

  // Initial load
  const initialHash = window.location.hash.slice(1);
  navigate(initialHash || 'home');

  // ─── Smooth scroll for in-page anchor links ───
  document.addEventListener('click', function (e) {
    const anchor = e.target.closest('a[href^="#"]');
    if (anchor && !anchor.hasAttribute('data-page')) {
      const targetId = anchor.getAttribute('href').slice(1);
      const el = document.getElementById(targetId);
      if (el) {
        e.preventDefault();
        el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }
  });

  // ─── Nav background on scroll ───
  const nav = document.querySelector('.nav');
  window.addEventListener('scroll', function () {
    if (window.scrollY > 20) {
      nav.style.borderBottomColor = 'rgba(30, 41, 59, 0.8)';
    } else {
      nav.style.borderBottomColor = 'rgba(30, 41, 59, 0.3)';
    }
  });

})();
