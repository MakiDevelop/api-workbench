const THEMES = [
  { id: "dark", label: "Dark" },
  { id: "light", label: "Light" },
  { id: "matrix", label: "The Matrix" },
];

const themeVars = {
  dark: {
    "--bg": "#181412",
    "--bg-soft": "#211b18",
    "--panel": "#1f1916",
    "--panel-muted": "#171210",
    "--panel-elevated": "#29211d",
    "--line": "rgba(255, 255, 255, 0.08)",
    "--line-strong": "rgba(255, 255, 255, 0.14)",
    "--text": "#f3eee9",
    "--muted": "#ac9f96",
    "--accent": "#ff6c37",
    "--accent-soft": "rgba(255, 108, 55, 0.14)",
    "--success": "#70d59a",
    "--warning": "#ffc870",
    "--danger": "#ff8a7a",
    "--shadow": "0 24px 72px rgba(0, 0, 0, 0.28)",
    "--bg-gradient": "linear-gradient(180deg, #16110f 0%, #1a1412 100%)",
    "--bg-radial": "radial-gradient(circle at top right, rgba(255, 108, 55, 0.12), transparent 24%)",
    "--topbar-bg": "rgba(18, 14, 12, 0.92)",
    "--sidebar-bg": "rgba(19, 15, 13, 0.92)",
    "--explorer-bg": "rgba(22, 17, 15, 0.96)",
    "--editor-bg": "linear-gradient(180deg, rgba(24, 19, 17, 0.98) 0%, rgba(22, 17, 15, 0.98) 100%)",
    "--input-bg": "#171210",
    "--pre-bg": "#110d0b",
    "--statusbar-bg": "rgba(17, 13, 11, 0.96)",
    "--statusbar-error-bg": "rgba(60, 24, 18, 0.92)",
    "--brand-gradient": "linear-gradient(135deg, #ff8d59 0%, #ff5b28 100%)",
    "--brand-text": "#1f130f",
    "--send-gradient": "linear-gradient(180deg, #ff7a46 0%, #ff6030 100%)",
    "--send-border": "rgba(255, 108, 55, 0.45)",
    "--send-text": "#230f07",
    "--matrix-rain": "none",
  },
  light: {
    "--bg": "#f5f3f0",
    "--bg-soft": "#eae7e3",
    "--panel": "#ffffff",
    "--panel-muted": "#f8f6f3",
    "--panel-elevated": "#ffffff",
    "--line": "rgba(0, 0, 0, 0.1)",
    "--line-strong": "rgba(0, 0, 0, 0.18)",
    "--text": "#1a1412",
    "--muted": "#6b5f56",
    "--accent": "#e0520f",
    "--accent-soft": "rgba(224, 82, 15, 0.1)",
    "--success": "#1a8a47",
    "--warning": "#b87900",
    "--danger": "#c93a2a",
    "--shadow": "0 24px 72px rgba(0, 0, 0, 0.06)",
    "--bg-gradient": "linear-gradient(180deg, #f5f3f0 0%, #eae7e3 100%)",
    "--bg-radial": "radial-gradient(circle at top right, rgba(224, 82, 15, 0.06), transparent 24%)",
    "--topbar-bg": "rgba(255, 255, 255, 0.95)",
    "--sidebar-bg": "rgba(248, 246, 243, 0.95)",
    "--explorer-bg": "rgba(245, 243, 240, 0.98)",
    "--editor-bg": "linear-gradient(180deg, #ffffff 0%, #f8f6f3 100%)",
    "--input-bg": "#f0ede9",
    "--pre-bg": "#f0ede9",
    "--statusbar-bg": "rgba(248, 246, 243, 0.98)",
    "--statusbar-error-bg": "rgba(255, 240, 238, 0.95)",
    "--brand-gradient": "linear-gradient(135deg, #ff8d59 0%, #ff5b28 100%)",
    "--brand-text": "#ffffff",
    "--send-gradient": "linear-gradient(180deg, #ff7a46 0%, #ff6030 100%)",
    "--send-border": "rgba(224, 82, 15, 0.35)",
    "--send-text": "#ffffff",
    "--matrix-rain": "none",
  },
  matrix: {
    "--bg": "#0a0a0a",
    "--bg-soft": "#0d0d0d",
    "--panel": "#0c0c0c",
    "--panel-muted": "#080808",
    "--panel-elevated": "#111111",
    "--line": "rgba(0, 255, 65, 0.12)",
    "--line-strong": "rgba(0, 255, 65, 0.2)",
    "--text": "#00ff41",
    "--muted": "#009926",
    "--accent": "#00ff41",
    "--accent-soft": "rgba(0, 255, 65, 0.1)",
    "--success": "#00ff41",
    "--warning": "#ccff00",
    "--danger": "#ff3333",
    "--shadow": "0 24px 72px rgba(0, 255, 65, 0.06)",
    "--bg-gradient": "linear-gradient(180deg, #0a0a0a 0%, #050505 100%)",
    "--bg-radial": "radial-gradient(circle at top right, rgba(0, 255, 65, 0.06), transparent 24%)",
    "--topbar-bg": "rgba(5, 5, 5, 0.95)",
    "--sidebar-bg": "rgba(8, 8, 8, 0.95)",
    "--explorer-bg": "rgba(10, 10, 10, 0.98)",
    "--editor-bg": "linear-gradient(180deg, rgba(12, 12, 12, 0.98) 0%, rgba(8, 8, 8, 0.98) 100%)",
    "--input-bg": "#060606",
    "--pre-bg": "#050505",
    "--statusbar-bg": "rgba(5, 5, 5, 0.98)",
    "--statusbar-error-bg": "rgba(40, 0, 0, 0.95)",
    "--brand-gradient": "linear-gradient(135deg, #00ff41 0%, #009926 100%)",
    "--brand-text": "#000000",
    "--send-gradient": "linear-gradient(180deg, #00ff41 0%, #009926 100%)",
    "--send-border": "rgba(0, 255, 65, 0.4)",
    "--send-text": "#000000",
    "--matrix-rain": "block",
  },
};

