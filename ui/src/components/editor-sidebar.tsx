import { useState } from 'react';
import type { NodeDefinition, NodeType } from '@/api';
import type { WSEvent, ConnectionStatus } from '@/hooks/use-websocket';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Settings, Variable, Bug, Braces } from 'lucide-react';
import { NodeConfigPanel } from '@/components/node-config-panel';
import { EnvPanel } from '@/components/env-panel';
import { DebugPanel } from '@/components/debug-panel';
import { VariablesPanel } from '@/components/variables-panel';

interface EditorSidebarProps {
  // Config tab
  selectedNode?: NodeDefinition;
  selectedNodeType?: NodeType;
  onUpdateNode: (updates: Partial<NodeDefinition>) => void;
  // Env tab
  environment: Record<string, string>;
  onUpdateEnv: (key: string, value: string) => void;
  onAddEnv: (key: string) => void;
  onDeleteEnv: (key: string) => void;
  // Debug tab
  flowEvents: WSEvent[];
  wsStatus: ConnectionStatus;
  onClearEvents: () => void;
  isPaused: boolean;
  onPause: () => void;
  onResume: () => void;
  nodes?: NodeDefinition[];
  // Tab control (optional external control)
  activeTab?: string;
  onTabChange?: (tab: string) => void;
}

export function EditorSidebar({
  selectedNode,
  selectedNodeType,
  onUpdateNode,
  environment,
  onUpdateEnv,
  onAddEnv,
  onDeleteEnv,
  flowEvents,
  wsStatus,
  onClearEvents,
  isPaused,
  onPause,
  onResume,
  nodes,
  activeTab: externalActiveTab,
  onTabChange,
}: EditorSidebarProps) {
  const [internalActiveTab, setInternalActiveTab] = useState('config');

  // Use external tab control if provided, otherwise internal
  const activeTab = externalActiveTab ?? internalActiveTab;
  const setActiveTab = onTabChange ?? setInternalActiveTab;

  return (
    <Tabs value={activeTab} onValueChange={setActiveTab} className="h-full flex flex-col">
      <TabsList className="w-full justify-start rounded-none border-b border-border h-10 px-2 bg-transparent flex-shrink-0">
        <TabsTrigger value="config" className="text-xs gap-1.5">
          <Settings className="w-3.5 h-3.5" />
          Config
        </TabsTrigger>
        <TabsTrigger value="env" className="text-xs gap-1.5">
          <Variable className="w-3.5 h-3.5" />
          Env
        </TabsTrigger>
        <TabsTrigger value="vars" className="text-xs gap-1.5">
          <Braces className="w-3.5 h-3.5" />
          Vars
        </TabsTrigger>
        <TabsTrigger value="debug" className="text-xs gap-1.5">
          <Bug className="w-3.5 h-3.5" />
          Debug
          {flowEvents.length > 0 && (
            <span className="ml-1 px-1.5 py-0.5 text-[10px] rounded-full bg-primary/20 text-primary">
              {flowEvents.length}
            </span>
          )}
        </TabsTrigger>
      </TabsList>

      <TabsContent value="config" className="flex-1 m-0 overflow-hidden">
        {selectedNode ? (
          <NodeConfigPanel
            node={selectedNode}
            nodeType={selectedNodeType}
            onUpdate={onUpdateNode}
          />
        ) : (
          <div className="h-full flex items-center justify-center text-muted-foreground text-sm">
            Select a node to configure
          </div>
        )}
      </TabsContent>

      <TabsContent value="env" className="flex-1 m-0 overflow-hidden">
        <EnvPanel
          environment={environment}
          onUpdate={onUpdateEnv}
          onAdd={onAddEnv}
          onDelete={onDeleteEnv}
        />
      </TabsContent>

      <TabsContent value="vars" className="flex-1 m-0 overflow-hidden">
        <VariablesPanel events={flowEvents} />
      </TabsContent>

      <TabsContent value="debug" className="flex-1 m-0 overflow-hidden">
        <DebugPanel
          events={flowEvents}
          wsStatus={wsStatus}
          onClear={onClearEvents}
          isPaused={isPaused}
          onPause={onPause}
          onResume={onResume}
          nodes={nodes}
        />
      </TabsContent>
    </Tabs>
  );
}
