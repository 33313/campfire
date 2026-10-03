<script>
  // A description or a comment, rendered (lib/markdown.js).
  import { markdown } from "../lib/markdown.js";

  let { text, anchors = false, class: cls = "", ...rest } = $props();
</script>

<div class="markdown {cls}" {...rest}>{@html markdown(text, anchors)}</div>
