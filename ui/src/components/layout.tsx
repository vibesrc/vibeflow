import { Outlet, Link, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getHealth } from '@/api';
import { Activity, Workflow, Settings } from 'lucide-react';
import { cn } from '@/lib/utils';

export function Layout() {
  const location = useLocation();
  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: getHealth,
    refetchInterval: 5000,
  });

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside className="w-16 bg-sidebar border-r border-sidebar-border flex flex-col items-center py-4 gap-2">
        {/* Logo */}
        <div className="w-10 h-10 rounded bg-primary/20 flex items-center justify-center mb-4">
          <Workflow className="w-6 h-6 text-primary" />
        </div>

        {/* Nav Items */}
        <NavItem
          to="/flows"
          icon={<Activity className="w-5 h-5" />}
          active={location.pathname.startsWith('/flows')}
          label="Flows"
        />
        <NavItem
          to="/settings"
          icon={<Settings className="w-5 h-5" />}
          active={location.pathname === '/settings'}
          label="Settings"
        />

        {/* Spacer */}
        <div className="flex-1" />

        {/* Status Indicator */}
        <div className="flex flex-col items-center gap-1 text-xs text-muted-foreground">
          <div
            className={cn(
              'w-3 h-3 rounded-full',
              health?.status === 'ok' ? 'bg-success glow-success' : 'bg-danger glow-danger'
            )}
          />
          <span className="text-[10px]">{health?.running_flows ?? 0}</span>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
    </div>
  );
}

function NavItem({
  to,
  icon,
  active,
  label,
}: {
  to: string;
  icon: React.ReactNode;
  active: boolean;
  label: string;
}) {
  return (
    <Link
      to={to}
      className={cn(
        'w-10 h-10 rounded flex items-center justify-center transition-all',
        active
          ? 'bg-primary/20 text-primary'
          : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
      )}
      title={label}
    >
      {icon}
    </Link>
  );
}
