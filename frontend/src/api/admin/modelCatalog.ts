import { apiClient } from "../client";

export interface OpenAIModelCatalog {
  display_models: string[];
  whitelist_models: string[];
  merged_account_count?: number;
}

export interface OpenAIModelCatalogMergePreview extends OpenAIModelCatalog {
  merge_keys: string[];
  merged_account_count: number;
}

export async function getOpenAIModelCatalog(): Promise<OpenAIModelCatalog> {
  const { data } = await apiClient.get<OpenAIModelCatalog>(
    "/admin/model-catalog/openai",
  );
  return data;
}

export async function previewOpenAIModelCatalogMerge(payload: {
  display_models: string[];
  whitelist_models: string[];
}): Promise<OpenAIModelCatalogMergePreview> {
  const { data } = await apiClient.post<OpenAIModelCatalogMergePreview>(
    "/admin/model-catalog/openai/preview-merge",
    payload,
  );
  return data;
}

export async function updateOpenAIModelCatalog(payload: {
  display_models: string[];
  whitelist_models: string[];
}): Promise<OpenAIModelCatalog> {
  const { data } = await apiClient.put<OpenAIModelCatalog>(
    "/admin/model-catalog/openai",
    payload,
  );
  return data;
}
