<script lang="ts">
  import Overview from "@/components/Overview.svelte";
  import Devices from "@/components/Devices.svelte";
  import Tokens from "@/components/Tokens.svelte";
  import Settings from "@/components/Settings.svelte";
  import PushTester from "@/components/PushTester.svelte";
  import type { Device, TokenItem } from "@/api/types";

  type Route = "overview" | "devices" | "keys" | "settings" | "push";
  let {
    activeRoute,
    devices,
    tokens,
    serverHealthy,
    isMaster,
    serverVersion,
    serverBuild,
    serverTime,
    onNavigate,
    onRefresh,
  }: {
    activeRoute: Route;
    devices: Device[];
    tokens: TokenItem[];
    serverHealthy: boolean;
    isMaster: boolean;
    serverVersion: string;
    serverBuild: string;
    serverTime: number;
    onNavigate: (route: Route) => void;
    onRefresh: () => void;
  } = $props();
</script>

{#if activeRoute === "overview"}
  <Overview
    {devices}
    {tokens}
    {serverHealthy}
    {isMaster}
    {serverVersion}
    {serverBuild}
    {serverTime}
    {onNavigate}
  />
{:else if activeRoute === "devices"}
  <Devices {devices} {onRefresh} />
{:else if activeRoute === "keys" && isMaster}
  <Tokens {tokens} {onRefresh} />
{:else if activeRoute === "settings" && isMaster}
  <Settings />
{:else if activeRoute === "push"}
  <PushTester {devices} />
{/if}
