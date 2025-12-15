import { useState, useMemo, useEffect, useRef, useCallback } from 'react';
import type { NodeDefinition } from '@/api';
import type { WSEvent, ConnectionStatus } from '@/hooks/use-websocket';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  Settings,
  Wifi,
  WifiOff,
  Trash2 as ClearIcon,
  ChevronRight,
  ChevronDown,
  Pause,
  Play,
  AlertCircle,
} from 'lucide-react';

// Simple JSON syntax highlighter
function JsonHighlight({ json }: { json: string }) {
  const highlighted = useMemo(() => {
    // Escape HTML first
    let html = json
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');

    // Highlight JSON syntax
    html = html
      // Strings (but not keys)
      .replace(/: ("(?:[^"\\]|\\.)*")/g, ': <span class="text-green-400">$1</span>')
      // Keys
      .replace(/"([^"]+)":/g, '<span class="text-blue-400">"$1"</span>:')
      // Numbers
      .replace(/: (-?\d+\.?\d*)/g, ': <span class="text-amber-400">$1</span>')
      // Booleans
      .replace(/: (true|false)/g, ': <span class="text-purple-400">$1</span>')
      // Null
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

// Expandable debug log entry component (like browser devtools)
function ExpandableDebugEntry({
  timestamp,
  nodeName,
  topic,
  payload,
}: {
  timestamp: string;
  nodeName: string;
  topic?: string;
  payload: unknown;
}) {
  const [isExpanded, setIsExpanded] = useState(false);

  // Format payload preview (single line, truncated)
  const payloadPreview = useMemo(() => {
    if (payload === null) return 'null';
    if (payload === undefined) return 'undefined';
    if (typeof payload === 'string') {
      return payload.length > 80 ? payload.slice(0, 80) + '...' : payload;
    }
    const str = JSON.stringify(payload);
    return str.length > 80 ? str.slice(0, 80) + '...' : str;
  }, [payload]);

  // Full formatted payload
  const payloadFull = useMemo(() => {
    if (typeof payload === 'string') return payload;
    return JSON.stringify(payload, null, 2);
  }, [payload]);

  // Check if payload is expandable (object/array or long string)
  const isExpandable = typeof payload === 'object' && payload !== null ||
    (typeof payload === 'string' && payload.length > 80);

  return (
    <div className="border-b border-border/30 hover:bg-muted/30">
      <button
        onClick={() => isExpandable && setIsExpanded(!isExpanded)}
        className={cn(
          "w-full text-left py-1.5 px-2 flex items-start gap-1.5 text-xs",
          isExpandable && "cursor-pointer"
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
        <span className="text-muted-foreground shrink-0">{timestamp}</span>
        <span className="text-purple-400 font-medium shrink-0">{nodeName}</span>
        {topic && <span className="text-muted-foreground shrink-0">: {topic}</span>}
        {!isExpanded && (
          <span className="text-foreground/70 truncate font-mono">{payloadPreview}</span>
        )}
      </button>
      {isExpanded && (
        <pre className="px-2 pb-2 pl-7 text-xs whitespace-pre-wrap break-all font-mono bg-muted/20">
          <JsonHighlight json={payloadFull} />
        </pre>
      )}
    </div>
  );
}

// Expandable error entry component
function ExpandableErrorEntry({
  timestamp,
  nodeName,
  message,
}: {
  timestamp: string;
  nodeName?: string;
  message: string;
}) {
  const [isExpanded, setIsExpanded] = useState(false);

  // Check if message is long enough to be expandable
  const isExpandable = message.length > 60 || message.includes('\n');

  // Truncated preview
  const messagePreview = useMemo(() => {
    const firstLine = message.split('\n')[0];
    return firstLine.length > 60 ? firstLine.slice(0, 60) + '...' : firstLine;
  }, [message]);

  return (
    <div className="border-b border-border/30 bg-destructive/10 border-l-2 border-l-destructive">
      <button
        onClick={() => isExpandable && setIsExpanded(!isExpanded)}
        className={cn(
          "w-full text-left py-1.5 px-2 flex items-start gap-1.5 text-xs",
          isExpandable && "cursor-pointer"
        )}
      >
        {isExpandable ? (
          isExpanded ? (
            <ChevronDown className="w-3 h-3 mt-0.5 text-destructive shrink-0" />
          ) : (
            <ChevronRight className="w-3 h-3 mt-0.5 text-destructive shrink-0" />
          )
        ) : (
          <AlertCircle className="w-3 h-3 mt-0.5 text-destructive shrink-0" />
        )}
        <span className="text-muted-foreground shrink-0">{timestamp}</span>
        {nodeName && <span className="text-cyan-400 font-medium shrink-0">{nodeName}</span>}
        {!isExpanded && (
          <span className="text-destructive truncate">{messagePreview}</span>
        )}
      </button>
      {isExpanded && (
        <pre className="px-2 pb-2 pl-7 text-xs text-destructive whitespace-pre-wrap break-all font-mono">
          {message}
        </pre>
      )}
    </div>
  );
}

// Event types to show by default (debug output, flow lifecycle, and errors)
const DEFAULT_VISIBLE_TYPES = new Set(['debug', 'flow.start', 'flow.stop', 'flow.error', 'node.error']);
// All available event types for filtering
const ALL_EVENT_TYPES = ['debug', 'flow.start', 'flow.stop', 'flow.error', 'node.error'];

interface DebugPanelProps {
  events: WSEvent[];
  wsStatus: ConnectionStatus;
  onClear: () => void;
  nodes?: NodeDefinition[];
}

export function DebugPanel({ events, wsStatus, onClear, nodes }: DebugPanelProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [visibleTypes, setVisibleTypes] = useState<Set<string>>(DEFAULT_VISIBLE_TYPES);
  const [selectedNode, setSelectedNode] = useState<string>('all');
  const [showFilters, setShowFilters] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [pausedEvents, setPausedEvents] = useState<WSEvent[] | null>(null);

  // Use paused snapshot or live events
  const displayEvents = pausedEvents ?? events;

  // Get unique node IDs from display events
  const nodeIds = useMemo(() => {
    const ids = new Set<string>();
    displayEvents.forEach((e) => {
      if (e.node_id) ids.add(e.node_id);
    });
    return Array.from(ids);
  }, [displayEvents]);

  // Filter events based on visible types and selected node
  const filteredEvents = useMemo(() => {
    return displayEvents.filter((e) => {
      if (!visibleTypes.has(e.type)) return false;
      if (selectedNode !== 'all' && e.node_id !== selectedNode) return false;
      return true;
    });
  }, [displayEvents, visibleTypes, selectedNode]);

  // Handle pause/resume
  const handleTogglePause = useCallback(() => {
    if (isPaused) {
      // Resume: clear paused snapshot
      setPausedEvents(null);
    } else {
      // Pause: capture current events
      setPausedEvents([...events]);
    }
    setIsPaused(!isPaused);
  }, [isPaused, events]);

  // Auto-scroll to bottom when new events arrive (unless paused)
  useEffect(() => {
    if (scrollRef.current && !isPaused) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [filteredEvents, isPaused]);

  const toggleType = (type: string) => {
    setVisibleTypes((prev) => {
      const next = new Set(prev);
      if (next.has(type)) {
        next.delete(type);
      } else {
        next.add(type);
      }
      return next;
    });
  };

  const formatTimestamp = (ts: string) => {
    try {
      const date = new Date(ts);
      return date.toLocaleTimeString('en-US', {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        fractionalSecondDigits: 3,
      });
    } catch {
      return ts;
    }
  };

  // Get display name for a node
  const getNodeName = (nodeId: string) => {
    const node = nodes?.find((n) => n.id === nodeId);
    return node?.name || nodeId;
  };

  // Render event based on type
  const renderEvent = (event: WSEvent, index: number) => {
    const timestamp = formatTimestamp(event.timestamp);

    if (event.type === 'debug') {
      const data = event.data as { node_name?: string; topic?: string; payload?: unknown; level?: string } | undefined;
      const nodeName = data?.node_name || (event.node_id ? getNodeName(event.node_id) : 'unknown');
      const topic = data?.topic;
      const payload = data?.payload;

      return (
        <ExpandableDebugEntry
          key={index}
          timestamp={timestamp}
          nodeName={nodeName}
          topic={topic}
          payload={payload}
        />
      );
    }

    if (event.type === 'flow.start') {
      return (
        <div key={index} className="py-1.5 px-2 text-xs flex items-center gap-2">
          <span className="text-muted-foreground">{timestamp}</span>
          <span className="text-blue-400">Flow started</span>
        </div>
      );
    }

    if (event.type === 'flow.stop') {
      return (
        <div key={index} className="py-1.5 px-2 text-xs flex items-center gap-2">
          <span className="text-muted-foreground">{timestamp}</span>
          <span className="text-blue-400">Flow stopped</span>
        </div>
      );
    }

    if (event.type === 'flow.error' || event.type === 'node.error') {
      const data = event.data as { message?: string; Error?: string } | undefined;
      const message = data?.message || data?.Error || 'Unknown error';
      const nodeName = event.node_id ? getNodeName(event.node_id) : undefined;

      return (
        <ExpandableErrorEntry
          key={index}
          timestamp={timestamp}
          nodeName={nodeName}
          message={message}
        />
      );
    }

    // For flow lifecycle events
    const typeColor = event.type === 'flow.start' ? 'text-green-400' : event.type === 'flow.stop' ? 'text-amber-400' : 'text-muted-foreground';

    return (
      <div key={index} className="py-1 px-2 text-xs flex gap-2 text-muted-foreground">
        <span>{timestamp}</span>
        <span className={typeColor}>[{event.type}]</span>
      </div>
    );
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between p-2 border-b border-border">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          {wsStatus === 'connected' ? (
            <>
              <Wifi className="w-3 h-3 text-success" />
              <span>Connected</span>
            </>
          ) : wsStatus === 'connecting' ? (
            <>
              <Wifi className="w-3 h-3 text-warning animate-pulse" />
              <span>Connecting...</span>
            </>
          ) : (
            <>
              <WifiOff className="w-3 h-3 text-muted-foreground" />
              <span>Disconnected</span>
            </>
          )}
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant={isPaused ? 'secondary' : 'ghost'}
            size="icon"
            className="h-6 w-6"
            onClick={handleTogglePause}
            title={isPaused ? 'Resume' : 'Pause'}
          >
            {isPaused ? <Play className="w-3 h-3" /> : <Pause className="w-3 h-3" />}
          </Button>
          <Button
            variant={showFilters ? 'secondary' : 'ghost'}
            size="icon"
            className="h-6 w-6"
            onClick={() => setShowFilters(!showFilters)}
            title="Toggle filters"
          >
            <Settings className="w-3 h-3" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={onClear}
            disabled={events.length === 0}
            title="Clear"
          >
            <ClearIcon className="w-3 h-3" />
          </Button>
        </div>
      </div>

      {/* Filters */}
      {showFilters && (
        <div className="p-2 border-b border-border space-y-2 text-xs">
          <div>
            <div className="text-muted-foreground mb-1">Event Types:</div>
            <div className="flex flex-wrap gap-1">
              {ALL_EVENT_TYPES.map((type) => (
                <button
                  key={type}
                  onClick={() => toggleType(type)}
                  className={cn(
                    'px-1.5 py-0.5 rounded text-[10px]',
                    visibleTypes.has(type)
                      ? 'bg-primary/20 text-primary'
                      : 'bg-muted text-muted-foreground'
                  )}
                >
                  {type}
                </button>
              ))}
            </div>
          </div>
          {nodeIds.length > 0 && (
            <div>
              <div className="text-muted-foreground mb-1">Node:</div>
              <select
                value={selectedNode}
                onChange={(e) => setSelectedNode(e.target.value)}
                className="w-full bg-muted border border-border rounded px-2 py-1 text-xs"
              >
                <option value="all">All nodes</option>
                {nodeIds.map((id) => (
                  <option key={id} value={id}>
                    {getNodeName(id)}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
      )}

      {/* Events */}
      <div ref={scrollRef} className="flex-1 overflow-auto font-mono">
        {filteredEvents.length === 0 ? (
          <div className="h-full flex items-center justify-center text-muted-foreground text-xs">
            {events.length === 0 ? 'No events yet' : 'No matching events'}
          </div>
        ) : (
          <div>{filteredEvents.map((event, i) => renderEvent(event, i))}</div>
        )}
      </div>
    </div>
  );
}
