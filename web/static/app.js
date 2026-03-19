const state = {
  auth: localStorage.getItem("airboard.auth_data") || "",
  profile: null,
  isAdmin: false,
  currentSubscriptionUserId: "",
  adminPath: document.body.dataset.adminPath || "admin",
  page: document.body.dataset.page || "index",
};

const toast = document.getElementById("toast");

document.addEventListener("DOMContentLoaded", () => {
  wireTabs();
  wireAuthSwitches();
  if (state.page === "admin") {
    bootAdmin();
    return;
  }
  bootIndex();
});

function bootIndex() {
  bindGuestForms();
  bindDashboardActions();
  consumeMagicLogin();
  loadGuestPlans();
  restoreDashboard();
}

function bootAdmin() {
  bindAdminLogin();
  bindAdminActions();
  restoreAdmin();
}

function wireTabs() {
  document.querySelectorAll("[data-tab-target]").forEach((button) => {
    button.addEventListener("click", () => {
      const prefix = state.page === "admin" ? "admin-tab-" : "tab-";
      document.querySelectorAll(`[id^="${prefix}"]`).forEach((pane) => pane.classList.remove("active"));
      document.querySelectorAll("[data-tab-target]").forEach((item) => {
        item.classList.remove("active", "ant-menu-item-selected");
      });
      document.getElementById(`${prefix}${button.dataset.tabTarget}`)?.classList.add("active");
      button.classList.add("active", "ant-menu-item-selected");
    });
  });
}

function wireAuthSwitches() {
  const switches = document.querySelectorAll("[data-auth-target]");
  if (!switches.length) return;
  switches.forEach((button) => {
    button.addEventListener("click", () => {
      switches.forEach((item) => item.classList.remove("active"));
      document.querySelectorAll(".auth-pane").forEach((pane) => pane.classList.add("hidden"));
      button.classList.add("active");
      document.getElementById(`auth-${button.dataset.authTarget}-pane`)?.classList.remove("hidden");
      document.getElementById(`auth-${button.dataset.authTarget}-pane`)?.classList.add("active");
      document.querySelectorAll(".auth-pane").forEach((pane) => {
        if (pane.id !== `auth-${button.dataset.authTarget}-pane`) {
          pane.classList.remove("active");
        }
      });
    });
  });
}

function bindGuestForms() {
  const loginForm = document.getElementById("login-form");
  const registerForm = document.getElementById("register-form");
  if (loginForm) {
    loginForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const response = await api("/api/v1/passport/auth/login", {
        method: "POST",
        body: formToObject(loginForm),
      });
      if (!response) return;
      setAuth(response.data.auth_data);
      notify("登录成功");
      await restoreDashboard();
    });
  }
  if (registerForm) {
    registerForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const response = await api("/api/v1/passport/auth/register", {
        method: "POST",
        body: formToObject(registerForm),
      });
      if (!response) return;
      setAuth(response.data.auth_data);
      notify("注册成功");
      await restoreDashboard();
    });
  }
}

async function consumeMagicLogin() {
  const params = new URLSearchParams(location.search);
  const verify = params.get("verify");
  if (!verify) return;
  const response = await api(`/api/v1/passport/auth/token2Login?verify=${encodeURIComponent(verify)}`);
  if (!response) return;
  setAuth(response.data.auth_data);
  history.replaceState({}, "", location.pathname);
  notify("快捷登录成功");
  await restoreDashboard();
}

async function restoreDashboard() {
  if (!state.auth) return;
  const result = await api("/api/v1/user/checkLogin", { auth: true, silent: true });
  if (!result?.data?.is_login) {
    clearAuth();
    return;
  }
  state.isAdmin = Boolean(result.data.is_admin);
  await showDashboard();
}

async function showDashboard() {
  toggleView("guest-view", false);
  toggleView("dashboard-view", true);
  await loadDashboardProfile();
  await Promise.all([
    loadGuestPlans("dashboard-plans-grid"),
    loadServers(),
    loadNotices(),
  ]);
}

