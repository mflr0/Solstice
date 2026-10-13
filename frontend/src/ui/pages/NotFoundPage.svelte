<script lang="ts">
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/fr.js';

  interface Props {
    message?: string | undefined;
  }
  const { message = t.errors.notFound }: Props = $props();
</script>

<div class="page">
  <h1>{t.notFound.title}</h1>
  <p class="empty">{message}</p>
  <a class="btn" href={paths.home()}>{t.notFound.back}</a>
</div>
