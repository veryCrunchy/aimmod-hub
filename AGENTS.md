# Repository Privacy

- Never commit or push local design reviews, QA reports, screenshots, browser captures, traces, recordings, scratch data, or personal replay files.
- Keep local verification output in ignored directories. Check both tracked files and the staged diff before committing; ignore rules do not protect files already tracked.
- Do not put personal account names, user IDs, machine-specific paths, credentials, tokens, cookies, or private URLs in source, documentation, fixtures, logs, or image assets.
- Use clearly synthetic test identities and reserved example domains. Preserve legitimate synthetic regression tests, including path-validation cases.
- Live tests must require explicitly supplied test identities through environment variables. Do not embed real account or score IDs, or print identifying response data.
- Use environment variables or platform directory APIs for local paths. Public project repository URLs and required package namespaces are not personal test data and must remain intact.
- Inspect screenshots for personal information before considering any product asset for publication. Do not substitute decorative mockups for real product evidence.
- If personal data was previously committed, removing it from the current tree does not remove it from history. Report that separately; do not rewrite shared history without explicit coordination.
