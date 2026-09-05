import { apiClient } from "../client";

export type CatalogPlatform = "openai" | "anthropic" | "gemini" | "antigravity";

export const CATALOG_PLATFORMS: CatalogPlatform[] = [
  "openai",
  "anthropic",
  "gemini",
  "antigravity",
];

export interface OpenAIModelCatalog {
  display_models: string[];
  whitelist_models: string[];
  merged_account_count?: number;
}

export interface OpenAIModelCatalogMergePreview extends OpenAIModelCatalog {
  merge_keys: string[];
  merged_account_count: number;
}

export async function getPlatformModelCatalog(
  platform: CatalogPlatform,
): Promise<OpenAIModelCatalog> {
  const { data } = await apiClient.get<OpenAIModelCatalog>(
    `/admin/model-catalog/${platform}`,
  );
  return data;
}

export async function previewPlatformModelCatalogMerge(
  platform: CatalogPlatform,
  payload: {
    display_models: string[];
    whitelist_models: string[];
  },
): Promise<OpenAIModelCatalogMergePreview> {
  const { data } = await apiClient.post<OpenAIModelCatalogMergePreview>(
    `/admin/model-catalog/${platform}/preview-merge`,
    payload,
  );
  return data;
}

export async function updatePlatformModelCatalog(
  platform: CatalogPlatform,
  payload: {
    display_models: string[];
    whitelist_models: string[];
  },
): Promise<OpenAIModelCatalog> {
  const { data } = await apiClient.put<OpenAIModelCatalog>(
    `/admin/model-catalog/${platform}`,
    payload,
  );
  return data;
}

export async function getOpenAIModelCatalog(): Promise<OpenAIModelCatalog> {
  return getPlatformModelCatalog("openai");
}

export async function previewOpenAIModelCatalogMerge(payload: {
  display_models: string[];
  whitelist_models: string[];
}): Promise<OpenAIModelCatalogMergePreview> {
  return previewPlatformModelCatalogMerge("openai", payload);
}

export async function updateOpenAIModelCatalog(payload: {
  display_models: string[];
  whitelist_models: string[];
}): Promise<OpenAIModelCatalog> {
  return updatePlatformModelCatalog("openai", payload);
}