function bindDashboardActions() {
  document.getElementById("refresh-dashboard")?.addEventListener("click", showDashboard);
  document.getElementById("logout-button")?.addEventListener("click", () => {
    clearAuth();
    toggleView("dashboard-view", false);
    toggleView("guest-view", true);
  });
  document.getElementById("reset-security")?.addEventListener("click", async () => {
    const response = await api("/api/v1/user/resetSecurity", { auth: true });
    if (!response) return;
    notify("订阅地址已重置");
    await loadDashboardProfile();
  });
  document.getElementById("copy-subscribe")?.addEventListener("click", async () => {
    const input = document.getElementById("subscribe-url");
    if (!input?.value) {
      notify("当前没有可复制的订阅地址", true);
      return;
    }
    await copyText(input.value);
    notify("订阅地址已复制");
  });
  document.getElementById("password-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const response = await api("/api/v1/user/changePassword", {
      method: "POST",
      auth: true,
      body: formToObject(form),
    });
    if (!response) return;
    form.reset();
    notify("密码已更新");
  });
  document.getElementById("preferences-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const response = await api("/api/v1/user/update", {
      method: "POST",
      auth: true,
      body: {
        remind_expire: Boolean(form.elements.remind_expire.checked),
        remind_traffic: Boolean(form.elements.remind_traffic.checked),
      },
    });
    if (!response) return;
    notify("提醒设置已保存");
  });
}

async function loadDashboardProfile() {
  const [infoResponse, subscribeResponse, statResponse] = await Promise.all([
    api("/api/v1/user/info", { auth: true }),
    api("/api/v1/user/getSubscribe", { auth: true }),
    api("/api/v1/user/getStat", { auth: true }),
  ]);
  if (!infoResponse || !subscribeResponse || !statResponse) return;

  const info = infoResponse.data || {};
  const subscribe = subscribeResponse.data || {};
  const plan = subscribe.plan || null;
  const invites = statResponse.data?.[2] ?? 0;

  state.profile = info;
  state.profile.plan_id = subscribe.plan_id || info.plan_id || 0;

  setText("dashboard-email", info.email || "-");
  setText("dashboard-plan-name", plan?.name || "未分配套餐");
  setText("dashboard-welcome-title", `欢迎回来，${(info.email || "用户").split("@")[0]}`);
  setText("dashboard-plan-meta", plan?.content || "当前订阅支持多客户端格式导入，包含默认、Clash、Shadowrocket 与 V2 入口。");
  setText("traffic-metric", trafficText(subscribe));
  setText("expiry-metric", formatTimestamp(subscribe.expired_at));
  setText("reset-metric", `${subscribe.reset_day || "-"} 日`);
  setText("invite-metric", String(invites));
  setText("account-email", info.email || "-");
  setText("account-plan", plan?.name || "未分配套餐");
  setText("account-uuid", info.uuid || "-");
  setText("account-expiry", formatTimestamp(subscribe.expired_at));
  setText("dashboard-expire-tag", `到期 ${formatTimestamp(subscribe.expired_at)}`);

  const avatar = document.getElementById("dashboard-avatar");
  if (avatar) {
    avatar.src = info.avatar_url || "";
  }

  document.getElementById("preferences-form").elements.remind_expire.checked = Boolean(info.remind_expire);
  document.getElementById("preferences-form").elements.remind_traffic.checked = Boolean(info.remind_traffic);

  applySubscriptionState(subscribe);

  const adminLink = document.getElementById("open-admin-link");
  if (adminLink) {
    adminLink.style.display = state.isAdmin ? "" : "none";
  }
}

function applySubscriptionState(subscribe) {
  const links = subscribe.subscribe_urls || [];
  const primary = links.find((item) => item.is_primary) || links[0] || null;
  document.getElementById("subscribe-url").value = primary?.urls?.default || subscribe.subscribe_url || "";
  setText("primary-suffix", primary?.suffix || "默认");
  renderSubscriptionLinks(document.getElementById("subscription-link-list"), links, { admin: false });
}

