<script lang="ts">
  import Header from "@/components/Header.svelte";
  import Router from "@/routes/Router.svelte";
  import Footer from "@/components/Footer.svelte";
  import type { Device, TokenItem } from "@/api/types";

  type Route = "overview" | "devices" | "keys" | "settings" | "push";

  interface Props {
    activeTab: Route;
    devices: Device[];
    tokens: TokenItem[];
    serverHealthy: boolean;
    isMaster: boolean;
    serverVersion: string;
    serverBuild: string;
    serverTime: number;
    refreshing: boolean;
    isDark: boolean;
    tokenName?: string;
    onTabChange: (tab: Route) => void;
    onLogout: () => void;
    onRefresh: () => void;
    toggleTheme: () => void;
  }

  let {
    activeTab,
    devices,
    tokens,
    serverHealthy,
    isMaster,
    serverVersion,
    serverBuild,
    serverTime,
    refreshing,
    isDark,
    tokenName,
    onTabChange,
    onLogout,
    onRefresh,
    toggleTheme,
  }: Props = $props();
</script>

<Header
  {activeTab}
  {onTabChange}
  {onLogout}
  {onRefresh}
  {refreshing}
  {isDark}
  {toggleTheme}
  {isMaster}
  {tokenName}
/>
<main class="main-content">
  <Router
    activeRoute={activeTab}
    {devices}
    {tokens}
    {serverHealthy}
    {isMaster}
    {serverVersion}
    {serverBuild}
    {serverTime}
    onNavigate={onTabChange}
    {onRefresh}
  />
</main>
<Footer version={serverVersion} build={serverBuild} />
