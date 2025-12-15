import { memo, useMemo } from 'react';
import { Position, type NodeProps, type Node } from '@xyflow/react';
import { ChevronDown } from 'lucide-react';
import { BaseNode, BaseNodeHeader, BaseNodeHeaderTitle } from '@/components/base-node';
import { BaseHandle } from '@/components/base-handle';
import type { PortInfo } from '@/api';

export type VibeflowNodeData = {
  name: string;
  type: string;
  config?: Record<string, unknown>;
  inputs?: PortInfo[];
  outputs?: PortInfo[];
  expanded?: boolean;
  disabled?: boolean;
  hasError?: boolean;
  onToggleExpanded?: (nodeId: string, expanded: boolean) => void;
};

export type VibeflowNodeType = Node<VibeflowNodeData, 'vibeflow'>;

// Compute dynamic outputs from config - looks for array config fields with items containing "output" field
function computeDynamicOutputs(config: Record<string, unknown> | undefined, baseOutputs: PortInfo[]): PortInfo[] {
  if (!config) return baseOutputs;

  const dynamicOutputs = new Set<string>();

  // Check all config fields for arrays that might define outputs
  for (const [, value] of Object.entries(config)) {
    if (Array.isArray(value)) {
      for (const item of value) {
        // If an item has an 'output' field, use it as an output port
        if (item && typeof item === 'object' && 'output' in item && typeof item.output === 'string' && item.output) {
          dynamicOutputs.add(item.output);
        }
      }
    }
  }

  // If we found dynamic outputs, combine with base outputs
  if (dynamicOutputs.size > 0) {
    const outputs: PortInfo[] = [];
    for (const output of dynamicOutputs) {
      outputs.push({ name: output, description: `Dynamic output: ${output}` });
    }
    // Add base outputs that aren't already included
    for (const baseOutput of baseOutputs) {
      if (!dynamicOutputs.has(baseOutput.name)) {
        outputs.push(baseOutput);
      }
    }
    return outputs;
  }

  return baseOutputs;
}

// Grid size for snapping
const GRID_SIZE = 20;
// Row height for expanded port labels (matches grid)
const PORT_ROW_HEIGHT = GRID_SIZE;
// Header height (2 grid units)
const HEADER_HEIGHT = 2 * GRID_SIZE;
// Minimum spacing between handles in collapsed mode
const MIN_HANDLE_SPACING = GRID_SIZE;

export const VibeflowNode = memo(function VibeflowNode({
  id,
  data,
  selected,
}: NodeProps<VibeflowNodeType>) {
  const expanded = data.expanded ?? false;
  const inputs = data.inputs ?? [];
  const baseOutputs = data.outputs ?? [];

  // Compute dynamic outputs based on config (e.g., switch rules define output ports)
  const outputs = useMemo(
    () => computeDynamicOutputs(data.config, baseOutputs),
    [data.config, baseOutputs]
  );

  const maxHandles = Math.max(inputs.length, outputs.length);

  // Calculate collapsed height - expand if handles need more space, snap to grid
  const collapsedHandleSpace = (maxHandles + 1) * MIN_HANDLE_SPACING;
  const rawCollapsedHeight = Math.max(HEADER_HEIGHT, collapsedHandleSpace);
  const collapsedHeight = Math.ceil(rawCollapsedHeight / GRID_SIZE) * GRID_SIZE;

  // Calculate expanded height, snap to grid
  // Header (2 units) + handles * row height (1 unit each) + 1 unit padding (half top, half bottom)
  const rawExpandedHeight = HEADER_HEIGHT + maxHandles * PORT_ROW_HEIGHT + GRID_SIZE;
  const expandedHeight = Math.ceil(rawExpandedHeight / GRID_SIZE) * GRID_SIZE;

  // Calculate height based on expanded state
  const nodeHeight = expanded ? expandedHeight : collapsedHeight;

  const handleToggle = (e: React.MouseEvent) => {
    e.stopPropagation();
    data.onToggleExpanded?.(id, !expanded);
  };

  // Calculate handle positions
  // Collapsed: evenly distributed with minimum spacing
  // Expanded: centered in each row with half-grid padding from header
  const getInputTop = (index: number) => {
    if (expanded) {
      // Header + half-grid padding + center of each row
      return HEADER_HEIGHT + GRID_SIZE / 2 + PORT_ROW_HEIGHT * index + PORT_ROW_HEIGHT / 2;
    }
    // Collapsed: distribute handles evenly within collapsed height
    const spacing = collapsedHeight / (inputs.length + 1);
    return spacing * (index + 1);
  };

  const getOutputTop = (index: number) => {
    if (expanded) {
      // Header + half-grid padding + center of each row
      return HEADER_HEIGHT + GRID_SIZE / 2 + PORT_ROW_HEIGHT * index + PORT_ROW_HEIGHT / 2;
    }
    // Collapsed: distribute handles evenly within collapsed height
    const spacing = collapsedHeight / (outputs.length + 1);
    return spacing * (index + 1);
  };

  const disabled = data.disabled ?? false;
  const hasError = data.hasError ?? false;

  return (
    <BaseNode
      selected={selected}
      disabled={disabled}
      hasError={hasError}
      className="min-w-[160px] transition-[height] duration-200"
      style={{ height: nodeHeight }}
    >
      <BaseNodeHeader
        showBorder={expanded}
        style={{ height: expanded ? HEADER_HEIGHT : collapsedHeight }}
        className="transition-[height] duration-200"
      >
        <BaseNodeHeaderTitle className="truncate">
          {data.name || 'Untitled'}
        </BaseNodeHeaderTitle>
        <button
          onClick={handleToggle}
          className="p-1.5 -m-1 rounded hover:bg-muted/50 transition-all opacity-0 group-hover:opacity-100 group-data-[selected]:opacity-100"
        >
          <ChevronDown
            className={`w-3.5 h-3.5 text-muted-foreground transition-transform duration-200 ${
              expanded ? 'rotate-180' : ''
            }`}
          />
        </button>
      </BaseNodeHeader>

      {/* Expanded: show port labels as rows */}
      <div
        className="overflow-hidden transition-all duration-200"
        style={{
          height: expanded ? expandedHeight - HEADER_HEIGHT : 0,
          opacity: expanded ? 1 : 0,
        }}
      >
        <div className="px-2" style={{ paddingTop: GRID_SIZE / 2, paddingBottom: GRID_SIZE / 2 }}>
          {Array.from({ length: maxHandles }).map((_, index) => {
            const input = inputs[index];
            const output = outputs[index];
            return (
              <div
                key={index}
                className="flex items-center justify-between text-[10px] text-muted-foreground"
                style={{ height: PORT_ROW_HEIGHT }}
              >
                <span className="truncate max-w-[60px]">{input?.name || ''}</span>
                <span className="truncate max-w-[60px] text-right">{output?.name || ''}</span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Input handles */}
      {inputs.map((input, index) => (
        <BaseHandle
          key={`input-${input.name}`}
          type="target"
          position={Position.Left}
          id={input.name}
          style={{
            top: getInputTop(index),
            transition: 'top 200ms',
          }}
        />
      ))}

      {/* Output handles */}
      {outputs.map((output, index) => (
        <BaseHandle
          key={`output-${output.name}`}
          type="source"
          position={Position.Right}
          id={output.name}
          style={{
            top: getOutputTop(index),
            transition: 'top 200ms',
          }}
        />
      ))}
    </BaseNode>
  );
});
