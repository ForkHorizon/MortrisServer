// houseart.mortris.forkhorizon.com is a second door onto the same app,
// dedicated to the Puzzle playtest. It is one binary, one port, one
// bundle — the hostname only changes where you land and what the chrome
// offers.
//
// This is presentation, never authorization. Nothing here grants access:
// sessions, CSRF and per-project checks all run server-side and are
// identical on both hostnames. Someone without access to the pinned
// project is refused here exactly as they are anywhere else, and someone
// who edits this value in their browser gains nothing.
export const HOUSE_ART_PROJECT = 'puzzle_gravity_test'

export function isHouseArtHost(): boolean {
  return window.location.hostname.startsWith('houseart.')
}
