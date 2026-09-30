<script lang="ts">
  import type { TokenItem, CreatedTokenResponse } from "@/api/types";
  import { formatTimestamp } from "@/api/time";
  import { api } from "@/api/index";
  import { t } from "@/i18n";
  import {
    Key,
    Plus,
    Trash2,
    Copy,
    Check,
    ShieldAlert,
    Power,
  } from "lucide-svelte";

  interface Props {
    tokens?: TokenItem[];
    onRefresh: () => void;
  }

  let { tokens = [], onRefresh }: Props = $props();
  let safeTokens = $derived(tokens ?? []);

  // 创建 Token 对话框状态
  let showCreateModal = $state(false);
  let newName = $state("");
  let creating = $state(false);
  let createError = $state("");

  // 成功创建后的展示弹窗
  let createdResult = $state<CreatedTokenResponse | null>(null);
  let copied = $state(false);

  // 操作中状态
  let togglingId = $state("");
  let deletingId = $state("");
  let actionError = $state("");

  async function handleCreate(e: SubmitEvent) {
    e.preventDefault();
    const name = newName.trim();
    if (!name) return;

    creating = true;
    createError = "";
    try {
      const res = await api.createToken(name);
      createdResult = res;
      showCreateModal = false;
      newName = "";
      onRefresh();
    } catch (err: any) {
      createError = err.message || t("tokens.create_failed");
    } finally {
      creating = false;
    }
  }

  async function handleToggleEnabled(tok: TokenItem) {
    togglingId = tok.id;
    actionError = "";
    try {
      await api.updateToken(tok.id, { enabled: tok.enabled === 1 ? 0 : 1 });
      onRefresh();
    } catch (err: any) {
      actionError = err.message || t("tokens.toggle_failed");
    } finally {
      togglingId = "";
    }
  }

  async function handleDelete(tok: TokenItem) {
    if (
      !confirm(t("tokens.confirm_delete", { name: tok.name }))
    ) {
      return;
    }

    deletingId = tok.id;
    actionError = "";
    try {
      await api.deleteToken(tok.id);
      onRefresh();
    } catch (err: any) {
      actionError = err.message || t("tokens.delete_failed");
    } finally {
      deletingId = "";
    }
  }

  async function copyToClipboard(text: string) {
    let success = false;
    // 1. 优先尝试现代 Clipboard API
    if (navigator?.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(text);
        success = true;
      } catch (err) {
        console.warn(
          "navigator.clipboard 写入失败，降级使用 execCommand:",
          err,
        );
      }
    }

    // 2. 降级方案：动态创建不可见 textarea 配合 execCommand('copy')（兼容 HTTP/内网 IP 环境）
    if (!success) {
      try {
        const textArea = document.createElement("textarea");
        textArea.value = text;
        textArea.style.position = "fixed";
        textArea.style.left = "-9999px";
        textArea.style.top = "0";
        textArea.setAttribute("readonly", "");
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        success = document.execCommand("copy");
        document.body.removeChild(textArea);
      } catch (err) {
        console.error("execCommand copy 也失败:", err);
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

<div class="tokens-container">
  <div class="header-bar">
    <div>
      <h2>{t("tokens.title")}</h2>
      <p class="subtitle">{t("tokens.subtitle")}</p>
    </div>
    <button class="add-btn" onclick={() => (showCreateModal = true)}>
      <Plus size={16} />
      <span>{t("tokens.create")}</span>
    </button>
  </div>

  {#if actionError}
    <div class="error-banner">
      {actionError}
    </div>
  {/if}

  <div class="token-table-wrap">
    <table class="token-table">
      <thead>
        <tr>
          <th>{t("tokens.col_name")}</th>
          <th>{t("tokens.col_id")}</th>
          <th>{t("tokens.col_type")}</th>
          <th>{t("tokens.col_status")}</th>
          <th>{t("tokens.col_last_used")}</th>
          <th>{t("tokens.col_created")}</th>
          <th style="text-align: right;">{t("tokens.col_actions")}</th>
        </tr>
      </thead>
      <tbody>
        {#each safeTokens as tok}
          <tr>
            <td>
              <div class="name-cell">
                <Key size={16} class="text-muted" />
                <span class="tok-name">{tok.name}</span>
              </div>
            </td>
            <td>
              <code class="tok-id">{tok.id}</code>
            </td>
            <td>
              {#if tok.role === "admin" || tok.is_master}
                <span class="badge master">Admin</span>
              {:else}
                <span class="badge sender">Secret Key</span>
              {/if}
            </td>
            <td>
              <div class="status-cell">
                <span class="status-dot" class:enabled={tok.enabled === 1}
                ></span>
                <span>{tok.enabled === 1 ? t("tokens.status_enabled") : t("tokens.status_disabled")}</span>
              </div>
            </td>
            <td class="text-muted">
              {tok.last_used
                 ? formatTimestamp(tok.last_used)
                : t("tokens.never_used")}
            </td>
            <td class="text-muted">
               {formatTimestamp(tok.created_at)}
            </td>
            <td style="text-align: right;">
              <div class="action-cell">
                {#if tok.role !== "admin" && !tok.is_master}
                  <button
                    class="toggle-btn"
                    class:active={tok.enabled === 1}
                    title={tok.enabled === 1 ? t("tokens.disable_title") : t("tokens.enable_title")}
                    onclick={() => handleToggleEnabled(tok)}
                    disabled={togglingId === tok.id}
                  >
                    <Power size={14} />
                    <span>{tok.enabled === 1 ? t("tokens.btn_disable") : t("tokens.btn_enable")}</span>
                  </button>

                  <button
                    class="icon-del-btn"
                    title={t("tokens.revoke_title")}
                    onclick={() => handleDelete(tok)}
                    disabled={deletingId === tok.id}
                  >
                    <Trash2 size={15} />
                  </button>
                {:else}
                  <span
                    class="protected-label"
                    title={t("tokens.protected_title")}
                    >{t("tokens.protected")}</span
                  >
                {/if}
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<!-- 创建 Token 弹窗 -->
{#if showCreateModal}
  <div class="modal-overlay">
    <div class="modal-box">
      <h3>{t("tokens.modal_title")}</h3>
      <p class="modal-subtitle">
        {t("tokens.modal_subtitle")}
      </p>

      {#if createError}
        <div class="error-banner">{createError}</div>
      {/if}

      <form onsubmit={handleCreate}>
        <div class="form-group">
          <label for="tok-name">{t("tokens.name_label")}</label>
          <input
            id="tok-name"
            type="text"
            placeholder={t("tokens.name_placeholder")}
            bind:value={newName}
            required
          />
        </div>

        <div class="modal-actions">
          <button
            type="button"
            class="cancel-btn"
            onclick={() => (showCreateModal = false)}
          >
            {t("tokens.cancel")}
          </button>
          <button type="submit" class="submit-btn" disabled={creating}>
            {creating ? t("tokens.generating") : t("tokens.confirm")}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- 结果展示弹窗 (仅展示一次明文) -->
{#if createdResult}
  <div class="modal-overlay">
    <div class="modal-box token-result-box">
      <div class="security-banner">
        <ShieldAlert size={20} />
        <span
          >{t("tokens.save_warning")}</span
        >
      </div>

      <div class="created-info">
        <span class="label">{t("tokens.name_with", { name: createdResult.name })}</span>
        <span class="label">{t("tokens.id_with", { id: createdResult.id })}</span>
      </div>

      <div class="token-display">
        <code>{createdResult.token}</code>
        <button
          type="button"
          class="copy-btn"
          onclick={() => copyToClipboard(createdResult!.token)}
        >
          {#if copied}
            <Check size={16} class="text-green" />
            <span>{t("common.copied")}</span>
          {:else}
            <Copy size={16} />
            <span>{t("common.copy")}</span>
          {/if}
        </button>
      </div>

      <div class="modal-actions">
        <button class="submit-btn" onclick={() => (createdResult = null)}>
          {t("tokens.close_saved")}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .tokens-container {
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

  .add-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 9px 16px;
    background: var(--primary);
    color: white;
    border: none;
    border-radius: 9px;
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
  }

  .add-btn:hover {
    background: var(--primary-hover);
  }

  .error-banner {
    padding: 10px 14px;
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 8px;
    color: #ef4444;
    font-size: 0.85rem;
  }

  .token-table-wrap {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    overflow-x: auto;
  }

  .token-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
    text-align: left;
  }

  th,
  td {
    padding: 14px 18px;
    border-bottom: 1px solid var(--border-color);
  }

  th {
    background: var(--hover-bg);
    font-weight: 600;
    font-size: 0.8rem;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  tr:last-child td {
    border-bottom: none;
  }

  .name-cell {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 600;
  }

  .tok-id {
    font-family: monospace;
    font-size: 0.8rem;
    background: var(--hover-bg);
    padding: 2px 6px;
    border-radius: 4px;
    color: var(--text-muted);
  }

  .badge {
    font-size: 0.75rem;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 20px;
  }

  .badge.master {
    background: var(--primary-light);
    color: var(--primary);
  }

  .badge.sender {
    background: var(--hover-bg);
    color: var(--text-muted);
  }

  .status-cell {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.85rem;
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

  .action-cell {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .toggle-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    border-radius: 6px;
    border: 1px solid var(--border-color);
    background: var(--btn-bg);
    color: var(--text-muted);
    font-size: 0.75rem;
    cursor: pointer;
    transition: all 0.15s;
  }

  .toggle-btn:hover {
    color: var(--text-color);
    border-color: var(--text-muted);
  }

  .toggle-btn.active {
    color: #22c55e;
  }

  .icon-del-btn {
    width: 28px;
    height: 28px;
    border-radius: 6px;
    border: 1px solid var(--border-color);
    background: var(--btn-bg);
    color: var(--text-muted);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.15s;
  }

  .icon-del-btn:hover:not(:disabled) {
    color: #ef4444;
    border-color: rgba(239, 68, 68, 0.4);
    background: rgba(239, 68, 68, 0.08);
  }

  .protected-label {
    font-size: 0.75rem;
    color: var(--text-muted);
    font-style: italic;
  }

  /* Modals */
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(6px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    padding: 20px;
  }

  .modal-box {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 16px;
    width: 100%;
    max-width: 480px;
    padding: 28px;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
  }

  .modal-box h3 {
    margin: 0 0 6px;
    font-size: 1.25rem;
    font-weight: 600;
  }

  .modal-subtitle {
    margin: 0 0 20px;
    color: var(--text-muted);
    font-size: 0.85rem;
  }

  .form-group {
    margin-bottom: 20px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    text-align: left;
  }

  .form-group label {
    font-size: 0.85rem;
    font-weight: 500;
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
  }

  input[type="text"]:focus {
    border-color: var(--primary);
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
  }

  .cancel-btn {
    padding: 9px 16px;
    background: var(--btn-bg);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    color: var(--text-color);
    cursor: pointer;
  }

  .submit-btn {
    padding: 9px 18px;
    background: var(--primary);
    border: none;
    border-radius: 8px;
    color: white;
    font-weight: 500;
    cursor: pointer;
  }

  .security-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 14px;
    background: rgba(245, 158, 11, 0.15);
    border: 1px solid rgba(245, 158, 11, 0.3);
    border-radius: 8px;
    color: #f59e0b;
    font-size: 0.85rem;
    margin-bottom: 16px;
  }

  .created-info {
    display: flex;
    gap: 16px;
    margin-bottom: 12px;
    font-size: 0.85rem;
    color: var(--text-muted);
  }

  .token-display {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--input-bg);
    border: 1px solid var(--border-color);
    padding: 10px 14px;
    border-radius: 8px;
    margin-bottom: 24px;
  }

  .token-display code {
    flex: 1;
    font-family: monospace;
    font-size: 0.85rem;
    word-break: break-all;
  }

  .copy-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 10px;
    background: var(--hover-bg);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    color: var(--text-color);
    font-size: 0.8rem;
    cursor: pointer;
  }
</style>
