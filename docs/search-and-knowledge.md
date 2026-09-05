# Search and Knowledge Publishing

## Content

`api/internal/seo/content.json` is shared by the Go server and the web application.
It holds public route titles/descriptions and the osu! knowledge-base articles.
Keep guide slugs stable; they are public URLs under `/osu/learn/`.

Each article needs a distinct title, description, useful sections and links to
primary sources. Separate documented gameplay rules from suggested review or
practice exercises. Recheck claims when osu! scoring or performance calculations
change. Update the content date when the published content changes, not on every
build. The existing KovaaK's knowledge base continues using its coaching data.

## Delivery

The web build prerenders both knowledge bases into HTML. The Go server serves
those files directly, so article content and metadata are available without
JavaScript. Other pages receive server-generated metadata; client navigation
updates the document head. Public static routes and knowledge articles are
included in `/sitemap.xml`, advertised by `/robots.txt`.

Search/filter parameters are not separate canonical pages. Account, admin,
device-link and search-result pages are non-indexable. Unlisted replay access
does not make a replay public: identifying social metadata is emitted only for
public shares. No private or unlisted replay URLs are enumerated in the sitemap.
Indexing directives are not an authorization mechanism; API access checks remain
responsible for protecting data.

## Verification

- Run `pnpm --dir web test`, `pnpm --dir web typecheck` and `pnpm --dir web build`.
- Run `go test ./...` from `api`.
- Inspect article HTML for one title, canonical, description and article schema.
- Check direct page loads and client navigation at desktop and mobile sizes.
- After deployment, verify `/sitemap.xml`, `/robots.txt` and a guide's HTML on the
  production host. Submit the sitemap in the site's Google Search Console.

Deployment and search-engine indexing are separate steps. A successful build or
push does not prove that a page is live or indexed.

## References

- [Google: robots metadata](https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag)
- [Google: article structured data](https://developers.google.com/search/docs/appearance/structured-data/article)
