<script>
  import { untrack } from "svelte";
  import Dialog from "../components/Dialog.svelte";
  import { defaultColors, themeState, themeTokens } from "../lib/themes.svelte.js";

  let { theme, close } = $props();
  let name = $state(untrack(() => theme?.name ?? ""));
  let colors = $state(untrack(() => ({ ...defaultColors, ...theme?.colors })));
  $effect(() => { themeState.preview = { colors: { ...colors } }; });
</script>

<Dialog title={theme ? "Edit theme" : "New theme"} {close} wide>
  {#snippet body()}
    <label class="field">
      <span class="field-label">Name</span>
      <input class="input" bind:value={name} maxlength="60" />
    </label>
    <p class="explain">Changes are previewed until the dialog closes.</p>
    <div class="theme-colors">
      {#each themeTokens as token}
        <label><span>{token.replaceAll("-", " ")}</span><input type="color" bind:value={colors[token]} /></label>
      {/each}
    </div>
  {/snippet}
  {#snippet footer()}
    <div class="actions">
      <button class="btn" onclick={() => close()}>Cancel</button>
      <button class="btn primary" disabled={!name.trim()}
              onclick={() => close({ name: name.trim(), colors: $state.snapshot(colors) })}>Save theme</button>
    </div>
  {/snippet}
</Dialog>
