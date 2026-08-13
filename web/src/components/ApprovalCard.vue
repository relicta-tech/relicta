<!--
  ApprovalCard.vue — single canonical renderer for the release-approval
  view. Consumes the ApprovalCard type from types/approvalCard.ts (mirrors
  pkg/cgp.ApprovalCard server-side).

  Tier-aware styling stays consistent with the CLI lipgloss renderer
  (internal/ui/risk.go) so users see the same visual hierarchy regardless of
  surface. Severity glyph (▲▲▲) carries severity even when colors fail.
-->

<template>
  <article
    :class="cardClasses"
    data-testid="approval-card"
    :data-tier="card.risk.tier"
  >
    <header class="flex items-start justify-between mb-4">
      <div>
        <h2 class="text-xl font-semibold">
          <span class="font-mono text-sm text-gray-600">{{ card.cardId }}</span>
        </h2>
        <p
          v-if="card.version || card.repository"
          class="text-sm text-gray-600 mt-1"
        >
          <span v-if="card.repository">{{ card.repository }}</span>
          <span
            v-if="card.version"
            class="font-mono"
          >@{{ card.version }}</span>
        </p>
      </div>

      <div :class="['text-right', tierStyle.text]">
        <div class="flex items-center gap-2 justify-end">
          <span
            class="font-mono text-lg"
            :aria-label="`risk ${card.risk.tier}`"
          >{{ glyph }}</span>
          <span class="font-bold text-2xl">{{ severityLabel }}</span>
        </div>
        <div class="text-sm">
          {{ Math.round(card.risk.score * 100) }} / 100
        </div>
      </div>
    </header>

    <div class="mb-4">
      <div class="w-full h-2 bg-gray-200 rounded-full overflow-hidden">
        <div
          :class="['h-full transition-all duration-300', tierStyle.bar]"
          :style="{ width: `${Math.round(card.risk.score * 100)}%` }"
          role="progressbar"
          :aria-label="`Risk score ${Math.round(card.risk.score * 100)} out of 100`"
          :aria-valuenow="Math.round(card.risk.score * 100)"
          aria-valuemin="0"
          aria-valuemax="100"
        />
      </div>
    </div>

    <section
      v-if="card.risk.factors && card.risk.factors.length"
      class="mb-4"
    >
      <h3 class="font-semibold text-sm uppercase tracking-wide text-gray-700 mb-2">
        Risk Factors
      </h3>
      <ul class="space-y-1">
        <li
          v-for="factor in topFactors"
          :key="factor.category"
          class="flex items-start gap-2 text-sm"
        >
          <span class="font-mono text-xs uppercase text-gray-600 min-w-[80px]">
            {{ factor.category }}
          </span>
          <span class="flex-1">{{ factor.description }}</span>
          <span class="font-mono text-xs text-gray-600">
            {{ Math.round(factor.score * 100) }}%
          </span>
        </li>
      </ul>
    </section>

    <section
      v-if="card.diffSummary"
      class="mb-4"
    >
      <h3 class="font-semibold text-sm uppercase tracking-wide text-gray-700 mb-2">
        Diff Summary
      </h3>
      <p class="text-sm text-gray-700 whitespace-pre-line">
        {{ card.diffSummary }}
      </p>
    </section>

    <section class="mb-4">
      <h3 class="font-semibold text-sm uppercase tracking-wide text-gray-700 mb-2">
        Actor
      </h3>
      <div class="flex items-center gap-2 text-sm">
        <span
          class="px-2 py-0.5 rounded-full text-xs font-mono uppercase"
          :class="actorBadgeClass"
        >{{ card.actor.kind }}</span>
        <span class="font-mono">{{ card.actor.id }}</span>
        <span
          v-if="card.actor.name"
          class="text-gray-600"
        >— {{ card.actor.name }}</span>
      </div>
    </section>

    <section
      v-if="card.rationale && card.rationale.length"
      class="mb-4"
    >
      <h3 class="font-semibold text-sm uppercase tracking-wide text-gray-700 mb-2">
        Rationale
      </h3>
      <ul class="list-disc list-inside text-sm space-y-1 text-gray-700">
        <li
          v-for="(line, idx) in card.rationale"
          :key="idx"
        >
          {{ line }}
        </li>
      </ul>
    </section>

    <section
      v-if="card.frameworks && card.frameworks.length"
      class="mb-4 flex gap-2 flex-wrap"
    >
      <span
        v-for="fw in card.frameworks"
        :key="fw"
        class="px-2 py-1 text-xs font-mono bg-gray-100 text-gray-700 rounded"
      >{{ fw }}</span>
    </section>

    <footer class="flex items-center justify-between pt-4 border-t border-gray-200">
      <div class="text-xs text-gray-600">
        <span class="font-semibold">Decision:</span>
        <span class="ml-1 font-mono">{{ card.decision }}</span>
      </div>
      <div class="flex gap-2">
        <button
          v-for="action in card.availableActions"
          :key="action.id"
          :class="actionButtonClasses(action)"
          :data-action-id="action.id"
          @click="$emit('action', action)"
        >
          {{ action.label }}
        </button>
      </div>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  type ApprovalAction,
  type ApprovalCard,
  riskGlyphForTier,
  riskSeverityLabel,
  tierClasses,
} from '@/types/approvalCard'

const props = defineProps<{ card: ApprovalCard }>()
defineEmits<{
  (e: 'action', action: ApprovalAction): void
}>()

const tierStyle = computed(() => tierClasses(props.card.risk.tier))
const glyph = computed(() => props.card.risk.glyph || riskGlyphForTier(props.card.risk.tier))
const severityLabel = computed(() => props.card.risk.severity || riskSeverityLabel(props.card.risk.tier))

const cardClasses = computed(() => [
  'rounded-lg border-2 p-6 shadow-sm',
  tierStyle.value.border,
  tierStyle.value.background,
])

// Top three factors by score, descending — keeps the card readable for
// releases with many factors.
const topFactors = computed(() => {
  const sorted = [...(props.card.risk.factors ?? [])].sort((a, b) => b.score - a.score)
  return sorted.slice(0, 3)
})

const actorBadgeClass = computed(() => {
  switch (props.card.actor.kind) {
    case 'human':
      return 'bg-blue-100 text-blue-800'
    case 'agent':
      return 'bg-purple-100 text-purple-800'
    case 'ci':
      return 'bg-gray-200 text-gray-800'
    default:
      return 'bg-gray-100 text-gray-700'
  }
})

function actionButtonClasses(action: ApprovalAction): string[] {
  const base = ['px-4 py-2 rounded text-sm font-medium transition-colors']
  switch (action.type) {
    case 'primary':
      // emerald-700: white text on emerald-600 is 3.76:1, below WCAG AA
      return [...base, 'bg-emerald-700 text-white hover:bg-emerald-800']
    case 'danger':
      return [...base, 'bg-red-600 text-white hover:bg-red-700']
    default:
      return [...base, 'bg-gray-200 text-gray-800 hover:bg-gray-300']
  }
}
</script>
