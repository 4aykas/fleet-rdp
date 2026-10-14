build.ps1 copies dist/fleet-rdp-launcher.exe here before it builds the maker,
and the maker embeds it (go:embed). The copy is git-ignored. Without it the
maker still builds, and its window says the template is missing.
