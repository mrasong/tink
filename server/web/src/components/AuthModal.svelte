<script lang="ts">
  import { api } from "@/api/index";
  import { t } from "@/i18n";
  import { Key, ShieldCheck, CircleAlert, ArrowRight } from "lucide-svelte";

  interface Props {
    onSuccess: () => void;
  }

  let { onSuccess }: Props = $props();

  let tokenInput = $state("");
  let loading = $state(false);
  let errorMsg = $state("");

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    const tok = tokenInput.trim();
    if (!tok) {
      errorMsg = t("auth.error_input");
      return;
    }

    loading = true;
    errorMsg = "";

    try {
      // 用 Secret Key 换取 httpOnly 会话 cookie，Key 本身不做任何本地存储
      await api.login(tok);
      tokenInput = "";
      onSuccess();
    } catch (err: any) {
      errorMsg = err.message || t("auth.error_verify");
    } finally {
      loading = false;
    }
  }
</script>

<div class="auth-overlay">
  <div class="auth-card">
    <div class="icon-wrap">
      <ShieldCheck size={36} class="text-primary" />
    </div>
    <h2>{t("auth.title")}</h2>
    <p class="subtitle">
      {t("auth.subtitle")}
    </p>

    {#if errorMsg}
      <div class="error-banner">
        <CircleAlert size={18} />
        <span>{errorMsg}</span>
      </div>
    {/if}

    <form onsubmit={handleSubmit}>
      <div class="input-group">
        <Key size={18} class="input-icon" />
        <input
          type="password"
          placeholder="sk-tink-..."
          bind:value={tokenInput}
          autocomplete="current-password"
          required
        />
      </div>

      <button type="submit" class="submit-btn" disabled={loading}>
        {#if loading}
          <span>{t("auth.verifying")}</span>
        {:else}
          <span>{t("auth.submit")}</span>
          <ArrowRight size={18} />
        {/if}
      </button>
    </form>
  </div>
</div>

<style>
  .auth-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 999;
    padding: 20px;
  }

  .auth-card {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 16px;
    width: 100%;
    max-width: 420px;
    padding: 32px 28px;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
    text-align: center;
  }

  .icon-wrap {
    width: 64px;
    height: 64px;
    margin: 0 auto 16px;
    background: var(--primary-light);
    border-radius: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  h2 {
    margin: 0 0 8px;
    font-size: 1.5rem;
    font-weight: 600;
  }

  .subtitle {
    margin: 0 0 24px;
    color: var(--text-muted);
    font-size: 0.9rem;
  }

  .error-banner {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px;
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.3);
    border-radius: 8px;
    color: #ef4444;
    font-size: 0.85rem;
    text-align: left;
    margin-bottom: 20px;
  }

  .input-group {
    position: relative;
    margin-bottom: 20px;
  }

  :global(.input-icon) {
    position: absolute;
    left: 14px;
    top: 50%;
    transform: translateY(-50%);
    color: var(--text-muted);
  }

  input {
    width: 100%;
    box-sizing: border-box;
    padding: 12px 14px 12px 42px;
    background: var(--input-bg);
    border: 1px solid var(--border-color);
    border-radius: 10px;
    color: var(--text-color);
    font-size: 0.95rem;
    outline: none;
    transition: all 0.2s;
  }

  input:focus {
    border-color: var(--primary);
    box-shadow: 0 0 0 2px var(--primary-light);
  }

  .submit-btn {
    width: 100%;
    padding: 12px 20px;
    background: var(--primary);
    color: white;
    border: none;
    border-radius: 10px;
    font-size: 0.95rem;
    font-weight: 500;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    transition: all 0.2s;
  }

  .submit-btn:hover:not(:disabled) {
    background: var(--primary-hover);
  }

  .submit-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
</style>
