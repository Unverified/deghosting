# Deghosting

The domain of converting a Ghost blog export into source files for a static
site generator. This glossary fixes the language; it is not a spec and holds no
implementation detail.

## Language

**Ghost export**:
A single JSON file produced by Ghost's Settings → Advanced → Import/Export,
containing all posts, pages, tags, and settings.
_Avoid_: dump, backup
See: [Exporting content and data](https://ghost.org/help/exports/)

**Post**:
A single blog entry — the content-bearing resource we convert. Carries a
rendered HTML body plus metadata (title, slug, excerpt, tags, dates).
_Avoid_: article, entry, page (a Ghost "page" is a distinct resource we ignore)
See: [Ghost posts](https://docs.ghost.org/admin-api/posts/overview)

**Status**:
Whether a Post is `published` (live on the site) or `draft`. Only published
Posts are converted.
_Avoid_: state

**Slug**:
A Post's URL path. It is also the directory name for the generated Markdown file
and any colocated assets for that Post.
_Avoid_: permalink, path, id, filename

**Tag**:
A label a Post is filed under. A Tag is reused across many Posts.
_Avoid_: category, label, topic

**Tag name**:
The human-facing Tag text that becomes a generated taxonomy value.
_Avoid_: tag slug, tag ID

**Author**:
A Ghost staff user credited on a Post. We collect the Author's display name and
email from the Ghost export's `users` table.
_Avoid_: user (unless discussing the raw export table), writer

**Excerpt**:
A Post's short hand-written summary (Ghost's `custom_excerpt`). Becomes the
`description` in generated front matter, falling back to the Post's SEO meta
description when absent.
_Avoid_: summary, description (in the output, "description" is the front-matter field)

**Feature image**:
A Post's hero image.
_Avoid_: cover, banner, thumbnail

**Read time**:
An estimated number of minutes needed to read a Post.
_Avoid_: reading time, time to read

**Front matter**:
The TOML metadata block at the top of a generated Markdown file (title, date,
description, tags), consumed by the static site generator.
_Avoid_: header, preamble, metadata

**Markdown body**:
The CommonMark-compatible content after the front matter in a generated Markdown
file, converted from a Post's rendered HTML.
_Avoid_: content, body (without "Markdown")

**Post bundle**:
The generated directory named from a Post's Slug, containing that Post's
`index.md` file and any colocated assets.
_Avoid_: folder, page bundle (unless referring specifically to Zola)

**Markdown content tree**:
The directory of Markdown files generated from a Ghost export, one file per
converted Post, for a static site generator to build.
_Avoid_: output, content folder

**Static site generator**:
Software that builds a static website from source content files.
[Zola](https://www.getzola.org/) is deghosting's target generator.
_Avoid_: SSG, site builder

## Relationships

- A **Ghost export** contains many **Posts** and many **Tags**.
- A **Post** has zero or more **Tags** and one or more **Authors**
  (many-to-many). In the export these links live in separate join tables, even
  though Ghost's API nests related resources inside the Post.
- A **Post**'s **Slug** becomes the directory name for its generated
  **Post bundle**.
- A **Tag name** becomes the generated taxonomy value; a Tag's slug remains a
  source identifier.
- Each **Markdown** file has **Front matter** derived from a **Post**'s metadata,
  plus a **Markdown body** converted from the Post's HTML.
- Only **Published** **Posts** enter the **Markdown content tree**; **Drafts** are
  skipped.

## Flagged ambiguities

- "description" was overloaded. Resolved: the *source* is the **Excerpt**
  (`custom_excerpt`, with the SEO meta description as fallback); the *output* is
  the front-matter `description` field. They are not the same thing.
- "page" is reserved for Ghost's page resource (which we ignore) and must not be
  used as a synonym for **Post**.

## References

- [Exporting content and data](https://ghost.org/help/exports/) — how the input
  JSON is produced and what it contains.
- [Ghost posts (Admin API)](https://docs.ghost.org/admin-api/posts/overview) —
  field semantics for the Post resource. Note the export's on-disk shape
  (normalized join tables) differs from the API's nested representation.
