<script>
  import Dialog from "../components/Dialog.svelte";
  import { toast } from "../lib/ui.svelte.js";

  /** A secret shown once: only its hash is kept. `install` is an optional
      second line — the command that installs the secret somewhere, which
      needs the two copy buttons told apart. */
  let { title, explain, value, copyLabel = "Copy", install = "", close } = $props();

  const copy = (text) => { navigator.clipboard?.writeText(text); toast("Copied"); };
</script>

<Dialog {title} {close}>
  {#snippet body()}
    <p class="explain">{explain}</p>
    <input class="input" {value} readonly onclick={(e) => e.currentTarget.select()} />
    {#if install}
      <p class="explain" style="margin-top:14px">
        Or install it into an MCP client in one step:
      </p>
      <input class="input" value={install} readonly onclick={(e) => e.currentTarget.select()} />
    {/if}
  {/snippet}
  {#snippet footer()}
    <div class="actions">
      {#if install}
        <button class="btn" onclick={() => copy(install)}>Copy command</button>
      {/if}
      <button class="btn" onclick={() => copy(value)}>{copyLabel}</button>
      <button class="btn primary" onclick={() => close(true)}>Done</button>
    </div>
  {/snippet}
</Dialog>
