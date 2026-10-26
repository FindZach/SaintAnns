# Photo slots

Drop a real parish photo into this folder named `<slot>.jpg` (or `.jpeg`, `.png`, `.webp`) and templates that call `{{photo "<slot>"}}` will pick it up automatically.

Current slots the templates look for:

- `st-anns` — St. Ann's Church, Hornell exterior
- `st-marys` — St. Mary's Church, Rexville exterior
- `interior-hornell` — nave or sanctuary interior
- `interior-rexville` — nave or sanctuary interior
- `pastor` — parish pastor headshot

Any slot without a file falls back to blank (`{{photo}}` returns `""`), so pages render without errors even before photos are added.
