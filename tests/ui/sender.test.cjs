const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const http = require("node:http");
const fs = require("node:fs/promises");
const path = require("node:path");
const { chromium } = require("playwright");
let server, browser, base;
before(async () => {
  const directory = path.resolve(__dirname, "../../web/sender");
  server = http.createServer(async (req, res) => {
    try {
      const file = path.resolve(directory, req.url === "/sender" ? "index.html" : "." + req.url.replace(/^\/sender/, ""));
      if (!file.startsWith(directory + path.sep)) throw Error("invalid path");
      res.setHeader("Content-Type", file.endsWith(".css") ? "text/css" : file.endsWith(".js") ? "text/javascript" : "text/html");
      res.end(await fs.readFile(file));
    } catch { res.writeHead(404).end(); }
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  base = `http://127.0.0.1:${server.address().port}`;
  browser = await chromium.launch({ headless: true, ...(process.env.PLAYWRIGHT_CHANNEL ? { channel: process.env.PLAYWRIGHT_CHANNEL } : {}) });
});
after(async () => { await browser?.close(); await new Promise(resolve => server.close(resolve)); });

async function setup(context) {
  const page = await context.newPage(), requests = [], sockets = [];
  await page.route("**/sender/session", async route => {
    const token = route.request().headers().apikey;
    await route.fulfill({ status: token === "bad" ? 401 : 200, json: { id:token,name:"Instância " + token,connected:true,websocketEnabled:true,events:["Message"] } });
  });
  await page.route("**/send/*", async route => {
    requests.push({ body:route.request().postData(), headers:route.request().headers(), url:route.request().url() });
    await route.fulfill({ json: { data: { id:"sent" } } });
  });
  await page.routeWebSocket("**/sender/ws?*", ws => { sockets.push(ws); });
  await page.goto(base + "/sender");
  return { page, requests, sockets };
}
async function login(page, token) { await page.locator("#token").fill(token); await page.getByRole("button",{name:"Entrar",exact:true}).click(); await page.locator("#workspace").waitFor({state:"visible"}); }
function incoming(id, text, instanceId = "a") { return JSON.stringify({payload:JSON.stringify({event:"Message",instanceId,data:{Info:{Chat:"5511999999999@s.whatsapp.net",ID:id,PushName:"Contato"},Message:{conversation:text}}})}); }

test("Sender isolates sessions, sends existing REST contracts and never persists keys", async () => {
  const context = await browser.newContext({ viewport:{width:390,height:844},hasTouch:true });
  try {
    const {page,requests,sockets} = await setup(context);
    await page.locator("#token").fill("bad"); await page.getByRole("button",{name:"Entrar",exact:true}).click();
    await page.getByRole("status").filter({hasText:"HTTP 401"}).waitFor();
    await login(page,"a"); await page.waitForFunction(() => document.getElementById("status").textContent.includes("Recebimento ativo"));
    await page.locator("#number").fill("5511999999999"); await page.getByRole("button",{name:"Abrir conversa"}).click();
    await page.locator("#text").fill("Olá"); await page.getByRole("button",{name:"Enviar",exact:true}).click();
    await page.getByRole("status").filter({hasText:"Mensagem enviada"}).waitFor();
    assert.equal(JSON.parse(requests[0].body).number,"5511999999999@s.whatsapp.net"); assert.equal(requests[0].headers.apikey,"a");
    await page.locator("#file").setInputFiles({name:"test.txt",mimeType:"text/plain",buffer:Buffer.from("fixture")});
    await page.getByRole("button",{name:"Enviar",exact:true}).click();
    await page.waitForFunction(() => document.querySelectorAll("#messages li.out small").length === 2 && [...document.querySelectorAll("#messages li.out small")].every(el => el.textContent === "Enviada"));
    assert.match(requests[1].headers["content-type"],/multipart\/form-data/); assert.match(requests[1].body,/name="type"\r\n\r\ndocument/);
    sockets[0].send(incoming("safe","<img src=x onerror=alert(1)>"));
    await page.getByText("<img src=x onerror=alert(1)>",{exact:true}).waitFor(); assert.equal(await page.locator("#messages img").count(),0);
    sockets[0].send(JSON.stringify({payload:JSON.stringify({event:"Receipt",state:"unexpected state",data:{MessageIDs:[JSON.parse(requests[0].body).id]}})}));
    // A subsequent event acts as a barrier without relying on a sleep.
    sockets[0].send(incoming("barrier","receipt barrier"));
    await page.getByText("receipt barrier",{exact:true}).waitFor();
    assert.equal(await page.locator("#messages li.out small").first().textContent(),"Enviada");
    assert.deepEqual(await page.evaluate(() => [localStorage.length,sessionStorage.length,document.cookie]),[0,0,""]);
    await page.getByRole("button",{name:"Sair / trocar instância"}).click(); await login(page,"b");
    assert.equal(await page.locator("#messages li").count(),0); assert.equal(await page.locator("#chats button").count(),0);
    await page.waitForFunction(() => document.getElementById("status").textContent.includes("Recebimento ativo"));
    sockets.at(-1).send(incoming("wrong","wrong instance","a"));
    sockets.at(-1).send(incoming("right","right instance","b"));
    await page.getByRole("button",{name:"Contato",exact:true}).click(); await page.getByText("right instance",{exact:true}).waitFor();
    assert.equal(await page.getByText("wrong instance",{exact:true}).count(),0);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),true);
    await page.reload(); assert.equal(await page.locator("#token").inputValue(),""); assert.equal(await page.locator("#workspace").isVisible(),false);
  } finally { await context.close(); }
});

test("Sender bounds history, reconnects and releases media on logout", async () => {
  const context = await browser.newContext({viewport:{width:1100,height:800},hasTouch:true});
  try {
    const {page,sockets} = await setup(context); await login(page,"a");
    await page.waitForFunction(() => document.getElementById("status").textContent.includes("Recebimento ativo"));
    for (let i=0;i<110;i++) sockets[0].send(incoming("id-"+i,"text-"+i));
    await page.getByRole("button",{name:"Contato",exact:true}).click(); await page.getByText("text-109",{exact:true}).waitFor();
    assert.equal(await page.locator("#messages li").count(),100); assert.equal(await page.getByText("text-0",{exact:true}).count(),0);
    await page.evaluate(() => { window.revoked=[]; const original=URL.revokeObjectURL; URL.revokeObjectURL=url=>{window.revoked.push(url);original(url);}; });
    sockets[0].send(JSON.stringify({payload:JSON.stringify({event:"Message",instanceId:"a",data:{Info:{Chat:"5511999999999@s.whatsapp.net",ID:"media"},Message:{imageMessage:{mimetype:"image/png"},base64:"aGVsbG8="}}})}));
    await page.locator("#messages img").waitFor();
    sockets[0].close({code:1011,reason:"fixture reconnect"});
    await page.waitForFunction(() => document.getElementById("status").textContent.includes("Reconectando"));
    await page.waitForFunction(() => document.getElementById("status").textContent.includes("Recebimento ativo"));
    assert.equal(sockets.length,2);
    sockets[1].close({code:4001,reason:"replaced"});
    await page.getByRole("status").filter({hasText:"outro consumidor"}).waitFor();
    assert.equal(await page.locator("#live").textContent(),"Ativar recebimento");
    await page.getByRole("button",{name:"Sair / trocar instância"}).click();
    assert.equal(await page.evaluate(() => window.revoked.length),1);
    assert.equal(await page.locator("#messages li").count(),0);
  } finally { await context.close(); }
});
