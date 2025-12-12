import { useCallback, useMemo } from 'react';
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  addEdge,
  type Connection,
  type Edge,
  type Node,
  type NodeTypes,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import type { NodeDefinition, Wire, NodeType, PortInfo } from '@/api';
import { VibeflowNode, type VibeflowNodeData } from '@/components/vibeflow-node';

interface FlowCanvasProps {
  nodes: NodeDefinition[];
  wires: Wire[];
  nodeTypes?: NodeType[];
  selectedNodeId: string | null;
  expandedNodes: Record<string, boolean>;
  onSelectNode: (id: string | null) => void;
  onUpdateNode: (id: string, updates: Partial<NodeDefinition>) => void;
  onDeleteNode: (id: string) => void;
  onAddWire: (from: string, to: string, output?: string, input?: string) => void;
  onDeleteWire: (from: string, to: string, output?: string, input?: string) => void;
  onToggleNodeExpanded: (nodeId: string, expanded: boolean) => void;
}

type VibeflowNode = Node<VibeflowNodeData, 'vibeflow'>;

// Node types registry
const nodeTypes: NodeTypes = {
  vibeflow: VibeflowNode,
};

// Grid size for snapping
const GRID_SIZE = 20;

// Default ports when type info is not available
const DEFAULT_INPUTS: PortInfo[] = [{ name: 'input', description: 'Default input' }];
const DEFAULT_OUTPUTS: PortInfo[] = [{ name: 'output', description: 'Default output' }];

// Convert our NodeDefinition to React Flow Node
function toFlowNode(
  node: NodeDefinition,
  nodeTypesMap: Map<string, NodeType>,
  expanded: boolean,
  onToggleExpanded: (nodeId: string, expanded: boolean) => void
): VibeflowNode {
  const nodeTypeInfo = nodeTypesMap.get(node.type);
  const inputs = nodeTypeInfo?.inputs ?? DEFAULT_INPUTS;
  const outputs = nodeTypeInfo?.outputs ?? DEFAULT_OUTPUTS;

  return {
    id: node.id,
    type: 'vibeflow',
    position: { x: node.x ?? 0, y: node.y ?? 0 },
    data: {
      name: node.name || node.id,
      type: node.type,
      config: node.config,
      inputs,
      outputs,
      expanded,
      onToggleExpanded,
    },
  };
}

// Wire colors
const WIRE_COLOR_DEFAULT = 'oklch(0.5 0.08 260)';
const WIRE_COLOR_SELECTED = 'oklch(0.78 0.18 75)';

// Convert our Wire to React Flow Edge
function toFlowEdge(wire: Wire, index: number, selected: boolean = false): Edge {
  return {
    id: `${wire.from}-${wire.to}-${index}`,
    source: wire.from,
    target: wire.to,
    sourceHandle: wire.output || 'output',
    targetHandle: wire.input || 'input',
    type: 'default',
    animated: false,
    zIndex: selected ? 1000 : 0,
    style: {
      stroke: selected ? WIRE_COLOR_SELECTED : WIRE_COLOR_DEFAULT,
      strokeWidth: 2,
    },
  };
}

