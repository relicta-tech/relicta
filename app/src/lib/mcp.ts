import { App } from "@modelcontextprotocol/ext-apps";

export interface ToolContent {
  type: string;
  text?: string;
  [key: string]: unknown;
}

export interface ToolResult {
  content?: ToolContent[];
  isError?: boolean;
}

export function createApp(name: string): App {
  return new App({ name, version: "0.1.0" });
}

export function extractText(result: ToolResult): string {
  return result.content?.find((c) => c.type === "text")?.text ?? "";
}

export function extractJSON<T>(result: ToolResult): T | null {
  const text = extractText(result);
  if (!text) return null;
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

export async function callTool(
  app: App,
  name: string,
  args: Record<string, unknown> = {},
): Promise<ToolResult> {
  return (await app.callServerTool({ name, arguments: args })) as ToolResult;
}
