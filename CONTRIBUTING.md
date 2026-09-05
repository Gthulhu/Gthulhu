# Contributing to Gthulhu

Thanks for your interest in Gthulhu. Contributions of code, documentation, testing, bug reports, use cases, and design feedback are welcome.

Gthulhu sits at the intersection of Kubernetes workload orchestration, eBPF observability, autoscaling, and Linux `sched_ext`. You do not need to understand every layer of the stack before contributing.

## Start here

1. Read the project overview in [README.md](README.md).
2. Check the [open issues](https://github.com/Gthulhu/Gthulhu/issues) for work that matches your interests.
3. If you are new to the project, look for issues labeled `good first issue` or `help wanted`.
4. For a larger change, open an issue first so we can align on the problem and approach before implementation.

The full contributor documentation is also available at [gthulhu.org/contributing](https://gthulhu.org/contributing/).

## Areas where you can contribute

- **Kubernetes integration** — controllers, CRDs, scheduling intents, Helm packaging, and deployment experience.
- **eBPF observability** — scheduling metrics, tracing, portability, and performance analysis.
- **Linux `sched_ext`** — custom CPU scheduling policies, scheduler experiments, and kernel compatibility.
- **Autoscaling** — KEDA integration and workload-aware scaling strategies.
- **Testing and portability** — validation across supported Linux kernel versions and Kubernetes environments.
- **Documentation and examples** — tutorials, architecture explanations, demos, and real-world use cases.

## Development workflow

Fork the repository, create a focused branch, make your changes, and open a pull request against `main`.

Before submitting a PR:

- keep the change focused on one problem;
- add or update tests when behavior changes;
- update documentation when user-facing behavior changes;
- run the relevant checks described in the README/Makefile;
- explain both **what** changed and **why** in the PR description.

## Issues

A useful bug report should include:

- Gthulhu version or commit;
- Kubernetes version;
- Linux kernel version, especially for `sched_ext`-related issues;
- deployment method;
- steps to reproduce;
- expected and actual behavior;
- relevant logs or metrics with secrets removed.

For feature requests, describe the workload or operational problem first. Concrete use cases help maintainers evaluate whether a proposed feature belongs in Gthulhu.

## Pull requests

Small, reviewable PRs are preferred. If your change introduces a new scheduling policy, API, architecture dependency, or significant behavior change, link the design discussion or issue that motivated it.

Please avoid drive-by changes that only reformat unrelated code or documentation.

## Community expectations

Participation in the Gthulhu community is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

For security vulnerabilities, do **not** open a public issue. Follow [SECURITY.md](SECURITY.md) instead.

## Questions

If you are unsure where to start, open a GitHub issue or join the community channels listed in the README. Tell us what area you are interested in and what environment you can test on; maintainers can help identify a useful first contribution.
