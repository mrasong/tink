<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "@/api/index";
  import { t } from "@/i18n";
  import {
    Save,
    Check,
    RefreshCw,
    Copy,
    ExternalLink,
    BellRing,
  } from "lucide-svelte";

  let loading = $state(true);
  let saving = $state(false);
  let successMsg = $state("");
  let errorMsg = $state("");
  let copied = $state(false);

  let barkRelayEnabled = $state(false);
  let barkServerURL = $state("https://api.day.app");
  let barkRoutePath = $state("/bark-relay");
  let hostOrigin = $state("");

  onMount(async () => {
    if (typeof window !== "undefined") {
      hostOrigin = window.location.origin;
    }
    await fetchSettings();
  });

  // 动态生成完整的本地 Bark Server 地址
  const generatedBarkServerUrl = $derived.by(() => {
    const origin = hostOrigin || "http://tink-server:5021";
    let path = barkRoutePath.trim();
    if (!path.startsWith("/")) {
      path = "/" + path;
    }
    path = path.replace(/\/+$/, "");
    return `${origin}${path}`;
  });

  async function fetchSettings() {
    loading = true;
    errorMsg = "";
    try {
      const data = await api.getSettings();
      barkRelayEnabled = data.bark_relay_enabled;
      barkServerURL = data.bark_server_url || "https://api.day.app";
      barkRoutePath = data.bark_route_path || "/bark-relay";
    } catch (err: any) {
      errorMsg = err.message || t("settings.fetch_failed");
    } finally {
      loading = false;
    }
  }

  async function handleSave(e: SubmitEvent) {
    e.preventDefault();
    saving = true;
    successMsg = "";
    errorMsg = "";

    try {
      const updated = await api.updateSettings({
        bark_relay_enabled: barkRelayEnabled,
        bark_server_url: barkServerURL.trim(),
        bark_route_path: barkRoutePath.trim(),
      });
      barkRelayEnabled = updated.bark_relay_enabled;
      barkServerURL = updated.bark_server_url;
      barkRoutePath = updated.bark_route_path;
      successMsg = t("settings.saved");
      setTimeout(() => {
        successMsg = "";
      }, 4000);
    } catch (err: any) {
      errorMsg = err.message || t("settings.save_failed");
    } finally {
      saving = false;
    }
  }

  async function copyToClipboard(text: string) {
    let success = false;
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(text);
        success = true;
      } catch (err) {
        console.warn("navigator.clipboard failed:", err);
      }
    }
    if (!success) {
      try {
        const textarea = document.createElement("textarea");
        textarea.value = text;
        textarea.style.position = "fixed";
        textarea.style.opacity = "0";
        document.body.appendChild(textarea);
        textarea.select();
        success = document.execCommand("copy");
        document.body.removeChild(textarea);
      } catch (err) {
        console.error("execCommand copy failed:", err);
      }
    }
    if (success) {
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 2000);
    }
  }
</script>

