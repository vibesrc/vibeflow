import type {
  Flow,
  NodeType,
  PluginKey,
  HealthResponse,
  CreateFlowRequest,
  UpdateFlowRequest,
  InstallPluginRequest,
} from './types';

const API_BASE = '/api';

async function fetchApi<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

// Health
export async function getHealth(): Promise<HealthResponse> {
  return fetchApi<HealthResponse>('/health');
}

// Flows
export async function listFlows(): Promise<Flow[]> {
  return fetchApi<Flow[]>('/flows');
}

export async function getFlow(id: string): Promise<Flow> {
  return fetchApi<Flow>(`/flows/${id}`);
}

export async function createFlow(data: CreateFlowRequest): Promise<Flow> {
  return fetchApi<Flow>('/flows', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function updateFlow(id: string, data: UpdateFlowRequest): Promise<Flow> {
  return fetchApi<Flow>(`/flows/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

export async function deleteFlow(id: string): Promise<void> {
  await fetchApi(`/flows/${id}`, { method: 'DELETE' });
}

export async function startFlow(id: string): Promise<void> {
  await fetchApi(`/flows/${id}/start`, { method: 'POST' });
}

export async function stopFlow(id: string): Promise<void> {
  await fetchApi(`/flows/${id}/stop`, { method: 'POST' });
}

export async function restartFlow(id: string): Promise<void> {
  await fetchApi(`/flows/${id}/restart`, { method: 'POST' });
}

export async function getFlowErrors(id: string): Promise<Record<string, string>> {
  return fetchApi<Record<string, string>>(`/flows/${id}/errors`);
}

// Node Types
export async function listNodeTypes(): Promise<NodeType[]> {
  return fetchApi<NodeType[]>('/node-types');
}

// Plugins
export async function listPlugins(): Promise<PluginKey[]> {
  return fetchApi<PluginKey[]>('/plugins');
}

export async function installPlugin(data: InstallPluginRequest): Promise<void> {
  await fetchApi('/plugins/install', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}
