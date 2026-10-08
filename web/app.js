"use strict";

const textEl = document.getElementById("text");
const sendEl = document.getElementById("send");
const toastEl = document.getElementById("toast");
const statusEl = document.getElementById("status");
const statusTextEl = document.getElementById("status-text");

let toastTimer = null;

function toast(msg, kind) {
  toastEl.textContent = msg;
  toastEl.className = "toast show " + (kind || "");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { toastEl.className = "toast"; }, 2600);
}

async function send() {
  const text = textEl.value;
  if (!text.trim()) {
    toast("Type something first", "err");
    textEl.focus();
    return;
  }
  sendEl.disabled = true;
  try {
    const res = await fetch("/api/sync", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    });
    const data = await res.json().catch(() => ({}));
    if (res.ok && data.success) {
      toast(data.injected ? "Pasted ✓" : "Copied (paste manually) ✓", "ok");
      textEl.value = "";
    } else {
      toast(data.error || ("Send failed (" + res.status + ")"), "err");
    }
  } catch (e) {
    toast("Host unreachable", "err");
    setStatus(false);
  } finally {
    sendEl.disabled = false;
    textEl.focus();
  }
}

function setStatus(online) {
  statusEl.classList.toggle("online", online);
  statusEl.classList.toggle("offline", !online);
  statusEl.classList.remove("unknown");
  statusTextEl.textContent = online ? "online" : "offline";
}

async function ping() {
  try {
    const ctl = new AbortController();
    const t = setTimeout(() => ctl.abort(), 4000);
    const res = await fetch("/api/health", { signal: ctl.signal });
    clearTimeout(t);
    setStatus(res.ok);
  } catch (e) {
    setStatus(false);
  }
}

sendEl.addEventListener("click", send);
textEl.addEventListener("keydown", (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
    e.preventDefault();
    send();
  }
});

window.addEventListener("load", () => {
  textEl.focus();
  ping();
  setInterval(ping, 15000);
});