<div class="settings-container">
  <div class="header-bar">
    <div>
      <h2>{t("settings.title")}</h2>
      <p class="subtitle">{t("settings.subtitle")}</p>
    </div>
  </div>

  {#if successMsg}
    <div class="success-banner">
      <Check size={18} />
      <span>{successMsg}</span>
    </div>
  {/if}

  {#if errorMsg}
    <div class="error-banner">
      {errorMsg}
    </div>
  {/if}

  {#if loading}
    <div class="loading-box">
      <RefreshCw size={20} class="spinning" />
      <span>{t("settings.loading")}</span>
    </div>
  {:else}
    <form onsubmit={handleSave} class="settings-form-wrapper">
      <!-- Section 1: Bark Relay 服务配置模块 -->
      <section class="settings-section">
        <div class="section-header">
          <div class="section-title-group">
            <div class="section-icon">
              <BellRing size={18} />
            </div>
            <div>
              <h3 class="section-heading">{t("settings.bark_heading")}</h3>
              <p class="section-desc">
                {t("settings.bark_desc")}
              </p>
            </div>
          </div>
          <span class="status-badge" class:active={barkRelayEnabled}>
            {barkRelayEnabled ? t("settings.badge_on") : t("settings.badge_off")}
          </span>
        </div>

        <div class="section-body">
          <div class="form-group">
            <div class="switch-row">
              <div class="switch-text">
                <label for="bark-toggle" class="switch-title"
                  >{t("settings.bark_switch_title")}</label
                >
                <p class="field-tip">
                  {t("settings.bark_switch_tip")}
                </p>
              </div>
              <label class="switch">
                <input
                  id="bark-toggle"
                  type="checkbox"
                  bind:checked={barkRelayEnabled}
                />
                <span class="slider round"></span>
              </label>
            </div>
          </div>

          <div class="form-group">
            <label for="bark-server-url"
              >{t("settings.upstream_label")}</label
            >
            <input
              id="bark-server-url"
              type="text"
              placeholder="https://api.day.app"
              bind:value={barkServerURL}
              required
            />
            <p class="field-tip">
              {t("settings.upstream_tip_a")}<code>https://api.day.app</code
              >{t("settings.upstream_tip_b")}
            </p>
          </div>

          <div class="form-group">
            <label for="bark-route-path"
              >{t("settings.route_label")}</label
            >
            <input
              id="bark-route-path"
              type="text"
              placeholder="/bark-relay"
              bind:value={barkRoutePath}
              required
            />
            <p class="field-tip">
              {t("settings.route_tip_a")}<code>/bark-relay</code
              >{t("settings.route_tip_b")}
            </p>
          </div>

          <!-- 开启后自动生成的 Bark 服务器地址展示区 -->
          {#if barkRelayEnabled}
            <div class="generated-url-card">
              <div class="generated-url-header">
                <span class="generated-url-title">{t("settings.gen_title")}</span>
                <span class="generated-url-tag">{t("settings.gen_tag")}</span>
              </div>
              <div class="generated-url-box">
                <code class="url-text">{generatedBarkServerUrl}</code>
                <button
                  type="button"
                  class="copy-btn"
                  onclick={() => copyToClipboard(generatedBarkServerUrl)}
                  title={t("settings.copy_url_title")}
                >
                  {#if copied}
                    <Check size={14} class="text-success" />
                    <span>{t("common.copied")}</span>
                  {:else}
                    <Copy size={14} />
                    <span>{t("common.copy")}</span>
                  {/if}
                </button>
              </div>
              <p class="generated-url-tip">
                {t("settings.gen_tip_a")}<code>device_key</code
                >{t("settings.gen_tip_b")}
              </p>
            </div>
          {/if}
        </div>
      </section>

      <!-- 底部全局保存操作栏 -->
      <div class="form-actions">
        <button type="submit" class="submit-btn" disabled={saving}>
          <Save size={16} />
          <span>{saving ? t("settings.saving") : t("settings.save")}</span>
        </button>
      </div>
    </form>
  {/if}
</div>

<style>
  .settings-container {
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

  .loading-box {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 60px 0;
    color: var(--text-muted);
    font-size: 0.9rem;
  }

  .loading-box :global(.spinning) {
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

  .error-banner {
    padding: 10px 14px;
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 8px;
    color: #ef4444;
    font-size: 0.85rem;
  }

  .settings-form-wrapper {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  /* 模块化 Section 卡片 */
  .settings-section {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    transition: all 0.2s;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--border-color);
  }

  .section-title-group {
    display: flex;
    align-items: flex-start;
    gap: 12px;
  }

  .section-icon {
    width: 34px;
    height: 34px;
    border-radius: 8px;
    background: var(--primary-light);
    color: var(--primary);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    margin-top: 2px;
  }

  .section-heading {
    margin: 0 0 4px;
    font-size: 1.05rem;
    font-weight: 600;
  }

  .section-desc {
    margin: 0;
    color: var(--text-muted);
    font-size: 0.82rem;
    line-height: 1.4;
  }

  .status-badge {
    font-size: 0.75rem;
    padding: 3px 10px;
    border-radius: 12px;
    font-weight: 500;
    background: var(--hover-bg);
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .status-badge.active {
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
  }

  .section-body {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  label {
    font-size: 0.85rem;
    font-weight: 500;
  }

  .field-tip {
    margin: 4px 0 0;
    font-size: 0.8rem;
    color: var(--text-muted);
    line-height: 1.4;
  }

  code {
    background: var(--hover-bg);
    padding: 2px 5px;
    border-radius: 4px;
    font-family: monospace;
    font-size: 0.85em;
  }

  input[type="text"] {
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

  input[type="text"]:focus {
    border-color: var(--primary);
  }

  .switch-row {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 4px 0;
  }

  .switch-title {
    font-size: 0.92rem;
    font-weight: 500;
    cursor: pointer;
  }

  .switch-text {
    flex: 1;
    padding-right: 20px;
  }

  /* Switch 切换开关 */
  .switch {
    position: relative;
    display: inline-block;
    width: 44px;
    height: 24px;
    flex-shrink: 0;
  }

  .switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: var(--border-color);
    transition: 0.2s;
  }

  .slider:before {
    position: absolute;
    content: "";
    height: 18px;
    width: 18px;
    left: 3px;
    bottom: 3px;
    background-color: white;
    transition: 0.2s;
  }

  input:checked + .slider {
    background-color: var(--primary);
  }

  input:checked + .slider:before {
    transform: translateX(20px);
  }

  .slider.round {
    border-radius: 24px;
  }

  .slider.round:before {
    border-radius: 50%;
  }

  /* 自动生成的 Bark Server 地址卡片 */
  .generated-url-card {
    margin-top: 6px;
    padding: 16px;
    background: var(--hover-bg);
    border: 1px solid var(--border-color);
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .generated-url-header {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .generated-url-title {
    font-size: 0.85rem;
    font-weight: 600;
  }

  .generated-url-tag {
    font-size: 0.7rem;
    padding: 1px 6px;
    border-radius: 10px;
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
    font-weight: 500;
  }

  .generated-url-box {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    background: var(--input-bg);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 8px 12px;
  }

  .url-text {
    font-family: monospace;
    font-size: 0.88rem;
    color: var(--primary);
    word-break: break-all;
    background: transparent;
    padding: 0;
  }

  .copy-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 5px 10px;
    border-radius: 6px;
    background: var(--hover-bg);
    border: 1px solid var(--border-color);
    color: var(--text-color);
    font-size: 0.8rem;
    cursor: pointer;
    flex-shrink: 0;
    transition: all 0.15s;
  }

  .copy-btn:hover {
    background: var(--border-color);
  }

  :global(.text-success) {
    color: #22c55e;
  }

  .generated-url-tip {
    margin: 0;
    font-size: 0.78rem;
    color: var(--text-muted);
    line-height: 1.4;
  }

  /* 操作栏 */
  .form-actions {
    display: flex;
    justify-content: flex-end;
  }

  .submit-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 24px;
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
</style>
