import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import './index.css'
import App, { appRoutes } from './App.tsx'
import { ThemeProvider } from 'next-themes'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Toaster as LegacyToaster } from '@/components/ui/toaster'
import { Toaster as SonnerToaster } from '@/components/ui/sonner'

// Data router (required by useBlocker in the add-connection flow).
const router = createBrowserRouter(
  [{ element: <App />, children: appRoutes }],
  { basename: '/dashboard' },
)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem disableTransitionOnChange>
      <TooltipProvider>
        <RouterProvider router={router} />
        <LegacyToaster />
        <SonnerToaster richColors position="top-right" />
      </TooltipProvider>
    </ThemeProvider>
  </StrictMode>,
)
