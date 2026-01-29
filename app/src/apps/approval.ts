import "@/style.css";
import { createApp as createVueApp, defineComponent, ref, onMounted } from "vue";
import { createApp, callTool, extractJSON } from "@/lib/mcp";

interface Status {
  state: string;
  version: string;
  commit_count: number;
  has_breaking: boolean;
}

interface Evaluation {
  risk_level: string;
  score: number;
  policy_compliant: boolean;
  recommendation: string;
}

interface Check {
  name: string;
  passed: boolean;
  reason?: string;
}

interface Validation {
  checks: Check[];
}

const riskBadgeClass = (level: string): string => {
  const map: Record<string, string> = {
    low: "badge-green",
    medium: "badge-yellow",
    high: "badge-red",
    critical: "badge-red",
  };
  return map[level] ?? "badge-gray";
};

const ApprovalApp = defineComponent({
  setup() {
    const loading = ref(true);
    const error = ref("");
    const status = ref<Status | null>(null);
    const evaluation = ref<Evaluation | null>(null);
    const validation = ref<Validation | null>(null);

    onMounted(async () => {
      try {
        const app = createApp("relicta-approval");

        const [statusRes, evalRes, valRes] = await Promise.all([
          callTool(app, "relicta.status"),
          callTool(app, "relicta.evaluate"),
          callTool(app, "relicta.validate_release", {
            check_git: true,
            check_plugins: true,
            check_governance: true,
          }),
        ]);

        status.value = extractJSON<Status>(statusRes);
        evaluation.value = extractJSON<Evaluation>(evalRes);
        validation.value = extractJSON<Validation>(valRes);
      } catch (e) {
        error.value = String(e);
      } finally {
        loading.value = false;
      }
    });

    return { loading, error, status, evaluation, validation, riskBadgeClass };
  },
  computed: {
    allPassed(): boolean {
      return this.validation?.checks.every((c: Check) => c.passed) ?? false;
    },
  },
  template: `
    <div>
      <p v-if="loading" class="loading">Loading approval data...</p>
      <p v-else-if="error" class="text-red-600">{{ error }}</p>
      <template v-else>
        <div class="flex items-center justify-between mb-4">
          <h1 class="mb-0">
            Release
            <span v-if="status" class="text-blue-600">v{{ status.version }}</span>
          </h1>
          <div class="flex gap-2" v-if="status">
            <span class="badge badge-purple">{{ status.state }}</span>
            <span class="badge badge-gray">{{ status.commit_count }} commits</span>
            <span v-if="status.has_breaking" class="badge badge-red">breaking</span>
          </div>
        </div>

        <div v-if="validation" class="card">
          <h2>Pre-flight Checks</h2>
          <table class="table">
            <thead>
              <tr>
                <th>Check</th>
                <th>Status</th>
                <th>Details</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="check in validation.checks" :key="check.name">
                <td class="font-medium">{{ check.name }}</td>
                <td>
                  <span v-if="check.passed" class="text-green-600 font-medium">Pass</span>
                  <span v-else class="text-red-600 font-medium">Fail</span>
                </td>
                <td class="text-gray-500">{{ check.reason ?? '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="evaluation" class="card">
          <h2>Risk Assessment</h2>
          <div class="flex items-center gap-3 mb-2">
            <span class="badge" :class="riskBadgeClass(evaluation.risk_level)">
              {{ evaluation.risk_level }}
            </span>
            <span class="text-gray-500">Score: {{ evaluation.score }}</span>
            <span
              class="badge"
              :class="evaluation.policy_compliant ? 'badge-green' : 'badge-red'"
            >
              {{ evaluation.policy_compliant ? 'Policy Compliant' : 'Non-Compliant' }}
            </span>
          </div>
          <p class="text-gray-600 text-xs">{{ evaluation.recommendation }}</p>
        </div>

        <div class="card" :class="allPassed ? 'border-green-300 bg-green-50' : 'border-yellow-300 bg-yellow-50'">
          <div class="flex items-center gap-2">
            <span v-if="allPassed" class="badge badge-green">Ready to Approve</span>
            <span v-else class="badge badge-yellow">Not Ready</span>
          </div>
        </div>
      </template>
    </div>
  `,
});

createVueApp(ApprovalApp).mount("#app");
