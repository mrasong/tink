<script lang="ts">
  import type { Device, SendMessageResponse } from "@/api/types";
  import { api } from "@/api/index";
  import { t } from "@/i18n";
  import { Send, CircleCheck, CircleAlert, Laptop } from "lucide-svelte";

  interface Props {
    devices?: Device[];
  }

  let { devices = [] }: Props = $props();
  let safeDevices = $derived(devices ?? []);

  let title = $state("");
  let body = $state("");
  let url = $state("");
  let sound = $state("default");
  let group = $state("");
  let selectedDevices = $state<string[]>([]);
  let barkDevicesInput = $state("");
  let barkParamsInput = $state("");

  let sending = $state(false);
  let successResult = $state<SendMessageResponse | null>(null);
  let errorMsg = $state("");

  async function handleSend(e: SubmitEvent) {
    e.preventDefault();
    if (!title && !body) {
      errorMsg = t("push.err_title");
      return;
    }

    const barkKeys = barkDevicesInput
      .split(/[\n,]+/)
      .map((k) => k.trim())
      .filter(Boolean);

    if (selectedDevices.length === 0 && barkKeys.length === 0) {
      errorMsg = t("push.err_target");
      return;
    }

    let parsedBarkParams: Record<string, any> | undefined = undefined;
    if (barkParamsInput.trim()) {
      try {
        parsedBarkParams = JSON.parse(barkParamsInput);
      } catch (err: any) {
        errorMsg = t("push.err_barkparams");
        return;
      }
    }

    sending = true;
    errorMsg = "";
    successResult = null;

    try {
      const res = await api.sendMessage({
        title,
        body,
        url: url.trim() || undefined,
        sound: sound.trim() || undefined,
        group: group.trim() || undefined,
        devices: selectedDevices.length > 0 ? selectedDevices : undefined,
        bark_devices: barkKeys.length > 0 ? barkKeys : undefined,
        bark_params: parsedBarkParams,
      });
      successResult = res;
    } catch (err: any) {
      errorMsg = err.message || t("push.send_failed");
    } finally {
      sending = false;
    }
  }

  function toggleDevice(id: string) {
    if (selectedDevices.includes(id)) {
      selectedDevices = selectedDevices.filter((d) => d !== id);
    } else {
      selectedDevices = [...selectedDevices, id];
    }
  }
</script>

