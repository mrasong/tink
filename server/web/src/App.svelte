<script lang="ts">
  import { onMount } from "svelte";
  import { purgeLegacyToken, api } from "@/api/api";
  import type { Device, TokenItem, CurrentTokenInfo } from "@/api/types";

  import AuthModal from "@/components/AuthModal.svelte";
  import LoadingScreen from "@/components/LoadingScreen.svelte";
  import AuthenticatedLayout from "@/components/AuthenticatedLayout.svelte";

  type TabRoute = "overview" | "devices" | "keys" | "settings" | "push";

  let authenticated = $state(false);
  let activeTab = $state<TabRoute>("overview");
  let isDark = $state(true);

  let currentToken = $state<CurrentTokenInfo | null>(null);
  let devices = $state<Device[]>([]);
  let tokens = $state<TokenItem[]>([]);
  let serverHealthy = $state(true);
  let serverVersion = $state("");
  let serverBuild = $state("");
  let serverTime = $state(0);
  let refreshing = $state(false);
  let initializing = $state(true);

  const isMaster = $derived(currentToken?.is_master ?? false);

  async function loadData() {
    refreshing = true;
    try {
      // 1. 先验证并拉取当前登录 Token 的身份信息与权限，同时探测服务端 Ping 心跳
      const [pingRes, me] = await Promise.all([
        api.getPing().catch(() => null),
        api.getMe(),
      ]);
      serverHealthy =
        pingRes !== null &&
        (pingRes.message === "pong" || (pingRes as any).status === "ok");
      if (pingRes) {
        serverVersion = pingRes.version || "";
        serverBuild = pingRes.build || "";
        serverTime = pingRes.st || 0;
      }
      currentToken = me;
      authenticated = true;

      // 2. 根据角色权限拉取关联资源：
      // - devices 接口在后端已自动根据 Token 角色分流
      // - tokens 仅在 Master Token 时请求，普通 SK 赋空列表，避免触发 403 异常
      if (me.is_master) {
        const [devs, toks] = await Promise.all([
          api.getDevices(),
          api.getTokens(),
        ]);
        devices = Array.isArray(devs) ? devs : [];
        tokens = Array.isArray(toks) ? toks : [];
      } else {
        const devs = await api.getDevices();
        devices = Array.isArray(devs) ? devs : [];
        tokens = [];
        // 若普通 SK 当前停留在 keys 或 settings 选项卡，强制跳转至概览
        if (activeTab === "keys" || activeTab === "settings") {
          setTab("overview");
        }
      }
    } catch (err: any) {
      // 如果鉴权失败，弹出登录框并重置状态
      authenticated = false;
      currentToken = null;
    } finally {
      refreshing = false;
    }
  }

  function handleLoginSuccess() {
    authenticated = true;
    loadData();
  }

  function handleLogout() {
    authenticated = false;
    currentToken = null;
  }

  function applyTheme(dark: boolean) {
    document.documentElement.classList.toggle("dark", dark);
    const favicon = document.getElementById("favicon") as HTMLLinkElement | null;
    if (favicon) {
      favicon.href = dark ? "./favicon.png" : "./favicon-light.png";
    }
  }

  function toggleTheme() {
    isDark = !isDark;
    localStorage.setItem("tink-theme", isDark ? "dark" : "light");
    applyTheme(isDark);
  }

  function parseHash(): TabRoute {
    const hash = window.location.hash.replace(/^#\/?/, "").toLowerCase();
    if (hash === "devices") return "devices";
    if (hash === "keys" || hash === "sks") {
      // 若已知非 master，不允许进入 tokens 路由
      if (currentToken && !currentToken.is_master) return "overview";
      return "keys";
    }
    if (hash === "settings" || hash === "setting") {
      if (currentToken && !currentToken.is_master) return "overview";
      return "settings";
    }
    if (hash === "push") return "push";
    return "overview";
  }

  function setTab(tab: TabRoute) {
    // 权限防卫：普通 SK 禁止切换到 keys 和 settings Tab
    if ((tab === "keys" || tab === "settings") && currentToken && !currentToken.is_master) {
      tab = "overview";
    }
    activeTab = tab;
    if (window.location.hash !== `#${tab}`) {
      window.location.hash = tab;
    }
    // 切换标签页时自动刷新最新数据
    if (authenticated) {
      loadData();
    }
  }

  onMount(() => {
    const handleHashChange = () => {
      const newTab = parseHash();
      if (newTab !== activeTab) {
        activeTab = newTab;
        if (authenticated) loadData();
      }
    };
    window.addEventListener("hashchange", handleHashChange);
    void initialize();
    return () => window.removeEventListener("hashchange", handleHashChange);
  });

  async function initialize() {
    const savedTheme = localStorage.getItem("tink-theme");
    isDark = savedTheme ? savedTheme === "dark" : true;
    applyTheme(isDark);

    // 初始化 Hash 路由
    activeTab = parseHash();

    // 清除旧版本残留的明文 Key；登录态完全由 httpOnly 会话 cookie 决定
    purgeLegacyToken();
    await loadData();
    initializing = false;
  }
</script>

<div class="app-layout">
  {#if initializing}
    <LoadingScreen />
  {:else if authenticated}
    <AuthenticatedLayout
      {activeTab}
      {devices}
      {tokens}
      {serverHealthy}
      {isMaster}
      {serverVersion}
      {serverBuild}
      {serverTime}
      {refreshing}
      {isDark}
      tokenName={currentToken?.name}
      onTabChange={setTab}
      onLogout={handleLogout}
      onRefresh={loadData}
      {toggleTheme}
    />
  {:else}
    <AuthModal onSuccess={handleLoginSuccess} />
  {/if}
</div>

<style>
  .app-layout {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-color);
    color: var(--text-color);
  }

  :global(.main-content) {
    flex: 1;
    max-width: 1120px;
    width: 100%;
    margin: 0 auto;
    padding: 28px 24px 60px;
    box-sizing: border-box;
  }

  @media (max-width: 640px) {
    :global(.main-content) {
      padding: 16px 14px 40px;
    }
  }
</style>
