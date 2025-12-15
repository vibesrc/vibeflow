import { useCallback, useMemo, useRef } from 'react';
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  type Connection,
  type Edge,
  type Node,
  type NodeTypes,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import type { NodeDefinition, Wire, NodeType } from '@/api';
import { VibeflowNode, type VibeflowNodeData } from '@/components/vibeflow-node';

interface FlowCanvasProps {
  nodes: NodeDefinition[];
  wires: Wire[];
  nodeTypes?: NodeType[];
  selectedNodeId: string | null;
  expandedNodes: Record<string, boolean>;
  errorNodeIds?: Set<string>;
  onSelectNode: (id: string | null) => void;
  onUpdateNode: (id: string, updates: Partial<NodeDefinition>) => void;
  onDeleteNode: (id: string) => void;
  onAddWire: (from: string, to: string, output: string, input: string) => void;
  onDeleteWire: (from: string, to: string, output: string, input: string) => void;
  onToggleNodeExpanded: (nodeId: string, expanded: boolean) => void;
}

type VibeflowNodeType = Node<VibeflowNodeData, 'vibeflow'>;

const nodeTypes: NodeTypes = {
  vibeflow: VibeflowNode,
};

const GRID_SIZE = 20;
const WIRE_COLOR_DEFAULT = 'oklch(0.5 0.08 260)';
const WIRE_COLOR_SELECTED = 'oklch(0.78 0.18 75)';

// Stable reference for callback
function useStableCallback<T extends (...args: unknown[]) => unknown>(callback: T): T {
  const ref = useRef(callback);
  ref.current = callback;
  return useCallback((...args: unknown[]) => ref.current(...args), []) as T;
}

