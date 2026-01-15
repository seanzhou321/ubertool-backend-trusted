# Backend Infrastructure Resource Estimation for 100K Users

## Executive Summary

This document provides detailed computing resource estimates for the Ubertool backend to serve a community of **100,000 registered users**.

**Estimated Monthly Infrastructure Cost: $850 - $1,200**

---

## User Activity Assumptions

### User Base Breakdown
- **Total Registered Users:** 100,000
- **Monthly Active Users (MAU):** 30,000 (30% engagement)
- **Daily Active Users (DAU):** 10,000 (33% of MAU)
- **Peak Concurrent Users:** 1,000 (10% of DAU)

### User Behavior Patterns
- **Tool Owners:** 40,000 users (40%)
  - Average tools per owner: 3
  - Total tools in system: 120,000
  
- **Tool Renters:** 60,000 users (60%)
  - Average searches per day: 2
  - Average rental requests per month: 1.5

### Traffic Patterns
- **API Requests per Day:** ~500,000
- **API Requests per Second (Average):** ~6 RPS
- **API Requests per Second (Peak):** ~30 RPS (5x average during peak hours)

---

## Detailed Resource Calculations

### 1. Application Servers (gRPC + Business Logic)

**Workload Analysis:**
- Average request processing time: 100ms
- Peak concurrent requests: 30 RPS × 0.1s = 3 concurrent
- With safety margin (3x): 9 concurrent requests
- Threads per server: 50
- Servers needed: 1 (with room to scale)

**Recommended Configuration:**
```
Service: Application Server (Node.js + gRPC)
Instances: 2 (for high availability)
CPU: 4 vCPUs per instance
RAM: 8 GB per instance
Storage: 20 GB SSD per instance

Total: 8 vCPUs, 16 GB RAM, 40 GB SSD
Cost: $120/month (2 × $60)
```

**Scaling Triggers:**
- CPU > 70% for 5 minutes → Add 1 instance
- CPU < 30% for 15 minutes → Remove 1 instance
- Max instances: 4 (for 100K users)

---

### 2. PostgreSQL Database

**Data Size Estimation:**

| Table | Rows | Avg Row Size | Total Size |
|-------|------|--------------|------------|
| users | 100,000 | 1 KB | 100 MB |
| tools | 120,000 | 2 KB | 240 MB |
| tool_images | 600,000 | 0.5 KB | 300 MB |
| rental_requests | 150,000 | 1.5 KB | 225 MB |
| reviews | 80,000 | 1 KB | 80 MB |
| notifications | 500,000 | 0.5 KB | 250 MB |
| device_tokens | 150,000 | 0.3 KB | 45 MB |
| push_logs | 2,000,000 | 0.5 KB | 1 GB |
| blocked_dates | 50,000 | 0.3 KB | 15 MB |
| sessions | 30,000 | 0.5 KB | 15 MB |
| **Total Data** | | | **~2.3 GB** |
| **With Indexes** | | | **~4.5 GB** |
| **With Growth (6 months)** | | | **~10 GB** |

**Query Load:**
- Read queries: 80% (~400K/day)
- Write queries: 20% (~100K/day)
- Average query time: 5-20ms
- Concurrent connections: 50-100

**Recommended Configuration:**
```
Service: PostgreSQL 15
Instance Type: db.t3.medium (or equivalent)
CPU: 2 vCPUs
RAM: 4 GB
Storage: 50 GB SSD (with auto-scaling to 100 GB)
IOPS: 3000 provisioned
Backup: Automated daily, 7-day retention

Cost: $80/month (managed service)
```

**Performance Optimizations:**
- Connection pooling: 100 max connections
- Shared buffers: 1 GB
- Effective cache size: 3 GB
- Work mem: 16 MB
- Maintenance work mem: 256 MB

---

### 3. Redis Cache & Sessions

**Cache Size Estimation:**

| Cache Type | Items | Avg Size | Total Size |
|------------|-------|----------|------------|
| User profiles | 30,000 (MAU) | 2 KB | 60 MB |
| Tool listings | 50,000 (popular) | 3 KB | 150 MB |
| Search results | 10,000 (cached) | 5 KB | 50 MB |
| Sessions | 30,000 (MAU) | 1 KB | 30 MB |
| Rate limits | 100,000 | 0.1 KB | 10 MB |
| OSRM distances | 100,000 | 0.2 KB | 20 MB |
| **Total** | | | **~320 MB** |
| **With Overhead** | | | **~500 MB** |

**Recommended Configuration:**
```
Service: Redis 7
Instance Type: cache.t3.small
CPU: 2 vCPUs
RAM: 1.5 GB (500 MB data + 1 GB overhead)
Replication: Single node (upgrade to cluster for HA)

Cost: $30/month
```

**Cache Policies:**
- Eviction: LRU (Least Recently Used)
- Max memory: 1 GB
- TTL strategy:
  - User profiles: 1 hour
  - Tool listings: 30 minutes
  - Search results: 5 minutes
  - Sessions: 7 days

---

