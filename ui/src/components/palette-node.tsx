import { memo } from 'react';
import type { NodeType } from '@/api';
import { BaseNode, BaseNodeHeader, BaseNodeHeaderTitle } from '@/components/base-node';

interface PaletteNodeProps {
  nodeType: NodeType;
  onDragStart: (e: React.DragEvent, nodeType: NodeType) => void;
}

export const PaletteNode = memo(function PaletteNode({
  nodeType,
  onDragStart,
}: PaletteNodeProps) {
  const handleDragStart = (e: React.DragEvent) => {
    // Store grab offset so drop position can account for it
    const rect = e.currentTarget.getBoundingClientRect();
    const offsetX = e.clientX - rect.left;
    const offsetY = e.clientY - rect.top;
    e.dataTransfer.setData('application/vibeflow-offset', JSON.stringify({ offsetX, offsetY }));
    onDragStart(e, nodeType);
  };

  return (
    <div
      draggable
      onDragStart={handleDragStart}
      className="cursor-grab active:cursor-grabbing"
    >
      <BaseNode className="min-w-0 w-full">
        <BaseNodeHeader showBorder={false} className="py-2 px-3">
          <BaseNodeHeaderTitle className="text-sm truncate">
            {nodeType.name}
          </BaseNodeHeaderTitle>
        </BaseNodeHeader>
      </BaseNode>
    </div>
  );
});
