<script setup>
defineProps({ eyebrow: String, title: String, subtitle: String })
</script>

<template>
  <main class="auth-page">
    <section class="auth-intro">
      <a class="brand" href="/"><span class="brand-mark">h</span> handy notes</a>
      <div class="intro-copy">
        <p class="eyebrow">{{ eyebrow }}</p>
        <h1>Good ideas deserve <em>room to breathe.</em></h1>
        <p>Capture small thoughts, shape bigger plans, and find everything again when you need it.</p>
      </div>
      <p class="quote">“Your mind feels lighter when it does not have to remember everything.”</p>
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