### 4. OSRM Routing Service

**Data Requirements:**
- OSM Region: US West Coast
- Compressed data: ~2 GB
- Processed OSRM data: ~8 GB
- Memory for routing: 4-6 GB

**Query Load:**
- Distance calculations: ~100,000/day
- Average response time: 10-30ms
- Peak queries: ~5 QPS

**Recommended Configuration:**
```
Service: OSRM Docker Container
CPU: 4 vCPUs
RAM: 8 GB (6 GB for data + 2 GB overhead)
Storage: 20 GB SSD
Instances: 1 (2 for HA)

Cost: $60/month (1 instance)
      $120/month (2 instances for HA)
```

**Scaling Considerations:**
- Single instance can handle ~50 QPS
- For 100K users: 1 instance sufficient
- Add read replicas for geographic distribution

---

### 5. Message Queue (Bull/Redis)

**Queue Metrics:**
- Notification jobs: ~50,000/day
- Average job size: 1 KB
- Queue depth: ~500 jobs (during peak)
- Processing rate: 100 jobs/second

**Recommended Configuration:**
```
Service: Redis (shared with cache) or separate
CPU: 2 vCPUs
RAM: 2 GB
Storage: 10 GB SSD

Cost: $30/month (if separate)
      $0 (if shared with cache Redis)
```

**Queue Workers:**
- Notification workers: 10 concurrent
- Runs on application servers (no extra cost)

---

### 6. Object Storage (S3/MinIO)

**Storage Estimation:**

| Content Type | Items | Avg Size | Total Size |
|--------------|-------|----------|------------|
| Tool images (original) | 600,000 | 2 MB | 1.2 TB |
| Tool images (medium) | 600,000 | 500 KB | 300 GB |
| Tool images (thumbnail) | 600,000 | 50 KB | 30 GB |
| Profile photos | 100,000 | 200 KB | 20 GB |
| **Total** | | | **~1.55 TB** |

**Transfer Estimation:**
- Image uploads: 10,000/day × 2 MB = 20 GB/day
- Image downloads: 100,000/day × 500 KB = 50 GB/day
- Monthly transfer: ~2.1 TB

**Recommended Configuration (AWS S3):**
```
Service: AWS S3 Standard
Storage: 1.6 TB
PUT requests: 300,000/month
GET requests: 3,000,000/month
Data transfer out: 2 TB/month

Cost Breakdown:
- Storage: 1,600 GB × $0.023 = $37/month
- PUT: 300K × $0.005/1000 = $1.50/month
- GET: 3M × $0.0004/1000 = $1.20/month
- Transfer: 2 TB × $0.09/GB = $180/month
Total: $220/month
```

**Alternative (Self-hosted MinIO):**
```
Service: MinIO on dedicated server
CPU: 2 vCPUs
RAM: 4 GB
Storage: 2 TB SSD
Bandwidth: Unlimited

Cost: $80/month (cheaper for high transfer)
```

---

### 7. CDN (CloudFlare/CloudFront)

**CDN Metrics:**
- Cached content: Tool images, profile photos
- Cache hit rate: 85%
- Bandwidth: 1.7 TB/month (85% of 2 TB)
- Requests: 2.5M/month

**Recommended Configuration:**
```
Service: CloudFlare Pro or AWS CloudFront
Bandwidth: 1.7 TB/month
Requests: 2.5M/month

Cost: $50/month (CloudFlare Pro)
      $100/month (AWS CloudFront)
```

---

### 8. Firebase Cloud Messaging (FCM)

**Push Notification Metrics:**
- Active devices: 150,000 (1.5 per user)
- Notifications sent: 200,000/day
- Monthly notifications: 6,000,000

**Cost:**
```
Service: Firebase Cloud Messaging
Cost: FREE (unlimited notifications)
```

---

### 9. Email Service (SendGrid/AWS SES)

**Email Metrics:**
- Transactional emails: 50,000/month
  - Email verification: 10,000
  - Password reset: 5,000
  - Rental notifications: 30,000
  - Review notifications: 5,000

**Recommended Configuration:**
```
Service: SendGrid Essentials or AWS SES
Volume: 50,000 emails/month

Cost: $20/month (SendGrid)
      $5/month (AWS SES)
```

---

### 10. Monitoring & Logging

**Log Volume:**
- Application logs: ~50 GB/month
- Access logs: ~30 GB/month
- Error logs: ~5 GB/month
- Total: ~85 GB/month

**Recommended Configuration:**
```
Service: ELK Stack (self-hosted) or CloudWatch
Storage: 100 GB
Retention: 30 days

Cost: $40/month (self-hosted)
      $80/month (managed CloudWatch)
```

---

## Total Infrastructure Summary

### Recommended Configuration (Production)

