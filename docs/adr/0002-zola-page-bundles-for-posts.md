# Use Zola page bundles for generated posts

Converted Posts are written as `content/{slug}/index.md` rather than
`content/{slug}.md`. This keeps the Ghost Slug as the page URL while allowing
later image collection to colocate downloaded assets beside each Post. The first
implementation may leave image URLs untouched, but the output layout is chosen
now to avoid a migration when image downloading is added.
