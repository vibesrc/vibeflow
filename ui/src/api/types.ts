export interface Flow {
  id: string;
  name: string;
  description: string;
  content: string;
  enabled: boolean;
  status: 'stopped' | 'running' | 'error';
  runtime_status: 'stopped' | 'running' | 'error';
  created_at: string;
  updated_at: string;
}

export interface PortInfo {
  name: string;
  description?: string;
}

// Conditions for when a config field should be visible
export interface ShowWhen {
  field: string;        // Name of the field to check
  eq?: unknown;         // Show when field equals this value
  ne?: unknown;         // Show when field does not equal this value
  in?: unknown[];       // Show when field value is in this list
  notIn?: unknown[];    // Show when field value is not in this list
  present?: boolean;    // Show when field has a non-empty value
  absent?: boolean;     // Show when field is empty/unset
}

export interface ConfigSpec {
  name: string;
  type: 'string' | 'int' | 'bool' | 'float' | 'object' | 'array';
  required?: boolean;
  default?: unknown;
  description?: string;
  options?: unknown[];
  items?: ConfigSpec[];
  // Code editor support
  format?: 'code' | 'template' | 'expression';
  language?: string; // e.g., 'javascript', 'json', 'yaml', 'template'
  // Conditional visibility
  showWhen?: ShowWhen;
}

export interface NodeType {
  type: string;
  category: string;
  name: string;
  description?: string;
  is_external: boolean;
  inputs?: PortInfo[];
  outputs?: PortInfo[];
  config?: ConfigSpec[];
}

export interface PluginKey {
  Name: string;
  Version: string;
}

export interface HealthResponse {
  status: string;
  running_flows: number;
}

// Flow YAML types
export interface FlowDefinition {
  version: string;
  metadata?: {
    name?: string;
    description?: string;
    tags?: string[];
  };
  environment?: Record<string, string>;
  nodes: NodeDefinition[];
  wires: Wire[];
}

export interface NodeDefinition {
  id: string;
  type: string;
  name?: string;
  config?: Record<string, unknown>;
  enabled?: boolean;
  // Position for UI
  x?: number;
  y?: number;
}

export interface Wire {
  from: string;
  to: string;
  output: string;
  input: string;
}

// API request types
export interface CreateFlowRequest {
  name: string;
  description?: string;
  content?: string;
  enabled?: boolean;
}

export interface UpdateFlowRequest {
  name?: string;
  description?: string;
  content?: string;
  enabled?: boolean;
}

export interface InstallPluginRequest {
  source: string;
  version?: string;
}
