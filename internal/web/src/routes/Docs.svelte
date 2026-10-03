<script>
  // How Campfire works, inside it (DEV-80): one page, written as Markdown in
  // docs.md and read like a description. The rail lists its sections and
  // follows the scroll; /docs#filtering opens at a section.

  import { tick } from "svelte";
  import text from "../docs.md?raw";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import SettingsRail from "../components/SettingsRail.svelte";
  import Markdown from "../components/Markdown.svelte";
  import { slug } from "../lib/markdown.js";
  const SECTIONS = [...text.matchAll(/^## (.+)$/gm)].map(([, label]) => ({ id: slug(label), label }));

  let body = $state(null);
  let active = $state(SECTIONS[0].id);

  const heads = () => [...body.querySelectorAll(".docs h2")];

  function go(id, smooth = true) {
    heads().find((h) => h.id === id)?.scrollIntoView({ behavior: smooth ? "smooth" : "instant" });
    history.replaceState(history.state, "", `#${id}`);
  }

  // The section in view is the last one whose heading has passed the top.
  function spy() {
    const top = body.getBoundingClientRect().top + 24;
    active = heads().findLast((h) => h.getBoundingClientRect().top <= top)?.id || SECTIONS[0].id;
  }

  $effect(() => {
    const at = location.hash.slice(1);
    if (at) tick().then(() => go(at, false));
  });
</script>

<ScreenHeader title="Docs" sub="How Campfire works" />

<div class="body" bind:this={body} onscroll={spy}>
  <SettingsRail sections={SECTIONS} {active} onpick={go}>
    {#snippet children()}
      <Markdown class="prose docs" {text} anchors />
    {/snippet}
  </SettingsRail>
</div>
