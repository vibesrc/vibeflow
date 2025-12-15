import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { X, Plus } from 'lucide-react';

interface EnvPanelProps {
  environment: Record<string, string>;
  onUpdate: (key: string, value: string) => void;
  onAdd: (key: string) => void;
  onDelete: (key: string) => void;
}

export function EnvPanel({ environment, onUpdate, onAdd, onDelete }: EnvPanelProps) {
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