<div class="push-container">
  <div class="header-bar">
    <div>
      <h2>{t("push.title")}</h2>
      <p class="subtitle">{t("push.subtitle")}</p>
    </div>
  </div>

  {#if successResult}
    <div class="success-banner">
      <CircleCheck size={20} />
      <div>
        <strong>{t("push.ok_title")}</strong>
        <span>
          {t("push.dispatched", {
            tink: successResult.dispatched_tink,
            bark: successResult.dispatched_bark,
          })}
          {#if successResult.id}{t("push.message_id", { id: successResult.id })}{/if}
        </span>
        {#if successResult.bark_errors && successResult.bark_errors.length > 0}
          <div class="bark-warning">
            {t("push.bark_errors", { errors: successResult.bark_errors.join("; ") })}
          </div>
        {/if}
      </div>
    </div>
  {/if}

  {#if errorMsg}
    <div class="error-banner">
      <CircleAlert size={20} />
      <span>{errorMsg}</span>
    </div>
  {/if}

  <form onsubmit={handleSend} class="push-form">
    <div class="form-row">
      <div class="form-group flex-2">
        <label for="p-title">{t("push.label_title")}</label>
        <input
          id="p-title"
          type="text"
          placeholder={t("push.title_placeholder")}
          bind:value={title}
        />
      </div>

      <div class="form-group flex-1">
        <label for="p-group">{t("push.label_group")}</label>
        <input
          id="p-group"
          type="text"
          placeholder={t("push.group_placeholder")}
          bind:value={group}
        />
      </div>
    </div>

    <div class="form-group">
      <label for="p-body">{t("push.label_body")}</label>
      <textarea
        id="p-body"
        rows="8"
        placeholder={t("push.body_placeholder")}
        bind:value={body}
      ></textarea>
    </div>

    <div class="form-row">
      <div class="form-group flex-2">
        <label for="p-url">{t("push.label_url")}</label>
        <input
          id="p-url"
          type="url"
          placeholder="https://..."
          bind:value={url}
        />
      </div>

      <div class="form-group flex-1">
        <label for="p-sound">{t("push.label_sound")}</label>
        <select id="p-sound" bind:value={sound}>
          <option value="default">{t("push.sound_default")}</option>
          <option value="glass">Glass</option>
          <option value="bell">Bell</option>
          <option value="silent">{t("push.sound_silent")}</option>
        </select>
      </div>
    </div>

    <div class="form-group">
      <div class="label-with-hint">
        <span class="group-title">{t("push.tink_group")}</span>
        <span class="hint">{t("push.tink_hint")}</span>
      </div>

      {#if safeDevices.length === 0}
        <div class="no-dev-hint">{t("push.no_devices")}</div>
      {:else}
        <div class="devices-select-grid">
          {#each safeDevices as dev}
            <button
              type="button"
              class="device-chip"
              class:selected={selectedDevices.includes(dev.id)}
              onclick={() => toggleDevice(dev.id)}
            >
              <Laptop size={14} />
              <span>{dev.name}</span>
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <div class="form-group">
      <div class="label-with-hint">
        <span class="group-title">{t("push.bark_group")}</span>
        <span class="hint">{t("push.bark_hint")}</span>
      </div>
      <textarea
        rows="2"
        class="form-control"
        placeholder={t("push.bark_placeholder")}
        bind:value={barkDevicesInput}
      ></textarea>
    </div>

    <div class="form-group">
      <div class="label-with-hint">
        <span class="group-title">{t("push.params_group")}</span>
        <span class="hint">{t("push.params_hint")}</span>
      </div>
      <textarea
        rows="2"
        class="form-control code-font"
        placeholder={t("push.params_placeholder")}
        bind:value={barkParamsInput}
      ></textarea>
    </div>

    <div class="form-actions">
      <button
        type="submit"
        class="submit-btn"
        disabled={sending ||
          (selectedDevices.length === 0 && !barkDevicesInput.trim())}
      >
        <Send size={16} />
        <span>{sending ? t("push.sending") : t("push.submit")}</span>
      </button>
    </div>
  </form>
</div>

<style>
  .push-container {
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

  .success-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    background: rgba(34, 197, 94, 0.12);
    border: 1px solid rgba(34, 197, 94, 0.3);
    border-radius: 10px;
    color: #22c55e;
    font-size: 0.9rem;
  }

  .bark-warning {
    margin-top: 6px;
    font-size: 0.8rem;
    color: #f59e0b;
  }

  .error-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 10px;
    color: #ef4444;
    font-size: 0.9rem;
  }

  .push-form {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .form-row {
    display: flex;
    gap: 16px;
  }

  .flex-1 {
    flex: 1;
  }

  .flex-2 {
    flex: 2;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  label,
  .group-title {
    font-size: 0.85rem;
    font-weight: 500;
  }

  .label-with-hint {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .hint {
    font-weight: normal;
    color: var(--text-muted);
    font-size: 0.8rem;
  }

  input,
  textarea,
  select {
    width: 100%;
    box-sizing: border-box;
    padding: 10px 14px;
    background: var(--input-bg);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    color: var(--text-color);
    font-size: 0.9rem;
    outline: none;
    font-family: inherit;
  }

  input:focus,
  textarea:focus,
  select:focus {
    border-color: var(--primary);
  }

  .devices-select-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 4px;
  }

  .device-chip {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    border-radius: 20px;
    background: var(--hover-bg);
    border: 1px solid var(--border-color);
    color: var(--text-muted);
    font-size: 0.85rem;
    cursor: pointer;
    transition: all 0.15s;
  }

  .device-chip:hover {
    color: var(--text-color);
  }

  .device-chip.selected {
    background: var(--primary-light);
    border-color: var(--primary);
    color: var(--primary);
    font-weight: 500;
  }

  .no-dev-hint {
    font-size: 0.85rem;
    color: var(--text-muted);
    background: var(--hover-bg);
    padding: 10px 14px;
    border-radius: 8px;
  }

  .form-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 8px;
  }

  .submit-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 22px;
    background: var(--primary);
    color: white;
    border: none;
    border-radius: 9px;
    font-size: 0.9rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
  }

  .submit-btn:hover:not(:disabled) {
    background: var(--primary-hover);
  }

  .submit-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  @media (max-width: 640px) {
    .form-row {
      flex-direction: column;
    }
  }
</style>
