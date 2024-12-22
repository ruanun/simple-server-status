<script lang="ts">
import 'flag-icons-svg/css/flag-icons.css';
export default {
  props: {
    countryCode: {
      type: String,
      required: true
    }
  }
}
</script>

<template>
  <div class="main" >
    <a-tooltip>
      <template #title>{{ countryCode.toUpperCase() }}</template>
      <span :class="`flag-icon flag-icon-${countryCode.toLowerCase()}`"></span>
    </a-tooltip>
  </div>
</template>

<style scoped>

.main {
  display: inline-block;
  line-height: 1;
  margin: 5px
}
</style>
