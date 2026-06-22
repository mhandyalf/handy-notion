<script setup>
defineProps({ eyebrow: String, title: String, subtitle: String })
</script>

<template>
  <main class="auth-page">
    <section class="auth-intro">
      <a class="brand" href="/"><span class="brand-mark">h</span> handy notes</a>
      <div class="intro-copy">
        <p class="eyebrow">{{ eyebrow }}</p>
        <h1>Ide yang bagus layak punya <em>ruang bernapas.</em></h1>
        <p>Catat hal kecil, susun rencana besar, dan temukan semuanya lagi saat kamu membutuhkannya.</p>
      </div>
      <p class="quote">“Pikiran jadi lebih ringan ketika tidak harus mengingat semuanya.”</p>
    </section>
    <section class="auth-panel">
      <div class="auth-card">
        <p class="eyebrow">{{ eyebrow }}</p>
        <h2>{{ title }}</h2>
        <p class="muted">{{ subtitle }}</p>
        <slot />
      </div>
    </section>
  </main>
</template>

