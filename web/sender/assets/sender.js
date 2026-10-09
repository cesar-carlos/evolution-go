/* Event shapes and chat workflow adapted from Evolution Go #182 (prakash-dev-code).
 * No storage, bootstrap key, global subscriptions or automatic send retries. */
(() => {
  "use strict";
  const $ = id => document.getElementById(id);
  const MAX_CHATS = 50, MAX_MESSAGES = 100, MAX_MEDIA = 20 * 1024 * 1024;
  let session = null, active = "", generation = 0, socket = null, retry = null;
  let live = false, attempts = 0, mediaBytes = 0, sending = false;
  const chats = new Map(), requests = new Set();
  const status = text => { $("status").textContent = text; };

  async function request(path, options = {}, token = session?.token) {
    const epoch = generation, controller = new AbortController();
    requests.add(controller);
    const timeout = setTimeout(() => controller.abort(), 60000);
    try {
      const response = await fetch(path, { ...options, credentials: "omit", cache: "no-store",
        headers: { ...options.headers, apikey: token }, signal: controller.signal });
      if (!response.ok) throw Error(`HTTP ${response.status}`);
      const result = await response.json();
      if (epoch !== generation) throw new DOMException("Session changed", "AbortError");
      return result;
    } finally { clearTimeout(timeout); requests.delete(controller); }
  }

  function release(message) {
    if (message.url) { URL.revokeObjectURL(message.url); mediaBytes -= message.bytes; delete message.url; }
  }
  function clearChat(chat) { chat.messages.forEach(release); chat.messages = []; }
  function closeSocket() {
    clearTimeout(retry); retry = null;
    if (socket) {
      socket.onclose = socket.onmessage = socket.onopen = socket.onerror = null;
      socket.close(); socket = null;
    }
  }
  function reset() {
    generation++; live = false; closeSocket();
    for (const controller of requests) controller.abort();
    requests.clear(); chats.forEach(clearChat); chats.clear();
    session = null; active = ""; mediaBytes = 0; sending = false; attempts = 0;
    $("login").hidden = false; $("workspace").hidden = true; $("logout").hidden = true;
    $("token").value = $("text").value = $("file").value = "";
    $("messages").replaceChildren(); $("chats").replaceChildren(); $("instanceName").textContent = "";
    $("chatName").textContent = "Selecione uma conversa"; $("send").disabled = true;
    $("live").textContent = "Ativar recebimento";
  }

  function validJID(raw) {
    if (/^\d{5,32}$/.test(raw)) return raw + "@s.whatsapp.net";
    return /^\d{1,32}(?:-\d{1,32})?@(s\.whatsapp\.net|lid|g\.us)$/.test(raw) ? raw : "";
  }
  function chatFor(jid, name = "") {
    if (!chats.has(jid)) {
      if (chats.size >= MAX_CHATS) {
        const oldest = [...chats.keys()].find(key => key !== active) || chats.keys().next().value;
        clearChat(chats.get(oldest)); chats.delete(oldest);
      }
      chats.set(jid, { name: name || jid, messages: [] });
    }
    const chat = chats.get(jid);
    if (name) chat.name = name.slice(0, 128);
    return chat;
  }
  function add(jid, message, name = "") {
    const chat = chatFor(jid, name);
    if (chat.messages.some(item => item.id === message.id)) { release(message); return; }
    chat.messages.push(message);
    while (chat.messages.length > MAX_MESSAGES) release(chat.messages.shift());
    renderChats(); if (active === jid) renderMessages();
  }
  function renderChats() {
    $("chats").replaceChildren();
    for (const [jid, chat] of chats) {
      const button = document.createElement("button"); button.type = "button"; button.textContent = chat.name;
      if (jid === active) button.setAttribute("aria-current", "true");
      button.addEventListener("click", () => open(jid)); $("chats").append(button);
    }
  }
  function open(jid) {
    active = jid; chatFor(jid); renderChats(); renderMessages();
    $("chatName").textContent = chats.get(jid).name;
    $("send").disabled = !session || sending; $("text").focus();
  }
  function renderMessages() {
    const list = $("messages");
    const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 60;
    list.replaceChildren();
    for (const message of chats.get(active)?.messages || []) {
      const item = document.createElement("li"); item.className = message.out ? "out" : "in";
      const body = document.createElement("span"); body.textContent = message.text; item.append(body);
      if (message.url) {
        const tag = message.kind === "image" ? "img" : ["audio", "video"].includes(message.kind) ? message.kind : "a";
        const media = document.createElement(tag);
        if (tag === "a") { media.href = message.url; media.download = message.filename || "anexo"; media.textContent = "Baixar anexo"; }
        else { media.src = message.url; if (tag === "img") media.alt = "Imagem recebida"; else media.controls = true; }
        item.append(media);
      } else if (message.descriptor && !message.loading) {
        const button = document.createElement("button"); button.textContent = "Carregar anexo";
        button.addEventListener("click", () => download(message)); item.append(button);
      }
      const state = document.createElement("small"); state.textContent = message.state || "Recebida"; item.append(state); list.append(item);
    }
    if (atBottom) list.scrollTop = list.scrollHeight;
  }
  function attach(message, encoded, mime) {
    if (!encoded || typeof encoded !== "string" || encoded.length > MAX_MEDIA * 1.4) return;
    const raw = encoded.replace(/^data:[^,]*,/, "");
    const bytes = Uint8Array.from(atob(raw), ch => ch.charCodeAt(0));
    if (bytes.length + mediaBytes > MAX_MEDIA) return;
    // Active HTML/SVG is only downloadable; it is never mounted as a document.
    const safe = /^(image\/(jpeg|png|webp|gif)|audio\/[a-z0-9.+-]+|video\/[a-z0-9.+-]+)$/.test(mime || "");
    message.kind = safe ? mime.split("/")[0] : "document";
    message.bytes = bytes.length; mediaBytes += bytes.length;
    message.url = URL.createObjectURL(new Blob([bytes], { type: safe ? mime : "application/octet-stream" }));
  }
  async function download(message) {
    const epoch = generation; message.loading = true; renderMessages();
    try {
      const result = await request("/message/downloadmedia", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ message: message.descriptor }) });
      if (epoch !== generation || !(chats.get(active)?.messages.includes(message))) return;
      attach(message, result.data?.base64, message.mime);
    } catch (error) { if (epoch === generation) status(`Não foi possível carregar o anexo: ${error.message}`); }
    finally { message.loading = false; if (epoch === generation) renderMessages(); }
  }
  function unwrap(message) {
    for (let i = 0; i < 32 && message; i++) {
      const nested = message.ephemeralMessage || message.viewOnceMessage || message.viewOnceMessageV2 || message.viewOnceMessageV2Extension || message.documentWithCaptionMessage;
      if (!nested?.message) break; message = nested.message;
    }
    return message || {};
  }
  async function event(payload, epoch) {
    if (epoch !== generation || !session) return;
    if (payload.instanceId && payload.instanceId !== session.id) return;
    const data = payload.data || {}, info = data.Info || {};
    if (payload.event === "Receipt") {
      for (const chat of chats.values()) for (const message of chat.messages) {
        if (message.out && data.MessageIDs?.includes(message.id) && message.state !== "Read") message.state = payload.state;
      }
      renderMessages(); return;
    }
    if (!["Message", "SendMessage"].includes(payload.event)) return;
    let jid = validJID(String(info.Chat || "")); if (!jid) return;
    // Resolve only explicit LIDs, never guess a phone number from their length.
    if (jid.endsWith("@lid")) {
      try {
        const lid = jid.split("@")[0]; const mapping = await request("/sender/resolve-lids?lids=" + encodeURIComponent(lid));
        if (epoch !== generation) return;
        if (/^\d{1,32}$/.test(mapping[lid] || "")) jid = mapping[lid] + "@s.whatsapp.net";
      } catch { if (epoch !== generation) return; }
    }
    const message = unwrap(data.Message), mediaKey = ["imageMessage", "videoMessage", "audioMessage", "documentMessage", "stickerMessage"].find(key => message[key]);
    const media = message[mediaKey];
    const text = message.conversation || message.extendedTextMessage?.text || media?.caption || (media ? "Anexo" : "");
    if (!text) return;
    let descriptor = null;
    if (media) {
      // Keep only the small protobuf fields needed for an explicit media download.
      const copy = { ...media }; delete copy.jpegThumbnail; delete copy.JPEGThumbnail; delete copy.thumbnail;
      if (JSON.stringify(copy).length <= 8192) descriptor = { [mediaKey]: copy };
    }
    const entry = { id: String(info.ID || crypto.randomUUID()).slice(0, 128), text: String(text).slice(0, 10000), out: !!info.IsFromMe,
      state: info.IsFromMe ? "Enviada" : "Recebida", mime: media?.mimetype, filename: media?.fileName,
      descriptor };
    if (message.base64) { try { attach(entry, message.base64, media?.mimetype); } catch { /* Keep descriptor for explicit download. */ } }
    add(jid, entry, info.IsFromMe ? "" : info.PushName);
  }
  function connectSocket() {
    closeSocket(); if (!live || !session) return;
    const epoch = generation;
    const url = new URL("/sender/ws", location.href); url.protocol = location.protocol === "https:" ? "wss:" : "ws:";
    url.searchParams.set("token", session.token); socket = new WebSocket(url);
    socket.onopen = () => { attempts = 0; status(`Recebimento ativo: ${session.name}`); };
    socket.onmessage = message => {
      if (message.data.length > 32 * 1024 * 1024) return;
      try { const frame = JSON.parse(message.data); const payload = typeof frame.payload === "string" ? JSON.parse(frame.payload) : frame.payload; if (payload) void event(payload, epoch).catch(() => {}); } catch { /* Invalid events do not alter the active session. */ }
    };
    socket.onclose = event => {
      if (epoch !== generation || !live) return;
      if (event.code === 4001) {
        live = false; $("live").textContent = "Ativar recebimento";
        status("Recebimento assumido por outro consumidor desta instância."); return;
      }
      if (attempts >= 8) {
        live = false; $("live").textContent = "Ativar recebimento";
        status("Recebimento indisponível. Verifique a instância e ative novamente."); return;
      }
      status("Recebimento desconectado. Reconectando…");
      retry = setTimeout(connectSocket, Math.min(30000, 1000 * 2 ** Math.min(attempts++, 5)));
    };
  }

  $("login").addEventListener("submit", async e => {
    e.preventDefault(); const token = $("token").value.trim(); reset(); const epoch = generation;
    try {
      const result = await request("/sender/session", {}, token);
      if (epoch !== generation) return;
      session = { ...result, token }; $("login").hidden = true; $("workspace").hidden = false; $("logout").hidden = false;
      $("instanceName").textContent = result.name; status(`${result.name}: ${result.connected ? "conectada" : "desconectada"}`);
      if (result.websocketEnabled) { live = true; $("live").textContent = "Pausar recebimento"; connectSocket(); }
    } catch (error) { if (epoch === generation) { status(`Falha ao autenticar: ${error.message}`); $("token").focus(); } }
  });
  $("logout").addEventListener("click", () => { reset(); status("Sessão encerrada nesta aba."); $("token").focus(); });
  $("newChat").addEventListener("submit", e => { e.preventDefault(); const jid = validJID($("number").value.trim()); if (jid) open(jid); else status("Informe um número ou JID válido."); });
  $("clear").addEventListener("click", () => { if (active && confirm("Apagar o histórico desta conversa nesta aba?")) { clearChat(chats.get(active)); renderMessages(); } });
  $("live").addEventListener("click", async () => {
    if (live) { live = false; closeSocket(); $("live").textContent = "Ativar recebimento"; status("Recebimento pausado."); return; }
    if (!session) return;
    const epoch = generation;
    if (!session.websocketEnabled) {
      if (!confirm("Ativar WebSocket e eventos Message/SendMessage/Receipt nesta instância?")) return;
      try {
        const subscribe = [...new Set([...(session.events || []).filter(Boolean), "Message", "SendMessage", "Receipt"])];
        await request("/instance/connect", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ subscribe, websocketEnable: "true" }) });
        if (epoch !== generation) return; session.websocketEnabled = true;
      } catch (error) { if (epoch === generation) status(`Falha ao ativar eventos: ${error.message}`); return; }
    }
    live = true; attempts = 0; $("live").textContent = "Pausar recebimento"; connectSocket();
  });
  $("composer").addEventListener("submit", async e => {
    e.preventDefault(); if (!session || !active || sending) return;
    const text = $("text").value.trim(), file = $("file").files[0], jid = active, epoch = generation;
    if (!text && !file) return;
    if (file?.size > 16 * 1024 * 1024) { status("Anexo maior que 16 MiB."); return; }
    const id = crypto.randomUUID(); sending = true; $("send").disabled = true;
    add(jid, { id, text: text || file.name, out: true, state: "Enviando…" });
    try {
      let path = "/send/text", options = { method: "POST" };
      if (file) {
        path = "/send/media"; const body = new FormData();
        body.set("number", jid); body.set("caption", text); body.set("filename", file.name); body.set("id", id); body.set("file", file);
        body.set("type", /^(image|video|audio)\//.test(file.type) ? file.type.split("/")[0] : "document"); options.body = body;
      } else { options.headers = { "Content-Type": "application/json" }; options.body = JSON.stringify({ number: jid, text, id }); }
      await request(path, options);
      if (epoch !== generation) return;
      const entry = chats.get(jid)?.messages.find(item => item.id === id); if (entry) entry.state = "Enviada";
      // Do not erase a draft typed while awaiting the server.
      if (active === jid && $("text").value.trim() === text) $("text").value = "";
      if ($("file").files[0] === file) $("file").value = "";
      status("Mensagem enviada.");
    } catch (error) {
      if (epoch !== generation) return;
      const entry = chats.get(jid)?.messages.find(item => item.id === id); if (entry) entry.state = "Resultado incerto";
      status(`Envio sem confirmação (${error.message}). Verifique antes de tentar novamente.`);
    } finally { if (epoch === generation) { sending = false; $("send").disabled = !active; renderMessages(); } }
  });
  window.addEventListener("pagehide", reset);
})();
