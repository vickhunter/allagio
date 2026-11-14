// The Allagio mark is the approved icon A ("rails merge") checked in under
// public/brand and public/favicon.svg. Those files are the export Victor
// approved on 14 Nov 2026. This script used to draw the old rail (two lines,
// four ties, one dot) and would overwrite that export. It now refuses.
console.error('public/brand is the approved icon A export. Do not regenerate the old rail mark.')
process.exit(1)
