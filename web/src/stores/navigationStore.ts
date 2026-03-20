import { create } from 'zustand'

export type PageId = 'dashboard' | 'settings-model' | 'settings-appearance'

interface NavigationState {
  activePage: PageId
  setActivePage: (page: PageId) => void
}

export const useNavigationStore = create<NavigationState>((set) => ({
  activePage: 'dashboard',
  setActivePage: (page) => set({ activePage: page }),
}))