# Security Policy

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Contact the repository owner through GitHub with the affected endpoint, a minimal reproduction, and the impact. Remove credentials and personal data from reports.

The development JWT secret and administrator password documented in `README.md` are for local use only. Deployments must set `BLOG_JWT_SECRET` and `BLOG_ADMIN_PASSWORD` to unique values before exposing the service.

## Supported versions

Only the latest commit on `main` receives security fixes while the project is under active development.
