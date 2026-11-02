import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App.tsx'

describe('App', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows the server as online when /healthz responds', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 200 })))
    render(<App />)

    expect(screen.getByRole('heading', { name: 'Poker Room' })).toBeInTheDocument()
    expect(await screen.findByText('Server: online')).toBeInTheDocument()
  })

  it('shows the server as offline when the request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')))
    render(<App />)

    expect(await screen.findByText('Server: offline')).toBeInTheDocument()
  })
})
