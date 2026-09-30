<script lang="ts">
  import type { Device } from "@/api/types";
  import { api } from "@/api/index";
  import { t } from "@/i18n";
  import { Laptop, Trash2, CircleCheck, Clock } from "lucide-svelte";
  import { formatTimestamp } from "@/api/time";

  interface Props {
    devices?: Device[];
    onRefresh: () => void;
  }

  let { devices = [], onRefresh }: Props = $props();
  let safeDevices = $derived(devices ?? []);
  let deletingId = $state("");
  let errorMsg = $state("");

  async function handleDelete(dev: Device) {
    if (
      !confirm(t("devices.confirm_delete", { name: dev.name, id: dev.id }))
    ) {
      return;
    }

    deletingId = dev.id;
    errorMsg = "";
    try {
      await api.deleteDevice(dev.id);
      onRefresh();
    } catch (err: any) {
      errorMsg = err.message || t("devices.delete_failed");
    } finally {
      deletingId = "";
    }
  }

  function isDeviceOnline(dev: Device) {
    return dev.status === 1;
  }
</script>

<div class="devices-container">
  <div class="header-bar">
    <div>
      <h2>{t("devices.title")}</h2>
      <p class="subtitle">{t("devices.subtitle")}</p>
    </div>
  </div>

  {#if errorMsg}
    <div class="error-banner">
      {errorMsg}
    </div>
  {/if}

  {#if safeDevices.length === 0}
    <div class="empty-box">
      <Laptop size={48} class="text-muted" />
      <h3>{t("devices.empty_title")}</h3>
      <p>{t("devices.empty_hint")}</p>
    </div>
  {:else}
    <div class="device-grid">
      {#each safeDevices as dev}
        {@const online = isDeviceOnline(dev)}
        <div class="device-card">
          <div class="dev-top">
            <div class="dev-icon-wrap" class:online>
              <Laptop size={24} />
            </div>
            <div class="status-info">
              <div class="status-pill" class:online>
                {#if online}
                  <CircleCheck size={13} />
                  <span>{t("devices.online")}</span>
                {:else}
                  <Clock size={13} />
                  <span>{t("devices.offline")}</span>
                {/if}
              </div>
              <div class="connection-times">
                <span
                  >{formatTimestamp(
                    online ? dev.last_connected_at : dev.last_disconnected_at,
                    "",
                  )}</span
                >
              </div>
            </div>
          </div>

          <div class="dev-main">
            <h4 class="dev-name">{dev.name}</h4>
            <div class="dev-id-tag" title={dev.id}>{dev.id}</div>
          </div>

          <div class="dev-meta">
            <div class="meta-row">
              <span class="meta-label"
                >{online ? t("devices.last_offline") : t("devices.last_online")}</span
              >
              <span class="meta-value"
                >{formatTimestamp(
                  online ? dev.last_disconnected_at : dev.last_connected_at,
                  "-",
                )}</span
              >
            </div>
            <div class="meta-row">
              <span class="meta-label">{t("devices.registered_at")}</span>
              <span class="meta-value">{formatTimestamp(dev.created_at)}</span>
            </div>
          </div>

          <div class="dev-actions">
            <button
              class="del-btn"
              onclick={() => handleDelete(dev)}
              disabled={deletingId === dev.id}
            >
              <Trash2 size={15} />
              <span>{deletingId === dev.id ? t("devices.removing") : t("devices.remove")}</span>
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .devices-container {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .header-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  h2 {
    margin: 0 0 4px;
    font-size: 1.3rem;
    font-weight: 600;
  }

  .subtitle {
    margin: 0;
    color: var(--text-muted);
    font-size: 0.85rem;
  }

  .error-banner {
    padding: 10px 14px;
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 8px;
    color: #ef4444;
    font-size: 0.85rem;
  }

  .empty-box {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    padding: 48px 24px;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
  }

  .empty-box h3 {
    margin: 0;
    font-size: 1.1rem;
  }

  .empty-box p {
    margin: 0;
    max-width: 480px;
    color: var(--text-muted);
    font-size: 0.9rem;
    line-height: 1.5;
  }

  .device-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 16px;
  }

  .device-card {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    padding: 20px;
    display: flex;
    flex-direction: column;
    transition: all 0.2s;
  }

  .device-card:hover {
    border-color: var(--primary);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.08);
  }

  .dev-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }

  .status-info {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 4px;
    min-width: 0;
  }

  .dev-icon-wrap {
    width: 42px;
    height: 42px;
    border-radius: 10px;
    background: var(--hover-bg);
    color: var(--text-muted);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .dev-icon-wrap.online {
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
  }

  .status-pill {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px 8px;
    border-radius: 20px;
    font-size: 0.75rem;
    font-weight: 500;
    background: var(--hover-bg);
    color: var(--text-muted);
  }

  .status-pill.online {
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
  }

  .connection-times {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 1px;
    max-width: 210px;
    color: var(--text-muted);
    font-size: 0.68rem;
    line-height: 1.25;
    white-space: nowrap;
  }

  .connection-times span {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .dev-main {
    margin-bottom: 14px;
  }

  .dev-name {
    margin: 0 0 6px;
    font-size: 1.05rem;
    font-weight: 600;
  }

  .dev-id-tag {
    display: inline-block;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: monospace;
    font-size: 0.75rem;
    padding: 2px 6px;
    border-radius: 4px;
    background: var(--hover-bg);
    color: var(--text-muted);
  }

  .dev-meta {
    border-top: 1px solid var(--border-color);
    border-bottom: 1px solid var(--border-color);
    padding: 12px 0;
    margin-bottom: 16px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .meta-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.8rem;
  }

  .meta-label {
    color: var(--text-muted);
  }

  .meta-value {
    font-weight: 500;
  }

  .dev-actions {
    display: flex;
    justify-content: flex-end;
  }

  .del-btn {
    background: none;
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 6px 12px;
    color: var(--text-muted);
    font-size: 0.8rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 6px;
    transition: all 0.15s;
  }

  .del-btn:hover:not(:disabled) {
    color: #ef4444;
    border-color: rgba(239, 68, 68, 0.4);
    background: rgba(239, 68, 68, 0.08);
  }
</style>
