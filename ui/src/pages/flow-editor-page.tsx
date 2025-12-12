import { useState, useCallback, useMemo, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ReactFlowProvider } from '@xyflow/react';
import { getFlow, updateFlow, listNodeTypes, startFlow, stopFlow } from '@/api';
import type { NodeType, FlowDefinition, NodeDefinition, Wire } from '@/api';
import { useFlowHistory } from '@/hooks/use-flow-history';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from '@/components/ui/resizable';
import {
  ArrowLeft,
  Play,
  Square,
  Save,
  Plus,
  Trash2,
  Settings,
  Boxes,
  Cable,
  AlertCircle,
  Search,
  Variable,
  X,
  Undo2,
  Redo2,
} from 'lucide-react';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { FlowCanvas } from '@/components/flow-canvas';
import { NodeConfigPanel } from '@/components/node-config-panel';
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
  const [searchTerm, setSearchTerm] = useState('');
  const [expandedNodes, setExpandedNodes] = useState<Record<string, boolean>>({});

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
      toast.success('Flow saved');
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to save flow'),
  });

  const startMutation = useMutation({
    mutationFn: () => startFlow(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flow', id] });
      toast.success('Flow started');
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to start flow'),
  });

  const stopMutation = useMutation({
    mutationFn: () => stopFlow(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flow', id] });
      toast.success('Flow stopped');
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to stop flow'),
  });

  const handleSave = useCallback(() => {
    if (!flowDefinition) return;
    const content = yaml.dump(flowDefinition, { noRefs: true, lineWidth: -1 });
    updateMutation.mutate({ content });
  }, [flowDefinition, updateMutation]);

  // Keyboard shortcuts for undo/redo
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

      // Ctrl+S or Cmd+S for save
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        if (hasUnsavedChanges && flowDefinition) {
          handleSave();
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [canUndo, canRedo, undo, redo, hasUnsavedChanges, flowDefinition, handleSave]);

  const handleAddNode = useCallback((nodeType: NodeType) => {
    if (!flowDefinition) return;

    const newNode: NodeDefinition = {
      id: `${nodeType.type.replace('.', '_')}_${Date.now()}`,
      type: nodeType.type,
      name: nodeType.name,
      config: {},
      x: 200 + Math.random() * 200,
      y: 100 + Math.random() * 200,
    };

    setFlowDefinition({
      ...flowDefinition,
      nodes: [...flowDefinition.nodes, newNode],
    });
    setHasUnsavedChanges(true);
    setSelectedNodeId(newNode.id);
  }, [flowDefinition]);

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

    setFlowDefinition({
      ...flowDefinition,
      wires: flowDefinition.wires.filter((w) => {
        // Match from/to
        if (w.from !== from || w.to !== to) return true;
        // Match output handle (default to 'output' if not specified)
        const wireOutput = w.output || 'output';
        const deleteOutput = output || 'output';
        if (wireOutput !== deleteOutput) return true;
        // Match input handle (default to 'input' if not specified)
        const wireInput = w.input || 'input';
        const deleteInput = input || 'input';
        if (wireInput !== deleteInput) return true;
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

  const groupedNodeTypes = useMemo(() => {
    if (!nodeTypes) return {};
    const filtered = searchTerm
      ? nodeTypes.filter(
          (nt) =>
            nt.type.toLowerCase().includes(searchTerm.toLowerCase()) ||
            nt.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
            nt.category.toLowerCase().includes(searchTerm.toLowerCase())
        )
      : nodeTypes;
    return filtered.reduce((acc, nt) => {
      const cat = nt.category || 'other';
      if (!acc[cat]) acc[cat] = [];
      acc[cat].push(nt);
      return acc;
    }, {} as Record<string, NodeType[]>);
  }, [nodeTypes, searchTerm]);

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
              disabled={stopMutation.isPending}
            >
              <Square className="w-4 h-4 mr-2" />
              Stop
            </Button>
          ) : (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => startMutation.mutate()}
              disabled={startMutation.isPending}
            >
              <Play className="w-4 h-4 mr-2" />
              Start
            </Button>
          )}

          <Button
            size="sm"
            onClick={handleSave}
            disabled={!hasUnsavedChanges || updateMutation.isPending}
          >
            <Save className="w-4 h-4 mr-2" />
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
          <div className="h-full flex flex-col">
            <div className="p-3 border-b border-border">
              <div className="relative">
                <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  placeholder="Search nodes..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="pl-8 h-8 text-sm"
                />
              </div>
            </div>
            <ScrollArea className="flex-1">
              <div className="p-2">
                {Object.entries(groupedNodeTypes).map(([category, types]) => (
                  <div key={category} className="mb-4">
                    <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                      {category}
                    </h3>
                    <div className="space-y-1">
                      {types.map((nodeType) => (
                        <button
                          key={nodeType.type}
                          onClick={() => handleAddNode(nodeType)}
                          className="w-full text-left px-3 py-2 text-sm rounded hover:bg-secondary transition-colors group"
                        >
                          <div className="flex items-center justify-between">
                            <span>{nodeType.name}</span>
                            <Plus className="w-3 h-3 opacity-0 group-hover:opacity-100 transition-opacity" />
                          </div>
                          <div className="text-xs text-muted-foreground">{nodeType.type}</div>
                        </button>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </ScrollArea>
          </div>
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
                  onSelectNode={setSelectedNodeId}
                  onUpdateNode={handleUpdateNode}
                  onDeleteNode={handleDeleteNode}
                  onAddWire={handleAddWire}
                  onDeleteWire={handleDeleteWire}
                  onToggleNodeExpanded={handleToggleNodeExpanded}
                />
              </ReactFlowProvider>
            )}
          </div>
        </ResizablePanel>

        <ResizableHandle />

        {/* Right Panel - Node Configuration */}
        <ResizablePanel
          defaultSize={defaultLayout[2]}
          minSize={15}
          maxSize={40}
          className="bg-card/50"
        >
          <Tabs defaultValue="config" className="h-full flex flex-col">
            <TabsList className="w-full justify-start rounded-none border-b border-border h-10 px-2 bg-transparent flex-shrink-0">
              <TabsTrigger value="config" className="text-xs gap-1.5">
                <Settings className="w-3.5 h-3.5" />
                Config
              </TabsTrigger>
              <TabsTrigger value="env" className="text-xs gap-1.5">
                <Variable className="w-3.5 h-3.5" />
                Env
              </TabsTrigger>
              <TabsTrigger value="nodes" className="text-xs gap-1.5">
                <Boxes className="w-3.5 h-3.5" />
                Nodes
              </TabsTrigger>
              <TabsTrigger value="wires" className="text-xs gap-1.5">
                <Cable className="w-3.5 h-3.5" />
                Wires
              </TabsTrigger>
            </TabsList>

            <TabsContent value="config" className="flex-1 m-0 overflow-hidden">
              {selectedNode ? (
                <NodeConfigPanel
                  node={selectedNode}
                  nodeType={selectedNodeType}
                  onUpdate={(updates) => handleUpdateNode(selectedNode.id, updates)}
                />
              ) : (
                <div className="h-full flex items-center justify-center text-muted-foreground text-sm">
                  Select a node to configure
                </div>
              )}
            </TabsContent>

            <TabsContent value="env" className="flex-1 m-0 overflow-hidden">
              <EnvironmentPanel
                environment={flowDefinition?.environment || {}}
                onUpdate={handleUpdateEnv}
                onAdd={handleAddEnv}
                onDelete={handleDeleteEnv}
              />
            </TabsContent>

            <TabsContent value="nodes" className="flex-1 m-0 overflow-hidden">
              <ScrollArea className="h-full">
                <div className="p-3 space-y-2">
                  {flowDefinition?.nodes.map((node) => (
                    <button
                      key={node.id}
                      onClick={() => setSelectedNodeId(node.id)}
                      className={cn(
                        'w-full text-left px-3 py-2 rounded text-sm transition-colors',
                        selectedNodeId === node.id
                          ? 'bg-primary/20 border border-primary/30'
                          : 'hover:bg-secondary'
                      )}
                    >
                      <div className="font-medium">{node.name || node.id}</div>
                      <div className="text-xs text-muted-foreground">{node.type}</div>
                    </button>
                  ))}
                  {(!flowDefinition?.nodes || flowDefinition.nodes.length === 0) && (
                    <div className="text-center text-muted-foreground text-sm py-8">
                      No nodes yet
                    </div>
                  )}
                </div>
              </ScrollArea>
            </TabsContent>

            <TabsContent value="wires" className="flex-1 m-0 overflow-hidden">
              <ScrollArea className="h-full">
                <div className="p-3 space-y-2">
                  {flowDefinition?.wires.map((wire, i) => (
                    <div
                      key={`${wire.from}-${wire.to}-${i}`}
                      className="px-3 py-2 rounded bg-secondary/50 text-sm flex items-center justify-between"
                    >
                      <div>
                        <span className="text-cyan">{wire.from}</span>
                        <span className="text-muted-foreground mx-2">→</span>
                        <span className="text-amber">{wire.to}</span>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => handleDeleteWire(wire.from, wire.to)}
                      >
                        <Trash2 className="w-3 h-3" />
                      </Button>
                    </div>
                  ))}
                  {(!flowDefinition?.wires || flowDefinition.wires.length === 0) && (
                    <div className="text-center text-muted-foreground text-sm py-8">
                      No connections yet
                    </div>
                  )}
                </div>
              </ScrollArea>
            </TabsContent>
          </Tabs>
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>
  );
}

function EnvironmentPanel({
  environment,
  onUpdate,
  onAdd,
  onDelete,
}: {
  environment: Record<string, string>;
  onUpdate: (key: string, value: string) => void;
  onAdd: (key: string) => void;
  onDelete: (key: string) => void;
}) {
  const [newKey, setNewKey] = useState('');

  const handleAdd = () => {
    if (newKey.trim() && !(newKey.trim() in environment)) {
      onAdd(newKey.trim());
      setNewKey('');
    }
  };

  return (
    <ScrollArea className="h-full">
      <div className="p-4 space-y-4">
        <div className="space-y-1">
          <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
            Environment Variables
          </h3>
          <p className="text-xs text-muted-foreground">
            Use <code className="bg-muted px-1 rounded">{'${VAR_NAME}'}</code> in node config
          </p>
        </div>

        <div className="space-y-3">
          {Object.entries(environment).map(([key, value]) => (
            <div key={key} className="space-y-1">
              <div className="flex items-center justify-between">
                <label className="text-xs font-mono text-primary">${key}</label>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-5 w-5"
                  onClick={() => onDelete(key)}
                >
                  <X className="w-3 h-3" />
                </Button>
              </div>
              <Input
                value={value}
                onChange={(e) => onUpdate(key, e.target.value)}
                placeholder={`Enter ${key} value...`}
                className="h-8 text-sm font-mono"
              />
            </div>
          ))}

          {Object.keys(environment).length === 0 && (
            <p className="text-sm text-muted-foreground text-center py-4">
              No environment variables defined
            </p>
          )}
        </div>

        <Separator />

        <div className="space-y-2">
          <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
            Add Variable
          </label>
          <div className="flex gap-2">
            <Input
              value={newKey}
              onChange={(e) => setNewKey(e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, ''))}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
              placeholder="VAR_NAME"
              className="h-8 text-sm font-mono flex-1"
            />
            <Button
              variant="secondary"
              size="icon"
              className="h-8 w-8"
              onClick={handleAdd}
              disabled={!newKey.trim() || newKey.trim() in environment}
            >
              <Plus className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </div>
    </ScrollArea>
  );
}
