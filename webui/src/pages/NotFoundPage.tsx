import { Link } from 'react-router'

export function NotFoundPage() {
  return (
    <section className="space-y-3">
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Page not found</h1>
      <p className="text-slate-600">The page you are looking for does not exist.</p>
      <Link
        to="/"
        className="inline-block text-sm font-medium text-slate-900 underline underline-offset-4 hover:text-slate-700"
      >
        Back to home
      </Link>
    </section>
  )
}
