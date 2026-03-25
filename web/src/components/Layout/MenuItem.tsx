import { ChevronDown, ChevronRight } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface MenuItemData {
  id: string
  label: string
  icon: LucideIcon
  children?: MenuItemData[]
}

interface MenuItemProps {
  item: MenuItemData
  activeId: string
  expandedIds: Set<string>
  onSelect: (id: string) => void
  onToggleExpand: (id: string) => void
  level?: number
}

export function MenuItem({
  item,
  activeId,
  expandedIds,
  onSelect,
  onToggleExpand,
  level = 0,
}: MenuItemProps) {
  const { id, label, icon: Icon, children } = item
  const isActive = activeId === id
  const hasChildren = children && children.length > 0
  const isExpanded = expandedIds.has(id)

  const handleClick = () => {
    if (hasChildren) {
      onToggleExpand(id)
    } else {
      onSelect(id)
    }
  }

  return (
    <div>
      <button
        onClick={handleClick}
        style={isActive && !hasChildren ? { backgroundColor: 'rgba(139, 92, 246, 0.3)' } : undefined}
        className={cn(
          'w-full flex items-center gap-3 px-3 py-2.5 text-sm rounded-lg transition-colors relative',
          'hover:bg-white/10',
          isActive && !hasChildren
            ? 'text-white font-medium border-l-2 border-violet-400 -ml-[2px] pl-[calc(0.75rem+2px)]'
            : 'text-sidebar-foreground/80',
          level > 0 && 'ml-4 text-xs py-2'
        )}
      >
        <Icon className={cn('w-4 h-4 shrink-0', level > 0 && 'w-3.5 h-3.5')} />
        <span className="flex-1 text-left truncate">{label}</span>
        {hasChildren && (
          <span className="text-sidebar-foreground/50">
            {isExpanded ? (
              <ChevronDown className="w-4 h-4" />
            ) : (
              <ChevronRight className="w-4 h-4" />
            )}
          </span>
        )}
      </button>

      {hasChildren && isExpanded && (
        <div className="mt-0.5 space-y-0.5">
          {children!.map((child) => (
            <MenuItem
              key={child.id}
              item={child}
              activeId={activeId}
              expandedIds={expandedIds}
              onSelect={onSelect}
              onToggleExpand={onToggleExpand}
              level={level + 1}
            />
          ))}
        </div>
      )}
    </div>
  )
}