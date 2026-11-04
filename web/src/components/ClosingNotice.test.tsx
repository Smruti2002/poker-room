import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ClosingNotice } from './ClosingNotice.tsx'

describe('closing notice', () => {
  it('stays quiet until the last hour, then gives the closing time', () => {
    const { rerender } = render(<ClosingNotice closesAt={Date.now() + 2 * 3600_000} clockOffset={0} />)
    expect(screen.queryByRole('status')).toBeNull()

    const closes = Date.now() + 30 * 60_000
    rerender(<ClosingNotice closesAt={closes} clockOffset={0} />)
    const at = new Date(closes).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    expect(screen.getByRole('status')).toHaveTextContent(`This room closes at ${at}`)
  })

  it("judges the last hour by the server's clock", () => {
    // The server's clock is an hour ahead of this phone's. By the server,
    // the room closes in 30 minutes, though that's 90 minutes away by the
    // phone's clock.
    const offset = 3600_000
    const closes = Date.now() + offset + 30 * 60_000
    render(<ClosingNotice closesAt={closes} clockOffset={offset} />)
    // Shown, at the phone's local time for that moment.
    const at = new Date(closes - offset).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    expect(screen.getByRole('status')).toHaveTextContent(`closes at ${at}`)
  })

  it('shows nothing for rooms that never close', () => {
    render(<ClosingNotice clockOffset={0} />)
    expect(screen.queryByRole('status')).toBeNull()
  })
})
