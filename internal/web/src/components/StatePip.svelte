<script>
  let { role = null, extra = "" } = $props();
</script>

<span class="pip {role === 'final' ? 'final' : role === 'rejected' ? 'rejected' : ''} {extra}"></span>
