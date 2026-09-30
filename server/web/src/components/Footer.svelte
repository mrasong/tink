<script lang="ts">
  interface Props {
    version?: string;
    build?: string;
  }

  let {
    version = typeof __APP_VERSION__ !== "undefined" ? __APP_VERSION__ : "",
    build = typeof __APP_BUILD__ !== "undefined" ? __APP_BUILD__ : "",
  }: Props = $props();

  const displayVersion = $derived(
    version ||
      (typeof __APP_VERSION__ !== "undefined" ? __APP_VERSION__ : "1.0.0"),
  );
  const displayBuild = $derived(
    build || (typeof __APP_BUILD__ !== "undefined" ? __APP_BUILD__ : "f8b3a06"),
  );
</script>

<footer class="app-footer">
  <span>
    © {new Date().getFullYear()}
    <a href="https://github.com/mrasong/tink" target="_blank" rel="noreferrer">
      Tink
    </a>
    {#if displayVersion}
      v{displayVersion}{#if displayBuild}({displayBuild}){/if}
    {/if}
  </span>
</footer>

<style>
  .app-footer {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 18px 24px 24px;
    color: var(--text-muted);
    font-size: 0.8rem;
  }

  .app-footer a {
    color: var(--primary);
    text-decoration: none;
  }

  .app-footer a:hover {
    text-decoration: underline;
  }
</style>
