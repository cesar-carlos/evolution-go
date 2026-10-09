const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const http = require("node:http");
const fs = require("node:fs/promises");
const path = require("node:path");
const { chromium } = require("playwright");

const repository = path.resolve(__dirname, "../..");
const markup = `<div class="hidden md:flex bg-sidebar"><a href="#instances">Instâncias</a><a href="#docs">Docs</a></div>
  <div><header class="flex h-16"><span>Manager</span></header><main><div class="group"><div class="flex border-t opacity-0"><button>Ação</button></div></div></main></div>`;
let server, browser, base;
before(async () => {
  server = http.createServer(async (req, res) => {
    try {
      if (req.url === "/fixture") {
        res.setHeader("Content-Type", "text/html");
        res.end(`<link rel="stylesheet" href="/assets/manager-mobile.css"><style>.bg-sidebar{background:white}.opacity-0{opacity:0}@media(min-width:768px){.md\\:flex{display:flex}}</style><div id="root">${markup}</div><script src="/assets/manager-mobile.js"></script>`);
      } else {
        const file = path.resolve(repository, "manager/dist", req.url.startsWith("/manager") ? "index.html" : "." + req.url);
        if (!file.startsWith(path.join(repository, "manager/dist") + path.sep)) throw Error("invalid path");
        res.setHeader("Content-Type", file.endsWith(".css") ? "text/css" : file.endsWith(".html") ? "text/html" : "text/javascript");
        res.end(await fs.readFile(file));
      }
    } catch { res.writeHead(404).end(); }
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  base = `http://127.0.0.1:${server.address().port}`;
  browser = await chromium.launch({ headless: true, ...(process.env.PLAYWRIGHT_CHANNEL ? { channel: process.env.PLAYWRIGHT_CHANNEL } : {}) });
});
after(async () => { await browser?.close(); await new Promise(resolve => server?.close(resolve)); });

test("mobile drawer owns focus, scroll, resize and remount lifecycle", async () => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true });
  try {
    const page = await context.newPage();
    await page.goto(base + "/fixture");
    await page.waitForSelector("#evo-menu-toggle");
    assert.equal(await page.locator("#evo-mobile-sidebar").evaluate(el => el.inert), true);
    await page.evaluate(() => { document.body.style.overflow = "auto"; });
    await page.getByRole("button", { name: "Abrir menu" }).click();
    assert.equal(await page.locator("body").evaluate(el => el.style.overflow), "hidden");
    assert.equal(await page.locator("#evo-menu-close").evaluate(el => el === document.activeElement), true);
    await page.keyboard.press("Shift+Tab");
    assert.equal(await page.evaluate(() => document.activeElement.textContent), "Docs");
    await page.keyboard.press("Tab");
    assert.equal(await page.evaluate(() => document.activeElement.id), "evo-menu-close");
    await page.keyboard.press("Escape");
    assert.equal(await page.evaluate(() => document.activeElement.id), "evo-menu-toggle");
    assert.equal(await page.locator("body").evaluate(el => el.style.overflow), "auto");
    await page.locator("#evo-menu-toggle").click();
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.waitForFunction(() => document.body.style.overflow === "auto");
    assert.equal(await page.locator("#evo-mobile-sidebar").evaluate(el => el.inert), false);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.locator("#evo-menu-toggle").click();
    await page.evaluate(html => { document.getElementById("root").innerHTML = html; }, markup);
    await page.waitForFunction(() => document.body.style.overflow === "auto" && document.querySelectorAll("#evo-menu-toggle").length === 1);
    assert.equal(await page.locator("#evo-menu-backdrop").count(), 1);
    await page.locator("#evo-menu-toggle").click();
    await page.getByRole("link", { name: "Instâncias" }).click();
    assert.equal(await page.locator("#evo-menu-toggle").getAttribute("aria-expanded"), "false");
  } finally { await context.close(); }
});

test("wide touch devices expose instance actions", async () => {
  const context = await browser.newContext({ viewport: { width: 1100, height: 800 }, hasTouch: true });
  try {
    const page = await context.newPage(); await page.goto(base + "/fixture");
    assert.equal(await page.locator(".opacity-0").evaluate(el => getComputedStyle(el).opacity), "1");
    assert.equal(await page.locator("#evo-menu-toggle").isVisible(), false);
  } finally { await context.close(); }
});

test("actual Manager bundle mounts the overlay after React and navigates on touch", async () => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true });
  try {
    await context.addInitScript(() => {
      localStorage.setItem("evolution-auth", JSON.stringify({state:{apiUrl:location.origin,apiKey:"test-only",isAuthenticated:true,licenseState:"licensed"},version:0}));
    });
    const page = await context.newPage();
    await page.route("**/license/status", route => route.fulfill({json:{status:"active"}}));
    await page.route("**/instance/all*", route => route.fulfill({json:{data:[]}}));
    await page.goto(base + "/manager");
    await page.locator("#evo-menu-toggle").waitFor();
    await page.locator("#evo-menu-toggle").click();
    assert.equal(await page.locator("#evo-mobile-sidebar").getAttribute("role"),"dialog");
    await page.locator("#evo-mobile-sidebar").getByRole("link",{name:"Instâncias",exact:true}).click();
    await page.waitForURL("**/manager/instances");
    assert.equal(await page.locator("#evo-menu-toggle").getAttribute("aria-expanded"),"false");
    await page.setViewportSize({width:1100,height:800});
    await page.waitForFunction(() => !document.getElementById("evo-mobile-sidebar").inert);
    assert.equal(await page.locator("body").evaluate(el => el.style.overflow),"");
  } finally { await context.close(); }
});
