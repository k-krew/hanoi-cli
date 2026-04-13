# hanoi-cli

![Claude Assisted](https://img.shields.io/badge/Made%20with-Claude-8A2BE2?logo=anthropic)
![CI](https://github.com/k-krew/hanoi-cli/actions/workflows/release.yml/badge.svg)

hanoi-cli analyzes pod distribution across nodes, detects resource imbalance, generates safe redistribution plans, and simulates node failures - all without touching your cluster. The name is inspired by the Tower of Hanoi puzzle: controlled movement of workloads between constrained pegs.

## Example

```bash
$ hanoi-cli analyze

Cluster Imbalance Score: 42.3% -> 9.1%
Improvement: 33.2%

Nodes:
  node-1               CPU:  82.5%  MEM:  71.0%  pods: 14 [HOTSPOT]
  node-2               CPU:  35.0%  MEM:  28.0%  pods: 6
  node-3               CPU:  20.0%  MEM:  15.0%  pods: 3

Suggested Moves: 2
  1. default/api-xyz: node-1 -> node-3
  2. default/worker-abc: node-1 -> node-2
...
```

## Features

- **Imbalance detection** - CPU and memory utilization per node, standard deviation, hotspot flagging (>=80%)
- **Redistribution planning** - greedy optimizer that suggests pod moves to reduce cluster drift while respecting all scheduling constraints
- **Node failure simulation** - remove a node, see which pods can be rescheduled and which become homeless
- **Move explanations** - deep-dive into *why* a specific move was chosen, which nodes were rejected and why, and whether preferred anti-affinity is violated
- **Full constraint awareness** - nodeSelector, node affinity, pod affinity/anti-affinity (required and preferred), taints/tolerations, DaemonSets, init containers
- **Multiple output formats** - detailed text, JSON, compact summary, or a colored pseudo-GUI with dynamic-width progress bars

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap k-krew/tap
brew install hanoi-cli
```

### From source

```bash
git clone https://github.com/k-krew/hanoi-cli.git && cd hanoi-cli
go build -o hanoi-cli .
```

### Binary download

Grab the latest binary from [GitHub Releases](https://github.com/k-krew/hanoi-cli/releases) and place it in your `$PATH`.

## Quick Start

```bash
# Analyze current cluster state and suggest rebalancing moves
hanoi-cli analyze

# Focus on memory instead of CPU
hanoi-cli analyze --resource memory

# Limit to 5 suggested moves
hanoi-cli analyze --max-moves 5

# Simulate what happens if node-3 goes down
hanoi-cli simulate node-3

# Explain why move #2 was suggested
hanoi-cli analyze --explain 2
```

## Commands

### `analyze`

Scans the cluster, computes per-node utilization, and generates a redistribution plan.

```bash
hanoi-cli analyze [flags]
```


| Flag          | Default | Description                             |
| ------------- | ------- | --------------------------------------- |
| `--resource`  | `cpu`   | Resource to optimize: `cpu` or `memory` |
| `--max-moves` | `0`     | Max moves to suggest (0 = unlimited)    |


### `simulate <node-name>`

Removes a node from the cluster model, attempts to reschedule its pods onto surviving nodes, and reports feasibility.

```bash
hanoi-cli simulate node-3 [flags]
```

### Global Flags


| Flag           | Short | Default          | Description                                  |
| -------------- | ----- | ---------------- | -------------------------------------------- |
| `--kubeconfig` |       | `~/.kube/config` | Path to kubeconfig file                      |
| `--context`    |       | current context  | Kubernetes context to use                    |
| `--namespace`  | `-n`  | all namespaces   | Limit to a specific namespace                |
| `--output`     | `-o`  | `text`           | Output format: `text`, `json`, `short`, `ui`, `md` |
| `--explain`    |       |                  | Explain why move N was chosen (1-based)      |


## Output Formats

### `text` - Detailed plain text

```
Cluster Imbalance Score: 42.3% -> 9.1%
Improvement: 33.2%

Nodes:
  node-1               CPU:  82.5%  MEM:  71.0%  pods: 14 [HOTSPOT]
  node-2               CPU:  35.0%  MEM:  28.0%  pods: 6
  node-3               CPU:  20.0%  MEM:  15.0%  pods: 3

Suggested Moves: 2
  1. default/api-xyz: node-1 -> node-3
  2. default/worker-abc: node-1 -> node-2
```

### `json` - Machine-readable

```bash
hanoi-cli analyze -o json | jq .
```

```json
{
  "imbalance_score_before": 42.3,
  "imbalance_score_after": 9.1,
  "nodes": [
    {
      "name": "node-1",
      "cpu_percent": 82.5,
      "mem_percent": 71.0,
      "pod_count": 14,
      "is_hotspot": true,
      "cordoned": false
    }
  ],
  "moves": [
    {
      "pod": "api-server-xyz",
      "namespace": "default",
      "from": "node-1",
      "to": "node-3"
    }
  ]
}
```

### `short` - Compact summary

```
Score: 42.3% -> 9.1% (improvement: 33.2%)

Suggested Moves (2):
  1. default/api-server-xyz: node-1 -> node-3
  2. default/worker-abc: node-1 -> node-2
```

### `ui` - Colored pseudo-GUI with progress bars

```
  Cluster Imbalance Score:
    Before: 42.3%
    After:  9.1%
    Improvement: 33.2%

  Current State:
  ! node-1  CPU [######################....] 82.5%  MEM [###################.......] 71.0%  pods: 14
    node-2  CPU [########..................] 35.0%  MEM [######....................] 28.0%  pods: 6
    node-3  CPU [####......................] 20.0%  MEM [####......................] 15.0%  pods: 3
```

Cordoned nodes appear in dark grey with a `C` marker. Hotspot nodes appear in red with a `!` marker.

### `md` - Markdown format for CI/CD integration

Perfect for GitHub Actions, GitLab CI, and other platforms that support Markdown rendering.

```bash
hanoi-cli analyze -o md
hanoi-cli simulate node-3 -o md
```

Example output:

```markdown
### Hanoi-CLI Cluster Analysis
**Imbalance Score:** 42.3% -> 9.1% (Improvement: **33.2%**)

**Hotspots:** 1
  - node-1

#### Nodes State
| Node | CPU | Memory | Pods | Status |
|------|-----|--------|------|--------|
| node-1 | 82.5% | 71.0% | 14 | HOTSPOT |
| node-2 | 35.0% | 28.0% | 6 | OK |
| node-3 | 20.0% | 15.0% | 3 | OK |

#### Suggested Moves (2)
1. `default/api-xyz`: `node-1` -> `node-3`
2. `default/worker-abc`: `node-1` -> `node-2`

#### Projected State
| Node | CPU | Memory | Pods | Status |
|------|-----|--------|------|--------|
| node-1 | 45.0% | 42.0% | 10 | OK |
| node-2 | 52.5% | 45.0% | 9 | OK |
| node-3 | 42.5% | 38.0% | 7 | OK |
```

## Move Explanation

Use `--explain N` to understand why a specific move was recommended:

```bash
hanoi-cli analyze --explain 1
hanoi-cli simulate node-3 --explain 2
```

```
--- Explanation for move #1 ---

Pod:    default/api-xyz
Owner:  Deployment/api-server
CPU:    500m    MEM: 256Mi
Move:   node-1 -> node-3

Source node (node-1) utilization: CPU 82.5%, MEM 71.0%
Cluster score: 42.3% -> 28.7%

Candidate nodes:
  node-2               eligible CPU: 35.0% -> 47.5%  MEM: 28.0% -> 38.2%  score: 30.1%
  node-3               CHOSEN   CPU: 20.0% -> 32.5%  MEM: 15.0% -> 25.2%  score: 28.7%
  node-4               REJECTED: node is cordoned (unschedulable)
  node-5               REJECTED: taint gpu=true:NoSchedule not tolerated

Verdict: node-3 produces the lowest imbalance score (28.7%) among all eligible nodes.
```

In simulation context, the failed node is clearly marked:

```
Source node (node-3): FAILED (simulated)
```

Works with `simulate` too - `--explain 1` explains why a displaced pod was rescheduled to a particular node. The simulate explain correctly accounts for prior moves in the sequence, so move #4's explanation reflects pods already placed by moves 1-3.

When a pod has preferred anti-affinity rules, eligible nodes show whether they're violated:

```
  node-2               CHOSEN   CPU: 47.3% -> 49.8%  ...  score: 36.0%  (preferred-anti-affinity: VIOLATED weight=100)
```

This means the move is allowed (soft constraint), but the Kubernetes scheduler would normally penalize this placement.

## Constraints

hanoi-cli respects Kubernetes scheduling rules:

- **DaemonSet pods** are never moved
- **Cordoned nodes** (unschedulable) never receive pods
- **Node selectors** and **node affinity** rules are enforced
- **Pod affinity/anti-affinity** (required) is enforced; preferred rules are reported in explain
- **Taints and tolerations** are checked (including `Exists` operator edge cases)
- **Capacity limits** are respected - both CPU and memory (no overcommit on simulated reschedules)
- **Init containers** are accounted for using `max(sum(containers), max(initContainers))`

## Environment Variables


| Variable           | Overrides                                      |
| ------------------ | ---------------------------------------------- |
| `HANOI_KUBECONFIG` | `--kubeconfig` default (highest priority)      |
| `KUBECONFIG`       | `--kubeconfig` default (standard k8s variable) |
| `HANOI_CONTEXT`    | `--context` default                            |


Kubeconfig resolution order: `HANOI_KUBECONFIG` > `KUBECONFIG` > `~/.kube/config`

## Contributing

Contributions are welcome! Feel free to open issues and pull requests. Whether it's a bug fix, new feature, documentation improvement, or just a question - all input is appreciated.

## License

Apache License 2.0 - see [LICENSE](LICENSE).
