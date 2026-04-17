# gqksyd Machine-Local Adapter Overlay Contract

Goal: review the smallest truthful runtime contract for machine-local adapter
repo paths in `kstoolchain`.

Checklist:
- Treat tracked adapter metadata as shared repo truth.
- Treat `repo_id -> local repo_path` as machine-local runtime state.
- Keep the review narrow to manifest loading, runtime precedence, `init`
  discovery and verification flow, and setup-blocked sync behavior.
- Prefer one canonical first-run path over multiple half-implemented override
  stories.
- Include enough operator evidence for a blind reviewer to understand why the
  current embedded-path model is false.
