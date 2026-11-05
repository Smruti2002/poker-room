Closes #

## What changes for players
<!-- What someone at the table will notice. Screenshots at phone width help for UI changes. -->

## How it works
<!-- The approach, and anything a reviewer should look at closely. -->

## Tests
<!-- What you added or ran: Go tests, Vitest, Playwright (@phone). -->

## Checklist
- [ ] `make ci` and `make e2e` pass locally
- [ ] Checked at 375px wide (no sideways scrolling), if the UI changed
- [ ] No gameplay timers added
- [ ] No hidden cards, splits or tokens in another player's view
- [ ] `docs/rules.md` updated, if how the game plays changed
- [ ] `make protocol` run and `types.ts` committed, if the protocol changed
- [ ] Snapshot save/restore and a round-trip test, if room state that must survive a restart changed
