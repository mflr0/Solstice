<script lang="ts">
  import { paths } from '../../core/route.js';
  import { t } from '../../i18n/fr.js';
  import Illustration from '../Illustration.svelte';

  interface Props {
    message?: string | undefined;
  }
  const { message = t.errors.notFound }: Props = $props();
</script>

<div class="page" data-title={t.notFound.title}>
  <Illustration kind="lost" />
  <h1 class="page-title">{t.notFound.title}</h1>
  <p class="empty">{message}</p>
  <a class="btn" href={paths.home()}>{t.notFound.back}</a>
</div>
