# Team Retrospective — Q1 2026

**Facilitator:** Amy Gonzalez, Director of Engineering
**Date:** April 4, 2026
**Participants:** 38 engineers, 4 engineering managers, 2 directors

## Format

This retrospective followed a structured Start/Stop/Continue format, with breakout sessions by team followed by a cross-team synthesis. Anonymous feedback was collected via Retrium prior to the session, with 82% participation rate.

## What Went Well

### Shipped the AI Assistant Beta on Schedule
The Data Engineering and Backend Services teams collaborated to deliver the AI Assistant beta to 47 customers by the February 15 target date. This was the most complex cross-team project in company history, involving 14 engineers across 3 teams over 11 weeks. Customer feedback has been overwhelmingly positive, with a 4.6/5.0 satisfaction score.

### Reduced Deployment Frequency from Weekly to Daily
The Infrastructure team completed the CI/CD pipeline overhaul in January, enabling daily deployments to production. Average deployment time decreased from 45 minutes to 12 minutes, and rollback capability improved from 30 minutes to under 5 minutes. The number of deployments increased from 12 in Q4 to 61 in Q1, with zero production incidents attributed to deployment issues.

### On-Call Improvements
After implementing the new on-call rotation with follow-the-sun coverage in February, median incident response time dropped from 14 minutes to 6 minutes. Off-hours pages decreased 40% due to improved alerting thresholds and automated remediation scripts. Engineer satisfaction with on-call duties improved from 2.8/5.0 to 3.9/5.0 in the March pulse survey.

## What Didn't Go Well

### Cross-Team Dependency Management
Three projects experienced delays due to unclear ownership of shared services. The authentication service migration blocked both Frontend and Backend teams for 9 days in February because neither team had allocated capacity for integration testing. A total of 42 engineer-days were lost to dependency-related blocks across Q1.

### Technical Debt Accumulation
The team estimated that 18% of sprint capacity was consumed by working around technical debt, up from 12% in Q3 2025. The top three areas of concern are the legacy notification system (4 incidents in Q1), the monolithic billing service (slowing feature development), and inconsistent API error handling patterns across microservices.

### Documentation Gaps
New hires reported that onboarding documentation was outdated or incomplete for 5 of 8 core services. Average onboarding time to first meaningful contribution was 3.2 weeks, above the 2-week target. Three senior engineers spent an estimated 60 hours total answering questions that should have been covered by documentation.

## Action Items

| Action | Owner | Due Date | Priority |
|--------|-------|----------|----------|
| Create cross-team dependency board in Jira | Amy Gonzalez | April 18 | High |
| Allocate 20% of Q2 sprint capacity to tech debt | David Park | April 11 | High |
| Audit and update onboarding docs for all services | Team leads | May 15 | Medium |
| Establish documentation review as part of PR checklist | Amy Gonzalez | April 25 | Medium |
| Pilot "office hours" for cross-team questions | Engineering Managers | April 14 | Low |
| Decompose billing service — create RFC | Backend Services lead | May 1 | High |
| Standardize API error handling — create style guide | Platform team | May 15 | Medium |

## Metrics Summary

| Metric | Q4 2025 | Q1 2026 | Target |
|--------|---------|---------|--------|
| Sprint velocity (avg story points) | 142 | 156 | 150 |
| Deployment frequency | 12/quarter | 61/quarter | 50/quarter |
| Mean time to recovery (MTTR) | 38 min | 22 min | 30 min |
| Production incidents (P1/P2) | 7 | 4 | ≤5 |
| Code review turnaround (median) | 8 hrs | 5 hrs | 6 hrs |
| Engineering satisfaction (pulse) | 3.6/5.0 | 4.1/5.0 | 4.0/5.0 |
