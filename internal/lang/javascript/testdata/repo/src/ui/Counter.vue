<template>
  <!-- <script>import 'in-comment'</script> -->
  <button :title="'<script>'" @click="count++">{{ "<script>import 'in-mustache'</script>" }}</button>
  <template v-if="count > 1">
    <script>import 'in-template'</script>
  </template>
  <Child />
</template>

<script lang="ts">
import { defineComponent } from 'vue'

export default defineComponent({ name: 'Counter' })
</script>

<script setup lang="ts">
import { ref } from 'vue'
import Child from './Child.vue'
import { add } from '@app/lib/math'
import chalk from 'chalk'

const count = ref(0)
const closing = '<\/script>'
function increment() {
  count.value = add(count.value, 1)
}
</script>

<style scoped>
@import './theme.css';
button::after { content: "<script>" }
</style>
