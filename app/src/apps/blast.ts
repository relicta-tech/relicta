import "@/style.css";
import { createApp as createVueApp, defineComponent, ref, computed, onMounted } from "vue";
import { createApp, callTool, extractJSON } from "@/lib/mcp";

interface ImpactedPackage {
  path: string;
  direct: boolean;
  risk: string;
  files_changed: number;
}

interface BlastRadius {
  changed_packages: string[];
  impacted_packages: ImpactedPackage[];
  total_files_changed: number;
  transitive_count: number;
  deployment_risk: string;
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

const BlastApp = defineComponent({
  setup() {
    const loading = ref(true);
    const error = ref("");
    const data = ref<BlastRadius | null>(null);

    const sorted = computed(() => {
      if (!data.value) return [];
      return [...data.value.impacted_packages].sort(
        (a, b) => Number(b.direct) - Number(a.direct),
      );
    });

    const directCount = computed(
      () => data.value?.impacted_packages.filter((p) => p.direct).length ?? 0,
    );

    onMounted(async () => {
      try {
        const app = createApp("relicta-blast-radius");
        const res = await callTool(app, "relicta.blast_radius", { transitive: true });
        data.value = extractJSON<BlastRadius>(res);
      } catch (e) {
        error.value = String(e);
      } finally {
        loading.value = false;
      }
    });

    return { loading, error, data, sorted, directCount, riskBadgeClass };
  },
  template: `
    <div>
      <h1>Blast Radius</h1>
      <p v-if="loading" class="loading">Analyzing impact...</p>
      <p v-else-if="error" class="text-red-600">{{ error }}</p>
      <template v-else-if="data">
        <div class="flex gap-3 mb-4">
          <div class="card flex-1 text-center">
            <div class="text-2xl font-bold">{{ data.total_files_changed }}</div>
            <div class="text-xs text-gray-500">Files Changed</div>
          </div>
          <div class="card flex-1 text-center">
            <div class="text-2xl font-bold">{{ directCount }}</div>
            <div class="text-xs text-gray-500">Direct</div>
          </div>
          <div class="card flex-1 text-center">
            <div class="text-2xl font-bold">{{ data.transitive_count }}</div>
            <div class="text-xs text-gray-500">Transitive</div>
          </div>
          <div class="card flex-1 text-center">
            <span class="badge" :class="riskBadgeClass(data.deployment_risk)">
              {{ data.deployment_risk }}
            </span>
            <div class="text-xs text-gray-500 mt-1">Deploy Risk</div>
          </div>
        </div>

        <h2>Impacted Packages</h2>
        <div
          v-for="pkg in sorted"
          :key="pkg.path"
          class="card flex items-center justify-between"
          :style="{ borderLeftWidth: '3px', borderLeftColor: pkg.direct ? '#3b82f6' : '#d1d5db' }"
        >
          <div>
            <span class="font-medium">{{ pkg.path }}</span>
            <span
              class="badge ml-2"
              :class="pkg.direct ? 'badge-blue' : 'badge-gray'"
            >
              {{ pkg.direct ? 'direct' : 'transitive' }}
            </span>
          </div>
          <div class="flex items-center gap-3">
            <span class="text-xs text-gray-500">{{ pkg.files_changed }} files</span>
            <span class="badge" :class="riskBadgeClass(pkg.risk)">{{ pkg.risk }}</span>
          </div>
        </div>
      </template>
    </div>
  `,
});

createVueApp(BlastApp).mount("#app");
