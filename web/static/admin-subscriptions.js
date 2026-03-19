(function () {
  var adminPath =
    (window.settings && (window.settings.admin_path || window.settings.api_secure_path || window.settings.secure_path)) ||
    "/admin";
  var apiPath =
    (window.settings && (window.settings.api_secure_path || window.settings.admin_path || window.settings.secure_path)) ||
    "";
  var tokenStorageKey = "Xboard_access_token";
  var apiBase = "/api/v2" + apiPath;
  var state = {
    users: [],
    filteredUsers: [],
    selectedUser: null,
    links: []
  };

  function escapeHTML(value) {
    return String(value || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/\"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function readAccessToken() {
    try {
      var raw = window.localStorage.getItem(tokenStorageKey);
      if (!raw) return "";
      var parsed = JSON.parse(raw);
      return parsed && parsed.value ? parsed.value : "";
    } catch (err) {
      return "";
    }
  }

  function authHeaders(extra) {
    return Object.assign(
      {
        Authorization: readAccessToken(),
        "Content-Type": "application/json",
        "Content-Language": window.localStorage.getItem("i18nextLng") || "zh-CN"
      },
      extra || {}
    );
  }

  async function request(path, options) {
    var response = await fetch(apiBase + path, Object.assign({ headers: authHeaders() }, options || {}));
    if (response.status === 401 || response.status === 403) {
      redirectToSignIn();
      throw new Error("未登录");
    }
    var payload = await response.json();
    if (!response.ok || (payload && typeof payload.code === "number" && payload.code >= 400)) {
      throw new Error((payload && payload.message) || "请求失败");
    }
    return payload;
  }

  function redirectToSignIn() {
    window.location.replace(adminPath + "#/sign-in");
  }

  function formatBytes(value) {
    var units = ["B", "KB", "MB", "GB", "TB"];
    var size = Number(value || 0);
    var unit = 0;
    while (size >= 1024 && unit < units.length - 1) {
      size /= 1024;
      unit += 1;
    }
    return size.toFixed(2) + " " + units[unit];
  }

  function formatTime(value) {
    if (!value) return "-";
    var date = new Date(Number(value) * 1000);
    if (Number.isNaN(date.getTime())) return "-";
    return date.toLocaleString("zh-CN");
  }

  function selectedLinkMap() {
    return new Map(state.links.map(function (link) { return [String(link.id || link.temp_id), link]; }));
  }

  function renderUsers() {
    var list = document.getElementById("user-list");
    if (!list) return;
    if (!state.filteredUsers.length) {
      list.innerHTML = '<div class="p-6 text-sm text-muted-foreground">没有匹配的用户。</div>';
      return;
    }
    list.innerHTML = state.filteredUsers.map(function (user) {
      var active = state.selectedUser && state.selectedUser.id === user.id;
      return '' +
        '<button type="button" data-user-id="' + user.id + '" class="flex w-full flex-col gap-2 px-4 py-4 text-left transition-colors hover:bg-accent/50 ' + (active ? 'bg-accent/70' : '') + '">' +
        '  <div class="flex items-center justify-between gap-3">' +
        '    <div class="min-w-0">' +
        '      <div class="truncate text-sm font-medium">' + escapeHTML(user.email) + '</div>' +
        '      <div class="truncate text-xs text-muted-foreground">套餐：' + escapeHTML(user.plan_name || "-") + ' / 到期：' + escapeHTML(formatTime(user.expired_at)) + '</div>' +
        '    </div>' +
        '    <span class="rounded-full border px-2 py-0.5 text-xs">' + escapeHTML(user.subscribe_suffix || "-") + '</span>' +
        '  </div>' +
        '  <div class="flex flex-wrap gap-3 text-xs text-muted-foreground">' +
        '    <span>流量 ' + escapeHTML(formatBytes(user.total_used || 0)) + ' / ' + escapeHTML(formatBytes(user.transfer_enable || 0)) + '</span>' +
        '    <span>' + (user.banned ? "已封禁" : "正常") + '</span>' +
        '  </div>' +
        '</button>';
    }).join("");
  }

  function renderLinks() {
    var container = document.getElementById("subscription-detail");
    if (!container) return;
    if (!state.selectedUser) {
      container.innerHTML = '' +
        '<div class="border-b p-4">' +
        '  <h2 class="text-lg font-medium">订阅详情</h2>' +
        '  <p class="text-sm text-muted-foreground">先从左侧选择一个用户。</p>' +
        '</div>' +
        '<div class="flex flex-1 items-center justify-center p-8 text-sm text-muted-foreground">暂未选择用户</div>';
      return;
    }

    container.innerHTML = '' +
      '<div class="border-b p-4">' +
      '  <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">' +
      '    <div>' +
      '      <h2 class="text-lg font-medium">' + escapeHTML(state.selectedUser.email) + '</h2>' +
      '      <p class="text-sm text-muted-foreground">当前套餐：' + escapeHTML(state.selectedUser.plan_name || "-") + '，默认订阅后缀：' + escapeHTML(state.selectedUser.subscribe_suffix || "-") + '</p>' +
      '    </div>' +
      '    <div class="flex gap-2">' +
      '      <button id="add-link" type="button" class="inline-flex items-center rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground">新增链接</button>' +
      '      <button id="reset-links" type="button" class="inline-flex items-center rounded-md border px-3 py-2 text-sm font-medium">重置全部</button>' +
      '    </div>' +
      '  </div>' +
      '</div>' +
      '<div class="space-y-4 p-4">' +
      '  <div class="grid gap-3 rounded-xl border p-4 text-sm md:grid-cols-2">' +
      '    <div><span class="text-muted-foreground">到期时间：</span>' + escapeHTML(formatTime(state.selectedUser.expired_at)) + '</div>' +
      '    <div><span class="text-muted-foreground">总流量：</span>' + escapeHTML(formatBytes(state.selectedUser.transfer_enable || 0)) + '</div>' +
      '    <div><span class="text-muted-foreground">已用上行：</span>' + escapeHTML(formatBytes(state.selectedUser.u || 0)) + '</div>' +
      '    <div><span class="text-muted-foreground">已用下行：</span>' + escapeHTML(formatBytes(state.selectedUser.d || 0)) + '</div>' +
      '  </div>' +
      '  <div id="link-list" class="space-y-4"></div>' +
      '</div>';

    var list = document.getElementById("link-list");
    list.innerHTML = state.links.map(function (link, index) {
      var key = String(link.id || link.temp_id || index);
      var urls = link.urls || {};
      return '' +
        '<section class="rounded-xl border p-4" data-link-key="' + escapeHTML(key) + '">' +
        '  <div class="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">' +
        '    <div>' +
        '      <h3 class="text-sm font-medium">链接 #' + (index + 1) + '</h3>' +
        '      <p class="text-xs text-muted-foreground">留空后缀可由系统自动生成。主链接只能有一个。</p>' +
        '    </div>' +
        '    <button type="button" class="delete-link inline-flex items-center rounded-md border px-3 py-2 text-sm font-medium text-destructive">删除</button>' +
        '  </div>' +
        '  <div class="grid gap-3 md:grid-cols-2">' +
        '    <label class="space-y-1 text-sm"><span class="text-muted-foreground">名称</span><input class="link-name flex h-10 w-full rounded-md border bg-background px-3 py-2 text-sm" value="' + escapeHTML(link.name || "") + '"></label>' +
        '    <label class="space-y-1 text-sm"><span class="text-muted-foreground">订阅后缀</span><input class="link-suffix flex h-10 w-full rounded-md border bg-background px-3 py-2 text-sm" value="' + escapeHTML(link.suffix || "") + '"></label>' +
        '    <label class="flex items-center gap-2 text-sm"><input class="link-primary h-4 w-4" type="checkbox" ' + (link.is_primary ? 'checked' : '') + '><span>设为主链接</span></label>' +
        '    <label class="flex items-center gap-2 text-sm"><input class="link-enabled h-4 w-4" type="checkbox" ' + (link.enabled ? 'checked' : '') + '><span>启用</span></label>' +
        '  </div>' +
        '  <div class="mt-4 space-y-2 rounded-lg bg-muted/40 p-3 text-xs">' +
        '    <div><span class="text-muted-foreground">默认：</span><a class="text-primary hover:underline" target="_blank" href="' + escapeHTML(urls.default || "#") + '">' + escapeHTML(urls.default || "-") + '</a></div>' +
        '    <div><span class="text-muted-foreground">Clash：</span><a class="text-primary hover:underline" target="_blank" href="' + escapeHTML(urls.clash || "#") + '">' + escapeHTML(urls.clash || "-") + '</a></div>' +
        '    <div><span class="text-muted-foreground">Shadowrocket：</span><a class="text-primary hover:underline" target="_blank" href="' + escapeHTML(urls.shadowrocket || "#") + '">' + escapeHTML(urls.shadowrocket || "-") + '</a></div>' +
        '  </div>' +
        '  <div class="mt-4 flex justify-end">' +
        '    <button type="button" class="save-link inline-flex items-center rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground">保存</button>' +
        '  </div>' +
        '</section>';
    }).join("");
  }

  function attachEvents() {
    document.getElementById("user-list").addEventListener("click", function (event) {
      var target = event.target.closest("[data-user-id]");
      if (!target) return;
      var userID = Number(target.getAttribute("data-user-id"));
      var user = state.users.find(function (item) { return item.id === userID; });
      if (!user) return;
      state.selectedUser = user;
      renderUsers();
      loadLinks(userID);
    });

    document.getElementById("user-search").addEventListener("input", function (event) {
      var keyword = String(event.target.value || "").trim().toLowerCase();
      state.filteredUsers = state.users.filter(function (user) {
        return !keyword ||
          String(user.email || "").toLowerCase().indexOf(keyword) !== -1 ||
          String(user.subscribe_suffix || "").toLowerCase().indexOf(keyword) !== -1;
      });
      renderUsers();
    });

    document.body.addEventListener("click", function (event) {
      if (event.target.id === "add-link") {
        state.links.push({
          temp_id: Date.now(),
          name: "Subscription",
          suffix: "",
          is_primary: state.links.length === 0,
          enabled: true,
          urls: {}
        });
        renderLinks();
        return;
      }
      if (event.target.id === "reset-links") {
        resetLinks();
        return;
      }
      if (event.target.classList.contains("save-link")) {
        var section = event.target.closest("[data-link-key]");
        if (section) saveLink(section);
        return;
      }
      if (event.target.classList.contains("delete-link")) {
        var current = event.target.closest("[data-link-key]");
        if (current) deleteLink(current);
      }
    });
  }

  async function loadUsers() {
    var payload = await request("/user/fetch", {
      method: "POST",
      body: JSON.stringify({
        current: 1,
        pageSize: 200,
        sort: "created_at",
        sort_type: "DESC"
      })
    });
    state.users = Array.isArray(payload.data) ? payload.data : [];
    state.filteredUsers = state.users.slice();
    if (!state.selectedUser && state.filteredUsers.length) {
      state.selectedUser = state.filteredUsers[0];
      loadLinks(state.selectedUser.id);
    }
    renderUsers();
  }

  async function loadLinks(userID) {
    var payload = await request("/user/subscription/fetch?user_id=" + encodeURIComponent(userID));
    state.links = Array.isArray(payload.data) ? payload.data : [];
    renderLinks();
  }

  async function saveLink(section) {
    if (!state.selectedUser) return;
    var key = section.getAttribute("data-link-key");
    var linksByKey = selectedLinkMap();
    var current = linksByKey.get(key) || {};
    var payload = {
      id: current.id || 0,
      user_id: state.selectedUser.id,
      name: section.querySelector(".link-name").value,
      suffix: section.querySelector(".link-suffix").value,
      is_primary: section.querySelector(".link-primary").checked,
      enabled: section.querySelector(".link-enabled").checked
    };
    await request("/user/subscription/save", {
      method: "POST",
      body: JSON.stringify(payload)
    });
    await loadUsers();
    await loadLinks(state.selectedUser.id);
  }

  async function deleteLink(section) {
    if (!state.selectedUser) return;
    var key = section.getAttribute("data-link-key");
    var linksByKey = selectedLinkMap();
    var current = linksByKey.get(key);
    if (!current) {
      state.links = state.links.filter(function (item) {
        return String(item.temp_id) !== key;
      });
      renderLinks();
      return;
    }
    await request("/user/subscription/drop", {
      method: "POST",
      body: JSON.stringify({ id: current.id })
    });
    await loadUsers();
    await loadLinks(state.selectedUser.id);
  }

  async function resetLinks() {
    if (!state.selectedUser) return;
    await request("/user/subscription/reset", {
      method: "POST",
      body: JSON.stringify({ user_id: state.selectedUser.id })
    });
    await loadUsers();
    await loadLinks(state.selectedUser.id);
  }

  async function bootstrap() {
    if (!readAccessToken()) {
      redirectToSignIn();
      return;
    }
    attachEvents();
    await loadUsers();
  }

  bootstrap().catch(function (error) {
    var detail = document.getElementById("subscription-detail");
    if (detail) {
      detail.innerHTML = '<div class="border-b p-4"><h2 class="text-lg font-medium">订阅详情</h2></div><div class="p-6 text-sm text-destructive">' + escapeHTML(error.message || "加载失败") + '</div>';
    }
  });
})();
