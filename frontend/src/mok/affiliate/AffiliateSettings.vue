<template>
  <!-- The fork's part of the affiliate block in views/admin/SettingsView.vue; each card saves on its own. -->
  <div class="space-y-6">
    <AffiliateScopeSettings />
    <AffiliateRebateModeSettings />
    <AffiliateRebateTiersSettings />
  </div>
</template>

<script setup lang="ts">
import AffiliateRebateModeSettings from './AffiliateRebateModeSettings.vue'
import AffiliateRebateTiersSettings from './AffiliateRebateTiersSettings.vue'
import AffiliateScopeSettings from './AffiliateScopeSettings.vue'
</script>
