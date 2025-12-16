import { useState, useMemo } from 'react';
import type { WSEvent } from '@/hooks/use-websocket';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import {
  ChevronRight,
  ChevronDown,
  Search,
  Globe,
  GitBranch,
  Box,
} from 'lucide-react';

// Simple JSON syntax highlighter (reused from debug-panel)
function JsonHighlight({ json }: { json: string }) {
  const highlighted = useMemo(() => {
    let html = json
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');

    html = html
      .replace(/: ("(?:[^"\\]|\\.)*")/g, ': <span class="text-green-400">$1</span>')
      .replace(/"([^"]+)":/g, '<span class="text-blue-400">"$1"</span>:')
      .replace(/: (-?\d+\.?\d*)/g, ': <span class="text-amber-400">$1</span>')
      .replace(/: (true|false)/g, ': <span class="text-purple-400">$1</span>')
      .replace(/: (null)/g, ': <span class="text-red-400">$1</span>');

    return html;
  }, [json]);

  return (
    <code
      className="text-foreground/90"
      dangerouslySetInnerHTML={{ __html: highlighted }}
    />
  );
}

// Variable entry with expand/collapse
function VariableEntry({
  varKey,
  value,
  scope,
  nodeId,
}: {
  varKey: string;
  value: unknown;
  scope: string;
  nodeId?: string;
}) {
  const [isExpanded, setIsExpanded] = useState(false);

  const valuePreview = useMemo(() => {
    if (value === null) return 'null';
    if (value === undefined) return 'undefined';
    if (typeof value === 'string') {
      return value.length > 50 ? `"${value.slice(0, 50)}..."` : `"${value}"`;
    }
    if (typeof value === 'number' || typeof value === 'boolean') {
      return String(value);
    }
    const str = JSON.stringify(value);
    return str.length > 50 ? str.slice(0, 50) + '...' : str;
  }, [value]);

  const valueFull = useMemo(() => {
    if (typeof value === 'string') return `"${value}"`;
    return JSON.stringify(value, null, 2);
  }, [value]);

  const isExpandable =
    (typeof value === 'object' && value !== null) ||
    (typeof value === 'string' && value.length > 50);

  const ScopeIcon = scope === 'global' ? Globe : scope === 'flow' ? GitBranch : Box;
  const scopeColor =
    scope === 'global'
      ? 'text-amber-400'
      : scope === 'flow'
        ? 'text-blue-400'
        : 'text-purple-400';

  return (
    <div className="border-b border-border/30 hover:bg-muted/30">
      <button
        onClick={() => isExpandable && setIsExpanded(!isExpanded)}
        className={cn(
          'w-full text-left py-1.5 px-2 flex items-start gap-1.5 text-xs',
          isExpandable && 'cursor-pointer'
        )}
      >
        {isExpandable ? (
          isExpanded ? (
            <ChevronDown className="w-3 h-3 mt-0.5 text-muted-foreground shrink-0" />
          ) : (
            <ChevronRight className="w-3 h-3 mt-0.5 text-muted-foreground shrink-0" />
          )
        ) : (
          <span className="w-3 shrink-0" />
        )}
        <ScopeIcon className={cn('w-3 h-3 mt-0.5 shrink-0', scopeColor)} />
        <span className="text-cyan-400 font-medium shrink-0">{varKey}</span>
        {nodeId && (
          <span className="text-muted-foreground shrink-0 text-[10px]">
            ({nodeId.slice(0, 12)}...)
          </span>
        )}
        {!isExpanded && (
          <span className="text-foreground/70 truncate font-mono">{valuePreview}</span>
        )}
      </button>
      {isExpanded && (
        <pre className="px-2 pb-2 pl-7 text-xs whitespace-pre-wrap break-all font-mono bg-muted/20">
          <JsonHighlight json={valueFull} />
        </pre>
      )}
    </div>
  );
}

// Scope filter types
type ScopeFilter = 'all' | 'global' | 'flow' | 'node';

interface VariablesPanelProps {
  events: WSEvent[];
}

// Variable data structure from WebSocket events
interface VariableData {
  scope: string;
  key: string;
  value: unknown;
}

