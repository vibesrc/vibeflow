import { useState, useMemo } from 'react';
import type { NodeType } from '@/api';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Search } from 'lucide-react';
import { PaletteNode } from '@/components/palette-node';

interface NodePaletteProps {
  nodeTypes: NodeType[];
  onAddNode: (nodeType: NodeType) => void;
  onDragStart: (e: React.DragEvent, nodeType: NodeType) => void;
}

export function NodePalette({ nodeTypes, onAddNode, onDragStart }: NodePaletteProps) {
  const [searchTerm, setSearchTerm] = useState('');

  // Filter and group node types
  const groupedNodeTypes = useMemo(() => {
    const filtered = nodeTypes.filter(
      (nt) =>
        nt.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        nt.type.toLowerCase().includes(searchTerm.toLowerCase()) ||
        nt.description?.toLowerCase().includes(searchTerm.toLowerCase())
    );

    return filtered.reduce(
      (acc, nt) => {
        const category = nt.category || 'other';
        if (!acc[category]) acc[category] = [];
        acc[category].push(nt);
        return acc;
      },
      {} as Record<string, NodeType[]>
    );
  }, [nodeTypes, searchTerm]);

  return (
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
        <div className="p-2 space-y-4">
          {Object.entries(groupedNodeTypes).map(([category, types]) => (
            <div key={category}>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-2 mb-2">
                {category}
              </h3>
              <div className="space-y-2 px-1">
                {types.map((nodeType) => (
                  <div
                    key={nodeType.type}
                    onClick={() => onAddNode(nodeType)}
                  >
                    <PaletteNode
                      nodeType={nodeType}
                      onDragStart={onDragStart}
                    />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
    </div>
  );
}
