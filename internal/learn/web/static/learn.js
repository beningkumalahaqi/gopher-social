// GopherSocial Learn: small, dependency-free enhancements.
// The site works without this file; the script only removes friction.
(function () {
  "use strict";

  const html = document.documentElement;
  const meta = document.querySelector('meta[name="csrf-token"]');
  const csrf = meta ? meta.content : "";

  function toast(message) {
    const region = document.getElementById("toast-region");
    if (!region) return;
    const node = document.createElement("div");
    node.className = "toast";
    node.textContent = message;
    region.appendChild(node);
    window.setTimeout(function () {
      node.remove();
    }, 4200);
  }

  function postJSON(url, body) {
    return fetch(url, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "X-CSRF-Token": csrf
      },
      body: new URLSearchParams(body).toString()
    });
  }

  // Mobile rail -----------------------------------------------------------
  const rail = document.getElementById("rail");
  const scrim = document.querySelector(".rail-scrim");
  const openButton = document.querySelector("[data-rail-open]");

  function openRail() {
    if (!rail) return;
    rail.classList.add("is-open");
    if (scrim) scrim.hidden = false;
    if (openButton) openButton.setAttribute("aria-expanded", "true");
    const first = rail.querySelector("a, button");
    if (first) first.focus();
  }

  function closeRail() {
    if (!rail) return;
    rail.classList.remove("is-open");
    if (scrim) scrim.hidden = true;
    if (openButton) openButton.setAttribute("aria-expanded", "false");
  }

  if (openButton) openButton.addEventListener("click", openRail);
  document.querySelectorAll("[data-rail-close]").forEach(function (node) {
    node.addEventListener("click", closeRail);
  });
  if (rail) {
    rail.addEventListener("click", function (event) {
      if (event.target.closest("a") && window.matchMedia("(max-width: 940px)").matches) {
        closeRail();
      }
    });
  }
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") closeRail();
  });

  // Preferences -----------------------------------------------------------
  function currentPrefs() {
    const form = document.querySelector("[data-settings-form]");
    if (form) {
      const motion = form.querySelector('input[name="reduced_motion"]');
      return {
        theme: form.querySelector('select[name="theme"]').value,
        font_scale: form.querySelector('select[name="font_scale"]').value,
        reduced_motion: motion && motion.checked ? "true" : "false"
      };
    }
    return {
      theme: html.dataset.theme || "auto",
      font_scale: html.dataset.font || "normal",
      reduced_motion: html.dataset.motion === "reduce" ? "true" : "false"
    };
  }

  function applyPrefs(prefs) {
    html.dataset.theme = prefs.theme;
    html.dataset.font = prefs.font_scale;
    if (prefs.reduced_motion === "true") {
      html.dataset.motion = "reduce";
    } else {
      delete html.dataset.motion;
    }
    const form = document.querySelector("[data-settings-form]");
    if (form) {
      const theme = form.querySelector('select[name="theme"]');
      const font = form.querySelector('select[name="font_scale"]');
      const motion = form.querySelector('input[name="reduced_motion"]');
      if (theme) theme.value = prefs.theme;
      if (font) font.value = prefs.font_scale;
      if (motion) motion.checked = prefs.reduced_motion === "true";
    }
  }

  function savePrefs(prefs, statusNode) {
    return postJSON("/learn/preferences", prefs)
      .then(function (response) {
        if (!response.ok) throw new Error("save failed");
        applyPrefs(prefs);
        if (statusNode) {
          statusNode.textContent = "Saved";
          window.setTimeout(function () {
            statusNode.textContent = "";
          }, 2500);
        }
      })
      .catch(function () {
        if (statusNode) statusNode.textContent = "Not saved";
        toast("That setting could not be saved. Reload and try again.");
      });
  }

  const themeButton = document.querySelector("[data-theme-toggle]");
  if (themeButton) {
    themeButton.addEventListener("click", function () {
      const prefs = currentPrefs();
      const order = ["auto", "light", "dark"];
      const next = order[(order.indexOf(prefs.theme) + 1) % order.length];
      prefs.theme = next;
      savePrefs(prefs).then(function () {
        toast("Theme: " + next);
      });
    });
  }

  const settingsForm = document.querySelector("[data-settings-form]");
  if (settingsForm) {
    settingsForm.addEventListener("submit", function (event) {
      event.preventDefault();
      const status = settingsForm.querySelector("[data-settings-status]");
      const formData = new FormData(settingsForm);
      const prefs = {
        theme: formData.get("theme"),
        font_scale: formData.get("font_scale"),
        reduced_motion: formData.get("reduced_motion") === "true" ? "true" : "false"
      };
      const selectTheme = settingsForm.querySelector('select[name="theme"]');
      const selectFont = settingsForm.querySelector('select[name="font_scale"]');
      const motion = settingsForm.querySelector('input[name="reduced_motion"]');
      if (selectTheme) selectTheme.value = prefs.theme;
      if (selectFont) selectFont.value = prefs.font_scale;
      if (motion) motion.checked = prefs.reduced_motion === "true";
      savePrefs(prefs, status);
    });
  }

  // Reading progress ------------------------------------------------------
  let readingRoot = null;
  let lastSent = -1;
  let readTimer = null;

  function setupReading() {
    readingRoot = document.querySelector("[data-reading-root]");
    const hidden = document.querySelector("[data-reading-percent]");
    lastSent = hidden ? Number(hidden.value) || 0 : 0;
  }

  function currentPercent() {
    if (!readingRoot) return 0;
    const rect = readingRoot.getBoundingClientRect();
    const start = window.scrollY + rect.top;
    const consumed = window.scrollY + window.innerHeight * 0.75 - start;
    const total = readingRoot.offsetHeight || 1;
    return Math.max(0, Math.min(100, Math.round((consumed / total) * 100)));
  }

  function updateReadingUI(percent) {
    const label = document.querySelector("[data-reading-label]");
    const hidden = document.querySelector("[data-reading-percent]");
    const meter = document.querySelector(".progress-card .meter span");
    if (hidden) hidden.value = String(percent);
    if (meter && label && !label.textContent.trim().toLowerCase().startsWith("completed")) {
      meter.style.width = percent + "%";
      label.textContent = percent + "% read";
    }
  }

  function sendReading(percent) {
    const shell = document.getElementById("lesson-content");
    if (!shell) return;
    const slug = shell.dataset.lesson;
    if (!slug) return;
    postJSON("/learn/progress", { lesson_slug: slug, percent: String(percent) })
      .then(function (response) {
        if (response.ok) {
          lastSent = percent;
          updateReadingUI(percent);
        }
      })
      .catch(function () {
        /* Reading progress is best effort; the explicit button still works. */
      });
  }

  window.addEventListener(
    "scroll",
    function () {
      if (!readingRoot || readTimer) return;
      readTimer = window.setTimeout(function () {
        readTimer = null;
        const percent = currentPercent();
        if (percent >= lastSent + 5) sendReading(percent);
      }, 1200);
    },
    { passive: true }
  );

  // Copy buttons ----------------------------------------------------------
  function addCopyButtons() {
    document.querySelectorAll(".prose pre, .code-block").forEach(function (pre) {
      if (pre.dataset.copyReady === "true" || !pre.querySelector("code")) return;
      pre.dataset.copyReady = "true";
      const button = document.createElement("button");
      button.type = "button";
      button.className = "copy-button";
      button.textContent = "Copy";
      button.addEventListener("click", function () {
        const code = pre.querySelector("code");
        const text = code ? code.textContent : pre.textContent;
        if (!navigator.clipboard) return;
        navigator.clipboard.writeText(text).then(function () {
          button.textContent = "Copied";
          window.setTimeout(function () {
            button.textContent = "Copy";
          }, 1600);
        });
      });
      pre.appendChild(button);
    });
  }

  document.body.addEventListener("htmx:afterSwap", function (event) {
    setupReading();
    addCopyButtons();
    const heading = document.querySelector(".lesson-head h1");
    if (heading && event.detail && event.detail.target && event.detail.target.id === "lesson-content") {
      document.title = heading.textContent.trim() + " · GopherSocial Learn";
      window.scrollTo({ top: 0, behavior: "auto" });
    }
  });

  document.body.addEventListener("htmx:responseError", function (event) {
    const status = event.detail && event.detail.xhr ? event.detail.xhr.status : 0;
    if (status === 403) {
      toast("That form expired. Reload the page and submit again.");
    } else if (status === 429) {
      toast("Too many requests. Wait a moment and try again.");
    } else {
      toast("That request did not complete. Try again.");
    }
  });

  // Search shortcut -------------------------------------------------------
  document.addEventListener("keydown", function (event) {
    const tag = (event.target.tagName || "").toLowerCase();
    const typing = tag === "input" || tag === "textarea" || tag === "select" || event.target.isContentEditable;
    if (event.key === "/" && !typing && !event.metaKey && !event.ctrlKey && !event.altKey) {
      const input = document.querySelector("[data-search-input]");
      if (input) {
        event.preventDefault();
        if (window.matchMedia("(max-width: 940px)").matches) openRail();
        input.focus();
      }
    }
  });

  setupReading();
  addCopyButtons();
})();
