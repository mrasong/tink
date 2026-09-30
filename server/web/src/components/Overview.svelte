<script lang="ts">
  import type { Device, TokenItem } from "@/api/types";
  import { t } from "@/i18n";
  import {
    Laptop,
    Key,
    Activity,
    Send,
    CircleCheck,
    TriangleAlert,
  } from "lucide-svelte";
  import { formatTimestamp } from "@/api/time";

  interface Props {
    devices?: Device[];
    tokens?: TokenItem[];
    serverHealthy: boolean;
    isMaster: boolean;
    onNavigate: (tab: "overview" | "devices" | "keys" | "settings" | "push") => void;
    serverVersion?: string;
    serverBuild?: string;
    serverTime?: number;
  }

  let {
    devices = [],
    tokens = [],
    serverHealthy,
    isMaster,
    onNavigate,
    serverVersion = "",
    serverBuild = "",
    serverTime = 0,
  }: Props = $props();

  const safeDevices = $derived(devices ?? []);
  const safeTokens = $derived(tokens ?? []);

  const formattedServerTime = $derived(
    formatTimestamp(serverTime),
  );

  const onlineDevicesCount = $derived(
    safeDevices.filter((d) => {
      return d.status === 1;
    }).length,
  );

  const activeTokensCount = $derived(
    safeTokens.filter((tok) => tok.enabled === 1).length,
  );
</script>