export function FlowCanvas({
  nodes: nodeDefinitions,
  wires,
  nodeTypes: nodeTypesList,
  selectedNodeId: _selectedNodeId,
  expandedNodes,
  onSelectNode,
  onUpdateNode,
  onDeleteNode,
  onAddWire,
  onDeleteWire,
  onToggleNodeExpanded,
}: FlowCanvasProps) {
  // Build a map of node type -> NodeType info for fast lookup
  const nodeTypesMap = useMemo(() => {
    const map = new Map<string, NodeType>();
    if (nodeTypesList) {
      for (const nt of nodeTypesList) {
        map.set(nt.type, nt);
      }
    }
    return map;
  }, [nodeTypesList]);

  // Convert props to React Flow format
  const initialNodes = useMemo(
    () => nodeDefinitions.map((n) => toFlowNode(n, nodeTypesMap, expandedNodes[n.id] ?? false, onToggleNodeExpanded)),
    [nodeDefinitions, nodeTypesMap, expandedNodes, onToggleNodeExpanded]
  );

  const initialEdges = useMemo(
    () => wires.map((w, i) => toFlowEdge(w, i)),
    [wires]
  );

  const [nodes, setNodes, onNodesChange] = useNodesState<VibeflowNode>(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  // Sync nodes from props when they change externally
  // Preserve the selected state when updating nodes
  useMemo(() => {
    setNodes((currentNodes) => {
      const selectedIds = new Set(currentNodes.filter(n => n.selected).map(n => n.id));
      return nodeDefinitions.map((nodeDef) => {
        const flowNode = toFlowNode(nodeDef, nodeTypesMap, expandedNodes[nodeDef.id] ?? false, onToggleNodeExpanded);
        flowNode.selected = selectedIds.has(flowNode.id);
        return flowNode;
      });
    });
  }, [nodeDefinitions, setNodes, nodeTypesMap, expandedNodes, onToggleNodeExpanded]);

  // Sync edges from props when they change externally
  // Preserve the selected state when updating edges
  useMemo(() => {
    setEdges((currentEdges) => {
      const selectedIds = new Set(currentEdges.filter(e => e.selected).map(e => e.id));
      return wires.map((w, i) => {
        const edge = toFlowEdge(w, i);
        const isSelected = selectedIds.has(edge.id);
        edge.selected = isSelected;
        edge.zIndex = isSelected ? 1000 : 0;
        // Update style based on selection
        edge.style = {
          stroke: isSelected ? WIRE_COLOR_SELECTED : WIRE_COLOR_DEFAULT,
          strokeWidth: 2,
        };
        return edge;
      });
    });
  }, [wires, setEdges]);

  // Handle node changes (position, selection, removal)
  const handleNodesChange = useCallback(
    (changes: Parameters<typeof onNodesChange>[0]) => {
      onNodesChange(changes);

      for (const change of changes) {
        if (change.type === 'position' && change.position && change.dragging === false) {
          // Snap to grid on drag end
          const snappedX = Math.round(change.position.x / GRID_SIZE) * GRID_SIZE;
          const snappedY = Math.round(change.position.y / GRID_SIZE) * GRID_SIZE;
          onUpdateNode(change.id, { x: snappedX, y: snappedY });
        } else if (change.type === 'remove') {
          onDeleteNode(change.id);
        } else if (change.type === 'select') {
          if (change.selected) {
            onSelectNode(change.id);
          }
        }
      }
    },
    [onNodesChange, onUpdateNode, onDeleteNode, onSelectNode]
  );

  // Handle edge changes (removal, selection)
  const handleEdgesChange = useCallback(
    (changes: Parameters<typeof onEdgesChange>[0]) => {
      // Check for selection changes and update styles
      const hasSelectionChange = changes.some(c => c.type === 'select');

      if (hasSelectionChange) {
        // Apply changes first, then update styles based on new selection state
        setEdges((eds) => {
          // First apply the changes
          let updatedEdges = [...eds];
          for (const change of changes) {
            if (change.type === 'select') {
              updatedEdges = updatedEdges.map((e) =>
                e.id === change.id
                  ? {
                      ...e,
                      selected: change.selected,
                      zIndex: change.selected ? 1000 : 0,
                      style: {
                        stroke: change.selected ? WIRE_COLOR_SELECTED : WIRE_COLOR_DEFAULT,
                        strokeWidth: 2,
                      },
                    }
                  : e
              );
            }
          }
          return updatedEdges;
        });
      } else {
        onEdgesChange(changes);
      }

      for (const change of changes) {
        if (change.type === 'remove') {
          // Find the edge to get source/target and handles
          const edge = edges.find((e) => e.id === change.id);
          if (edge) {
            onDeleteWire(
              edge.source,
              edge.target,
              edge.sourceHandle || undefined,
              edge.targetHandle || undefined
            );
          }
        }
      }
    },
    [onEdgesChange, edges, onDeleteWire, setEdges]
  );

  // Handle new connections
  const handleConnect = useCallback(
    (connection: Connection) => {
      if (connection.source && connection.target) {
        onAddWire(
          connection.source,
          connection.target,
          connection.sourceHandle || undefined,
          connection.targetHandle || undefined
        );
        setEdges((eds) => addEdge(connection, eds));
      }
    },
    [onAddWire, setEdges]
  );

  // Handle selection changes
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

  // Handle pane click (deselect)
  const handlePaneClick = useCallback(() => {
    onSelectNode(null);
  }, [onSelectNode]);

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
        snapToGrid={true}
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
