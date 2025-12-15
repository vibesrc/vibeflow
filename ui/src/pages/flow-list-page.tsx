import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { listFlows, createFlow, deleteFlow, startFlow, stopFlow } from '@/api';
import type { Flow } from '@/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Plus,
  Play,
  Square,
  MoreVertical,
  Trash2,
  Edit,
  Activity,
  AlertCircle,
  Clock,
} from 'lucide-react';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';

export function FlowListPage() {
  const queryClient = useQueryClient();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [newFlowName, setNewFlowName] = useState('');
  const [newFlowDescription, setNewFlowDescription] = useState('');

  const { data: flows, isLoading, error } = useQuery({
    queryKey: ['flows'],
    queryFn: listFlows,
    refetchInterval: 3000,
  });

  const createMutation = useMutation({
    mutationFn: createFlow,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flows'] });
      setCreateDialogOpen(false);
      setNewFlowName('');
      setNewFlowDescription('');
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to create flow'),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteFlow,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flows'] });
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to delete flow'),
  });

  const startMutation = useMutation({
    mutationFn: startFlow,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flows'] });
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to start flow'),
  });

  const stopMutation = useMutation({
    mutationFn: stopFlow,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flows'] });
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to stop flow'),
  });

  const handleCreate = () => {
    if (!newFlowName.trim()) return;
    createMutation.mutate({
      name: newFlowName.trim(),
      description: newFlowDescription.trim(),
      enabled: false,
    });
  };

  if (isLoading) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-muted-foreground animate-pulse">Loading flows...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-destructive flex items-center gap-2">
          <AlertCircle className="w-5 h-5" />
          Failed to load flows
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <header className="flex-shrink-0 h-14 border-b border-border flex items-center justify-between px-6">
        <div className="flex items-center gap-3">
          <Activity className="w-5 h-5 text-primary" />
          <h1 className="text-lg font-semibold">Flows</h1>
          <Badge variant="secondary" className="text-xs">
            {flows?.length ?? 0}
          </Badge>
        </div>

        <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
          <DialogTrigger asChild>
            <Button size="sm" className="gap-2">
              <Plus className="w-4 h-4" />
              New Flow
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create New Flow</DialogTitle>
              <DialogDescription>
                Create a new automation flow. You can configure nodes and connections in the editor.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  value={newFlowName}
                  onChange={(e) => setNewFlowName(e.target.value)}
                  placeholder="my-flow"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="description">Description</Label>
                <Textarea
                  id="description"
                  value={newFlowDescription}
                  onChange={(e) => setNewFlowDescription(e.target.value)}
                  placeholder="What does this flow do?"
                  rows={3}
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="secondary" onClick={() => setCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button onClick={handleCreate} disabled={!newFlowName.trim() || createMutation.isPending}>
                {createMutation.isPending ? 'Creating...' : 'Create Flow'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </header>

      {/* Flow List */}
      <ScrollArea className="flex-1">
        <div className="p-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {flows?.length === 0 ? (
            <div className="col-span-full text-center py-12 text-muted-foreground">
              <Activity className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>No flows yet</p>
              <p className="text-sm">Create your first flow to get started</p>
            </div>
          ) : (
            flows?.map((flow) => <FlowCard key={flow.id} flow={flow} onStart={startMutation.mutate} onStop={stopMutation.mutate} onDelete={deleteMutation.mutate} />)
          )}
        </div>
      </ScrollArea>
    </div>
  );
}

function FlowCard({
  flow,
  onStart,
  onStop,
  onDelete,
}: {
  flow: Flow;
  onStart: (id: string) => void;
  onStop: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  const isRunning = flow.runtime_status === 'running';
  const hasError = flow.runtime_status === 'error';

  return (
    <Card className={cn(
      'group transition-all hover:border-primary/50',
      isRunning && 'border-success/50',
      hasError && 'border-destructive/50'
    )}>
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2">
            <StatusIndicator status={flow.runtime_status} />
            <CardTitle className="text-base">
              <Link to={`/flows/${flow.id}`} className="hover:text-primary transition-colors">
                {flow.name}
              </Link>
            </CardTitle>
          </div>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8 opacity-0 group-hover:opacity-100">
                <MoreVertical className="w-4 h-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem asChild>
                <Link to={`/flows/${flow.id}`}>
                  <Edit className="w-4 h-4 mr-2" />
                  Edit
                </Link>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              {isRunning ? (
                <DropdownMenuItem onClick={() => onStop(flow.id)}>
                  <Square className="w-4 h-4 mr-2" />
                  Stop
                </DropdownMenuItem>
              ) : (
                <DropdownMenuItem onClick={() => onStart(flow.id)}>
                  <Play className="w-4 h-4 mr-2" />
                  Start
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => onDelete(flow.id)} className="text-destructive focus:text-destructive">
                <Trash2 className="w-4 h-4 mr-2" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
        <CardDescription className="text-sm line-clamp-2">
          {flow.description || 'No description'}
        </CardDescription>
      </CardHeader>

      <CardContent>
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <div className="flex items-center gap-1">
            <Clock className="w-3 h-3" />
            {new Date(flow.updated_at).toLocaleDateString()}
          </div>
          <Badge
            variant={isRunning ? 'default' : hasError ? 'destructive' : 'secondary'}
            className={cn(
              'text-[10px] uppercase tracking-wider',
              isRunning && 'bg-success/20 text-success border-success/30'
            )}
          >
            {flow.runtime_status}
          </Badge>
        </div>
      </CardContent>
    </Card>
  );
}

function StatusIndicator({ status }: { status: string }) {
  if (status === 'running') {
    return (
      <span className="relative flex h-2 w-2">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-success opacity-75" />
        <span className="relative inline-flex rounded-full h-2 w-2 bg-success" />
      </span>
    );
  }
  return (
    <div
      className={cn(
        'w-2 h-2 rounded-full',
        status === 'error' && 'bg-destructive',
        status === 'stopped' && 'bg-muted-foreground'
      )}
    />
  );
}
