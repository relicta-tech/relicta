import "@/style.css";
import { createApp, callTool, extractJSON } from "@/lib/mcp";
import { createApp as createVueApp, defineComponent, ref, onMounted } from "vue";

interface Classification {
  hash: string;
  short_hash: string;
  message: string;
  type: string;
  breaking: boolean;
  confidence: number;
}

interface Summary {
  features: number;
  fixes: number;
  breaking: number;
  other: number;
}

interface PlanData {
  version: string;
  bump_level: string;
  total_commits: number;
  classifications: Classification[];
  summary: Summary;
}

function typeBadgeClass(type: string, breaking: boolean): string {
  if (breaking) return "badge badge-red";
  if (type === "feat") return "badge badge-blue";
  if (type === "fix") return "badge badge-green";
  if (type === "refactor") return "badge badge-purple";
  return "badge badge-gray";
}

function bumpBadgeClass(level: string): string {
  if (level === "major") return "badge badge-red";
  if (level === "minor") return "badge badge-yellow";
  return "badge badge-green";
}

const CommitReview = defineComponent({
  setup() {
    const data = ref<PlanData | null>(null);
    const loading = ref(true);
    const error = ref("");

    onMounted(async () => {
      try {
        const app = createApp("commit-review");
        const result = await callTool(app, "relicta_plan", { analyze: true });
        data.value = extractJSON<PlanData>(result);
        if (!data.value) error.value = "No data returned from plan.";
      } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : "Failed to load plan.";
      } finally {
        loading.value = false;
      }
    });

    return { data, loading, error, typeBadgeClass, bumpBadgeClass };
  },
  template: `
    <div class="p-4 space-y-4 max-w-2xl">
      <h1 class="text-lg font-semibold">Commit Classification Review</h1>

      <div v-if="loading" class="loading">Analyzing commits...</div>
      <div v-else-if="error" class="text-red-500 text-sm">{{ error }}</div>

      <template v-else-if="data">
        <div class="flex flex-wrap gap-3">
          <div class="card flex-1 min-w-[100px] text-center">
            <div class="text-2xl font-bold">{{ data.summary.features }}</div>
            <div class="text-xs text-gray-500">Features</div>
          </div>
          <div class="card flex-1 min-w-[100px] text-center">
            <div class="text-2xl font-bold">{{ data.summary.fixes }}</div>
            <div class="text-xs text-gray-500">Fixes</div>
          </div>
          <div class="card flex-1 min-w-[100px] text-center">
            <div class="text-2xl font-bold text-red-600">{{ data.summary.breaking }}</div>
            <div class="text-xs text-gray-500">Breaking</div>
          </div>
          <div class="card flex-1 min-w-[100px] text-center">
            <div class="text-2xl font-bold">{{ data.summary.other }}</div>
            <div class="text-xs text-gray-500">Other</div>
          </div>
          <div class="card flex-1 min-w-[100px] text-center">
            <span :class="bumpBadgeClass(data.bump_level)">{{ data.bump_level }}</span>
            <div class="text-xs text-gray-500 mt-1">{{ data.version }}</div>
          </div>
        </div>

        <table class="table w-full text-sm">
          <thead>
            <tr>
              <th class="text-left">Hash</th>
              <th class="text-left">Message</th>
              <th class="text-left">Type</th>
              <th class="text-right">Confidence</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="c in data.classifications"
              :key="c.hash"
              :class="{ 'border-l-2 border-l-red-500': c.breaking }"
            >
              <td class="font-mono text-xs">{{ c.short_hash }}</td>
              <td>{{ c.message }}</td>
              <td><span :class="typeBadgeClass(c.type, c.breaking)">{{ c.breaking ? 'breaking' : c.type }}</span></td>
              <td class="text-right font-mono">{{ (c.confidence * 100).toFixed(0) }}%</td>
            </tr>
          </tbody>
        </table>
      </template>
    </div>
  `,
});

createVueApp(CommitReview).mount("#app");
