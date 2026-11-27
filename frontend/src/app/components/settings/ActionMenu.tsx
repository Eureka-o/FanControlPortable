'use client';

import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { AnimatePresence, m } from 'framer-motion';
import { Check, ChevronDown, Loader2, MoreHorizontal } from 'lucide-react';
import { Button } from '../ui';

export type ActionMenuItem =
  | { type: 'separator' }
  | { type: 'label'; label: string }
  | {
      type?: 'item';
      id: string;
      label: string;
      icon?: ReactNode;
      active?: boolean;
      danger?: boolean;
      disabled?: boolean;
      loading?: boolean;
      onSelect: () => void;
    };

interface ActionMenuProps {
  items: ActionMenuItem[];
  label?: string;
  icon?: ReactNode;
  showChevron?: boolean;
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  align?: 'right' | 'left';
  ariaLabel: string;
  disabled?: boolean;
  badgeCount?: number;
}

interface MenuPosition {
  top: number;
  left: number;
  maxHeight: number;
  placement: 'bottom' | 'top';
  ready: boolean;
}

const VIEWPORT_MARGIN = 12;
const MENU_GAP = 6;

export default function ActionMenu({
  items,
  label,
  icon,
  showChevron = false,
  variant = 'outline',
  size = 'sm',
  align = 'right',
  ariaLabel,
  disabled = false,
  badgeCount,
}: ActionMenuProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<MenuPosition>({
    top: 0,
    left: 0,
    maxHeight: 320,
    placement: 'bottom',
    ready: false,
  });
  const containerRef = useRef<HTMLDivElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);

  const updatePosition = useCallback(() => {
    const triggerEl = containerRef.current;
    if (!triggerEl) return;

    const triggerRect = triggerEl.getBoundingClientRect();
    if (
      triggerRect.bottom < 0 ||
      triggerRect.top > window.innerHeight ||
      triggerRect.right < 0 ||
      triggerRect.left > window.innerWidth
    ) {
      setOpen(false);
      return;
    }

    const menuWidth = menuRef.current?.offsetWidth || 192;
    const menuHeight = menuRef.current?.offsetHeight || 220;
    const maxHeight = Math.max(160, window.innerHeight - VIEWPORT_MARGIN * 2);
    const spaceBelow = window.innerHeight - triggerRect.bottom - VIEWPORT_MARGIN;
    const spaceAbove = triggerRect.top - VIEWPORT_MARGIN;

    let placement: 'bottom' | 'top' = 'bottom';
    let top = triggerRect.bottom + MENU_GAP;
    if (triggerRect.bottom + MENU_GAP + menuHeight > window.innerHeight - VIEWPORT_MARGIN) {
      if (spaceAbove > spaceBelow) {
        placement = 'top';
        top = Math.max(VIEWPORT_MARGIN, triggerRect.top - MENU_GAP - menuHeight);
      } else {
        top = Math.max(
          VIEWPORT_MARGIN,
          window.innerHeight - VIEWPORT_MARGIN - Math.min(menuHeight, maxHeight),
        );
      }
    }

    const rawLeft = align === 'left' ? triggerRect.left : triggerRect.right - menuWidth;
    const left = Math.min(
      Math.max(VIEWPORT_MARGIN, rawLeft),
      Math.max(VIEWPORT_MARGIN, window.innerWidth - menuWidth - VIEWPORT_MARGIN),
    );

    setPos({
      top,
      left,
      maxHeight,
      placement,
      ready: true,
    });
  }, [align]);

  useLayoutEffect(() => {
    if (!open) {
      setPos((prev) => (prev.ready ? { ...prev, ready: false } : prev));
      return undefined;
    }

    updatePosition();
    const handleReposition = () => updatePosition();
    window.addEventListener('resize', handleReposition);
    window.addEventListener('scroll', handleReposition, true);
    return () => {
      window.removeEventListener('resize', handleReposition);
      window.removeEventListener('scroll', handleReposition, true);
    };
  }, [open, updatePosition, items.length]);

  useEffect(() => {
    if (!open) return undefined;

    const handleMouseDown = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (!target) return;
      const inTrigger = Boolean(containerRef.current && containerRef.current.contains(target));
      const inMenu = Boolean(menuRef.current && menuRef.current.contains(target));
      if (!inTrigger && !inMenu) {
        setOpen(false);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', handleMouseDown);
    document.addEventListener('keydown', handleKeyDown, true);
    return () => {
      document.removeEventListener('mousedown', handleMouseDown);
      document.removeEventListener('keydown', handleKeyDown, true);
    };
  }, [open]);

  const hasVisibleBadge = typeof badgeCount === 'number' && badgeCount > 0;

  return (
    <div ref={containerRef} className="relative inline-flex shrink-0">
      <Button
        type="button"
        variant={variant}
        size={size}
        disabled={disabled}
        aria-label={ariaLabel}
        aria-expanded={open}
        aria-haspopup="menu"
        className={!label && !hasVisibleBadge ? 'px-2' : undefined}
        icon={icon ?? <MoreHorizontal className="h-4 w-4" />}
        onClick={() => setOpen((prev) => !prev)}
      >
        {(label || hasVisibleBadge || (showChevron && label)) && (
          <span className="inline-flex items-center gap-1.5">
            {label && <span>{label}</span>}
            {hasVisibleBadge && (
              <span className="inline-flex min-w-[1.125rem] items-center justify-center rounded-full bg-amber-500/18 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-amber-600 dark:text-amber-300">
                {badgeCount}
              </span>
            )}
            {label && showChevron && <ChevronDown className="h-3.5 w-3.5 opacity-70" />}
          </span>
        )}
      </Button>

      {typeof document !== 'undefined' &&
        createPortal(
          <AnimatePresence>
            {open && (
              <m.div
                ref={menuRef}
                role="menu"
                aria-label={ariaLabel}
                data-action-menu-portal="true"
                initial={{ opacity: 0, scale: 0.96, y: pos.placement === 'top' ? 4 : -4 }}
                animate={{ opacity: 1, scale: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.96, y: pos.placement === 'top' ? 4 : -4 }}
                transition={{ duration: 0.14, ease: [0.22, 1, 0.36, 1] }}
                style={{
                  position: 'fixed',
                  top: pos.top,
                  left: pos.left,
                  maxHeight: pos.maxHeight,
                  zIndex: 'calc(var(--layer-dialog-content) + 2)',
                  visibility: pos.ready ? 'visible' : 'hidden',
                  pointerEvents: 'auto',
                }}
                className="pointer-events-auto min-w-[184px] overflow-y-auto rounded-xl border border-border/80 bg-popover p-1.5 text-popover-foreground shadow-xl"
              >
                {items.map((item, idx) => {
                  if (item.type === 'separator') {
                    return <div key={`sep-${idx}`} className="my-1 h-px bg-border/60" role="separator" />;
                  }
                  if (item.type === 'label') {
                    return (
                      <div
                        key={`label-${idx}`}
                        className="px-2.5 pb-1 pt-1.5 text-[11px] font-medium text-muted-foreground"
                      >
                        {item.label}
                      </div>
                    );
                  }
                  const isDisabled = Boolean(item.disabled || item.loading);
                  return (
                    <button
                      key={item.id}
                      type="button"
                      role="menuitem"
                      disabled={isDisabled}
                      onClick={() => {
                        if (isDisabled) return;
                        setOpen(false);
                        item.onSelect();
                      }}
                      className={`flex w-full cursor-pointer items-center justify-between gap-2 rounded-lg px-2.5 py-2 text-left text-xs font-medium transition-colors hover:bg-muted/80 disabled:cursor-not-allowed disabled:opacity-45 ${
                        item.danger ? 'text-destructive hover:bg-destructive/10' : ''
                      }`}
                    >
                      <span className="flex min-w-0 items-center gap-2">
                        {item.loading ? (
                          <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" />
                        ) : item.icon ? (
                          <span className="shrink-0 opacity-85">{item.icon}</span>
                        ) : null}
                        <span className="truncate">{item.label}</span>
                      </span>
                      {item.active && <Check className="h-3.5 w-3.5 shrink-0 text-primary" />}
                    </button>
                  );
                })}
              </m.div>
            )}
          </AnimatePresence>,
          document.body,
        )}
    </div>
  );
}
