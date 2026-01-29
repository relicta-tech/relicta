import "@/style.css";
import { createApp as createVueApp, defineComponent, ref, onMounted } from "vue";
import { createApp, callTool, extractJSON, extractText } from "@/lib/mcp";

interface ReleaseStatus {
  release_id: string;
  state: string;
  version: string;
  created_at: string;
  steps_completed: string[];
  next_action: string;
  commit_count: number;
  has_breaking: boolean;
}

const STAGES = ["plan", "bump", "notes", "evaluate", "approve", "publish"];

const StatusApp = defineComponent({
  setup() {
    const status = ref<ReleaseStatus | null>(null);
    const message = ref("");
    const loading = ref(true);
    const error = ref("");

    onMounted(async () => {
      try {
        const app = createApp("relicta-status");
        const result = await callTool(app, "relicta.status");
        const data = extractJSON<ReleaseStatus>(result);
        if (data && data.release_id) {
          status.value = data;
        } else {
          message.value = extractText(result) || "No active release.";
        }
      } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : "Failed to fetch status.";
      } finally {
        loading.value = false;
      }
    });

    function stateBadgeClass(state: string): string {
      const map: Record<string, string> = {
        planned: "badge-blue",
        bumped: "badge-purple",
        noted: "badge-yellow",
        evaluated: "badge-yellow",
        approved: "badge-green",
        published: "badge-green",
        cancelled: "badge-red",
        failed: "badge-red",
      };
      return map[state] ?? "badge-gray";
    }

    function stageClass(stage: string): string {
      if (!status.value) return "bg-gray-100 text-gray-400";
      if (status.value.steps_completed.includes(stage)) {
        return "bg-green-100 text-green-700";
      }
      if (status.value.next_action === stage) {
        return "bg-blue-100 text-blue-700";
      }
      return "bg-gray-100 text-gray-400";
    }

    return { status, message, loading, error, stateBadgeClass, stageClass, STAGES };
  },
  template: `
    <div>
      <h1>Release Status</h1>

      <p v-if="loading" class="loading">Loading status...</p>
      <p v-else-if="error" class="text-red-600">{{ error }}</p>

      <div v-else-if="status" class="card">
        <div class="flex items-center justify-between mb-2">
          <span class="font-semibold text-base">v{{ status.version }}</span>
          <span class="badge" :class="stateBadgeClass(status.state)">{{ status.state }}</span>
        </div>

        <div class="flex gap-1 mb-3">
          <span
            v-for="stage in STAGES"
            :key="stage"
            :class="stageClass(stage)"
            class="px-2 py-0.5 rounded text-xs font-medium"
          >{{ stage }}</span>
        </div>

        <div class="grid grid-cols-2 gap-x-4 gap-y-1 text-xs text-gray-600">
          <span>Next action</span>
          <span class="font-medium text-gray-900">{{ status.next_action }}</span>
          <span>Commits</span>
          <span class="font-medium text-gray-900">{{ status.commit_count }}</span>
          <span>Breaking</span>
          <span class="font-medium" :class="status.has_breaking ? 'text-red-600' : 'text-gray-900'">
            {{ status.has_breaking ? 'Yes' : 'No' }}
          </span>
          <span>ID</span>
          <span class="font-medium text-gray-500 font-mono">{{ status.release_id }}</span>
        </div>
      </div>

      <div v-else class="card text-gray-500">{{ message }}</div>
    </div>
  `,
});

createVueApp(StatusApp).mount("#app");
