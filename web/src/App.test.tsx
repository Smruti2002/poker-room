import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App.tsx'

describe('App routing', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    window.history.pushState(null, '', '/')
  })

  it('shows the create-room page at /', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Poker Room' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create room' })).toBeDisabled()
  })

  it('shows a join form for a room link without a saved session', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{"code":"ABCDEF"}', { status: 200 })))
    window.history.pushState(null, '', '/r/abcdef')
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Join room ABCDEF' })).toBeInTheDocument()
  })

  it('says when a room does not exist', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 404 })))
    window.history.pushState(null, '', '/r/NOPE42')
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Room not found' })).toBeInTheDocument()
  })
})
