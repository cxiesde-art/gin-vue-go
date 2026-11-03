# Contributing

Thanks for helping improve Gin Vue Blog Go. Contributions should make the API safer, easier to operate, or more useful to a Vue client.

## Before opening a pull request

1. Create a focused branch from `main`.
2. Run `make fmt`, `make vet`, and `make coverage`.
3. Add a regression test for behavior you change.
4. Update `README.md` when an endpoint, environment variable, or operator workflow changes.

Keep commits small and describe the user-visible or operational effect in English. Pull requests should explain the problem, the chosen approach, and the verification performed. Avoid committing databases, credentials, generated binaries, or coverage artifacts.

## Review expectations

Every change needs one approving review and passing CI. Changes to authentication, authorization, migrations, or public API response shapes need an explicit compatibility note. Maintainers may request a follow-up issue when a change is useful but outside the current scope.
