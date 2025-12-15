import { useState, useCallback, useMemo, useEffect, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ReactFlowProvider } from '@xyflow/react';
import { getFlow, updateFlow, listNodeTypes, startFlow, stopFlow } from '@/api';
import type { NodeType, FlowDefinition, NodeDefinition, Wire } from '@/api';
import { useFlowHistory } from '@/hooks/use-flow-history';
import { useFlowEvents, type WSEvent } from '@/hooks/use-websocket';
import { useMinDuration } from '@/hooks/use-min-duration';
import { Spinner } from '@/components/ui/spinner';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from '@/components/ui/resizable';
import {
  AlertCircle,
  ArrowLeft,
  Play,
  Square,
  Save,
  Undo2,
  Redo2,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { FlowCanvas } from '@/components/flow-canvas';
import { NodePalette } from '@/components/node-palette';
import { EditorSidebar } from '@/components/editor-sidebar';
import yaml from 'js-yaml';

const DEFAULT_FLOW_CONTENT = `version: "1.0"
metadata:
  name: ""
  description: ""
nodes: []
wires: []
`;

const PANEL_STORAGE_KEY = 'vibeflow-editor-panels';
const NODE_UI_STATE_KEY = 'vibeflow-node-ui-state';

// Type for per-node UI state (can be extended later)
type NodeUIState = {
  expanded?: boolean;
};

// Get expanded states for all nodes in a flow
function getNodeUIState(flowId: string): Record<string, NodeUIState> {
  try {
    const stored = localStorage.getItem(NODE_UI_STATE_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      return parsed[flowId] ?? {};
    }
  } catch {
    // Ignore parse errors
  }
  return {};
}

// Save expanded state for a node
function saveNodeUIState(flowId: string, nodeId: string, state: NodeUIState) {
  try {
    const stored = localStorage.getItem(NODE_UI_STATE_KEY);
    const allState = stored ? JSON.parse(stored) : {};
    if (!allState[flowId]) {
      allState[flowId] = {};
    }
    allState[flowId][nodeId] = { ...allState[flowId][nodeId], ...state };
    localStorage.setItem(NODE_UI_STATE_KEY, JSON.stringify(allState));
  } catch {
    // Ignore storage errors
  }
}

function getPanelLayout(): number[] {
  try {
    const stored = localStorage.getItem(PANEL_STORAGE_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      if (Array.isArray(parsed) && parsed.length === 3) {
        return parsed;
      }
    }
  } catch {
    // Ignore parse errors
  }
  return [15, 60, 25]; // Default: 15% left, 60% center, 25% right
}

function savePanelLayout(sizes: number[]) {
  try {
    localStorage.setItem(PANEL_STORAGE_KEY, JSON.stringify(sizes));
  } catch {
    // Ignore storage errors
  }
}

export function FlowEditorPage() {
  const { id } = useParams<{ id: string }>();
  const queryClient = useQueryClient();

  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [initialFlowDef, setInitialFlowDef] = useState<FlowDefinition | null>(null);
  const {
    state: flowDefinition,
    set: setFlowDefinition,
    undo,
    redo,
    reset: _resetHistory,
    canUndo,
    canRedo,
  } = useFlowHistory(initialFlowDef);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [expandedNodes, setExpandedNodes] = useState<Record<string, boolean>>({});
  const [errorNodeIds, setErrorNodeIds] = useState<Set<string>>(new Set());
  const [activeTab, setActiveTab] = useState('config');

  // WebSocket for real-time events
  const handleFlowEvent = useCallback((event: WSEvent) => {
    if (event.type === 'node.error' && event.node_id) {
      setErrorNodeIds((prev) => new Set([...prev, event.node_id!]));
    } else if (event.type === 'flow.stop') {
      // Clear errors when flow stops (so next start has clean slate)
      setErrorNodeIds(new Set());
    }
  }, []);

  const { status: wsStatus, events: flowEvents, clearEvents } = useFlowEvents(id ?? null, handleFlowEvent);

  const defaultLayout = useMemo(() => getPanelLayout(), []);

  // Load node UI state (expanded) from localStorage when flow id changes
  useMemo(() => {
    if (id) {
      const nodeUIState = getNodeUIState(id);
      const expanded: Record<string, boolean> = {};
      for (const [nodeId, state] of Object.entries(nodeUIState)) {
        if (state.expanded !== undefined) {
          expanded[nodeId] = state.expanded;
        }
      }
      setExpandedNodes(expanded);
    }
  }, [id]);

  // Handle node expanded toggle - update state and persist to localStorage
  const handleToggleNodeExpanded = useCallback((nodeId: string, expanded: boolean) => {
    setExpandedNodes((prev) => ({ ...prev, [nodeId]: expanded }));
    if (id) {
      saveNodeUIState(id, nodeId, { expanded });
    }
  }, [id]);

  const { data: flow, isLoading: flowLoading } = useQuery({
    queryKey: ['flow', id],
    queryFn: () => getFlow(id!),
    enabled: !!id,
    refetchInterval: 5000,
  });

  const { data: nodeTypes } = useQuery({
    queryKey: ['nodeTypes'],
    queryFn: listNodeTypes,
  });

  // Parse flow content when flow loads
  useMemo(() => {
    if (flow && !initialFlowDef) {
      try {
        const content = flow.content || DEFAULT_FLOW_CONTENT;
        const parsed = yaml.load(content) as FlowDefinition;
        setInitialFlowDef(parsed);
      } catch (e) {
        console.error('Failed to parse flow content:', e);
        setInitialFlowDef({
          version: '1.0',
          nodes: [],
          wires: [],
        });
      }
    }
  }, [flow, initialFlowDef]);

  const updateMutation = useMutation({
    mutationFn: (data: { name?: string; content?: string }) => updateFlow(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flow', id] });
      setHasUnsavedChanges(false);
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to save'),
  });

  const startMutation = useMutation({
    mutationFn: () => startFlow(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flow', id] });
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to start'),
  });

  const stopMutation = useMutation({
    mutationFn: () => stopFlow(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flow', id] });
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to stop'),
  });

  // Extend loading states to prevent flash
  const isSaving = useMinDuration(updateMutation.isPending);
  const isStarting = useMinDuration(startMutation.isPending);
  const isStopping = useMinDuration(stopMutation.isPending);

  const handleSave = useCallback(() => {
    if (!flowDefinition) return;

    // Deduplicate wires before saving
    const seenWires = new Set<string>();
    const uniqueWires = flowDefinition.wires.filter((w) => {
      const key = `${w.from}:${w.output}->${w.to}:${w.input}`;
      if (seenWires.has(key)) return false;
      seenWires.add(key);
      return true;
    });

    const cleanedFlow = {
      ...flowDefinition,
      wires: uniqueWires,
    };

    const content = yaml.dump(cleanedFlow, { noRefs: true, lineWidth: -1 });
    updateMutation.mutate({ content });
  }, [flowDefinition, updateMutation]);

  // Global Ctrl+S handler - always saves regardless of focus (uses capture phase)
  useEffect(() => {
    const handleGlobalSave = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        if (hasUnsavedChanges && flowDefinition) {
          handleSave();
        }
      }
    };

    window.addEventListener('keydown', handleGlobalSave, { capture: true });
    return () => window.removeEventListener('keydown', handleGlobalSave, { capture: true });
  }, [hasUnsavedChanges, flowDefinition, handleSave]);

  // Keyboard shortcuts for undo/redo (only when not in input fields)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Check if user is typing in an input field
      const target = e.target as HTMLElement;
      if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
        return;
      }

      // Ctrl+Z or Cmd+Z for undo
      if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) {
        e.preventDefault();
        if (canUndo) {
          undo();
          setHasUnsavedChanges(true);
        }
      }

      // Ctrl+Y or Cmd+Y or Ctrl+Shift+Z for redo
      if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || (e.key === 'z' && e.shiftKey))) {
        e.preventDefault();
        if (canRedo) {
          redo();
          setHasUnsavedChanges(true);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [canUndo, canRedo, undo, redo]);

  // Track dragged node type for drag and drop
  const draggedNodeTypeRef = useRef<NodeType | null>(null);

  const handleAddNode = useCallback((nodeType: NodeType, position?: { x: number; y: number }) => {
    if (!flowDefinition) return;

    const newNode: NodeDefinition = {
      id: `${nodeType.type.replace('.', '_')}_${Date.now()}`,
      type: nodeType.type,
      name: nodeType.name,
      config: {},
      x: position?.x ?? 200 + Math.random() * 200,
      y: position?.y ?? 100 + Math.random() * 200,
    };

    setFlowDefinition({
      ...flowDefinition,
      nodes: [...flowDefinition.nodes, newNode],
    });
    setHasUnsavedChanges(true);
    setSelectedNodeId(newNode.id);
  }, [flowDefinition]);

  const handleDragStart = useCallback((_e: React.DragEvent, nodeType: NodeType) => {
    draggedNodeTypeRef.current = nodeType;
  }, []);

  const handleCanvasDrop = useCallback((x: number, y: number) => {
    if (draggedNodeTypeRef.current) {
      handleAddNode(draggedNodeTypeRef.current, { x, y });
      draggedNodeTypeRef.current = null;
    }
  }, [handleAddNode]);

  const handleNodeDoubleClick = useCallback((nodeId: string) => {
    setSelectedNodeId(nodeId);
    setActiveTab('config');
  }, []);

  const handleUpdateNode = useCallback((nodeId: string, updates: Partial<NodeDefinition>) => {
    if (!flowDefinition) return;

    setFlowDefinition({
      ...flowDefinition,
      nodes: flowDefinition.nodes.map((n) =>
        n.id === nodeId ? { ...n, ...updates } : n
      ),
    });
    setHasUnsavedChanges(true);
  }, [flowDefinition]);

  const handleDeleteNode = useCallback((nodeId: string) => {
    if (!flowDefinition) return;

    setFlowDefinition({
      ...flowDefinition,
      nodes: flowDefinition.nodes.filter((n) => n.id !== nodeId),
      wires: flowDefinition.wires.filter((w) => w.from !== nodeId && w.to !== nodeId),
    });
    setHasUnsavedChanges(true);
    if (selectedNodeId === nodeId) {
      setSelectedNodeId(null);
    }
  }, [flowDefinition, selectedNodeId]);

  const handleAddWire = useCallback((from: string, to: string, output?: string, input?: string) => {
    if (!flowDefinition) return;
    // Port names are required - if not provided, cannot create wire
    if (!output || !input) {
      console.error('Cannot create wire without explicit output and input port names');
      return;
    }

    // Check if wire already exists
    const exists = flowDefinition.wires.some(
      (w) => w.from === from && w.to === to && w.output === output && w.input === input
    );
    if (exists) return;

    const newWire: Wire = { from, to, output, input };
    setFlowDefinition({
      ...flowDefinition,
      wires: [...flowDefinition.wires, newWire],
    });
    setHasUnsavedChanges(true);
  }, [flowDefinition]);

  const handleDeleteWire = useCallback((from: string, to: string, output?: string, input?: string) => {
    if (!flowDefinition) return;
    // Port names are required - if not provided, cannot match wire
    if (!output || !input) {
      console.error('Cannot delete wire without explicit output and input port names');
      return;
    }

    setFlowDefinition({
      ...flowDefinition,
      wires: flowDefinition.wires.filter((w) => {
        // Match from/to and exact port names - no fallbacks
        if (w.from !== from || w.to !== to) return true;
        if (w.output !== output || w.input !== input) return true;
        // All match - delete this wire
        return false;
      }),
    });
    setHasUnsavedChanges(true);
  }, [flowDefinition]);

  const handleUpdateEnv = useCallback((key: string, value: string) => {
    if (!flowDefinition) return;

    setFlowDefinition({
      ...flowDefinition,
      environment: {
        ...flowDefinition.environment,
        [key]: value,
      },
    });
    setHasUnsavedChanges(true);
  }, [flowDefinition]);

  const handleAddEnv = useCallback((key: string) => {
    if (!flowDefinition || !key.trim()) return;

    setFlowDefinition({
      ...flowDefinition,
      environment: {
        ...flowDefinition.environment,
        [key.trim()]: '',
      },
    });
    setHasUnsavedChanges(true);
  }, [flowDefinition]);

  const handleDeleteEnv = useCallback((key: string) => {
    if (!flowDefinition?.environment) return;

    const { [key]: _, ...rest } = flowDefinition.environment;
    setFlowDefinition({
      ...flowDefinition,
      environment: Object.keys(rest).length > 0 ? rest : undefined,
    });
    setHasUnsavedChanges(true);
  }, [flowDefinition]);

  const selectedNode = useMemo(() => {
    if (!flowDefinition || !selectedNodeId) return null;
    return flowDefinition.nodes.find((n) => n.id === selectedNodeId) || null;
  }, [flowDefinition, selectedNodeId]);

  const selectedNodeType = useMemo(() => {
    if (!selectedNode || !nodeTypes) return undefined;
    return nodeTypes.find((nt) => nt.type === selectedNode.type);
  }, [selectedNode, nodeTypes]);

  const isRunning = flow?.runtime_status === 'running';

  if (flowLoading) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-muted-foreground animate-pulse">Loading flow...</div>
      </div>
    );
  }

  if (!flow) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-destructive flex items-center gap-2">
          <AlertCircle className="w-5 h-5" />
          Flow not found
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <header className="flex-shrink-0 h-14 border-b border-border flex items-center justify-between px-4">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" asChild>
            <Link to="/flows">
              <ArrowLeft className="w-4 h-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-base font-semibold flex items-center gap-2">
              {flow.name}
              {hasUnsavedChanges && (
                <span className="text-xs text-warning">*</span>
              )}
            </h1>
            <p className="text-xs text-muted-foreground">{flow.description || 'No description'}</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Badge
            variant={isRunning ? 'default' : flow.runtime_status === 'error' ? 'destructive' : 'secondary'}
            className={cn(
              'text-xs flex items-center gap-1.5',
              isRunning && 'bg-success/20 text-success border-success/30'
            )}
          >
            {isRunning && (
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-success opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-success" />
              </span>
            )}
            {flow.runtime_status === 'error' && (
              <AlertCircle className="w-3 h-3" />
            )}
            {flow.runtime_status}
          </Badge>

          <div className="flex items-center gap-1 border-r border-border pr-2 mr-1">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => { undo(); setHasUnsavedChanges(true); }}
              disabled={!canUndo}
              title="Undo (Ctrl+Z)"
              className="h-8 w-8"
            >
              <Undo2 className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => { redo(); setHasUnsavedChanges(true); }}
              disabled={!canRedo}
              title="Redo (Ctrl+Y)"
              className="h-8 w-8"
            >
              <Redo2 className="w-4 h-4" />
            </Button>
          </div>

          {isRunning ? (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => stopMutation.mutate()}
              disabled={isStopping}
            >
              {isStopping ? <Spinner /> : <Square className="w-4 h-4" />}
              Stop
            </Button>
          ) : (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => startMutation.mutate()}
              disabled={isStarting}
            >
              {isStarting ? <Spinner /> : <Play className="w-4 h-4" />}
              Start
            </Button>
          )}

          <Button
            size="sm"
            onClick={handleSave}
            disabled={!hasUnsavedChanges || isSaving}
          >
            {isSaving ? <Spinner /> : <Save className="w-4 h-4" />}
            Save
          </Button>
        </div>
      </header>

      {/* Main Editor Area with Resizable Panels */}
      <ResizablePanelGroup
        direction="horizontal"
        className="flex-1"
        onLayout={savePanelLayout}
      >
        {/* Left Panel - Node Palette */}
        <ResizablePanel
          defaultSize={defaultLayout[0]}
          minSize={10}
          maxSize={30}
          className="bg-card/50"
        >
          <NodePalette
            nodeTypes={nodeTypes || []}
            onAddNode={handleAddNode}
            onDragStart={handleDragStart}
          />
        </ResizablePanel>

        <ResizableHandle />

        {/* Center - Canvas */}
        <ResizablePanel defaultSize={defaultLayout[1]} minSize={30}>
          <div className="h-full bg-background">
            {flowDefinition && (
              <ReactFlowProvider>
                <FlowCanvas
                  nodes={flowDefinition.nodes}
                  wires={flowDefinition.wires}
                  nodeTypes={nodeTypes}
                  selectedNodeId={selectedNodeId}
                  expandedNodes={expandedNodes}
                  errorNodeIds={errorNodeIds}
                  onSelectNode={setSelectedNodeId}
                  onUpdateNode={handleUpdateNode}
                  onDeleteNode={handleDeleteNode}
                  onAddWire={handleAddWire}
                  onDeleteWire={handleDeleteWire}
                  onToggleNodeExpanded={handleToggleNodeExpanded}
                  onNodeDoubleClick={handleNodeDoubleClick}
                  onDrop={handleCanvasDrop}
                />
              </ReactFlowProvider>
            )}
          </div>
        </ResizablePanel>

        <ResizableHandle />

        {/* Right Panel - Editor Sidebar */}
        <ResizablePanel
          defaultSize={defaultLayout[2]}
          minSize={15}
          maxSize={40}
          className="bg-card/50"
        >
          <EditorSidebar
            selectedNode={selectedNode ?? undefined}
            selectedNodeType={selectedNodeType}
            onUpdateNode={(updates) => selectedNode && handleUpdateNode(selectedNode.id, updates)}
            environment={flowDefinition?.environment || {}}
            onUpdateEnv={handleUpdateEnv}
            onAddEnv={handleAddEnv}
            onDeleteEnv={handleDeleteEnv}
            flowEvents={flowEvents}
            wsStatus={wsStatus}
            onClearEvents={clearEvents}
            nodes={flowDefinition?.nodes}
            activeTab={activeTab}
            onTabChange={setActiveTab}
          />
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>
  );
}
