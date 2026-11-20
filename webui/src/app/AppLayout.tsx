import { NavLink, Outlet } from 'react-router'

export function AppLayout() {
  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 antialiased">
      <header className="border-b border-slate-200 bg-white">
        <nav aria-label="Main" className="mx-auto flex max-w-5xl items-center gap-6 px-6 py-4">
          <NavLink
            to="/"
            end
            className={({ isActive }) =>
              isActive
                ? 'text-sm font-semibold text-slate-900'
                : 'text-sm font-medium text-slate-600 hover:text-slate-900'
            }
          >
            Home
          </NavLink>
        </nav>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-10">
        <Outlet />
      </main>
    </div>
  )
}
