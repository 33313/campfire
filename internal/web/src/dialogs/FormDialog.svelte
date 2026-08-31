<script>
  import { untrack } from "svelte";
  import Dialog from "../components/Dialog.svelte";

  /** A short form in a dialog. Closes with the field values, keyed by name. */
  let { title, fields, explain = null, action = "Save", consequence = null,
        destructive = false, wide = false, close } = $props();

  let values = $state(untrack(() => Object.fromEntries(fields.map((f) => [f.name, f.value ?? ""]))));
</script>

<Dialog {title} {destructive} {wide} {close}>
  {#snippet body()}
    {#each fields as f}
      {#if f.type === "checkbox"}
        <label style="display:flex;gap:8px;align-items:center">
          <input class="check" type="checkbox" bind:checked={values[f.name]} />
          <span>{f.label}</span>
        </label>
      {:else}
        <label class="field">
          <span class="field-label">{f.label}</span>
          {#if f.type === "textarea"}
            <textarea class="input" rows={f.rows || 5} placeholder={f.placeholder}
                      bind:value={values[f.name]}></textarea>
          {:else if f.type === "select"}
            <select class="input" bind:value={values[f.name]}>
              {#each f.options as o}<option value={o.value}>{o.label}</option>{/each}
            </select>
          {:else}
            <input class="input" type={f.type || "text"} placeholder={f.placeholder}
                   maxlength={f.maxlength} inputmode={f.inputmode}
                   bind:value={values[f.name]} />
          {/if}
          {#if f.explain}<span class="explain">{f.explain}</span>{/if}
        </label>
      {/if}
    {/each}
    {#if explain}<p class="explain">{explain}</p>{/if}
  {/snippet}
  {#snippet footer()}
    {#if consequence}<span class="consequence">{consequence}</span>{/if}
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn" class:primary={!destructive} class:destructive
              onclick={() => close({ ...values })}>{action}</button>
    </div>
  {/snippet}
</Dialog>
