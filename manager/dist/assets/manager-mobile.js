/* Adapted from Evolution Go PR #184 (douglasanpa), with owned lifecycle/focus. */
(() => {
  "use strict";
  const root = document.getElementById("root");
  if (!root) return;
  const mobile = window.matchMedia("(max-width: 767px)");
  let current = null;

  function mount(sidebar, header) {
    const controller = new AbortController();
    const listen = (target, event, fn) => target.addEventListener(event, fn, { signal: controller.signal });
    const saved = new Map(["id", "role", "aria-modal", "aria-label", "aria-hidden"].map(name => [name, sidebar.getAttribute(name)]));
    const oldInert = sidebar.inert;
    sidebar.id = "evo-mobile-sidebar";
    sidebar.dataset.evoMobileSidebar = "";

    const toggle = document.createElement("button");
    toggle.type = "button";
    toggle.id = "evo-menu-toggle";
    toggle.textContent = "☰";
    toggle.setAttribute("aria-label", "Abrir menu");
    toggle.setAttribute("aria-controls", sidebar.id);
    const close = document.createElement("button");
    close.type = "button";
    close.id = "evo-menu-close";
    close.textContent = "Fechar menu";
    const backdrop = document.createElement("div");
    backdrop.id = "evo-menu-backdrop";
    backdrop.setAttribute("aria-hidden", "true");
    sidebar.prepend(close);
    header.prepend(toggle);
    document.body.append(backdrop);

    let open = false;
    let oldOverflow = "";
    const inertSiblings = new Map();
    const focusable = () => [...sidebar.querySelectorAll("a[href],button,input,select,textarea,[tabindex]")]
      .filter(el => !el.disabled && el.tabIndex >= 0 && !el.closest("[inert]") && el.getClientRects().length);
    function restoreBackground() {
      for (const [element, inert] of inertSiblings) element.inert = inert;
      inertSiblings.clear();
    }
    function reflect() {
      toggle.setAttribute("aria-expanded", String(open));
      toggle.setAttribute("aria-label", open ? "Fechar menu" : "Abrir menu");
      sidebar.inert = mobile.matches && !open;
      sidebar.toggleAttribute("data-open", open);
      backdrop.toggleAttribute("data-open", open);
      if (mobile.matches) {
        sidebar.setAttribute("aria-hidden", String(!open));
        sidebar.setAttribute("role", "dialog");
        sidebar.setAttribute("aria-label", "Menu de navegação");
        sidebar.setAttribute("aria-modal", String(open));
      } else {
        for (const [name, value] of saved) {
          if (name === "id") continue;
          if (value === null) sidebar.removeAttribute(name); else sidebar.setAttribute(name, value);
        }
      }
    }
    function closeDrawer(returnFocus = true) {
      if (open) {
        open = false;
        document.body.style.overflow = oldOverflow;
        restoreBackground();
      }
      reflect();
      if (returnFocus && mobile.matches && toggle.isConnected) toggle.focus();
    }
    function openDrawer() {
      if (!mobile.matches || open) return;
      open = true;
      oldOverflow = document.body.style.overflow;
      document.body.style.overflow = "hidden";
      // Only nodes outside the sidebar become inert; retain their original state.
      for (let element = sidebar; element && element !== root; element = element.parentElement) {
        for (const sibling of element.parentElement.children) {
          if (sibling !== element && !inertSiblings.has(sibling)) {
            inertSiblings.set(sibling, sibling.inert);
            sibling.inert = true;
          }
        }
      }
      reflect();
      (focusable()[0] || close).focus();
    }
    listen(toggle, "click", () => open ? closeDrawer() : openDrawer());
    listen(close, "click", () => closeDrawer());
    listen(backdrop, "click", () => closeDrawer());
    listen(sidebar, "click", event => { if (event.target.closest("a[href]")) closeDrawer(false); });
    listen(document, "keydown", event => {
      if (!open) return;
      if (event.key === "Escape") { event.preventDefault(); closeDrawer(); }
      if (event.key === "Tab") {
        const items = focusable();
        const first = items[0] || close, last = items.at(-1) || close;
        if (event.shiftKey && (document.activeElement === first || !sidebar.contains(document.activeElement))) {
          event.preventDefault(); last.focus();
        } else if (!event.shiftKey && (document.activeElement === last || !sidebar.contains(document.activeElement))) {
          event.preventDefault(); first.focus();
        }
      }
    });
    listen(mobile, "change", () => closeDrawer(false));
    reflect();
    return { sidebar, header, dispose() {
      closeDrawer(false);
      controller.abort();
      toggle.remove(); close.remove(); backdrop.remove();
      delete sidebar.dataset.evoMobileSidebar;
      sidebar.removeAttribute("data-open");
      sidebar.inert = oldInert;
      for (const [name, value] of saved) {
        if (value === null) sidebar.removeAttribute(name); else sidebar.setAttribute(name, value);
      }
    }};
  }

  function sync() {
    const sidebar = root.querySelector("div.hidden.md\\:flex.bg-sidebar");
    const header = root.querySelector("header.flex.h-16");
    if (current && current.sidebar === sidebar && current.header === header) return;
    current?.dispose();
    current = sidebar && header ? mount(sidebar, header) : null;
  }
  const observer = new MutationObserver(sync);
  const start = () => { observer.observe(root, { childList: true, subtree: true }); sync(); };
  window.addEventListener("pagehide", () => { observer.disconnect(); current?.dispose(); current = null; });
  window.addEventListener("pageshow", event => { if (event.persisted) start(); });
  start();
})();
