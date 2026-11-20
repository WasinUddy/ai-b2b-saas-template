import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { routes } from './routes'

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  render(<RouterProvider router={router} />)
  return router
}

describe('routes', () => {
  it('renders the home page at /', () => {
    renderAt('/')
    expect(screen.getByRole('heading', { name: 'AI B2B SaaS' })).toBeInTheDocument()
  })

  it('renders the not-found page for an unknown path', () => {
    renderAt('/does-not-exist')
    expect(screen.getByRole('heading', { name: 'Page not found' })).toBeInTheDocument()
  })

  it('navigates from the not-found page back home without a reload', async () => {
    const router = renderAt('/missing')
    await userEvent.click(screen.getByRole('link', { name: 'Back to home' }))
    expect(router.state.location.pathname).toBe('/')
    expect(screen.getByRole('heading', { name: 'AI B2B SaaS' })).toBeInTheDocument()
  })

  it('marks the Home link as current on /', () => {
    renderAt('/')
    expect(screen.getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page')
  })
})
