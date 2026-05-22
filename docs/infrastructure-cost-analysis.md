# Infrastructure Cost Analysis — Q1 2026

**Prepared by:** Kevin Okafor, Director of Infrastructure
**Date:** April 8, 2026
**Period:** January 1 – March 31, 2026

## Summary

Total cloud infrastructure spend for Q1 2026 was $1.42M, a 16% increase from Q4 2025 ($1.22M). The primary cost drivers were the AI Assistant beta environment (GPU compute), increased data storage for healthcare customers requiring dedicated HIPAA-compliant environments, and standard growth in application traffic. Cost per customer decreased 4% to $4,950 per quarter, reflecting improved efficiency despite rising absolute costs.

## Cost Breakdown by Service

| Service | Q1 2026 | Q4 2025 | Change |
|---------|---------|---------|--------|
| Compute (EC2/EKS) | $612K | $548K | +12% |
| GPU Instances (AI workloads) | $198K | $72K | +175% |
| Database (RDS/DynamoDB) | $215K | $201K | +7% |
| Storage (S3/EBS) | $124K | $108K | +15% |
| Data Transfer | $89K | $82K | +9% |
| CDN (CloudFront) | $43K | $41K | +5% |
| Monitoring & Logging | $52K | $48K | +8% |
| Security Services | $38K | $35K | +9% |
| Other (DNS, SES, Lambda) | $49K | $87K | -44% |
| **Total** | **$1,420K** | **$1,222K** | **+16%** |

## Key Observations

### GPU Cost Trajectory

GPU costs increased 175% quarter-over-quarter as we scaled the AI Assistant beta from 12 to 47 customers. We are currently running 8x NVIDIA A100 instances in US-East and 4x in EU-Frankfurt. At general availability, projected GPU spend is $350K–$450K per quarter. We have secured a 1-year reserved instance commitment for 12 A100 instances at a 38% discount, effective May 1.

### Storage Growth

S3 storage grew to 84 TB, up from 71 TB in Q4. Healthcare customer data accounted for 22 TB of this growth, stored in a dedicated HIPAA-compliant bucket with versioning and cross-region replication enabled. We implemented S3 Intelligent-Tiering for non-healthcare data in February, which is projected to save $12K per quarter once fully effective.

### Cost Optimization Wins

- Migrated 14 development environments to ARM-based Graviton3 instances, saving $18K/quarter
- Implemented automated scaling policies for non-production environments (shut down nights/weekends), saving $24K/quarter
- Consolidated 3 underutilized RDS instances into a single Aurora Serverless cluster, saving $8K/quarter
- Total Q1 optimization savings: $50K

## Projections and Recommendations

Full-year 2026 infrastructure spend is projected at $6.2M–$6.8M, depending on AI Assistant adoption rates and multi-region deployment timing. We recommend:

1. Approve the reserved instance commitment for GPU compute ($198K annual savings)
2. Evaluate spot instances for batch processing workloads (estimated $30K/quarter savings)
3. Implement a FinOps tagging policy to improve cost attribution by team and feature
4. Budget $180K for the multi-region deployment infrastructure buildout in Q3
