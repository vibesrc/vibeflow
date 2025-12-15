import { useState, useEffect, useMemo, useCallback } from 'react';
import { useDebouncedCallback } from '@/hooks/use-debounce';
import type { NodeDefinition, NodeType, ConfigSpec } from '@/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { CodeEditor, CodeInput } from '@/components/code-editor';
import { Plus, X, GripVertical } from 'lucide-react';

interface NodeConfigPanelProps {
  node: NodeDefinition;
  nodeType?: NodeType;
  onUpdate: (updates: Partial<NodeDefinition>) => void;
}

export function NodeConfigPanel({ node, nodeType, onUpdate }: NodeConfigPanelProps) {
  const [localConfig, setLocalConfig] = useState<Record<string, unknown>>(node.config || {});
  const [localName, setLocalName] = useState(node.name || '');
  const [newKey, setNewKey] = useState('');

  // Sync local state when node changes (but not during active editing)
  useEffect(() => {
    setLocalConfig(node.config || {});
    setLocalName(node.name || '');
  }, [node.id]); // Only sync on node ID change, not during editing

  // Get config schema for this node type
  const configSchema = useMemo(() => nodeType?.config || [], [nodeType]);

  // Custom fields that aren't in the schema
  const customFields = useMemo(() => {
    const schemaNames = new Set(configSchema.map(s => s.name));
    return Object.keys(localConfig).filter(key => !schemaNames.has(key));
  }, [configSchema, localConfig]);

  // Debounced updates to parent - prevents lag during typing
  const debouncedConfigUpdate = useDebouncedCallback((config: Record<string, unknown>) => {
    onUpdate({ config });
  }, 300);

  const debouncedNameUpdate = useDebouncedCallback((name: string) => {
    onUpdate({ name });
  }, 300);

  const handleConfigChange = useCallback((key: string, value: unknown) => {
    setLocalConfig(prev => {
      const updated = { ...prev, [key]: value };
      debouncedConfigUpdate(updated);
      return updated;
    });
  }, [debouncedConfigUpdate]);

  const handleNameChange = useCallback((name: string) => {
    setLocalName(name);
    debouncedNameUpdate(name);
  }, [debouncedNameUpdate]);

  const handleAddConfig = () => {
    if (!newKey.trim() || newKey in localConfig) return;
    handleConfigChange(newKey.trim(), '');
    setNewKey('');
  };

  const handleRemoveConfig = (key: string) => {
    const updated = { ...localConfig };
    delete updated[key];
    setLocalConfig(updated);
    onUpdate({ config: updated });
  };

  return (
    <ScrollArea className="h-full">
      <div className="p-4 space-y-6">
        {/* Node Identity */}
        <section>
          <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-3">
            Node
          </h3>
          <div className="space-y-3">
            <div>
              <Label htmlFor="node-id" className="text-xs">ID</Label>
              <Input
                id="node-id"
                value={node.id}
                disabled
                className="h-8 text-sm font-mono bg-muted"
              />
            </div>
            <div>
              <Label htmlFor="node-type" className="text-xs">Type</Label>
              <Input
                id="node-type"
                value={node.type}
                disabled
                className="h-8 text-sm font-mono bg-muted"
              />
            </div>
            {nodeType?.description && (
              <p className="text-xs text-muted-foreground">{nodeType.description}</p>
            )}
            <div>
              <Label htmlFor="node-name" className="text-xs">Name</Label>
              <Input
                id="node-name"
                value={localName}
                onChange={(e) => handleNameChange(e.target.value)}
                placeholder="Display name"
                className="h-8 text-sm"
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="node-enabled" className="text-xs">Enabled</Label>
              <Switch
                id="node-enabled"
                checked={node.enabled !== false}
                onCheckedChange={(checked) => onUpdate({ enabled: checked })}
              />
            </div>
          </div>
        </section>

        <Separator />

        {/* Configuration */}
        <section>
          <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-3">
            Configuration
          </h3>
          <div className="space-y-3">
            {/* Schema-defined fields - always show all */}
            {configSchema.map((spec) => {
              // Use value from localConfig, or fall back to spec default, or type default
              const value = spec.name in localConfig
                ? localConfig[spec.name]
                : (spec.default ?? getDefaultForType(spec.type));
              return (
                <SchemaConfigField
                  key={spec.name}
                  spec={spec}
                  value={value}
                  onChange={(v) => handleConfigChange(spec.name, v)}
                />
              );
            })}

            {/* Custom fields (not in schema) */}
            {customFields.map((key) => (
              <ConfigField
                key={key}
                fieldKey={key}
                value={localConfig[key]}
                onChange={(v) => handleConfigChange(key, v)}
                onRemove={() => handleRemoveConfig(key)}
              />
            ))}

            {configSchema.length === 0 && customFields.length === 0 && (
              <p className="text-sm text-muted-foreground text-center py-2">
                No configuration
              </p>
            )}

            {/* Add custom config */}
            <div className="flex gap-2 pt-2 border-t border-border/50">
              <Input
                value={newKey}
                onChange={(e) => setNewKey(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleAddConfig()}
                placeholder="Add custom field..."
                className="h-8 text-sm flex-1"
              />
              <Button
                variant="secondary"
                size="icon"
                className="h-8 w-8"
                onClick={handleAddConfig}
                disabled={!newKey.trim()}
              >
                <Plus className="w-4 h-4" />
              </Button>
            </div>
          </div>
        </section>

      </div>
    </ScrollArea>
  );
}

function getDefaultForType(type: string): unknown {
  switch (type) {
    case 'bool':
      return false;
    case 'int':
    case 'float':
      return 0;
    case 'array':
      return [];
    case 'object':
      return {};
    default:
      return '';
  }
}

// Schema-driven array items editor - uses the items schema to render each item's fields
function ArrayItemsEditor({
  value,
  itemsSchema,
  onChange,
}: {
  value: unknown;
  itemsSchema: ConfigSpec[];
  onChange: (value: Record<string, unknown>[]) => void;
}) {
  const items = Array.isArray(value) ? (value as Record<string, unknown>[]) : [];

  const addItem = () => {
    // Create a new item with default values from schema
    const newItem: Record<string, unknown> = {};
    for (const fieldSpec of itemsSchema) {
      newItem[fieldSpec.name] = fieldSpec.default ?? getDefaultForType(fieldSpec.type);
    }
    onChange([...items, newItem]);
  };

  const updateItem = (index: number, fieldName: string, fieldValue: unknown) => {
    const updated = [...items];
    updated[index] = { ...updated[index], [fieldName]: fieldValue };
    onChange(updated);
  };

  const removeItem = (index: number) => {
    onChange(items.filter((_, i) => i !== index));
  };

  return (
    <div className="space-y-2">
      {items.map((item, index) => (
        <div key={index} className="border border-border rounded-md p-2 space-y-2 bg-muted/30">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1">
              <GripVertical className="w-3 h-3 text-muted-foreground" />
              <span className="text-xs font-medium">Item {index + 1}</span>
            </div>
            <Button
              variant="ghost"
              size="icon"
              className="h-5 w-5"
              onClick={() => removeItem(index)}
            >
              <X className="w-3 h-3" />
            </Button>
          </div>
          <div className="space-y-2">
            {itemsSchema.map((fieldSpec) => (
              <ItemField
                key={fieldSpec.name}
                spec={fieldSpec}
                value={item[fieldSpec.name]}
                onChange={(v) => updateItem(index, fieldSpec.name, v)}
              />
            ))}
          </div>
        </div>
      ))}
      <Button
        variant="outline"
        size="sm"
        className="w-full h-7 text-xs"
        onClick={addItem}
      >
        <Plus className="w-3 h-3 mr-1" />
        Add Item
      </Button>
    </div>
  );
}

// Renders a single field inside an array item based on schema
function ItemField({
  spec,
  value,
  onChange,
}: {
  spec: ConfigSpec;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const hasOptions = spec.options && spec.options.length > 0;

  return (
    <div>
      <Label className="text-[10px] text-muted-foreground">
        {spec.name}
        {spec.required && <span className="text-destructive ml-1">*</span>}
      </Label>
      {spec.description && (
        <p className="text-[9px] text-muted-foreground/70 mb-1">{spec.description}</p>
      )}

      {hasOptions ? (
        <Select
          value={String(value ?? '')}
          onValueChange={(v) => onChange(v)}
        >
          <SelectTrigger className="h-7 text-xs">
            <SelectValue placeholder={`Select ${spec.name}...`} />
          </SelectTrigger>
          <SelectContent>
            {spec.options!.map((opt) => (
              <SelectItem key={String(opt)} value={String(opt)}>
                {String(opt)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : spec.type === 'bool' ? (
        <Switch
          checked={Boolean(value)}
          onCheckedChange={onChange}
        />
      ) : spec.type === 'int' || spec.type === 'float' ? (
        <Input
          type="number"
          value={value as number ?? ''}
          onChange={(e) => {
            const val = spec.type === 'int'
              ? parseInt(e.target.value)
              : parseFloat(e.target.value);
            onChange(isNaN(val) ? 0 : val);
          }}
          className="h-7 text-xs"
          placeholder={spec.default !== undefined ? `Default: ${spec.default}` : undefined}
        />
      ) : (
        <Input
          value={String(value ?? '')}
          onChange={(e) => onChange(e.target.value)}
          className="h-7 text-xs"
          placeholder={spec.default !== undefined ? `Default: ${spec.default}` : `Enter ${spec.name}...`}
        />
      )}
    </div>
  );
}

function SchemaConfigField({
  spec,
  value,
  onChange,
}: {
  spec: ConfigSpec;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const hasOptions = spec.options && spec.options.length > 0;
  // Use items schema if available for arrays
  const hasItemsSchema = spec.type === 'array' && spec.items && spec.items.length > 0;

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-1">
        <Label className="text-xs font-mono">{spec.name}</Label>
        {spec.required && <span className="text-destructive text-xs">*</span>}
      </div>
      {spec.description && (
        <p className="text-[10px] text-muted-foreground">{spec.description}</p>
      )}

      {hasOptions ? (
        <Select
          value={String(value ?? '')}
          onValueChange={(v) => onChange(v)}
        >
          <SelectTrigger className="h-8 text-sm">
            <SelectValue placeholder={`Select ${spec.name}...`} />
          </SelectTrigger>
          <SelectContent>
            {spec.options!.map((opt) => (
              <SelectItem key={String(opt)} value={String(opt)}>
                {String(opt)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : spec.type === 'bool' ? (
        <Switch
          checked={Boolean(value)}
          onCheckedChange={onChange}
        />
      ) : spec.type === 'int' || spec.type === 'float' ? (
        <Input
          type="number"
          value={value as number ?? ''}
          onChange={(e) => {
            const val = spec.type === 'int'
              ? parseInt(e.target.value)
              : parseFloat(e.target.value);
            onChange(isNaN(val) ? 0 : val);
          }}
          className="h-8 text-sm"
          placeholder={spec.default !== undefined ? `Default: ${spec.default}` : undefined}
        />
      ) : hasItemsSchema ? (
        <ArrayItemsEditor
          value={value}
          itemsSchema={spec.items!}
          onChange={onChange}
        />
      ) : spec.type === 'array' || spec.type === 'object' ? (
        <Textarea
          value={typeof value === 'string' ? value : JSON.stringify(value, null, 2)}
          onChange={(e) => {
            try {
              onChange(JSON.parse(e.target.value));
            } catch {
              // Keep as string while editing
            }
          }}
          className="text-sm font-mono min-h-[80px]"
          placeholder={spec.type === 'array' ? '[]' : '{}'}
        />
      ) : spec.format === 'code' ? (
        <CodeEditor
          value={String(value ?? '')}
          onChange={(v) => onChange(v)}
          language={spec.language || 'javascript'}
          height={150}
          label={spec.name}
        />
      ) : spec.format === 'template' ? (
        <CodeInput
          value={String(value ?? '')}
          onChange={(v) => onChange(v)}
          language={spec.language || 'plaintext'}
        />
      ) : (
        <Input
          value={String(value ?? '')}
          onChange={(e) => onChange(e.target.value)}
          className="h-8 text-sm"
          placeholder={spec.default !== undefined ? `Default: ${spec.default}` : `Enter ${spec.name}...`}
        />
      )}
    </div>
  );
}

function ConfigField({
  fieldKey,
  value,
  onChange,
  onRemove,
}: {
  fieldKey: string;
  value: unknown;
  onChange: (value: unknown) => void;
  onRemove: () => void;
}) {
  const isBoolean = typeof value === 'boolean';
  const isNumber = typeof value === 'number';
  const isObject = typeof value === 'object' && value !== null;
  const isMultiline = typeof value === 'string' && (value.includes('\n') || value.length > 50);

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <Label className="text-xs font-mono">{fieldKey}</Label>
        <Button
          variant="ghost"
          size="icon"
          className="h-5 w-5"
          onClick={onRemove}
        >
          <X className="w-3 h-3" />
        </Button>
      </div>

      {isBoolean ? (
        <Switch
          checked={value}
          onCheckedChange={onChange}
        />
      ) : isNumber ? (
        <Input
          type="number"
          value={value}
          onChange={(e) => onChange(parseFloat(e.target.value) || 0)}
          className="h-8 text-sm"
        />
      ) : isObject ? (
        <Textarea
          value={JSON.stringify(value, null, 2)}
          onChange={(e) => {
            try {
              onChange(JSON.parse(e.target.value));
            } catch {
              // Keep the text for now
            }
          }}
          className="text-sm font-mono min-h-[80px]"
        />
      ) : isMultiline ? (
        <Textarea
          value={String(value || '')}
          onChange={(e) => onChange(e.target.value)}
          className="text-sm min-h-[60px]"
        />
      ) : (
        <Input
          value={String(value || '')}
          onChange={(e) => onChange(e.target.value)}
          className="h-8 text-sm"
          placeholder={`Enter ${fieldKey}...`}
        />
      )}
    </div>
  );
}
