"use strict";

const textEl = document.getElementById("text");
const sendEl = document.getElementById("send");
const toastEl = document.getElementById("toast");
const statusEl = document.getElementById("status");
const statusTextEl = document.getElementById("status-text");
const hintEl = document.getElementById("hint");
const btnScreenshotEl = document.getElementById("btn-screenshot");
const shotModalEl = document.getElementById("shot-modal");
const shotImgEl = document.getElementById("shot-img");
const shotPreviewEl = document.getElementById("shot-preview");
const shotCloseEl = document.getElementById("shot-close");
const shotCopyEl = document.getElementById("shot-copy");
const shotShareEl = document.getElementById("shot-share");
const shotDlEl = document.getElementById("shot-dl");
const shotHintEl = document.getElementById("shot-hint");

const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) || 
  (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);

let currentShotBlob = null;
let currentShotUrl = null;

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
    const res = await fetch("/api/type", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    });
    const data = await res.json().catch(() => ({}));
    if (res.ok && data.success) {
      toast("Typed ✓ (" + (data.typed || "?") + " chars)", "ok");
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

function highlightImage() {
  if (shotPreviewEl) {
    shotPreviewEl.classList.remove("highlight");
    void shotPreviewEl.offsetWidth;
    shotPreviewEl.classList.add("highlight");
    setTimeout(() => shotPreviewEl.classList.remove("highlight"), 1500);
  }
}

async function captureScreenshot() {
  btnScreenshotEl.disabled = true;
  btnScreenshotEl.textContent = "Capturing…";
  try {
    const res = await fetch("/api/screenshot", { method: "POST" });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      toast(data.error || ("Screenshot failed (" + res.status + ")"), "err");
      return;
    }
    const blob = await res.blob();
    currentShotBlob = blob;
    if (currentShotUrl) {
      URL.revokeObjectURL(currentShotUrl);
    }
    currentShotUrl = URL.createObjectURL(blob);
    shotImgEl.src = currentShotUrl;
    shotDlEl.href = currentShotUrl;

    // Check Web Share API capability
    if (navigator.share && navigator.canShare) {
      try {
        const testFile = new File([blob], "screenshot.png", { type: "image/png" });
        if (navigator.canShare({ files: [testFile] })) {
          shotShareEl.hidden = false;
        }
      } catch (e) {
        shotShareEl.hidden = true;
      }
    } else {
      shotShareEl.hidden = true;
    }

    let copied = false;
    if (window.isSecureContext && navigator.clipboard && window.ClipboardItem) {
      try {
        const item = new ClipboardItem({
          "image/png": Promise.resolve(blob)
        });
        await navigator.clipboard.write([item]);
        copied = true;
      } catch (clipErr) {
        console.warn("Auto clipboard write:", clipErr);
      }
    }

    if (copied) {
      toast("Screenshot copied to clipboard ✓", "ok");
    } else if (isIOS) {
      toast("Touch & hold image to Copy or Save", "ok");
    } else {
      toast("Screenshot captured ✓", "ok");
    }

    if (isIOS && !window.isSecureContext && shotHintEl) {
      shotHintEl.textContent = "Touch & hold image to Copy or Save to Photos";
    }

    shotModalEl.hidden = false;
  } catch (e) {
    toast("Host unreachable", "err");
    setStatus(false);
  } finally {
    btnScreenshotEl.disabled = false;
    btnScreenshotEl.textContent = "Screenshot";
  }
}

btnScreenshotEl.addEventListener("click", captureScreenshot);

shotCopyEl.addEventListener("click", async () => {
  if (!currentShotBlob) return;

  if (window.isSecureContext && navigator.clipboard && window.ClipboardItem) {
    try {
      const item = new ClipboardItem({
        "image/png": Promise.resolve(currentShotBlob)
      });
      await navigator.clipboard.write([item]);
      toast("Copied to clipboard ✓", "ok");
      return;
    } catch (err) {
      try {
        const item = new ClipboardItem({
          "image/png": currentShotBlob
        });
        await navigator.clipboard.write([item]);
        toast("Copied to clipboard ✓", "ok");
        return;
      } catch (e2) {
        console.warn("Clipboard copy failed:", e2);
      }
    }
  }

  highlightImage();
  if (isIOS) {
    toast("Touch & hold image above to Copy or Save", "ok");
  } else {
    toast("Long press or right-click image to Copy", "ok");
  }
});

shotShareEl.addEventListener("click", async () => {
  if (!currentShotBlob) return;
  try {
    const file = new File([currentShotBlob], "screenshot.png", { type: "image/png" });
    await navigator.share({
      files: [file],
      title: "Pasta Screenshot",
    });
  } catch (err) {
    if (err.name !== "AbortError") {
      toast("Share failed", "err");
    }
  }
});

function closeShotModal() {
  shotModalEl.hidden = true;
}

shotCloseEl.addEventListener("click", closeShotModal);
shotModalEl.addEventListener("click", (e) => {
  if (e.target === shotModalEl) {
    closeShotModal();
  }
});
window.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !shotModalEl.hidden) {
    closeShotModal();
  }
});

window.addEventListener("load", () => {
  textEl.focus();
  ping();
  setInterval(ping, 15000);
});