async function loadServers() {
  const response = await api("/api/v1/user/server/fetch", { auth: true, raw: true });
  const container = document.getElementById("server-list");
  if (!response || !container) return;

  renderTable(
    container,
    [
      {
        title: "节点",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(item.name)}</strong>
            <span class="airboard-cell-sub">${escapeHtml(item.type)} · 版本 ${item.version ?? 1}</span>
          </div>
        `,
      },
      {
        title: "状态",
        render: (item) => badgeStatus(item.is_online ? "在线" : "离线", item.is_online ? "success" : "error"),
      },
      {
        title: "倍率",
        render: (item) => escapeHtml(String(item.rate ?? 1)),
      },
      {
        title: "标签",
        render: (item) => renderTagRow(item.tags || [], "暂无标签"),
      },
      {
        title: "最近检查",
        render: (item) => escapeHtml(formatTimestamp(item.last_check_at)),
      },
    ],
    response.data || [],
    "暂无可用节点",
  );
}

async function loadNotices() {
  const response = await api("/api/v1/user/notice/fetch", { auth: true, raw: true });
  renderNoticeCards(document.getElementById("notice-list"), response?.data || []);
}

async function loadGuestPlans(targetId = "plans-grid") {
  const response = await api("/api/v1/guest/plan/fetch");
  const container = document.getElementById(targetId);
  if (!response || !container) return;
  const plans = response.data || [];
  container.innerHTML = "";
  if (!plans.length) {
    showEmpty(container, "暂无可用套餐");
    return;
  }
  plans.forEach((item) => {
    const active = Number(state.profile?.plan_id || 0) === Number(item.id || 0);
    const card = document.createElement("article");
    card.className = `ant-card airboard-plan-card${active ? " airboard-plan-card-active" : ""}`;
    card.innerHTML = `
      <div class="ant-card-body">
        <div class="airboard-plan-topline">
          <span class="ant-tag airboard-tag airboard-tag-primary">PLAN ${escapeHtml(String(item.id))}</span>
          ${active ? '<span class="ant-tag airboard-tag">当前套餐</span>' : ""}
        </div>
        <h3 class="airboard-plan-name">${escapeHtml(item.name)}</h3>
        <strong class="airboard-plan-price">${formatPrice(item.price)}</strong>
        <p class="airboard-plan-desc">${escapeHtml(item.content || "暂无描述")}</p>
        <div class="airboard-tag-row">
          <span class="ant-tag airboard-tag">${escapeHtml(formatBytes(item.transfer_enable || 0))}</span>
          <span class="ant-tag airboard-tag">${escapeHtml(String(item.speed_limit || 0))} Mbps</span>
        </div>
      </div>
    `;
    container.appendChild(card);
  });
}

function bindAdminLogin() {
  const form = document.getElementById("admin-login-form");
  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const response = await api("/api/v1/passport/auth/login", {
      method: "POST",
      body: formToObject(form),
    });
    if (!response) return;
    setAuth(response.data.auth_data);
    notify("管理员登录成功");
    await restoreAdmin();
  });
}

async function restoreAdmin() {
  if (!state.auth) {
    toggleView("admin-login-view", true);
    toggleView("admin-view", false);
    return;
  }
  const check = await api("/api/v1/user/checkLogin", { auth: true, silent: true });
  if (!check?.data?.is_admin) {
    clearAuth();
    toggleView("admin-login-view", true);
    toggleView("admin-view", false);
    notify("当前账号不是管理员", true);
    return;
  }
  state.isAdmin = true;
  toggleView("admin-login-view", false);
  toggleView("admin-view", true);
  await loadAdminData();
}

function bindAdminActions() {
  document.getElementById("admin-refresh")?.addEventListener("click", loadAdminData);
  document.getElementById("admin-logout")?.addEventListener("click", () => {
    clearAuth();
    restoreAdmin();
  });

  document.getElementById("admin-generate-user-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const response = await adminApi("/user/generate", { method: "POST", body: formToObject(form) });
    if (!response) return;
    form.reset();
    notify("用户已创建");
    await loadAdminUsers();
  });

  document.getElementById("admin-plan-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const response = await adminApi("/plan/save", { method: "POST", body: formToObject(form) });
    if (!response) return;
    form.reset();
    notify("套餐已保存");
    await loadAdminPlans();
  });

  document.getElementById("admin-server-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const payload = formToObject(form);
    const type = payload.type;
    delete payload.type;
    const response = await adminApi(`/server/${type}/save`, { method: "POST", body: payload });
    if (!response) return;
    form.reset();
    notify("节点已保存");
    await loadAdminServers();
  });

  document.getElementById("admin-notice-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const response = await adminApi("/notice/save", { method: "POST", body: formToObject(form) });
    if (!response) return;
    form.reset();
    notify("公告已发布");
    await loadAdminNotices();
  });

  document.getElementById("admin-settings-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const response = await adminApi("/config/save", { method: "POST", body: formToObject(form) });
    if (!response) return;
    notify("配置已保存，修改安全路径后请重启服务");
  });

  document.getElementById("admin-subscription-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const payload = formToObject(form);
    payload.is_primary = Boolean(form.elements.is_primary.checked);
    const response = await adminApi("/user/subscription/save", { method: "POST", body: payload });
    if (!response) return;
    state.currentSubscriptionUserId = String(payload.user_id || "");
    notify("订阅后缀已保存");
    await loadAdminSubscriptions(payload.user_id);
    form.elements.name.value = "";
    form.elements.suffix.value = "";
    form.elements.is_primary.checked = false;
  });

  document.getElementById("admin-subscription-fetch")?.addEventListener("click", async () => {
    const userId = document.querySelector("#admin-subscription-form [name='user_id']").value.trim();
    if (!userId) {
      notify("先输入用户 ID", true);
      return;
    }
    state.currentSubscriptionUserId = userId;
    await loadAdminSubscriptions(userId);
  });

  document.getElementById("admin-subscription-reset")?.addEventListener("click", async () => {
    const userId = document.querySelector("#admin-subscription-form [name='user_id']").value.trim();
    if (!userId) {
      notify("先输入用户 ID", true);
      return;
    }
    const response = await adminApi("/user/subscription/reset", {
      method: "POST",
      body: { user_id: Number(userId) },
    });
    if (!response) return;
    notify("该用户全部订阅链接已重置");
    await loadAdminSubscriptions(userId);
  });
}

async function loadAdminData() {
  await Promise.all([
    loadAdminStats(),
    loadAdminUsers(),
    loadAdminPlans(),
    loadAdminServers(),
    loadAdminNotices(),
    loadAdminSettings(),
  ]);
  if (state.currentSubscriptionUserId) {
    await loadAdminSubscriptions(state.currentSubscriptionUserId);
  }
}

async function loadAdminStats() {
  const response = await adminApi("/stat/getStat");
  const grid = document.getElementById("admin-stat-grid");
  if (!response || !grid) return;
  const labels = {
    users: "用户",
    plans: "套餐",
    servers: "节点",
    notices: "公告",
  };
  grid.innerHTML = "";
  ["users", "plans", "servers", "notices"].forEach((key) => {
    const card = document.createElement("div");
    card.className = "ant-card airboard-stat-card";
    card.innerHTML = `
      <div class="ant-card-body">
        <span>${escapeHtml(labels[key])}</span>
        <strong>${escapeHtml(String(response.data?.[key] ?? 0))}</strong>
      </div>
    `;
    grid.appendChild(card);
  });
}

async function loadAdminUsers() {
  const response = await adminApi("/user/fetch", { raw: true });
  const container = document.getElementById("admin-user-list");
  if (!response || !container) return;

  renderTable(
    container,
    [
      {
        title: "用户",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(item.email)}</strong>
            <span class="airboard-cell-sub">ID ${item.id} · 创建于 ${escapeHtml(formatTimestamp(item.created_at))}</span>
          </div>
        `,
      },
      {
        title: "套餐 / 状态",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(item.plan_name || "未分配套餐")}</strong>
            <div class="airboard-cell-tags">
              ${badgeStatus(item.banned ? "已禁用" : "正常", item.banned ? "error" : "success")}
              ${item.is_admin ? '<span class="ant-tag airboard-tag">管理员</span>' : ""}
            </div>
          </div>
        `,
      },
      {
        title: "流量",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(formatBytes(item.total_used || 0))} / ${escapeHtml(formatBytes(item.transfer_enable || 0))}</strong>
            <span class="airboard-cell-sub">到期 ${escapeHtml(formatTimestamp(item.expired_at))}</span>
          </div>
        `,
      },
      {
        title: "主订阅",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(item.subscribe_suffix || "-")}</strong>
            <span class="airboard-cell-sub">${item.subscribe_url ? `<a href="${escapeAttribute(item.subscribe_url)}" target="_blank" rel="noreferrer">打开订阅地址</a>` : "暂无订阅地址"}</span>
          </div>
        `,
      },
      {
        title: "操作",
        render: (item) => `
          <div class="airboard-cell-actions">
            <button type="button" class="ant-btn ant-btn-sm" data-user-ban="${item.id}">${item.banned ? "解除禁用" : "禁用"}</button>
            <button type="button" class="ant-btn ant-btn-sm" data-user-reset="${item.id}">重置订阅</button>
            <button type="button" class="ant-btn ant-btn-sm" data-user-subscriptions="${item.id}">管理后缀</button>
          </div>
        `,
      },
    ],
    response.data || [],
    "暂无用户",
  );

  container.querySelectorAll("[data-user-ban]").forEach((button) => {
    button.addEventListener("click", async () => {
      const response = await adminApi("/user/ban", {
        method: "POST",
        body: { id: Number(button.dataset.userBan) },
      });
      if (!response) return;
      notify("用户状态已更新");
      await loadAdminUsers();
    });
  });

  container.querySelectorAll("[data-user-reset]").forEach((button) => {
    button.addEventListener("click", async () => {
      const response = await adminApi("/user/resetSecret", {
        method: "POST",
        body: { id: Number(button.dataset.userReset) },
      });
      if (!response) return;
      notify("订阅令牌已重置");
      await loadAdminUsers();
      if (state.currentSubscriptionUserId === button.dataset.userReset) {
        await loadAdminSubscriptions(button.dataset.userReset);
      }
    });
  });

  container.querySelectorAll("[data-user-subscriptions]").forEach((button) => {
    button.addEventListener("click", async () => {
      state.currentSubscriptionUserId = button.dataset.userSubscriptions;
      document.querySelector("#admin-subscription-form [name='user_id']").value = button.dataset.userSubscriptions;
      await loadAdminSubscriptions(button.dataset.userSubscriptions);
      document.querySelector("#admin-view [data-tab-target='users']")?.click();
    });
  });
}

async function loadAdminPlans() {
  const response = await adminApi("/plan/fetch");
  const container = document.getElementById("admin-plan-list");
  if (!response || !container) return;

  renderTable(
    container,
    [
      {
        title: "套餐",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(item.name)}</strong>
            <span class="airboard-cell-sub">ID ${item.id}</span>
          </div>
        `,
      },
      {
        title: "流量 / 速率",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(formatBytes(item.transfer_enable || 0))}</strong>
            <span class="airboard-cell-sub">${escapeHtml(String(item.speed_limit || 0))} Mbps</span>
          </div>
        `,
      },
      {
        title: "价格 / 用户",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${formatPrice(item.price)}</strong>
            <span class="airboard-cell-sub">${escapeHtml(String(item.count || 0))} 用户</span>
          </div>
        `,
      },
      {
        title: "操作",
        render: (item) => `
          <div class="airboard-cell-actions">
            <button type="button" class="ant-btn ant-btn-sm" data-plan-drop="${item.id}">删除</button>
          </div>
        `,
      },
    ],
    response.data || [],
    "暂无套餐",
  );

  container.querySelectorAll("[data-plan-drop]").forEach((button) => {
    button.addEventListener("click", async () => {
      const response = await adminApi("/plan/drop", {
        method: "POST",
        body: { id: Number(button.dataset.planDrop) },
      });
      if (!response) return;
      notify("套餐已删除");
      await Promise.all([loadAdminPlans(), loadAdminUsers()]);
    });
  });
}

