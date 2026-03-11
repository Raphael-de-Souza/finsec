# Scores API

Public REST API for financial security scores, deployed on AWS with ECS Fargate + Terraform.

---

## Architecture

```
Internet
   │
   ▼ HTTPS (443)
┌──────────────────────────────────────────────┐
│  Application Load Balancer (public subnets)  │  ← TLS termination, HTTP→HTTPS redirect
└──────────────────────┬───────────────────────┘
                       │ HTTP (8080)
          ┌────────────▼────────────┐
          │  ECS Fargate Service    │  ← 2+ tasks, awsvpc networking
          │  (private subnets)      │
          │  ┌──────────────────┐   │
          │  │  Go API container│   │
          │  └──────────────────┘   │
          └────────────┬────────────┘
                       │
          ┌────────────▼────────────┐
          │  NAT Gateways           │  ← outbound only, one per AZ
          └─────────────────────────┘

Supporting services:
  ECR  → private container registry
  ACM  → managed TLS certificate
  R53  → DNS alias record → ALB
  CW   → logs (/ecs/scores-api-prod), Container Insights
```

---

## Repository layout

```
.
├── app/                          # Go application
│   ├── cmd/api/main.go           # Entry point, HTTP server, graceful shutdown
│   ├── internal/
│   │   ├── handler/securities.go # GET /securities, GET /securities/{id}/scores
│   │   ├── middleware/           # RequestID, structured logger, panic recover
│   │   └── model/model.go        # Domain types + rating helper
│   └── Dockerfile                # Multi-stage build → scratch image
│
├── terraform/
│   ├── modules/
│   │   ├── vpc/    # VPC, 2×public + 2×private subnets, NAT GWs, route tables
│   │   ├── ecr/    # Private registry, image scanning, lifecycle policy
│   │   ├── alb/    # Public ALB, HTTPS listener, HTTP redirect, target group
│   │   └── ecs/    # Fargate cluster, task def, service, auto-scaling, IAM
│   └── environments/
│       └── prod/   # Root module wiring all modules + ACM + Route53
│
└── .github/workflows/deploy.yml  # CI/CD pipeline
```

---

## Go application

### Design decisions

| Decision | Rationale |
|---|---|
| Standard library only (`net/http`) | Go 1.26 added `{path_param}` |
| `log/slog` (structured JSON logs) | Native since Go 1.21, CloudWatch-friendly |
| Multi-stage Docker build → `scratch` | Final image < 10 MB, zero attack surface |
| Graceful shutdown (30 s) | ECS sends `SIGTERM` before forceful stop |
| Middleware chain pattern | Composable, testable, decoupled from handlers |

### Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/securities` | Returns list of all security objects |
| GET | `/securities/{id}/scores` | Returns score for one security; 404 if unknown |
| GET | `/health` | ALB health check; always returns 200 |

### Example responses

```jsonc
// GET /securities
[
  {"id":"AAPL","name":"Apple Inc."},
  {"id":"MSFT","name":"Microsoft Corporation"}
]

// GET /securities/AAPL/scores
{
  "security_id": "AAPL",
  "score": 92.4,
  "rating": "AAA",
  "computed_at": "2024-03-01T12:00:00Z",
  "valid_until": "2024-03-02T12:00:00Z",
  "methodology": "v2-weighted-composite"
}

// GET /securities/UNKNOWN/scores → 404
{"code":404,"message":"security not found"}
```

---

## Terraform infrastructure

### Design decisions

| Decision | Rationale |
|---|---|
| Module-per-component | Reusable, independently testable, follows HashiCorp style |
| Fargate (serverless containers) | No EC2 nodes to patch; scales to zero cost on idle |
| Private subnets for tasks | Tasks never directly exposed; only ALB is public |
| 2 AZs throughout | High availability; single AZ loss has zero downtime |
| `IMMUTABLE` ECR tags | Prevents tag overwrites; image digests are stable |
| Auto Scaling on CPU 70% | Horizontal scaling before saturation |
| S3 + DynamoDB remote state | Shared state with locking for team use |
| `lifecycle { ignore_changes = [task_definition] }` | Task definition is managed by CI/CD, not Terraform |

---

## CI/CD pipeline (GitHub Actions)

```
PR opened         → go test + terraform plan (preview only)
Push to main      → go test → docker build+push (SHA tag) → terraform apply → ECS deploy
```

- Uses **OIDC** (no long-lived AWS keys stored in GitHub)
- Docker layer cache via GitHub Actions cache
- `deployment_circuit_breaker` on the ECS service auto-rolls back on unhealthy deploys

---

## First-time setup

### 1. Terraform backend

Create an S3 bucket and DynamoDB table, then fill in `backend {}` in `environments/prod/main.tf`:

```bash
aws s3api create-bucket \
    --bucket terraform-state-154845614889 \
    --region eu-central-1 \
    --create-bucket-configuration LocationConstraint=eu-central-1
aws s3api put-bucket-encryption \
    --bucket terraform-state-154845614889 \
    --server-side-encryption-configuration '{
        "Rules": [
            {
                "ApplyServerSideEncryptionByDefault": {
                    "SSEAlgorithm": "AES256"
                }
            }
        ]
    }'
aws s3api put-bucket-versioning \
    --bucket terraform-state-154845614889 \
    --versioning-configuration Status=Enabled
aws s3api put-bucket-lifecycle-configuration \
    --bucket terraform-state-154845614889 \
    --lifecycle-configuration '{
        "Rules": [
            {
                "ID": "ManterUltimas5Versoes",
                "Status": "Enabled",
                "Filter": {"Prefix": ""},
                "NoncurrentVersionExpiration": {
                    "NoncurrentDays": 1,
                    "NewerNoncurrentVersions": 5
                }
            }
        ]
    }'
```

### 2. Deploy infrastructure

```bash
cd terraform/environments/prod

terraform init
terraform apply
```

### 3. Build app

```bash
cd app
go mod init github.com/finsec/scores-api
go mod tidy
```


### 4. Build and push first image

```bash
cd app
aws ecr get-login-password | docker login --username AWS --password-stdin <ECR_URL>
docker build -t <ECR_URL>:latest .
docker push <ECR_URL>:latest
```

### 5. Validate

```bash
curl https://api.adenir.com/securities
curl https://api.adenir.com/securities/AAPL/scores
curl https://api.adenir.com/securities/{security_id}/scores
curl https://api.adenir.com/securities/INVALID/scores  # → 404
```
