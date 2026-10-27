# Swagger UI assets

Vendored from **swagger-ui-dist 5.34.0**, licensed under Apache 2.0.
The original license and bundled third-party license notices are included here.

`manifest.json` pins the npm archive URL and SHA-512 integrity, plus SHA-256 hashes
of the vendored files. The archive integrity was verified before extraction.
Only the browser bundle and stylesheet are shipped; source-map comments were
removed. `init.js` is our local configuration, not a vendor file.

Go embeds only runtime assets and license notices. This README and manifest stay
in source. Building or running requires no npm, CDN, or external service. To update: download a specific swagger-ui-dist archive from the official
npm registry, verify its integrity, replace these assets/licenses, and update the
manifest. Then run Go checks and test Try it out in the browser.
