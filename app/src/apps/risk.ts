import "@/style.css";
import { createApp, callTool, extractJSON } from "@/lib/mcp";
import { createApp as createVueApp, defineComponent, ref, computed, onMounted } from "vue";

interface RiskFactor {
  name: string;
  score: number;
  max: number;
  description: string;
}

interface RiskData {
  risk_level: string;
  score: number;
  factors: RiskFactor[];
  recommendation: string;
  policy_compliant: boolean;
}

const RiskDashboard = defineComponent({
  setup() {
    const data = ref<RiskData | null>(null);
    const loading = ref(true);
    const error = ref("");

    const riskBadgeClass = computed(() => {
      if (!data.value) return "badge";
      const level = data.value.risk_level;
      if (level === "low") return "badge badge-green";
      if (level === "medium") return "badge badge-yellow";
      return "badge badge-red";
    });

    onMounted(async () => {
      try {
        const app = createApp("risk-dashboard");
        const result = await callTool(app, "relicta.evaluate");
        data.value = extractJSON<RiskData>(result);
        if (!data.value) error.value = "No data returned from evaluation.";
      } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : "Failed to evaluate risk.";
      } finally {
        loading.value = false;
      }
    });

    return { data, loading, error, riskBadgeClass };
  },
  template: `
    <div class="p-4 space-y-4 max-w-xl">
      <h1 class="text-lg font-semibold">Risk Dashboard</h1>

      <div v-if="loading" class="loading">Evaluating risk...</div>
      <div v-else-if="error" class="text-red-500 text-sm">{{ error }}</div>

      <template v-else-if="data">
        <div class="card flex items-center justify-between">
          <div class="flex items-center gap-3">
            <span :class="riskBadgeClass" class="uppercase text-xs font-bold">{{ data.risk_level }}</span>
            <span class="text-2xl font-mono font-bold">{{ data.score }}</span>
          </div>
          <span :class="data.policy_compliant ? 'badge badge-green' : 'badge badge-red'">
            {{ data.policy_compliant ? 'Policy Compliant' : 'Non-Compliant' }}
          </span>
        </div>

        <div class="card space-y-3">
          <h2 class="text-sm font-semibold mb-2">Risk Factors</h2>
          <div v-for="f in data.factors" :key="f.name" class="space-y-1">
            <div class="flex justify-between text-xs">
              <span>{{ f.description }}</span>
              <span class="font-mono">{{ f.score }}/{{ f.max }}</span>
            </div>
            <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-2">
              <div
                class="h-2 rounded"
                :style="{ width: (f.score / f.max * 100) + '%' }"
                :class="{
                  'bg-green-500': f.score / f.max < 0.4,
                  'bg-yellow-500': f.score / f.max >= 0.4 && f.score / f.max < 0.7,
                  'bg-red-500': f.score / f.max >= 0.7
                }"
              ></div>
            </div>
          </div>
        </div>

        <div class="card text-sm">
          <span class="font-semibold">Recommendation:</span> {{ data.recommendation }}
        </div>
      </template>
    </div>
  `,
});

createVueApp(RiskDashboard).mount("#app");
