import React from 'react';
import { Layers, Link2, GitBranch, ArrowRight } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

export interface RoutingCardProps {
  title: string;
  type: 'model' | 'alias' | 'combo';
  provider?: string;
  targets?: string[];
  badges?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}

const TYPE_CONFIG = {
  model: {
    icon: Layers,
    color: 'text-emerald-500',
    bg: 'bg-emerald-500/10',
    border: 'border-emerald-500/20',
    glow: 'group-hover:shadow-emerald-500/5',
  },
  alias: {
    icon: Link2,
    color: 'text-blue-500',
    bg: 'bg-blue-500/10',
    border: 'border-blue-500/20',
    glow: 'group-hover:shadow-blue-500/5',
  },
  combo: {
    icon: GitBranch,
    color: 'text-violet-500',
    bg: 'bg-violet-500/10',
    border: 'border-violet-500/20',
    glow: 'group-hover:shadow-violet-500/5',
  },
};

export function RoutingCard({
  title,
  type,
  provider,
  targets,
  badges,
  actions,
  children,
  className,
}: RoutingCardProps) {
  const config = TYPE_CONFIG[type];
  const Icon = config.icon;

  return (
    <div
      className={cn(
        'group relative flex flex-col gap-2 px-4 py-3.5 transition-all duration-200',
        'hover:bg-muted/40',
        className
      )}
    >
      {/* Main row */}
      <div className="flex items-center gap-3 min-w-0">
        {/* Icon */}
        <div className={cn(
          'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border transition-all duration-200',
          config.bg, config.border,
          'group-hover:scale-105'
        )}>
          <Icon className={cn('h-4 w-4', config.color)} />
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-sm truncate">{title}</span>
            {provider && (
              <Badge variant="outline" className="font-mono text-[10px] uppercase h-5 px-1.5 shrink-0 bg-muted/50">
                {provider}
              </Badge>
            )}
            {badges}
          </div>

          {/* Target chain */}
          {targets && targets.length > 0 && (
            <div className="flex flex-wrap items-center gap-1">
              {targets.map((target, index) => (
                <React.Fragment key={`${target}-${index}`}>
                  <Badge
                    variant="secondary"
                    className="font-mono text-[11px] px-2 py-0.5 font-normal bg-background border border-border/60 text-muted-foreground truncate max-w-[200px]"
                  >
                    {target}
                  </Badge>
                  {index < targets.length - 1 && (
                    <ArrowRight className="h-3 w-3 text-muted-foreground/30 shrink-0" />
                  )}
                </React.Fragment>
              ))}
            </div>
          )}

          {children}
        </div>

        {/* Actions */}
        {actions && (
          <div className="flex items-center gap-1 shrink-0 sm:opacity-0 sm:group-hover:opacity-100 transition-opacity duration-200">
            {actions}
          </div>
        )}
      </div>
    </div>
  );
}
