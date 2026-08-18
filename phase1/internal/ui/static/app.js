let me = null;
let labActor = localStorage.getItem("pimLabActor") || "";

const $ = (id) => document.getElementById(id);

function headers(json = true) {
  const h = {};
  if (json) h["Content-Type"] = "application/json";
  if (labActor) h["X-Actor-Username"] = labActor;
  return h;
}

async function api(path, opts = {}) {
  const res = await fetch(path, {
    ...opts,
    headers: { ...headers(!(opts.body instanceof FormData) && opts.method !== "GET"), ...(opts.headers || {}) },
    credentials: "same-origin",
  });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  if (!res.ok) {
    const err = (data && data.error) || res.statusText;
    throw new Error(err);
  }
  return data;
}

function fmt(ts) {
  if (!ts) return "—";
  const d = new Date(ts);
  return isNaN(d) ? String(ts) : d.toLocaleString();
}

function countdown(to) {
  if (!to) return "—";
  const end = new Date(to).getTime();
  const ms = end - Date.now();
  if (ms <= 0) return "expired";
  const s = Math.floor(ms / 1000);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  return `${h}h ${m}m ${sec}s`;
}

async function loadMe() {
  try {
    me = await api("/api/v1/me");
    $("gate").classList.add("hidden");
    $("app").classList.remove("hidden");
    $("who").textContent = `${me.username} · eligible=${me.eligible} · permanent=${me.permanent} · active=${me.active}${me.breakGlass ? " · BREAK-GLASS" : ""}`;
    $("eligibleHint").textContent = me.eligible
      ? `You can request ${me.activeTTL} in admin-active (request TTL ${me.requestTTL}). Justification is required.`
      : "You are not in admin-eligible — request form disabled.";
    $("btnRequest").disabled = !me.eligible;
    $("requestPanel").classList.toggle("hidden", !me.eligible && me.permanent);
    const st = $("statusList");
    st.innerHTML = `
      <li>Eligible: <strong>${me.eligible}</strong></li>
      <li>Approver (permanent): <strong>${me.permanent}</strong></li>
      <li>Currently active: <strong>${me.active}</strong></li>
      <li>Break-glass: <strong>${me.breakGlass}</strong></li>`;
    return true;
  } catch {
    $("gate").classList.remove("hidden");
    $("app").classList.add("hidden");
    $("who").textContent = "not signed in";
    return false;
  }
}

async function loadRequests() {
  const notesDraft = {};
  document.querySelectorAll("[data-notes-for]").forEach((el) => {
    notesDraft[el.getAttribute("data-notes-for")] = el.value;
  });
  const items = await api("/api/v1/requests");
  const el = $("requests");
  const focusId = sessionStorage.getItem("pimFocusRequest") || "";
  if (!items || !items.length) {
    el.textContent = "No requests yet.";
    return;
  }
  el.innerHTML = `<table>
    <thead><tr><th>User</th><th>Status</th><th>Justification</th><th>TTL</th><th></th></tr></thead>
    <tbody>
      ${items.map((r) => {
        const ttl = r.Status === "pending"
          ? countdown(r.RequestExpiresAt)
          : r.Status === "approved"
            ? countdown(r.ActiveExpiresAt)
            : "—";
        const canApprove = me && me.permanent && r.Status === "pending";
        const canRelease = me && r.Status === "approved" && (me.username === r.Username || me.permanent);
        return `<tr id="req-${esc(r.ID)}" class="${focusId === r.ID ? "focus-row" : ""}">
          <td>${esc(r.Username)}<div class="muted">${esc(r.ID.slice(0, 8))}…</div></td>
          <td><span class="badge ${esc(r.Status)}">${esc(r.Status)}</span>
            ${r.ApprovedBy ? `<div class="muted">by ${esc(r.ApprovedBy)}</div>` : ""}</td>
          <td>${esc(r.Justification)}</td>
          <td class="countdown" data-pending="${r.Status === "pending" ? r.RequestExpiresAt : ""}" data-active="${r.Status === "approved" ? r.ActiveExpiresAt : ""}">${ttl}</td>
          <td class="row-actions">
            ${canApprove ? `<div class="approve-box">
              <textarea data-notes-for="${r.ID}" rows="2" placeholder="Approver notes / guidance (required)"></textarea>
              <button class="btn" data-approve="${r.ID}">Approve</button>
            </div>` : ""}
            ${canRelease ? `<button class="btn secondary" data-release="${r.ID}">Release</button>` : ""}
          </td>
        </tr>`;
      }).join("")}
    </tbody></table>`;
  el.querySelectorAll("[data-approve]").forEach((b) => b.addEventListener("click", () => approve(b.dataset.approve)));
  el.querySelectorAll("[data-release]").forEach((b) => b.addEventListener("click", () => release(b.dataset.release)));
  Object.entries(notesDraft).forEach(([id, v]) => {
    const ta = document.querySelector(`[data-notes-for="${id}"]`);
    if (ta) ta.value = v;
  });
  if (focusId) {
    const row = document.getElementById("req-" + focusId);
    if (row) row.scrollIntoView({ behavior: "smooth", block: "center" });
  }
}