async function loadAdminServers() {
  const response = await adminApi("/server/manage/getNodes");
  const container = document.getElementById("admin-server-list");
  if (!response || !container) return;

  renderTable(
    container,
    [
      {
        title: "节点",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(item.name)}</strong>
            <span class="airboard-cell-sub">${escapeHtml(item.type)} · 倍率 ${escapeHtml(String(item.rate ?? 1))}</span>
          </div>
        `,
      },
      {
        title: "地址",
        render: (item) => `
          <div class="airboard-cell-stack">
            <strong>${escapeHtml(item.host)}:${escapeHtml(String(item.port || 0))}</strong>
            <span class="airboard-cell-sub">${escapeHtml(item.network || "-")}</span>
          </div>
        `,
      },
      {
        title: "标签 / 套餐",
        render: (item) => `
          <div class="airboard-cell-stack">
            <div class="airboard-cell-tags">${renderTagRow(item.tags || [], "无标签")}</div>
            <span class="airboard-cell-sub">套餐 ${escapeHtml((item.plan_ids || []).join(", ") || "全部")}</span>
          </div>
        `,
      },
      {
        title: "状态",
        render: (item) => `
          <div class="airboard-cell-stack">
            ${badgeStatus(item.is_online ? "在线" : "离线", item.is_online ? "success" : "error")}
            <span class="airboard-cell-sub">${item.show ? "前台展示" : "前台隐藏"}</span>
          </div>
        `,
      },
      {
        title: "操作",
        render: (item) => `
          <div class="airboard-cell-actions">
            <button type="button" class="ant-btn ant-btn-sm" data-server-drop="${item.id}" data-server-type="${escapeAttribute(item.type)}">删除</button>
          </div>
        `,
      },
    ],
    response.data || [],
    "暂无节点",
  );

  container.querySelectorAll("[data-server-drop]").forEach((button) => {
    button.addEventListener("click", async () => {
      const response = await adminApi(`/server/${button.dataset.serverType}/drop`, {
        method: "POST",
        body: { id: Number(button.dataset.serverDrop) },
      });
      if (!response) return;
      notify("节点已删除");
      await loadAdminServers();
    });
  });
}

async function loadAdminNotices() {
  const response = await adminApi("/notice/fetch", { raw: true });
  const container = document.getElementById("admin-notice-list");
  if (!container) return;
  renderNoticeCards(container, response?.data || [], { admin: true });
  container.querySelectorAll("[data-notice-drop]").forEach((button) => {
    button.addEventListener("click", async () => {
      const response = await adminApi("/notice/drop", {
        method: "POST",
        body: { id: Number(button.dataset.noticeDrop) },
      });
      if (!response) return;
      notify("公告已删除");
      await loadAdminNotices();
    });
  });
}

async function loadAdminSettings() {
  const response = await adminApi("/config/fetch");
  const form = document.getElementById("admin-settings-form");
  if (!response || !form) return;
  Object.entries(response.data || {}).forEach(([key, value]) => {
    const input = form.elements[key];
    if (input) input.value = value || "";
  });
}

async function loadAdminSubscriptions(userId) {
  const response = await adminApi(`/user/subscription/fetch?user_id=${encodeURIComponent(userId)}`);
  if (!response) return;
  renderSubscriptionLinks(document.getElementById("admin-subscription-list"), response.data || [], { admin: true });
  state.currentSubscriptionUserId = String(userId);
}

function renderSubscriptionLinks(container, links, options = {}) {
  if (!container) return;
  container.innerHTML = "";
  if (!links.length) {
    showEmpty(container, "暂无订阅链接");
    return;
  }

  links.forEach((item) => {
    const urls = item.urls || {};
    const card = document.createElement("article");
    card.className = "ant-card airboard-subscription-card";
    card.innerHTML = `
      <div class="ant-card-body">
        <div class="airboard-subscription-head">
          <div>
            <h4>${escapeHtml(item.name || "Subscription")}</h4>
            <div class="airboard-subscription-meta">
              <span class="ant-tag airboard-tag">后缀 ${escapeHtml(item.suffix || "-")}</span>
              <span class="ant-tag airboard-tag">${item.is_primary ? "主订阅" : "附加订阅"}</span>
              <span class="ant-tag airboard-tag">${item.enabled ? "启用" : "停用"}</span>
              <span class="ant-tag airboard-tag">最近使用 ${escapeHtml(formatUsedAt(item.last_used_at))}</span>
            </div>
          </div>
        </div>
        <div class="airboard-code">${escapeHtml(urls.default || "")}</div>
        <div class="airboard-link-actions">
          <button type="button" class="ant-btn ant-btn-sm" data-copy-text="${escapeAttribute(urls.default || "")}">复制默认</button>
          <a class="ant-btn ant-btn-sm" href="${escapeAttribute(urls.default || "#")}" target="_blank" rel="noreferrer">默认</a>
          <a class="ant-btn ant-btn-sm" href="${escapeAttribute(urls.v2 || "#")}" target="_blank" rel="noreferrer">V2</a>
          <a class="ant-btn ant-btn-sm" href="${escapeAttribute(urls.clash || "#")}" target="_blank" rel="noreferrer">Clash</a>
          <a class="ant-btn ant-btn-sm" href="${escapeAttribute(urls.shadowrocket || "#")}" target="_blank" rel="noreferrer">Shadowrocket</a>
          ${options.admin ? `<button type="button" class="ant-btn ant-btn-sm" data-subscription-drop="${item.id}">删除</button>` : ""}
        </div>
      </div>
    `;
    container.appendChild(card);
  });

  container.querySelectorAll("[data-copy-text]").forEach((button) => {
    button.addEventListener("click", async () => {
      if (!button.dataset.copyText) {
        notify("没有可复制的内容", true);
        return;
      }
      await copyText(button.dataset.copyText);
      notify("已复制到剪贴板");
    });
  });

  if (options.admin) {
    container.querySelectorAll("[data-subscription-drop]").forEach((button) => {
      button.addEventListener("click", async () => {
        const response = await adminApi("/user/subscription/drop", {
          method: "POST",
          body: { id: Number(button.dataset.subscriptionDrop) },
        });
        if (!response) return;
        notify("订阅链接已删除");
        if (state.currentSubscriptionUserId) {
          await loadAdminSubscriptions(state.currentSubscriptionUserId);
        }
      });
    });
  }
}

function renderNoticeCards(container, notices, options = {}) {
  if (!container) return;
  container.innerHTML = "";
  if (!notices.length) {
    showEmpty(container, "暂无公告");
    return;
  }
  notices.forEach((item) => {
    const card = document.createElement("article");
    card.className = "airboard-notice-card";
    card.innerHTML = `
      <div class="airboard-subscription-head">
        <div>
          <h4>${escapeHtml(item.title)}</h4>
          <small>${escapeHtml(formatTimestamp(item.created_at))}</small>
        </div>
        ${options.admin ? `<button type="button" class="ant-btn ant-btn-sm" data-notice-drop="${item.id}">删除</button>` : ""}
      </div>
      <p>${escapeHtml(item.content || "暂无内容")}</p>
    `;
    container.appendChild(card);
  });
}

function renderTable(container, columns, rows, emptyMessage) {
  if (!container) return;
  if (!rows.length) {
    showEmpty(container, emptyMessage);
    return;
  }
  container.innerHTML = `
    <div class="ant-table-wrapper airboard-table-wrapper">
      <div class="ant-spin-nested-loading">
        <div class="ant-spin-container">
          <div class="ant-table ant-table-default ant-table-scroll-position-left">
            <div class="ant-table-content">
              <div class="ant-table-body">
                <table>
                  <thead class="ant-table-thead">
                    <tr>${columns.map((column) => `<th>${escapeHtml(column.title)}</th>`).join("")}</tr>
                  </thead>
                  <tbody class="ant-table-tbody">
                    ${rows.map((row) => `
                      <tr>
                        ${columns.map((column) => `<td>${column.render(row)}</td>`).join("")}
                      </tr>
                    `).join("")}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `;
}

function showEmpty(container, message) {
  container.innerHTML = `<div class="airboard-empty">${escapeHtml(message)}</div>`;
}

async function api(path, options = {}) {
  const request = {
    method: options.method || "GET",
    headers: {
      "Content-Type": "application/json",
    },
  };
  if (options.auth && state.auth) {
    request.headers.Authorization = state.auth;
  }
  if (options.body) {
    request.body = JSON.stringify(options.body);
  }
  try {
    const response = await fetch(path, request);
    if (!response.ok && response.status !== 304) {
      let payload = {};
      try {
        payload = await response.json();
      } catch (_) {}
      if (!options.silent) {
        notify(payload.message || `请求失败 (${response.status})`, true);
      }
      return null;
    }
    if (response.status === 304) {
      return null;
    }
    return await response.json();
  } catch (error) {
    if (!options.silent) {
      notify(error.message || "网络请求失败", true);
    }
    return null;
  }
}

function adminApi(path, options = {}) {
  return api(`/api/v1/${state.adminPath}${path}`, { ...options, auth: true });
}

function setAuth(value) {
  state.auth = value || "";
  localStorage.setItem("airboard.auth_data", state.auth);
}

function clearAuth() {
  state.auth = "";
  state.profile = null;
  state.isAdmin = false;
  localStorage.removeItem("airboard.auth_data");
}

function notify(message, danger = false) {
  toast.textContent = message;
  toast.classList.remove("hidden");
  toast.style.background = danger ? "rgba(207, 77, 81, 0.95)" : "rgba(7, 18, 40, 0.94)";
  window.clearTimeout(notify.timer);
  notify.timer = window.setTimeout(() => toast.classList.add("hidden"), 2600);
}

function toggleView(id, visible) {
  const element = document.getElementById(id);
  if (!element) return;
  element.classList.toggle("hidden", !visible);
}

function setText(id, value) {
  const element = document.getElementById(id);
  if (element) element.textContent = value;
}

function formToObject(form) {
  const result = {};
  new FormData(form).forEach((value, key) => {
    result[key] = typeof value === "string" ? value.trim() : value;
  });
  return result;
}

function badgeStatus(text, status) {
  return `
    <span class="ant-badge ant-badge-status">
      <span class="ant-badge-status-dot ant-badge-status-${escapeHtml(status)}"></span>
      <span class="ant-badge-status-text">${escapeHtml(text)}</span>
    </span>
  `;
}

function renderTagRow(values, emptyText) {
  if (!values?.length) return `<span class="airboard-cell-sub">${escapeHtml(emptyText)}</span>`;
  return values.map((value) => `<span class="ant-tag airboard-tag">${escapeHtml(String(value))}</span>`).join("");
}

function formatTimestamp(value) {
  if (!value) return "长期有效";
  const date = new Date(Number(value) * 1000);
  if (Number.isNaN(date.getTime())) return "未知";
  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatUsedAt(value) {
  if (!value) return "未使用";
  return formatTimestamp(value);
}

function trafficText(subscribe) {
  const used = Number(subscribe.u || 0) + Number(subscribe.d || 0);
  const total = Number(subscribe.transfer_enable || 0);
  return `${formatBytes(Math.max(total - used, 0))} / ${formatBytes(total)}`;
}

function formatBytes(value) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let current = Number(value || 0);
  let index = 0;
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024;
    index += 1;
  }
  return `${current.toFixed(index === 0 ? 0 : 2)} ${units[index]}`;
}

function formatPrice(value) {
  return `¥ ${Number(value || 0).toFixed(2)}`;
}

async function copyText(text) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const input = document.createElement("textarea");
  input.value = text;
  document.body.appendChild(input);
  input.select();
  document.execCommand("copy");
  document.body.removeChild(input);
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function escapeAttribute(value) {
  return escapeHtml(value).replaceAll("`", "");
}
