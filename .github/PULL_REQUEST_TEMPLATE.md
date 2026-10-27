<!-- The PR title must follow Conventional Commits 1.0.0, e.g. `feat(api): add RSS limit validation`. -->

## Summary

<!-- What changes and why. Link the design or spec section if there is one. -->

## References

<!-- Closes #123 / Refs: #123 -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor or performance (include the measurement)
- [ ] Documentation
- [ ] Build, CI or dependencies

## Checks

- [ ] `cd api && make check` passes (if `api/` changed)
- [ ] `cd web && pnpm lint && pnpm typecheck && pnpm format:check && pnpm build` passes (if `web/` changed)
- [ ] `mkdocs build --strict` passes (if `docs/` changed)
- [ ] Tests cover the change, or I explain below why they do not
- [ ] Swagger (`make swag`) and `web/src/types/api.ts` (`pnpm gen:api`) are regenerated if the API contract changed
- [ ] Idea dedup path and its tests are untouched or still green
- [ ] No secrets, `.env` files or raw third-party content were added
- [ ] I did not edit `CHANGELOG.md` by hand

## Notes for reviewers

<!-- Trade-offs, follow-ups, screenshots of UI changes. -->