| Service | Instances | vCPUs | RAM | Storage | Monthly Cost |
|---------|-----------|-------|-----|---------|--------------|
| **Application Servers** | 2 | 8 | 16 GB | 40 GB | $120 |
| **PostgreSQL** | 1 | 2 | 4 GB | 50 GB | $80 |
| **Redis (Cache + Queue)** | 1 | 2 | 2 GB | 10 GB | $30 |
| **OSRM** | 1 | 4 | 8 GB | 20 GB | $60 |
| **Object Storage (S3)** | - | - | - | 1.6 TB | $220 |
| **CDN** | - | - | - | - | $50 |
| **Email (SendGrid)** | - | - | - | - | $20 |
| **Monitoring** | 1 | 2 | 4 GB | 100 GB | $40 |
| **Load Balancer** | 1 | - | - | - | $20 |
| **Backups & Snapshots** | - | - | - | - | $30 |
| **FCM** | - | - | - | - | $0 |
| **TOTAL** | **7** | **18** | **34 GB** | **1.82 TB** | **$670/month** |

### With High Availability (Recommended)

| Additional Resources | Monthly Cost |
|---------------------|--------------|
| +1 Application Server | $60 |
| +1 OSRM Instance | $60 |
| PostgreSQL Read Replica | $80 |
| Redis Cluster (3 nodes) | $60 |
| **HA Premium** | **$260** |
| **TOTAL WITH HA** | **$930/month** |

---

## Scaling Projections

### 250K Users (2.5x growth)

| Service | Change | New Cost |
|---------|--------|----------|
| Application Servers | 3 → 4 instances | $240 |
| PostgreSQL | Upgrade to db.t3.large | $160 |
| Redis | Upgrade to cache.t3.medium | $60 |
| OSRM | 2 instances | $120 |
| Object Storage | 4 TB | $550 |
| CDN | 4 TB bandwidth | $120 |
| **Total** | | **$1,850/month** |

### 500K Users (5x growth)

| Service | Change | New Cost |
|---------|--------|----------|
| Application Servers | 6 instances | $360 |
| PostgreSQL | db.m5.large + replica | $400 |
| Redis | cache.m5.large cluster | $200 |
| OSRM | 3 instances | $180 |
| Object Storage | 8 TB | $1,100 |
| CDN | 8 TB bandwidth | $240 |
| **Total** | | **$3,280/month** |

---

## Cost Optimization Strategies

### 1. Reserved Instances (30-40% savings)
- Commit to 1-year reserved instances for stable services
- Savings: ~$200/month on base infrastructure

### 2. Spot Instances for Workers
- Use spot instances for notification workers
- Savings: ~$30/month

### 3. Self-hosted Object Storage
- Use MinIO instead of S3 for high transfer volumes
- Savings: ~$140/month (at 100K users)

### 4. CDN Optimization
- Aggressive caching policies
- Image optimization (WebP, compression)
- Savings: ~$20/month

### 5. Database Query Optimization
- Proper indexing
- Query caching
- Connection pooling
- Potential to delay scaling by 6-12 months

**Total Potential Savings: $390/month**  
**Optimized Cost: $540/month (from $930)**

---

## Performance Targets

### Response Times (95th percentile)
- API requests: < 200ms
- Database queries: < 50ms
- OSRM distance calculations: < 100ms
- Image loading (CDN): < 500ms
- Search results: < 1 second

### Availability
- Uptime target: 99.9% (43 minutes downtime/month)
- With HA: 99.95% (22 minutes downtime/month)

### Scalability
- Current capacity: 100K users, 30 RPS
- Burst capacity: 200K users, 60 RPS (with auto-scaling)
- Scale-out time: 5 minutes (auto-scaling)

---

## Deployment Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Load Balancer                        │
│                   (CloudFlare/ALB)                      │
└────────────────────┬────────────────────────────────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
        ▼                         ▼
┌───────────────┐         ┌───────────────┐
│  App Server 1 │         │  App Server 2 │
│  4 vCPU, 8GB  │         │  4 vCPU, 8GB  │
└───────┬───────┘         └───────┬───────┘
        │                         │
        └────────────┬────────────┘
                     │
        ┌────────────┼────────────┬────────────┐
        │            │            │            │
        ▼            ▼            ▼            ▼
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│PostgreSQL│  │  Redis   │  │   OSRM   │  │   S3     │
│ 2vCPU,4GB│  │ 2vCPU,2GB│  │ 4vCPU,8GB│  │  1.6TB   │
└──────────┘  └──────────┘  └──────────┘  └──────────┘
```

---

## Summary

**For 100,000 registered users:**

✅ **Base Infrastructure:** $670/month  
✅ **With High Availability:** $930/month  
✅ **Optimized (with cost savings):** $540/month  

**Key Metrics:**
- Total vCPUs: 18 (HA: 26)
- Total RAM: 34 GB (HA: 50 GB)
- Total Storage: 1.82 TB
- Peak capacity: 30 RPS
- Burst capacity: 60 RPS

**Recommended Starting Point:**
- Start with base infrastructure ($670/month)
- Add HA components as user base grows
- Implement cost optimizations after 3-6 months
- Plan for scaling at 75K users (before hitting limits)

This infrastructure can comfortably serve 100K users with room for growth to 150K before requiring significant upgrades.
