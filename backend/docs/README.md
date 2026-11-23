# DaGrouper School backend documentation

This service backs the DaGrouper School analysis dashboard. It holds people, sessions, classrooms and everything specific to DaGrouper School, and is SAM's first tenant, relying on SAM for access control and observation data.

So far the service runs with its database and migrations, calls SAM as itself, as a person or as anonymous, serves `/healthz` and shuts down cleanly, and its integration tests run it against a SAM built from source, holding every response to the API contract. The design documents describe the whole service to be built; every other page describes only what exists.

## Design

| Page | Read it to |
|---|---|
| [Overview](design/README.md) | understand the product, its users, the boundary with SAM, the architecture and the open questions |
| [SAM integration](design/sam-integration.md) | see how SAM is kept in step and whose authority each call carries |
| [Accounts and sign-in](design/accounts-and-sign-in.md) | see how people sign in and which conveniences exist only in development |
| [Classrooms and permissions](design/classrooms-and-permissions.md) | see the roles, the permission grid, invitations and consent |
| [Labs and readings](design/labs-and-readings.md) | see how labs, devices and readings reach SAM |
| [Dashboards and public site](design/dashboards-and-public-site.md) | see how pages are computed, and how data is shared and released |
| [Registry and content](design/registry-and-content.md) | see how the registry and operator configuration work |
| [Security and privacy](design/security-and-privacy.md) | see the threat model and the data-protection points |

## Working on the service

| Page | Read it to |
|---|---|
| [Development](development.md) | build, run, configure and test the service; learn the conventions its code follows |
| [Operations](operations.md) | run the deployed service: its signing keys and the tenant file |
| [Decisions](decisions/README.md) | record a decision taken while building |
| [API (Swagger UI)](index.html) | browse the contract served to the front end |
