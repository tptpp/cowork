import { useState } from 'react'
import {
  LayoutDashboard,
  Settings,
  Bot,
  ChevronLeft,
  ChevronRight,
  Zap,
  Palette,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { MenuItem, type MenuItemData } from './MenuItem'

const menuItems: MenuItemData[] = [
  {
    id: 'dashboard',
    label: 'Dashboard',
    icon: LayoutDashboard,
  },
  {
    id: 'settings',
    label: 'Settings',
    icon: Settings,
    children: [
      {
        id: 'settings-model',
        label: 'Model Settings',
        icon: Bot,
      },
      {
        id: 'settings-appearance',
        label: 'Appearance',
        icon: Palette,
      },
    ],
  },
]

interface SidebarProps {
  activeId: string
  onSelect: (id: string) => void
}

export function Sidebar({ activeId, onSelect }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false)
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set(['settings']))

  const handleToggleExpand = (id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const handleSelect = (id: string) => {
    onSelect(id)
  }

  return (
    <aside
      className={cn(
        'h-screen sticky top-0 flex flex-col bg-sidebar border-r border-sidebar-border transition-all duration-300',
        collapsed ? 'w-16' : 'w-56'
      )}
    >
      {/* Header */}
      <div className="h-16 flex items-center justify-between px-4 border-b border-sidebar-border">
        {!collapsed && (
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary to-primary/60 flex items-center justify-center">
              <Zap className="w-4 h-4 text-primary-foreground" />
            </div>
            <span className="font-bold text-sidebar-foreground">Cowork</span>
          </div>
        )}
        <button
          onClick={() => setCollapsed(!collapsed)}
          className={cn(
            'p-1.5 rounded-md text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent transition-colors',
            collapsed && 'mx-auto'
          )}
        >
          {collapsed ? (
            <ChevronRight className="w-5 h-5" />
          ) : (
            <ChevronLeft className="w-5 h-5" />
          )}
        </button>
      </div>

      {/* Menu */}
      <nav className="flex-1 overflow-y-auto p-3">
        <div className="space-y-1">
          {menuItems.map((item) => (
            <MenuItem
              key={item.id}
              item={item}
              activeId={activeId}
              expandedIds={expandedIds}
              onSelect={handleSelect}
              onToggleExpand={handleToggleExpand}
            />
          ))}
        </div>
      </nav>

      {/* Footer */}
      {!collapsed && (
        <div className="p-3 border-t border-sidebar-border">
          <div className="text-xs text-sidebar-foreground/50 text-center">
            Distributed Task Processing
          </div>
        </div>
      )}
    </aside>
  )
}