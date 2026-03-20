import type { ReactNode } from 'react'
import { Sidebar } from './Sidebar'

interface LayoutProps {
  children: ReactNode
  activeId: string
  onSelectPage: (id: string) => void
}

export function Layout({ children, activeId, onSelectPage }: LayoutProps) {
  return (
    <div className="flex min-h-screen bg-background">
      <Sidebar activeId={activeId} onSelect={onSelectPage} />
      <main className="flex-1 overflow-auto">
        {children}
      </main>
    </div>
  )
}