let currentTheme = localStorage.getItem("apiw.theme") || "dark";

export function getTheme() {
  return currentTheme;
}

export function setTheme(themeId) {
  if (!themeVars[themeId]) return;
  currentTheme = themeId;
  localStorage.setItem("apiw.theme", themeId);
  applyTheme();
}

export function applyTheme() {
  const vars = themeVars[currentTheme] || themeVars.dark;
  const root = document.documentElement;
  for (const [key, value] of Object.entries(vars)) {
    root.style.setProperty(key, value);
  }

  root.style.colorScheme = currentTheme === "light" ? "light" : "dark";

  // Background
  document.body.style.background = `${vars["--bg-radial"]}, ${vars["--bg-gradient"]}`;

  // Matrix rain canvas
  const existing = document.getElementById("matrix-rain");
  if (currentTheme === "matrix") {
    if (!existing) createMatrixRain();
  } else if (existing) {
    existing.remove();
    if (window._matrixInterval) {
      cancelAnimationFrame(window._matrixInterval);
      window._matrixInterval = null;
    }
  }
}

function createMatrixRain() {
  const canvas = document.createElement("canvas");
  canvas.id = "matrix-rain";
  canvas.style.cssText =
    "position:fixed;top:0;left:0;width:100%;height:100%;pointer-events:none;z-index:0;opacity:0.08;";
  document.body.prepend(canvas);

  const ctx = canvas.getContext("2d");
  let columns, drops;

  function resize() {
    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;
    columns = Math.floor(canvas.width / 14);
    drops = Array.from({ length: columns }, () => Math.random() * canvas.height);
  }

  resize();
  window.addEventListener("resize", resize);

  const chars = "アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲン0123456789ABCDEF{}[]<>/\\";

  function draw() {
    ctx.fillStyle = "rgba(0, 0, 0, 0.05)";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = "#00ff41";
    ctx.font = "14px monospace";

    for (let i = 0; i < columns; i++) {
      const char = chars[Math.floor(Math.random() * chars.length)];
      ctx.fillText(char, i * 14, drops[i]);
      if (drops[i] > canvas.height && Math.random() > 0.975) {
        drops[i] = 0;
      }
      drops[i] += 14;
    }
    window._matrixInterval = requestAnimationFrame(draw);
  }

  draw();
}

export { THEMES };
