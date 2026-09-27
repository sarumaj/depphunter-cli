<script context="module" lang="ts">
  import type { Load } from './types'
  export function preload() {}
</script>

<script lang="ts">
  import Counter from './Counter.vue'
  import { onMount } from 'svelte'
  export let name: string
  const greet = () => `hello ${name}`
</script>

<svelte:head>
  <script src="https://cdn.example.com/analytics.js"></script>
  <script>import 'in-head'</script>
</svelte:head>

<h1 title={"</script><script>import 'in-attribute'"}>{greet()}</h1>
{@html "<script>import 'in-expression'</script>"}
<Counter />
