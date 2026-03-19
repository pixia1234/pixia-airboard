(function () {
  var blockedRoutes = ["/order", "/ticket", "/knowledge", "/invite"];
  var blockedHrefs = ["/order", "/ticket", "/knowledge", "/invite"];
  var blockedTexts = ["我的订单", "订单", "我的工单", "工单", "使用文档", "文档", "我的邀请", "邀请"];
  var redirectTarget = "/profile";

  function isBlocked(pathname) {
    return blockedRoutes.some(function (prefix) {
      return pathname === prefix || pathname.indexOf(prefix + "/") === 0;
    });
  }

  function redirectIfNeeded() {
    var pathname = window.location.pathname.replace(/\/+$/, "") || "/";
    if (isBlocked(pathname)) {
      window.location.replace(redirectTarget);
    }
  }

  function hideItem(el) {
    if (!el) return;
    var target = el.closest("a,button,li,[role='menuitem'],[role='button'],div");
    if (!target) target = el;
    target.style.display = "none";
  }

  function scan() {
    var clickable = document.querySelectorAll("a[href], button, [role='menuitem'], [role='button']");
    clickable.forEach(function (el) {
      var href = el.getAttribute("href") || "";
      var text = (el.textContent || "").trim();
      if (
        blockedHrefs.some(function (item) {
          return href === item || href.indexOf(item + "/") === 0;
        }) ||
        blockedTexts.some(function (item) {
          return text === item || text.indexOf(item) !== -1;
        })
      ) {
        hideItem(el);
      }
    });
  }

  redirectIfNeeded();
  window.addEventListener("popstate", redirectIfNeeded);
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
