<script lang="ts">
  import { type CustomDraft } from '../core/custom-fields.js';
  import type { OfferedField, UserRef } from '../core/types.js';
  import { t } from '../i18n/t.js';
  import { api } from '../lib/api.js';
  import Dropdown from './Dropdown.svelte';
  import LabelEditor from './LabelEditor.svelte';
  import MultiPick from './MultiPick.svelte';
  import UserPicker from './UserPicker.svelte';
  import WikiEditor from './WikiEditor.svelte';
  import Who from './Who.svelte';

  /** One editor for a custom field, by its type. */
  interface Props {
    field: OfferedField;
    value: CustomDraft;
    onchange: (value: CustomDraft) => void;
    /** The id of the input, for a `<label for>`. */
    id: string;
    invalid?: boolean | undefined;
    /** The id of the field's error message. */
    describedby?: string | undefined;
  }
  const { field, value, onchange, id, invalid = false, describedby }: Props = $props();

  const text = $derived(typeof value === 'string' ? value : '');
  const ids = $derived(Array.isArray(value) ? (value as number[]) : []);
  const users = $derived(Array.isArray(value) ? (value as UserRef[]) : []);

  function toggle(option: number, on: boolean) {
    onchange(on ? [...ids, option] : ids.filter((x) => x !== option));
  }
</script>

{#if field.type === 'textarea'}
  <WikiEditor bind:value={() => text, (v) => onchange(v)} label={field.name} rows={3} />
{:else if field.type === 'labels'}
  <label class="boxed" for={id}>
    <LabelEditor {id} labels={value as string[]} onchange={(l) => onchange(l)} />
  </label>
{:else if field.type === 'select'}
  <Dropdown
    {id}
    label={field.name}
    value={ids[0] === undefined ? '' : String(ids[0])}
    options={[
      { value: '', label: t.common.none },
      ...field.options.map((o) => ({ value: String(o.id), label: o.value })),
    ]}
    {invalid}
    onchange={(v) => onchange(v ? [Number(v)] : [])}
  />
{:else if field.type === 'multiselect'}
  <MultiPick
    field
    label={field.name}
    options={field.options.map((o) => ({ id: o.id, name: o.value }))}
    selected={ids}
    onchange={(next) => onchange(next)}
  />
{:else if field.type === 'radio' || field.type === 'checkboxes'}
  <div
    class="choices"
    {id}
    role={field.type === 'radio' ? 'radiogroup' : 'group'}
    aria-label={field.name}
    aria-invalid={field.type === 'radio' ? invalid : undefined}
    aria-describedby={describedby}
  >
    {#if field.type === 'radio'}
      <label class="choice">
        <input type="radio" name={id} checked={ids.length === 0} onchange={() => onchange([])} />{t
          .common.none}
      </label>
      {#each field.options as o (o.id)}
        <label class="choice">
          <input
            type="radio"
            name={id}
            checked={ids[0] === o.id}
            onchange={() => onchange([o.id])}
          />{o.value}
        </label>
      {/each}
    {:else}
      {#each field.options as o (o.id)}
        <label class="choice">
          <input
            type="checkbox"
            checked={ids.includes(o.id)}
            onchange={(e) => toggle(o.id, e.currentTarget.checked)}
          />{o.value}
        </label>
      {/each}
    {/if}
  </div>
{:else if field.type === 'user'}
  <UserPicker
    field
    value={users[0] ?? null}
    label={field.name}
    noneLabel={t.common.none}
    search={(q) => api.users(q)}
    onpick={(u) => onchange(u ? [u] : [])}
  />
{:else if field.type === 'multiuser'}
  <div class="people" {id} role="group" aria-label={field.name} aria-describedby={describedby}>
    {#if users.length}
      <ul class="chosen">
        {#each users as u (u.id)}
          <li>
            <span class="chip">
              <Who user={u} link={false} />
              <button
                type="button"
                aria-label={`${t.common.remove} ${u.name}`}
                onclick={() => onchange(users.filter((x) => x.id !== u.id))}>×</button
              ></span
            >
          </li>
        {/each}
      </ul>
    {/if}
    <UserPicker
      field
      value={null}
      allowNone={false}
      label={`${field.name}: ${t.create.addPerson}`}
      noneLabel={t.create.addPerson}
      search={(q) =>
        api.users(q).then((list) => list.filter((u) => !users.some((x) => x.id === u.id)))}
      onpick={(u) => {
        if (u) onchange([...users, u]);
      }}
    />
  </div>
{:else}
  <input
    class="input"
    {id}
    type={field.type === 'number'
      ? 'number'
      : field.type === 'url'
        ? 'url'
        : field.type === 'date'
          ? 'date'
          : field.type === 'datetime'
            ? 'datetime-local'
            : 'text'}
    step={field.type === 'number' ? 'any' : undefined}
    maxlength={field.type === 'text' || field.type === 'url' ? 255 : undefined}
    value={text}
    aria-invalid={invalid}
    aria-describedby={describedby}
    oninput={(e) => onchange(e.currentTarget.value)}
  />
{/if}

<style>
  .boxed {
    border: 1px solid var(--rule-strong);
    background: var(--paper-raised);
    padding: 6px 8px;
    min-height: 2.125rem;
    display: flex;
    align-items: center;
    cursor: text;
  }

  .choices {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 16px;
    padding: 4px 0;
  }

  .choice {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .people {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .chosen {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 0;
    padding: 0;
    list-style: none;
  }
</style>
