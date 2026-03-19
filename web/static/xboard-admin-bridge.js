(function () {
  var adminPath =
    (window.settings && (window.settings.admin_path || window.settings.api_secure_path || window.settings.secure_path)) ||
    "/admin";
  var securePath = adminPath;
  var tokenStorageKey = "Xboard_access_token";
  var blockedRoutes = [
    adminPath + "/payment",
    adminPath + "/plugin",
    adminPath + "/theme",
    adminPath + "/notice/knowledge",
    adminPath + "/server/group",
    adminPath + "/server/route",
    adminPath + "/finance/order",
    adminPath + "/finance/coupon",
    adminPath + "/finance/gift-card",
    adminPath + "/user/ticket"
  ];
  var blockedHashRoutes = [
    "/config/plugin",
    "/config/payment",
    "/config/knowledge",
    "/config/theme",
    "/server/group",
    "/server/route",
    "/finance/order",
    "/finance/coupon",
    "/finance/gift-card",
    "/user/ticket",
    "/config/system/subscribe",
    "/config/system/invite",
    "/config/system/email",
    "/config/system/telegram",
    "/config/system/app",
    "/config/system/subscribe-template"
  ];
  var blockedHrefNeedles = [
    "/payment",
    "/plugin",
    "/theme",
    "/knowledge",
    "/server/group",
    "/server/route",
    "/finance/order",
    "/finance/coupon",
    "/finance/gift-card",
    "/user/ticket",
    "/config/system/subscribe",
    "/config/system/invite",
    "/config/system/email",
    "/config/system/telegram",
    "/config/system/app",
    "/config/system/subscribe-template"
  ];
  var blockedTexts = [
    "Payment",
    "Theme",
    "Plugin",
    "Knowledge",
    "Order",
    "Coupon",
    "Gift Card",
    "Ticket",
    "Route",
    "Group",
    "Invitation & Commission Settings",
    "Email Settings",
    "Telegram Settings",
    "APP Settings",
    "Subscribe Templates",
    "支付",
    "主题",
    "插件",
    "文档",
    "知识库",
    "订单",
    "优惠券",
    "工单",
    "路由",
    "分组",
    "邀请&佣金设置",
    "邮件设置",
    "Telegram设置",
    "APP设置",
    "订阅模板"
  ];
  var dashboardPlaceholderTexts = [
    "Today's Income",
    "Monthly Income",
    "Pending Tickets",
    "Pending Commission",
    "Revenue Overview",
    "Queue Status",
    "Recent Jobs",
    "Job Details",
    "Failed Jobs (7 days)",
    "Longest Running Queue",
    "Active Processes",
    "今日收入",
    "本月收入",
    "待处理工单",
    "待处理佣金",
    "收入概览",
    "队列状态",
    "最近任务",
    "任务明细",
    "失败任务(7天)",
    "最长运行队列",
    "活跃进程"
  ];
  var sitePlaceholderLabels = [
    "Force HTTPS",
    "Stop New User Registration",
    "Reply Wait Restriction",
    "Registration Trial",
    "Currency Unit",
    "Currency Symbol",
    "强制HTTPS",
    "停止新用户注册",
    "工单等待回复限制",
    "注册试用",
    "货币单位",
    "货币符号"
  ];
  var safePlaceholderLabels = [
    "Email Verification",
    "Disable Gmail Aliases",
    "Safe Mode",
    "Email Suffix Whitelist",
    "Enable Captcha",
    "IP Registration Limit",
    "Password Attempt Limit",
    "邮箱验证",
    "禁止使用Gmail多别名",
    "安全模式",
    "邮箱后缀白名单",
    "开启验证码",
    "IP注册限制",
    "密码尝试限制"
  ];
  var serverPlaceholderLabels = [
    "Node Pull Action Polling Interval",
    "Node Push Action Polling Interval",
    "Device Limit Mode",
    "Enable WebSocket Communication",
    "WebSocket URL",
    "Node clients that currently support WebSocket communication: Xboard Node",
    "节点拉取动作轮询间隔",
    "节点推送动作轮询间隔",
    "设备限制模式",
    "启用 WebSocket 通信",
    "WebSocket 地址",
    "目前支持 WebSocket 通信的节点端：Xboard Node"
  ];
  var userPlaceholderFieldTexts = [
    "Commission",
    "Inviter Email",
    "Inviter ID",
    "Commission Balance",
    "Commission Type",
    "Commission Rate",
    "佣金",
    "邀请人邮箱",
    "邀请人ID",
    "佣金余额",
    "佣金类型",
    "佣金比例",
    "TA的邀请",
    "邀请信息"
  ];
  var sidebarSectionLabels = [
    "Subscription",
    "订阅管理",
    "Finance",
    "财务",
    "Financial",
    "财务信息"
  ];
  var helpCardTexts = [
    "Shortcuts",
    "View Tutorial",
    "Learn how to use Pixia Airboard",
    "Need Help",
    "Open Ticket",
    "Encountered issues",
    "If you encounter any issues, please contact us through a support ticket",
    "捷径",
    "查看教程",
    "学习如何使用 Pixia Airboard",
    "遇到问题",
    "发起工单",
    "遇到问题可以通过工单与我们沟通"
  ];

  function normalizeText(value) {
    return String(value || "").replace(/\s+/g, " ").trim();
  }

  function currentHashRoute() {
    var hash = window.location.hash || "#/";
    if (hash.indexOf("#") === 0) hash = hash.slice(1);
    if (!hash) return "/";
    return hash.charAt(0) === "/" ? hash : "/" + hash;
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

  function isBlocked(pathname) {
    return blockedRoutes.some(function (prefix) {
      return pathname === prefix || pathname.indexOf(prefix + "/") === 0;
    });
  }

  function isBlockedHashRoute(route) {
    return blockedHashRoutes.some(function (prefix) {
      return route === prefix || route.indexOf(prefix + "/") === 0;
    });
  }

  function hideItem(el) {
    if (!el) return;
    var target = el.closest("a,button,li,[role='menuitem'],[role='button'],div");
    if (!target) target = el;
    target.style.display = "none";
  }

  function findBlockContainer(el) {
    var current = el;
    var fallback = el.parentElement || el;
    while (current && current !== document.body) {
      var text = normalizeText(current.textContent);
      var cls = String(current.className || "");
      if (
        text &&
        text.length < 1500 &&
        current.children &&
        current.children.length > 1 &&
        /rounded|border|shadow|card/i.test(cls)
      ) {
        return current;
      }
      if (
        text &&
        text.length < 1200 &&
        (current.matches("section,article,th,td,tr,table") ||
          /card|border|rounded|shadow|grid|space-y|table|tabs|panel/i.test(cls))
      ) {
        fallback = current;
      }
      current = current.parentElement;
    }
    return fallback;
  }

  function hideBlocksByExactText(texts) {
    var lookup = {};
    texts.forEach(function (text) {
      lookup[normalizeText(text)] = true;
    });
    document.querySelectorAll("body *").forEach(function (el) {
      var text = normalizeText(el.textContent);
      if (!text || !lookup[text]) return;
      findBlockContainer(el).style.display = "none";
    });
  }

  function hideBlocksByContainsText(texts) {
    var normalized = texts.map(normalizeText).filter(Boolean);
    document.querySelectorAll("body *").forEach(function (el) {
      var text = normalizeText(el.textContent);
      if (!text) return;
      if (
        normalized.some(function (item) {
          return text.indexOf(item) !== -1;
        })
      ) {
        findBlockContainer(el).style.display = "none";
      }
    });
  }

  function hideFieldBlocksByLabelTexts(texts) {
    var lookup = {};
    texts.forEach(function (text) {
      lookup[normalizeText(text)] = true;
    });
    document.querySelectorAll("body *").forEach(function (el) {
      var text = normalizeText(el.textContent);
      if (!text || !lookup[text]) return;
      var current = el;
      while (current && current !== document.body) {
        var currentText = normalizeText(current.textContent);
        var fields =
          current.querySelectorAll &&
          current.querySelectorAll("input, textarea, select, button, [role='combobox']");
        var fieldCount = fields ? fields.length : 0;
        if (fieldCount > 0 && fieldCount <= 2 && currentText.length < 800) {
          current.style.display = "none";
          return;
        }
        current = current.parentElement;
      }
    });
  }

  function hideTableColumnsByHeaderTexts(texts) {
    var lookup = {};
    texts.forEach(function (text) {
      lookup[normalizeText(text)] = true;
    });
    document.querySelectorAll("table").forEach(function (table) {
      var headers = Array.prototype.slice.call(table.querySelectorAll("thead th"));
      headers.forEach(function (header, index) {
        if (!lookup[normalizeText(header.textContent)]) return;
        header.style.display = "none";
        table.querySelectorAll("tbody tr").forEach(function (row) {
          if (row.children[index]) {
            row.children[index].style.display = "none";
          }
        });
      });
    });
  }

  function hideSidebarSectionLabels(texts) {
    var lookup = {};
    texts.forEach(function (text) {
      lookup[normalizeText(text)] = true;
    });
    document.querySelectorAll("body *").forEach(function (el) {
      var text = normalizeText(el.textContent);
      if (!text || !lookup[text]) return;
      var sidebarRoot = el.closest("aside, nav, [data-sidebar]");
      if (!sidebarRoot) {
        var probe = el.parentElement;
        while (probe && probe !== document.body) {
          var probeCls = String(probe.className || "");
          if (/sidebar|menu|nav|aside/i.test(probeCls)) {
            sidebarRoot = probe;
            break;
          }
          probe = probe.parentElement;
        }
      }
      if (!sidebarRoot) return;
      if (el.querySelector && el.querySelector("a[href], button, input, textarea, select")) return;
      var current = el;
      while (current && current !== sidebarRoot) {
        if (current.matches("li,div,p,span")) {
          current.style.display = "none";
          return;
        }
        current = current.parentElement;
      }
    });
  }

  function applyContextualHides() {
    var route = currentHashRoute();
    hideSidebarSectionLabels(sidebarSectionLabels);
    if (route === "/" || route === "") {
      hideBlocksByExactText(dashboardPlaceholderTexts);
      hideBlocksByContainsText(helpCardTexts);
      return;
    }
    if (route === "/config/system") {
      hideFieldBlocksByLabelTexts(sitePlaceholderLabels);
      return;
    }
    if (route.indexOf("/config/system/safe") === 0) {
      hideFieldBlocksByLabelTexts(safePlaceholderLabels);
      return;
    }
    if (route.indexOf("/config/system/server") === 0) {
      hideFieldBlocksByLabelTexts(serverPlaceholderLabels);
      return;
    }
    if (route.indexOf("/user/manage") === 0) {
      hideTableColumnsByHeaderTexts(["Commission", "佣金"]);
      hideFieldBlocksByLabelTexts(userPlaceholderFieldTexts);
    }
  }

  function scan() {
    var clickable = document.querySelectorAll("a[href], button, [role='menuitem'], [role='button']");
    clickable.forEach(function (el) {
      var href = el.getAttribute("href") || "";
      var text = normalizeText(el.textContent);
      if (
        blockedHrefNeedles.some(function (item) {
          return href.indexOf(item) !== -1;
        }) ||
        blockedTexts.some(function (item) {
          return text === normalizeText(item);
        })
      ) {
        hideItem(el);
      }
    });
    applyContextualHides();
    ensureSubscriptionButton();
  }

  function redirectIfNeeded() {
    var pathname = window.location.pathname.replace(/\/+$/, "") || "/";
    var route = currentHashRoute();
    if (isBlocked(pathname)) {
      window.location.replace(securePath);
      return;
    }
    if (isBlockedHashRoute(route)) {
      var fallback = route.indexOf("/config/system/") === 0 ? "#/config/system" : "#/";
      window.location.replace(adminPath + fallback);
    }
  }

  function ensureSubscriptionButton() {
    if (!readAccessToken()) return;
    if (document.getElementById("airboard-subscription-entry")) return;
    var button = document.createElement("a");
    button.id = "airboard-subscription-entry";
    button.href = adminPath + "/subscription-links";
    button.textContent = "订阅链接";
    button.className = "fixed bottom-6 right-6 z-[90] inline-flex items-center rounded-full bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-lg transition-opacity hover:opacity-90";
    document.body.appendChild(button);
  }

  redirectIfNeeded();
  window.addEventListener("popstate", redirectIfNeeded);
  window.addEventListener("hashchange", function () {
    redirectIfNeeded();
    setTimeout(scan, 50);
  });
  var originalPushState = history.pushState;
  history.pushState = function () {
    originalPushState.apply(history, arguments);
    redirectIfNeeded();
    setTimeout(scan, 50);
  };
  var originalReplaceState = history.replaceState;
  history.replaceState = function () {
    originalReplaceState.apply(history, arguments);
    redirectIfNeeded();
    setTimeout(scan, 50);
  };
  document.addEventListener("DOMContentLoaded", function () {
    scan();
    var observer = new MutationObserver(scan);
    observer.observe(document.body, { childList: true, subtree: true });
  });
})();