export function FlowCanvas({
  nodes: nodeDefinitions,
  wires,
  nodeTypes: nodeTypesList,
  expandedNodes,
  errorNodeIds,
  onSelectNode,
  onUpdateNode,
  onDeleteNode,
  onAddWire,
  onDeleteWire,
  onToggleNodeExpanded,
}: FlowCanvasProps) {
  // Stable callback ref to avoid re-renders
  const stableOnToggle = useStableCallback(onToggleNodeExpanded);

  // Build nodeTypes map
  const nodeTypesMap = useMemo(() => {
    const map = new Map<string, NodeType>();
    nodeTypesList?.forEach((nt) => map.set(nt.type, nt));
    return map;
  }, [nodeTypesList]);

  const nodeTypesLoaded = nodeTypesMap.size > 0;

  // Build nodes - only when nodeTypes are loaded
  const flowNodes: VibeflowNodeType[] = useMemo(() => {
    if (!nodeTypesLoaded) return [];

    return nodeDefinitions.map((node): VibeflowNodeType => {
      const typeInfo = nodeTypesMap.get(node.type);
      return {
        id: node.id,
        type: 'vibeflow',
        position: { x: node.x ?? 0, y: node.y ?? 0 },
        data: {
          name: node.name || node.id,
          type: node.type,
          config: node.config,
          inputs: typeInfo?.inputs ?? [],
          outputs: typeInfo?.outputs ?? [],
          expanded: expandedNodes[node.id] ?? false,
          disabled: node.enabled === false,
          hasError: errorNodeIds?.has(node.id) ?? false,
          onToggleExpanded: stableOnToggle,
        },
      };
    });
  }, [nodeDefinitions, nodeTypesMap, nodeTypesLoaded, expandedNodes, errorNodeIds, stableOnToggle]);

  // Helper to get all valid output port names for a node (static + dynamic from config)
  const getNodeOutputs = useCallback((nodeDef: NodeDefinition): Set<string> => {
    const outputs = new Set<string>();
    const typeInfo = nodeTypesMap.get(nodeDef.type);

    // Add static outputs from type definition
    typeInfo?.outputs?.forEach((o) => outputs.add(o.name));

    // Add dynamic outputs from config (e.g., switch rules with 'output' field)
    if (nodeDef.config) {
      for (const value of Object.values(nodeDef.config)) {
        if (Array.isArray(value)) {
          for (const item of value) {
            if (item && typeof item === 'object' && 'output' in item && typeof item.output === 'string') {
              outputs.add(item.output);
            }
          }
        }
      }
    }
    return outputs;
  }, [nodeTypesMap]);

  // Helper to get all valid input port names for a node
  const getNodeInputs = useCallback((nodeDef: NodeDefinition): Set<string> => {
    const inputs = new Set<string>();
    const typeInfo = nodeTypesMap.get(nodeDef.type);
    typeInfo?.inputs?.forEach((i) => inputs.add(i.name));
    return inputs;
  }, [nodeTypesMap]);

  // Build edges - only when nodeTypes are loaded, filter out invalid ones
  const flowEdges: Edge[] = useMemo(() => {
    if (!nodeTypesLoaded) return [];

    const nodeMap = new Map(nodeDefinitions.map((n) => [n.id, n]));

    return wires
      .filter((wire) => {
        const sourceNode = nodeMap.get(wire.from);
        const targetNode = nodeMap.get(wire.to);
        if (!sourceNode || !targetNode) return false;

        const sourceOutputs = getNodeOutputs(sourceNode);
        const targetInputs = getNodeInputs(targetNode);

        return sourceOutputs.has(wire.output) && targetInputs.has(wire.input);
      })
      .map((wire, i): Edge => ({
        id: `${wire.from}-${wire.to}-${wire.output}-${wire.input}-${i}`,
        source: wire.from,
        target: wire.to,
        sourceHandle: wire.output,
        targetHandle: wire.input,
        type: 'default',
        style: { stroke: WIRE_COLOR_DEFAULT, strokeWidth: 2 },
      }));
  }, [wires, nodeTypesLoaded, nodeDefinitions, getNodeOutputs, getNodeInputs]);

  // Use React Flow's internal state for smooth interactions
  const [nodes, setNodes, onNodesChange] = useNodesState(flowNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(flowEdges);

  // Track what we last synced to avoid unnecessary updates
  const lastSyncRef = useRef({ nodeKey: '', edgeKey: '' });

  // Sync from props when data actually changes (including config for dynamic ports)
  const nodeKey = useMemo(() =>
    nodeDefinitions.map(n => `${n.id}:${n.x}:${n.y}:${n.name}:${n.enabled}:${JSON.stringify(n.config)}`).join('|') +
    Object.entries(expandedNodes).map(([k, v]) => `${k}=${v}`).join(','),
    [nodeDefinitions, expandedNodes]
  );

  const edgeKey = useMemo(() =>
    wires.map(w => `${w.from}:${w.to}:${w.output}:${w.input}`).join('|'),
    [wires]
  );

  // Sync nodes if data changed - preserve selection state
  if (nodeTypesLoaded && nodeKey !== lastSyncRef.current.nodeKey) {
    lastSyncRef.current.nodeKey = nodeKey;
    setNodes((currentNodes) => {
      const selectedIds = new Set(currentNodes.filter((n) => n.selected).map((n) => n.id));
      return flowNodes.map((n) => ({ ...n, selected: selectedIds.has(n.id) }));
    });
  }

  // Sync edges if data changed - preserve selection state
  if (nodeTypesLoaded && edgeKey !== lastSyncRef.current.edgeKey) {
    lastSyncRef.current.edgeKey = edgeKey;
    setEdges((currentEdges) => {
      const selectedIds = new Set(currentEdges.filter((e) => e.selected).map((e) => e.id));
      return flowEdges.map((e) => ({
        ...e,
        selected: selectedIds.has(e.id),
        style: {
          stroke: selectedIds.has(e.id) ? WIRE_COLOR_SELECTED : WIRE_COLOR_DEFAULT,
          strokeWidth: 2,
        },
      }));
    });
  }

  // Handle node changes
  const handleNodesChange = useCallback(
    (changes: Parameters<typeof onNodesChange>[0]) => {
      onNodesChange(changes);

      for (const change of changes) {
        if (change.type === 'position' && change.position && !change.dragging) {
          const x = Math.round(change.position.x / GRID_SIZE) * GRID_SIZE;
          const y = Math.round(change.position.y / GRID_SIZE) * GRID_SIZE;
          onUpdateNode(change.id, { x, y });
        } else if (change.type === 'remove') {
          onDeleteNode(change.id);
        } else if (change.type === 'select' && change.selected) {
          onSelectNode(change.id);
        }
      }
    },
    [onNodesChange, onUpdateNode, onDeleteNode, onSelectNode]
  );

  // Handle edge changes
  const handleEdgesChange = useCallback(
    (changes: Parameters<typeof onEdgesChange>[0]) => {
      // Handle selection styling
      setEdges((eds) =>
        eds.map((e) => {
          const selectChange = changes.find((c) => c.type === 'select' && c.id === e.id);
          if (selectChange && selectChange.type === 'select') {
            return {
              ...e,
              selected: selectChange.selected,
              style: {
                stroke: selectChange.selected ? WIRE_COLOR_SELECTED : WIRE_COLOR_DEFAULT,
                strokeWidth: 2,
              },
            };
          }
          return e;
        })
      );

      for (const change of changes) {
        if (change.type === 'remove') {
          const edge = edges.find((e) => e.id === change.id);
          if (edge?.sourceHandle && edge?.targetHandle) {
            onDeleteWire(edge.source, edge.target, edge.sourceHandle, edge.targetHandle);
          }
        }
      }
    },
    [edges, onDeleteWire, setEdges]
  );

  // Handle new connections
  const handleConnect = useCallback(
    (connection: Connection) => {
      if (connection.source && connection.target && connection.sourceHandle && connection.targetHandle) {
        onAddWire(connection.source, connection.target, connection.sourceHandle, connection.targetHandle);
      }
    },
    [onAddWire]
  );

  // Handle pane click
  const handlePaneClick = useCallback(() => {
    onSelectNode(null);
  }, [onSelectNode]);

  // Handle selection
  const handleSelectionChange = useCallback(
    ({ nodes: selectedNodes }: { nodes: Node[] }) => {
      if (selectedNodes.length === 0) {
        onSelectNode(null);
      } else if (selectedNodes.length === 1) {
        onSelectNode(selectedNodes[0].id);
      }
    },
    [onSelectNode]
  );

  return (
    <div className="w-full h-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={handleNodesChange}
        onEdgesChange={handleEdgesChange}
        onConnect={handleConnect}
        onSelectionChange={handleSelectionChange}
        onPaneClick={handlePaneClick}
        nodeTypes={nodeTypes}
        snapToGrid
        snapGrid={[GRID_SIZE, GRID_SIZE]}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        defaultEdgeOptions={{
          type: 'default',
          style: { stroke: WIRE_COLOR_DEFAULT, strokeWidth: 2 },
        }}
        connectionLineStyle={{
          stroke: 'oklch(0.78 0.18 75)',
          strokeWidth: 2,
          strokeDasharray: '5,5',
        }}
        deleteKeyCode={['Backspace', 'Delete']}
        selectionKeyCode={null}
        multiSelectionKeyCode={['Shift', 'Meta']}
        proOptions={{ hideAttribution: true }}
        className="bg-background"
      >
        <Background
          variant={BackgroundVariant.Lines}
          gap={GRID_SIZE}
          size={1}
          color="oklch(0.25 0.015 260 / 0.5)"
        />
      </ReactFlow>
    </div>
  );
}
