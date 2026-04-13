# ksmem

Orientation commands:

```bash
ksmem show context --domain shared
ksmem show surface
ksmem list stones --domain shared
ksmem show stone <id>
```

Prefer ksmem CLI for memory operations. Use stdin for prose:

```bash
echo "..." | ksmem note stone <id>
echo "..." | ksmem add stone --domain shared --title "..."
```

Delegation posture:

- Specs are intent plus constraints, not literal scripts
- Keep hard invariants and acceptance criteria intact
- Improve local implementation choices when justified
- Explain deviations and land code, tests, and docs together