<div class="overview-container">
  <div class="stats-grid">
    <div class="stat-card">
      <div class="card-header">
        <span class="card-title">{t("overview.server_status")}</span>
        <div
          class="icon-box"
          class:healthy={serverHealthy}
          class:unhealthy={!serverHealthy}
        >
          <Activity size={20} />
        </div>
      </div>
      <div class="card-value">
        {#if serverHealthy}
          <div class="status-indicator">
            <CircleCheck size={18} class="text-green" />
            <span>{t("overview.healthy")}</span>
          </div>
        {:else}
          <div class="status-indicator">
            <TriangleAlert size={18} class="text-red" />
            <span>{t("overview.unhealthy")}</span>
          </div>
        {/if}
      </div>
      <span class="card-desc">
        {#if serverHealthy && serverVersion}
          {t("overview.sync_info", {
            version: serverVersion,
            build: serverBuild,
            time: formattedServerTime,
          })}
        {:else}
          {t("overview.server_desc")}
        {/if}
      </span>
    </div>

    <button
      type="button"
      class="stat-card clickable"
      onclick={() => onNavigate("devices")}
    >
      <div class="card-header">
        <span class="card-title">{t("overview.devices_title")}</span>
        <div class="icon-box info">
          <Laptop size={20} />
        </div>
      </div>
      <div class="card-value">
        <span class="number">{safeDevices.length}</span>
        <span class="sub-number">{t("overview.online_count", { count: onlineDevicesCount })}</span>
      </div>
      <span class="card-desc">
        {isMaster ? t("overview.devices_desc_all") : t("overview.devices_desc_key")}
      </span>
    </button>

    {#if isMaster}
      <button
        type="button"
        class="stat-card clickable"
        onclick={() => onNavigate("keys")}
      >
        <div class="card-header">
          <span class="card-title">{t("overview.keys_title")}</span>
          <div class="icon-box warning">
            <Key size={20} />
          </div>
        </div>
        <div class="card-value">
          <span class="number">{safeTokens.length}</span>
          <span class="sub-number">{t("overview.enabled_count", { count: activeTokensCount })}</span>
        </div>
        <span class="card-desc">{t("overview.keys_desc")}</span>
      </button>
    {/if}

    <button
      type="button"
      class="stat-card clickable"
      onclick={() => onNavigate("push")}
    >
      <div class="card-header">
        <span class="card-title">{t("overview.push_title")}</span>
        <div class="icon-box primary">
          <Send size={20} />
        </div>
      </div>
      <div class="card-value">
        <span class="action-text">{t("overview.push_action")}</span>
      </div>
      <span class="card-desc">{t("overview.push_desc")}</span>
    </button>
  </div>

  <div class="recent-sections">
    <div class="section-card">
      <div class="section-header">
        <h3>{t("overview.recent_devices")}</h3>
        <button class="view-all-btn" onclick={() => onNavigate("devices")}
          >{t("overview.view_all")}</button
        >
      </div>
      {#if safeDevices.length === 0}
        <div class="empty-hint">
          {isMaster
            ? t("overview.empty_devices_admin")
            : t("overview.empty_devices_user")}
        </div>
      {:else}
        <div class="list-table">
          {#each safeDevices.slice(0, 5) as dev}
            <div class="list-row">
              <div class="device-info">
                <Laptop size={18} class="text-muted" />
                <span class="dev-name">{dev.name}</span>
                <span class="dev-id">({dev.id})</span>
              </div>
              <span class="time-text">
                 {formatTimestamp(dev.last_connected_at)}
              </span>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    {#if isMaster}
      <div class="section-card">
        <div class="section-header">
          <h3>{t("overview.issued_keys")}</h3>
          <button class="view-all-btn" onclick={() => onNavigate("keys")}
            >{t("overview.view_all")}</button
          >
        </div>
        {#if tokens.length === 0}
          <div class="empty-hint">{t("overview.empty_tokens")}</div>
        {:else}
          <div class="list-table">
            {#each tokens.slice(0, 5) as tok}
              <div class="list-row">
                <div class="token-info">
                  <Key size={18} class="text-muted" />
                  <span class="tok-name">{tok.name}</span>
                  {#if tok.is_master}
                    <span class="master-tag">Master</span>
                  {/if}
                </div>
                <div class="token-status">
                  <span class="status-dot" class:enabled={tok.enabled === 1}
                  ></span>
                  <span>{tok.enabled === 1 ? t("overview.enabled") : t("overview.disabled")}</span>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .overview-container {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 16px;
  }

  .stat-card {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    padding: 20px;
    display: flex;
    flex-direction: column;
    text-align: left;
    font-family: inherit;
    color: inherit;
    transition: all 0.2s;
  }

  .stat-card.clickable {
    cursor: pointer;
  }

  .stat-card.clickable:hover {
    border-color: var(--primary);
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .card-title {
    font-size: 0.85rem;
    font-weight: 500;
    color: var(--text-muted);
  }

  .icon-box {
    width: 36px;
    height: 36px;
    border-radius: 9px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .icon-box.healthy {
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
  }

  .icon-box.unhealthy {
    background: rgba(239, 68, 68, 0.15);
    color: #ef4444;
  }

  .icon-box.info {
    background: rgba(59, 130, 246, 0.15);
    color: #3b82f6;
  }

  .icon-box.warning {
    background: rgba(245, 158, 11, 0.15);
    color: #f59e0b;
  }

  .icon-box.primary {
    background: var(--primary-light);
    color: var(--primary);
  }

  .card-value {
    margin-bottom: 8px;
    display: flex;
    align-items: baseline;
    gap: 8px;
  }

  .number {
    font-size: 1.8rem;
    font-weight: 700;
    line-height: 1;
  }

  .sub-number {
    font-size: 0.9rem;
    color: var(--text-muted);
  }

  .action-text {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--primary);
  }

  .status-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 0.95rem;
    font-weight: 600;
  }

  .card-desc {
    font-size: 0.8rem;
    color: var(--text-muted);
  }

  .recent-sections {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
    gap: 20px;
  }

  .section-card {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    padding: 20px;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }

  .section-header h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
  }

  .view-all-btn {
    background: none;
    border: none;
    color: var(--primary);
    font-size: 0.85rem;
    cursor: pointer;
    font-weight: 500;
  }

  .view-all-btn:hover {
    text-decoration: underline;
  }

  .empty-hint {
    color: var(--text-muted);
    font-size: 0.85rem;
    text-align: center;
    padding: 24px 0;
  }

  .list-table {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .list-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 12px;
    background: var(--hover-bg);
    border-radius: 8px;
  }

  .device-info,
  .token-info {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .dev-name,
  .tok-name {
    font-weight: 500;
    font-size: 0.9rem;
  }

  .dev-id {
    color: var(--text-muted);
    font-size: 0.8rem;
  }

  .master-tag {
    font-size: 0.7rem;
    padding: 2px 6px;
    border-radius: 4px;
    background: var(--primary);
    color: white;
    font-weight: 600;
  }

  .time-text {
    font-size: 0.8rem;
    color: var(--text-muted);
  }

  .token-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.8rem;
    color: var(--text-muted);
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #ef4444;
  }

  .status-dot.enabled {
    background: #22c55e;
  }
</style>
