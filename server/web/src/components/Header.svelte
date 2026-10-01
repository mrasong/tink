<script lang="ts">
  import { LogOut, RefreshCw, Sun, Moon, Languages } from "lucide-svelte";
  import { api } from "@/api/index";
  import { t, toggleLocale } from "@/i18n";
  import logo from "@/assets/logo.png";
  import logoLight from "@/assets/logo-light.png";

  type RouteTab = "overview" | "devices" | "keys" | "settings" | "push";

  interface Props {
    activeTab: RouteTab;
    onTabChange: (tab: RouteTab) => void;
    onLogout: () => void;
    onRefresh: () => void;
    refreshing: boolean;
    isDark: boolean;
    toggleTheme: () => void;
    isMaster: boolean;
    tokenName?: string;
  }

  let {
    activeTab,
    onTabChange,
    onLogout,
    onRefresh,
    refreshing,
    isDark,
    toggleTheme,
    isMaster,
    tokenName = "",
  }: Props = $props();

  async function handleLogout() {
    try {
      await api.logout();
    } catch {
      // 会话可能已过期，忽略登出接口错误
    }
    onLogout();
  }
</script>

<header class="navbar">
  <div class="nav-left">
    <div class="brand">
      <div class="logo">
        <img src={logo} alt="Tink Logo" class="logo-img logo-dark" />
        <img src={logoLight} alt="Tink Logo" class="logo-img logo-light" />
      </div>
      <span class="brand-name">Tink</span>
      <span class="badge">Console</span>
      {#if isMaster}
        <span class="role-badge master" title={t("header.master_title")}
          >Master</span
        >
      {:else}
        <span class="role-badge sk" title={t("header.key_title", { name: tokenName })}
          >SK: {tokenName || "Default"}</span
        >
      {/if}
    </div>

    <nav class="nav-links">
      <button
        type="button"
        class="nav-item"
        class:active={activeTab === "overview"}
        onclick={() => onTabChange("overview")}
      >
        {t("nav.overview")}
      </button>
      <button
        type="button"
        class="nav-item"
        class:active={activeTab === "devices"}
        onclick={() => onTabChange("devices")}
      >
        {t("nav.devices")}
      </button>
      {#if isMaster}
        <button
          type="button"
          class="nav-item"
          class:active={activeTab === "keys"}
          onclick={() => onTabChange("keys")}
        >
          {t("nav.keys")}
        </button>
        <button
          type="button"
          class="nav-item"
          class:active={activeTab === "settings"}
          onclick={() => onTabChange("settings")}
        >
          {t("nav.settings")}
        </button>
      {/if}
      <button
        type="button"
        class="nav-item"
        class:active={activeTab === "push"}
        onclick={() => onTabChange("push")}
      >
        {t("nav.push")}
      </button>
    </nav>
  </div>

  <div class="nav-right">
    <button
      class="icon-btn"
      title={t("header.refresh")}
      onclick={onRefresh}
      disabled={refreshing}
    >
      <RefreshCw size={17} class={refreshing ? "spinning" : ""} />
    </button>

    <button
      class="icon-btn"
      title={isDark ? t("header.theme_to_light") : t("header.theme_to_dark")}
      onclick={toggleTheme}
    >
      {#if isDark}
        <Sun size={18} />
      {:else}
        <Moon size={18} />
      {/if}
    </button>

    <button
      class="icon-btn lang-btn"
      title={t("header.lang_title")}
      onclick={toggleLocale}
    >
      <Languages size={17} />
      <span class="lang-label">{t("header.lang")}</span>
    </button>

    <button class="logout-btn" title={t("header.logout_title")} onclick={handleLogout}>
      <LogOut size={16} />
      <span>{t("header.logout")}</span>
    </button>
  </div>
</header>

<style>
  .navbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 64px;
    padding: 0 24px;
    background: var(--card-bg);
    border-bottom: 1px solid var(--border-color);
    position: sticky;
    top: 0;
    z-index: 50;
  }

  .nav-left {
    display: flex;
    align-items: center;
    gap: 32px;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .logo {
    width: 34px;
    height: 34px;
    border-radius: 9px;
    background: rgba(255, 255, 255, 0.06);
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 4px 12px var(--primary-light);
    overflow: hidden;
  }

  .logo-img {
    width: 26px;
    height: 26px;
    object-fit: contain;
  }

  /* 默认（深色主题）显示 dark logo；浅色主题（无 html.dark）切换为 light */
  .logo-light {
    display: none;
  }

  :global(html:not(.dark)) .logo-light {
    display: block;
  }

  :global(html:not(.dark)) .logo-dark {
    display: none;
  }

  .brand-name {
    font-size: 1.15rem;
    font-weight: 700;
    letter-spacing: -0.5px;
  }

  .badge {
    font-size: 0.7rem;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 6px;
    background: var(--badge-bg);
    color: var(--text-muted);
    border: 1px solid var(--border-color);
  }

  @media (max-width: 640px) {
    .navbar {
      height: auto;
      min-height: 56px;
      padding: 8px 14px;
      gap: 10px;
    }

    .nav-left {
      min-width: 0;
      flex: 1;
      gap: 10px;
    }

    .brand {
      min-width: 0;
      gap: 6px;
    }

    .brand-name,
    .badge,
    .role-badge {
      display: none;
    }

    .nav-links {
      flex: 1;
      gap: 2px;
      overflow-x: auto;
      scrollbar-width: none;
    }

    .nav-links::-webkit-scrollbar {
      display: none;
    }

    .nav-item {
      padding: 7px 8px;
      font-size: 0.75rem;
      white-space: nowrap;
    }

    .nav-right {
      gap: 4px;
    }

    .logout-btn span {
      display: none;
    }

    .logout-btn {
      padding: 7px;
    }
  }

  .role-badge {
    font-size: 0.7rem;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 6px;
    letter-spacing: 0.2px;
  }

  .role-badge.master {
    background: rgba(139, 92, 246, 0.18);
    color: #a78bfa;
    border: 1px solid rgba(139, 92, 246, 0.35);
  }

  .role-badge.sk {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
    border: 1px solid rgba(59, 130, 246, 0.3);
  }

  .nav-links {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .nav-item {
    background: transparent;
    border: none;
    padding: 8px 14px;
    border-radius: 8px;
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--text-muted);
    text-decoration: none;
    user-select: none;
    display: inline-flex;
    align-items: center;
    cursor: pointer;
    transition: all 0.15s;
  }

  .nav-item:hover {
    color: var(--text-color);
    background: var(--hover-bg);
  }

  .nav-item.active {
    color: var(--primary);
    background: var(--primary-light);
    font-weight: 600;
  }

  .nav-right {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .icon-btn {
    width: 36px;
    height: 36px;
    border-radius: 8px;
    border: 1px solid var(--border-color);
    background: var(--btn-bg);
    color: var(--text-muted);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.15s;
  }

  .icon-btn:hover {
    color: var(--text-color);
    border-color: var(--primary);
  }

  .lang-btn {
    width: auto;
    padding: 0 10px;
    gap: 5px;
  }

  .lang-label {
    font-size: 0.78rem;
    font-weight: 600;
  }

  @media (max-width: 640px) {
    .lang-label {
      display: none;
    }
  }

  .logout-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 36px;
    padding: 0 14px;
    border-radius: 8px;
    border: 1px solid var(--border-color);
    background: var(--btn-bg);
    color: var(--text-muted);
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
  }

  .logout-btn:hover {
    color: #ef4444;
    border-color: rgba(239, 68, 68, 0.4);
    background: rgba(239, 68, 68, 0.08);
  }

  :global(.spinning) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  @media (max-width: 640px) {
    .navbar {
      padding: 0 14px;
    }
    .badge {
      display: none;
    }
    .nav-left {
      gap: 12px;
    }
    .nav-item {
      padding: 6px 10px;
      font-size: 0.85rem;
    }
  }
</style>