/** Elevation ledger — replaces ops dashboard for who/when/why/approver/notes */
async function loadElevationLedger() {
  const items = await api("/api/v1/requests");
  const el = $("audit");
  if (!items || !items.length) {
    el.textContent = "No elevation records yet.";
    return;
  }
  el.innerHTML = `<table>
    <thead><tr>
      <th>User</th>
      <th>Time of request</th>
      <th>Justification / details</th>
      <th>Approver</th>
      <th>Time of approval</th>
      <th>Notes / guidance</th>
      <th>Status</th>
    </tr></thead>
    <tbody>
      ${items.map((r) => `<tr>
        <td>${esc(r.Username)}</td>
        <td>${fmt(r.CreatedAt)}</td>
        <td>${esc(r.Justification)}</td>
        <td>${esc(r.ApprovedBy || "—")}</td>
        <td>${fmt(r.ApprovedAt)}</td>
        <td>${esc(r.ApprovalNotes || "—")}</td>
        <td><span class="badge ${esc(r.Status)}">${esc(r.Status)}</span></td>
      </tr>`).join("")}
    </tbody></table>`;
}

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

async function approve(id) {
  const notesEl = document.querySelector(`[data-notes-for="${id}"]`);
  const notes = notesEl ? notesEl.value.trim() : "";
  if (!notes || notes.length < 8) {
    alert("Approver notes are required (at least 8 characters).");
    if (notesEl) notesEl.focus();
    return;
  }
  try {
    await api(`/api/v1/requests/${id}/approve`, {
      method: "POST",
      body: JSON.stringify({ notes }),
    });
    await refresh();
  } catch (e) {
    alert(e.message);
  }
}

async function release(id) {
  try {
    await api(`/api/v1/requests/${id}/release`, { method: "POST", body: "{}" });
    await refresh();
  } catch (e) {
    alert(e.message);
  }
}

async function refresh() {
  if (!(await loadMe())) return;
  await loadRequests();
  await loadElevationLedger();
}

$("btnRequest").addEventListener("click", async () => {
  const justification = $("justification").value.trim();
  const msg = $("requestMsg");
  msg.classList.remove("err");
  if (!justification || justification.length < 8) {
    msg.classList.add("err");
    msg.textContent = "Justification is required (at least 8 characters).";
    return;
  }
  try {
    const req = await api("/api/v1/requests", {
      method: "POST",
      body: JSON.stringify({ justification }),
    });
    msg.textContent = `Created ${req.ID.slice(0, 8)}… (expires ${fmt(req.RequestExpiresAt)})`;
    $("justification").value = "";
    await refresh();
  } catch (e) {
    msg.classList.add("err");
    msg.textContent = e.message;
  }
});

$("btnAudit").addEventListener("click", () => loadElevationLedger().catch((e) => alert(e.message)));

$("labGo").addEventListener("click", async () => {
  labActor = $("labActor").value.trim();
  localStorage.setItem("pimLabActor", labActor);
  await refresh();
});

$("btnLogout").addEventListener("click", () => {
  labActor = "";
  localStorage.removeItem("pimLabActor");
  window.location.href = "/auth/logout";
});

setInterval(() => {
  document.querySelectorAll(".countdown").forEach((el) => {
    const p = el.getAttribute("data-pending");
    const a = el.getAttribute("data-active");
    if (p) el.textContent = countdown(p);
    else if (a) el.textContent = countdown(a);
  });
}, 1000);

(function stashDeepLink() {
  const id = new URLSearchParams(location.search).get("requestId");
  if (id) {
    sessionStorage.setItem("pimFocusRequest", id);
    history.replaceState({}, "", "/");
  }
})();

refresh().catch(console.error);
setInterval(() => {
  if (!me) return;
  refresh().catch(() => {});
}, 10000);
