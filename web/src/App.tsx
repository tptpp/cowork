import { Layout } from '@/components/Layout'
import { DashboardContent } from '@/components/DashboardContent'
import { ModelSettingsPage, AppearanceSettings } from '@/components/Settings'
import { useNavigationStore, type PageId } from '@/stores/navigationStore'

function App() {
  const { activePage, setActivePage } = useNavigationStore()

  // Map page ID to sidebar active ID
  const getActiveId = (): string => {
    if (activePage === 'dashboard') return 'dashboard'
    if (activePage === 'settings-model') return 'settings-model'
    if (activePage === 'settings-appearance') return 'settings-appearance'
    return 'dashboard'
  }

  // Map sidebar selection to page ID
  const handleSelectPage = (id: string) => {
    const pageMap: Record<string, PageId> = {
      'dashboard': 'dashboard',
      'settings-model': 'settings-model',
      'settings-appearance': 'settings-appearance',
    }
    if (pageMap[id]) {
      setActivePage(pageMap[id])
    }
  }

  const renderContent = () => {
    switch (activePage) {
      case 'dashboard':
        return <DashboardContent />
      case 'settings-model':
        return <ModelSettingsPage />
      case 'settings-appearance':
        return (
          <div className="p-6 max-w-2xl mx-auto">
            <h1 className="text-xl font-semibold mb-6">Appearance Settings</h1>
            <AppearanceSettings />
          </div>
        )
      default:
        return <DashboardContent />
    }
  }

  return (
    <Layout activeId={getActiveId()} onSelectPage={handleSelectPage}>
      {renderContent()}
    </Layout>
  )
}

export default App