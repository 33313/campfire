<script>
  import { api } from "../lib/api.js";
  import { choices, loadThemes, themeState } from "../lib/themes.svelte.js";
  import { failure, openModal } from "../lib/ui.svelte.js";
  import ConfirmDialog from "../dialogs/ConfirmDialog.svelte";
  import ThemeDialog from "../dialogs/ThemeDialog.svelte";
  import Section from "./Section.svelte";

  let { scope = "personal" } = $props();
  let busy = $state(false);
  let settings = $derived(themeState[scope]);
  let options = $derived(choices(scope));
  let selected = $derived(options.some((t) => t.key === settings?.selected) ? settings.selected : scope === "instance" ? "campfire" : "");

  async function save(selection, themes) {
    busy = true;
    try {
      await api.setThemes(scope, { selected: selection, themes });
      await loadThemes();
    } catch (err) { failure(err); }
    finally { busy = false; }
  }

  async function edit(theme) {
    let result;
    try { result = await openModal(ThemeDialog, { theme }); }
    finally { themeState.preview = null; }
    if (!result) return;
    const key = theme?.key || Array.from(crypto.getRandomValues(new Uint8Array(8)),
      (n) => n.toString(16).padStart(2, "0")).join("");
    const next = { key, ...result };
    await save(selected, theme ? settings.themes.map((t) => t.key === key ? next : t) : [...settings.themes, next]);
  }

  async function remove(theme) {
    const ok = await openModal(ConfirmDialog, {
      title: `Delete ${theme.name}`,
      lines: ["If this theme is selected, the default theme takes its place."],
      consequence: "Cannot be undone", action: "Delete",
    });
    if (ok) await save(selected === `${scope}:${theme.key}` ? (scope === "instance" ? "campfire" : "") : selected,
      settings.themes.filter((t) => t.key !== theme.key));
  }
</script>

<Section title={scope === "instance" ? "Instance themes" : "Themes"}>
  {#snippet children()}
    {#if settings}
      <label class="field">
        <span class="field-label">{scope === "instance" ? "Instance default" : "Your theme"}</span>
        <select class="input" value={selected} disabled={busy}
                onchange={(e) => save(e.currentTarget.value, settings.themes)}>
          {#if scope === "personal"}<option value="">Use instance default</option>{/if}
          {#each options as t (t.key)}<option value={t.key}>{t.name}</option>{/each}
        </select>
      </label>
      {#each settings.themes as theme (theme.key)}
        <div class="theme-row">
          <span>{theme.name}</span>
          <button class="btn small" disabled={busy} onclick={() => edit(theme)}>Edit</button>
          <button class="btn small" disabled={busy} onclick={() => remove(theme)}>Delete</button>
        </div>
      {/each}
      <button class="btn small" style="margin-top:8px" disabled={busy || settings.themes.length >= 24}
              onclick={() => edit()}>Add theme</button>
    {:else}
      <p class="explain">Theme settings are unavailable.</p>
    {/if}
  {/snippet}
</Section>
