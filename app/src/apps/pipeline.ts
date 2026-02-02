import "@/style.css";
import { createApp as createVueApp, defineComponent, ref, onMounted, computed } from "vue";
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

const PipelineApp = defineComponent({
  setup() {
    const status = ref<ReleaseStatus | null>(null);
    const message = ref("");
    const loading = ref(true);
    const error = ref("");

    onMounted(async () => {
      try {
        const app = createApp("relicta-pipeline");
        const result = await callTool(app, "relicta_status");
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

    const createdDate = computed(() => {
      if (!status.value?.created_at) return "";
      return new Date(status.value.created_at).toLocaleString();
    });

    function stageState(stage: string): "completed" | "current" | "pending" {
      if (!status.value) return "pending";
      if (status.value.steps_completed.includes(stage)) return "completed";
      if (status.value.next_action === stage) return "current";
      return "pending";
    }

    return { status, message, loading, error, createdDate, stageState, STAGES };
  },
  template: `
    <div>
      <h1>Release Pipeline</h1>

      <p v-if="loading" class="loading">Loading pipeline...</p>
      <p v-else-if="error" class="text-red-600">{{ error }}</p>

      <template v-else-if="status">
        <div class="flex items-center gap-3 mb-4 text-xs text-gray-500">
          <span class="font-semibold text-base text-gray-900">v{{ status.version }}</span>
          <span class="badge badge-gray font-mono">{{ status.release_id }}</span>
          <span>{{ status.state }}</span>
          <span v-if="createdDate">{{ createdDate }}</span>
        </div>

        <div class="flex items-center gap-0.5 mb-4">
          <template v-for="(stage, i) in STAGES" :key="stage">
            <div
              class="flex items-center justify-center rounded-md px-3 py-2 text-xs font-semibold min-w-[72px] text-center"
              :class="{
                'bg-green-500 text-white': stageState(stage) === 'completed',
                'bg-blue-500 text-white animate-pulse': stageState(stage) === 'current',
                'bg-gray-100 text-gray-400': stageState(stage) === 'pending',
              }"
            >
              <span v-if="stageState(stage) === 'completed'" class="mr-1">&#10003;</span>
              {{ stage }}
            </div>
            <svg v-if="i < STAGES.length - 1" class="w-4 h-4 text-gray-300 shrink-0" viewBox="0 0 16 16" fill="currentColor">
              <path d="M6 3l5 5-5 5" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </template>
        </div>

        <div class="card">
          <div class="grid grid-cols-2 gap-x-6 gap-y-1 text-xs">
            <span class="text-gray-500">Commits</span>
            <span class="font-medium">{{ status.commit_count }}</span>
            <span class="text-gray-500">Breaking changes</span>
            <span class="font-medium" :class="status.has_breaking ? 'text-red-600' : ''">
              {{ status.has_breaking ? 'Yes' : 'No' }}
            </span>
            <span class="text-gray-500">Next action</span>
            <span class="font-medium">{{ status.next_action }}</span>
          </div>
        </div>
      </template>

      <div v-else class="card text-gray-500">{{ message }}</div>
    </div>
  `,
});

createVueApp(PipelineApp).mount("#app");