export function VariablesPanel({ events }: VariablesPanelProps) {
  const [scopeFilter, setScopeFilter] = useState<ScopeFilter>('all');
  const [searchQuery, setSearchQuery] = useState('');

  // Build variables map from events (latest value wins)
  const variables = useMemo(() => {
    const vars = new Map<string, { scope: string; key: string; value: unknown; nodeId?: string }>();

    events
      .filter((e) => e.type === 'variable')
      .forEach((e) => {
        const data = e.data as VariableData | undefined;
        if (!data) return;

        // Create unique key for deduplication
        const uniqueKey =
          data.scope === 'node'
            ? `node:${e.node_id}:${data.key}`
            : data.scope === 'flow'
              ? `flow:${e.flow_id}:${data.key}`
              : `global:${data.key}`;

        vars.set(uniqueKey, {
          scope: data.scope,
          key: data.key,
          value: data.value,
          nodeId: data.scope === 'node' ? e.node_id : undefined,
        });
      });

    return Array.from(vars.values());
  }, [events]);

  // Filter variables by scope and search query
  const filteredVariables = useMemo(() => {
    return variables.filter((v) => {
      if (scopeFilter !== 'all' && v.scope !== scopeFilter) return false;
      if (searchQuery) {
        const query = searchQuery.toLowerCase();
        if (!v.key.toLowerCase().includes(query)) return false;
      }
      return true;
    });
  }, [variables, scopeFilter, searchQuery]);

  // Group by scope for display
  const groupedVariables = useMemo(() => {
    const groups: Record<string, typeof filteredVariables> = {
      global: [],
      flow: [],
      node: [],
    };
    filteredVariables.forEach((v) => {
      groups[v.scope]?.push(v);
    });
    return groups;
  }, [filteredVariables]);

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="p-2 border-b border-border space-y-2">
        {/* Search */}
        <div className="relative">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-muted-foreground" />
          <Input
            placeholder="Search variables..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="h-7 pl-7 text-xs"
          />
        </div>

        {/* Scope filters */}
        <div className="flex gap-1">
          {(['all', 'global', 'flow', 'node'] as const).map((scope) => (
            <button
              key={scope}
              onClick={() => setScopeFilter(scope)}
              className={cn(
                'px-2 py-0.5 rounded text-[10px] capitalize',
                scopeFilter === scope
                  ? 'bg-primary/20 text-primary'
                  : 'bg-muted text-muted-foreground hover:bg-muted/80'
              )}
            >
              {scope}
            </button>
          ))}
        </div>
      </div>

      {/* Variables list */}
      <div className="flex-1 overflow-auto font-mono">
        {filteredVariables.length === 0 ? (
          <div className="h-full flex items-center justify-center text-muted-foreground text-xs">
            {variables.length === 0
              ? 'No variables set yet'
              : 'No matching variables'}
          </div>
        ) : scopeFilter === 'all' ? (
          // Show grouped when "all" is selected
          <div>
            {groupedVariables.global.length > 0 && (
              <div>
                <div className="px-2 py-1 text-[10px] text-muted-foreground bg-muted/50 font-medium flex items-center gap-1">
                  <Globe className="w-3 h-3 text-amber-400" />
                  Global ({groupedVariables.global.length})
                </div>
                {groupedVariables.global.map((v) => (
                  <VariableEntry
                    key={`global:${v.key}`}
                    varKey={v.key}
                    value={v.value}
                    scope={v.scope}
                  />
                ))}
              </div>
            )}
            {groupedVariables.flow.length > 0 && (
              <div>
                <div className="px-2 py-1 text-[10px] text-muted-foreground bg-muted/50 font-medium flex items-center gap-1">
                  <GitBranch className="w-3 h-3 text-blue-400" />
                  Flow ({groupedVariables.flow.length})
                </div>
                {groupedVariables.flow.map((v) => (
                  <VariableEntry
                    key={`flow:${v.key}`}
                    varKey={v.key}
                    value={v.value}
                    scope={v.scope}
                  />
                ))}
              </div>
            )}
            {groupedVariables.node.length > 0 && (
              <div>
                <div className="px-2 py-1 text-[10px] text-muted-foreground bg-muted/50 font-medium flex items-center gap-1">
                  <Box className="w-3 h-3 text-purple-400" />
                  Node ({groupedVariables.node.length})
                </div>
                {groupedVariables.node.map((v) => (
                  <VariableEntry
                    key={`node:${v.nodeId}:${v.key}`}
                    varKey={v.key}
                    value={v.value}
                    scope={v.scope}
                    nodeId={v.nodeId}
                  />
                ))}
              </div>
            )}
          </div>
        ) : (
          // Show flat list when specific scope is selected
          <div>
            {filteredVariables.map((v) => (
              <VariableEntry
                key={`${v.scope}:${v.nodeId || ''}:${v.key}`}
                varKey={v.key}
                value={v.value}
                scope={v.scope}
                nodeId={v.nodeId}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
