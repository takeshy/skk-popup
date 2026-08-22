//go:build bindings

package main

// bindingsMode is set when Wails' CLI builds the project with `-tags
// bindings` to generate frontend bindings by running this binary as a
// subprocess. In that mode wails.Run only prints the bindings JSON and
// exits, so the daemon-specific guards must stay out of its way.
const bindingsMode = true
