# Backend Infrastructure Resource Estimation for 10K Users (MVP)

## Executive Summary

This document provides detailed computing resource estimates for the Ubertool backend to serve a community of **10,000 registered users** - ideal for MVP launch and early growth.

**Estimated Monthly Infrastructure Cost: $180 - $280**

---

## User Activity Assumptions

### User Base Breakdown
- **Total Registered Users:** 10,000
- **Monthly Active Users (MAU):** 3,000 (30% engagement)
- **Daily Active Users (DAU):** 1,000 (33% of MAU)
- **Peak Concurrent Users:** 100 (10% of DAU)

### User Behavior Patterns
- **Tool Owners:** 4,000 users (40%)
  - Average tools per owner: 3
  - Total tools in system: 12,000
  
- **Tool Renters:** 6,000 users (60%)
  - Average searches per day: 2
  - Average rental requests per month: 1.5

### Traffic Patterns
- **API Requests per Day:** ~50,000
- **API Requests per Second (Average):** ~0.6 RPS
- **API Requests per Second (Peak):** ~3 RPS (5x average during peak hours)

---

## Simplified Infrastructure (MVP)

### 1. Application Server (All-in-One)

**Workload Analysis:**
- Peak concurrent requests: 3 RPS × 0.1s = 0.3 concurrent
- Single server can easily handle this load
- Room for 10x growth before scaling

**Recommended Configuration:**
```
Service: Single Application Server
- gRPC API
- Business logic
- Background workers (notifications)

CPU: 2 vCPUs
RAM: 4 GB
Storage: 20 GB SSD

Cost: $40/month
```

---

### 2. PostgreSQL Database

**Data Size Estimation:**

| Table | Rows | Avg Row Size | Total Size |
|-------|------|--------------|------------|
| users | 10,000 | 1 KB | 10 MB |
| tools | 12,000 | 2 KB | 24 MB |
| tool_images | 60,000 | 0.5 KB | 30 MB |
| rental_requests | 15,000 | 1.5 KB | 22 MB |
| reviews | 8,000 | 1 KB | 8 MB |
| notifications | 50,000 | 0.5 KB | 25 MB |
| device_tokens | 15,000 | 0.3 KB | 4.5 MB |
| push_logs | 200,000 | 0.5 KB | 100 MB |
| blocked_dates | 5,000 | 0.3 KB | 1.5 MB |
| sessions | 3,000 | 0.5 KB | 1.5 MB |
| **Total Data** | | | **~230 MB** |
| **With Indexes** | | | **~450 MB** |
| **With Growth (6 months)** | | | **~1 GB** |

**Recommended Configuration:**
```
Service: PostgreSQL 15 (managed or self-hosted)
CPU: 1 vCPU
RAM: 2 GB
Storage: 20 GB SSD
Connections: 25 max

Cost: $25/month (managed: DigitalOcean, Render)
      $15/month (self-hosted on same server)
```

---

### 3. Redis (Cache + Queue)

**Cache Size Estimation:**

| Cache Type | Items | Avg Size | Total Size |
|------------|-------|----------|------------|
| User profiles | 3,000 (MAU) | 2 KB | 6 MB |
| Tool listings | 5,000 (popular) | 3 KB | 15 MB |
| Search results | 1,000 (cached) | 5 KB | 5 MB |
| Sessions | 3,000 (MAU) | 1 KB | 3 MB |
| Rate limits | 10,000 | 0.1 KB | 1 MB |
| OSRM distances | 10,000 | 0.2 KB | 2 MB |
| **Total** | | | **~32 MB** |
| **With Overhead** | | | **~50 MB** |

**Recommended Configuration:**
```
Service: Redis 7 (can run on same server as app)
RAM: 256 MB (50 MB data + 200 MB overhead)
Storage: 1 GB

Cost: $0 (runs on app server)
      $15/month (if separate managed instance)
```

---

### 4. OSRM Routing Service

**Recommended Configuration:**
```
Service: OSRM Docker Container (on app server)
CPU: Shared (2 vCPUs)
RAM: 2 GB (shared with app)
Storage: 10 GB (US West Coast data)

Cost: $0 (runs on app server)
      $30/month (if separate instance)
```

**Note:** For 10K users, OSRM can run on the same server as the application with minimal performance impact.

---

### 5. Object Storage (S3/MinIO)

**Storage Estimation:**

| Content Type | Items | Avg Size | Total Size |
|--------------|-------|----------|------------|
| Tool images (original) | 60,000 | 2 MB | 120 GB |
| Tool images (medium) | 60,000 | 500 KB | 30 GB |
| Tool images (thumbnail) | 60,000 | 50 KB | 3 GB |
| Profile photos | 10,000 | 200 KB | 2 GB |
| **Total** | | | **~155 GB** |

**Transfer Estimation:**
- Image uploads: 1,000/day × 2 MB = 2 GB/day
- Image downloads: 10,000/day × 500 KB = 5 GB/day
- Monthly transfer: ~210 GB

**Recommended Configuration:**
```
Service: AWS S3 Standard (or Backblaze B2)

Storage: 160 GB
PUT requests: 30,000/month
GET requests: 300,000/month
Data transfer out: 210 GB/month

Cost Breakdown:
- Storage: 160 GB × $0.023 = $3.70/month
- PUT: 30K × $0.005/1000 = $0.15/month
- GET: 300K × $0.0004/1000 = $0.12/month
- Transfer: 210 GB × $0.09/GB = $19/month
Total: $23/month

Alternative (Backblaze B2):
- Storage: 160 GB × $0.005 = $0.80/month
- Transfer: 210 GB × $0.01/GB = $2.10/month
Total: $3/month (much cheaper!)
```

---

### 6. CDN (Optional for MVP)

**For MVP:** Use CloudFlare Free Tier
```
Service: CloudFlare Free
Bandwidth: Unlimited
Requests: Unlimited
Features: Basic caching, DDoS protection

Cost: $0/month
```

---

### 7. Firebase Cloud Messaging (FCM)

```
Service: Firebase Cloud Messaging
Notifications: 600,000/month
Cost: FREE
```

---

### 8. Email Service

**Email Metrics:**
- Transactional emails: 5,000/month

**Recommended Configuration:**
```
Service: SendGrid Free Tier or AWS SES
Volume: 5,000 emails/month

Cost: $0/month (SendGrid Free: 100/day)
      $0.50/month (AWS SES)
```

---

### 9. Monitoring (Simplified)

**For MVP:**
```
Service: Self-hosted logging + free tier monitoring
- Application logs to file
- Basic metrics with Prometheus (free)
- Uptime monitoring with UptimeRobot (free)

Cost: $0/month
```

---

## Total Infrastructure Summary (MVP)

### Option 1: All-in-One Server (Cheapest)

```
Single Server Configuration:
- Application (Node.js + gRPC)
- PostgreSQL
- Redis
- OSRM
- Monitoring

CPU: 4 vCPUs
RAM: 8 GB
Storage: 50 GB SSD

Provider: DigitalOcean, Hetzner, or Linode
Cost: $48/month (DigitalOcean)
      $35/month (Hetzner - cheaper!)

Additional Services:
- Object Storage (Backblaze B2): $3/month
- CDN (CloudFlare): $0/month
- Email (SendGrid Free): $0/month
- FCM: $0/month

TOTAL: $51/month (DigitalOcean)
       $38/month (Hetzner)
```

### Option 2: Managed Services (Easier)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| **Application Server** | 2 vCPU, 4 GB | $40 |
| **PostgreSQL (Managed)** | 1 vCPU, 2 GB | $25 |
| **Redis (Managed)** | 256 MB | $15 |
| **OSRM (on app server)** | Shared | $0 |
| **Object Storage (B2)** | 160 GB | $3 |
| **CDN (CloudFlare)** | Free tier | $0 |
| **Email (SendGrid)** | Free tier | $0 |
| **FCM** | Unlimited | $0 |
| **Monitoring** | Free tools | $0 |
| **TOTAL** | | **$83/month** |

### Option 3: With Redundancy (Recommended)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| **Application Servers** | 2 × (2 vCPU, 4 GB) | $80 |
| **PostgreSQL (Managed)** | 1 vCPU, 2 GB + backups | $30 |
| **Redis (Managed)** | 256 MB | $15 |
| **OSRM** | 2 vCPU, 4 GB | $30 |
| **Object Storage (B2)** | 160 GB | $3 |
| **CDN (CloudFlare)** | Free tier | $0 |
| **Email (SendGrid)** | Free tier | $0 |
| **Load Balancer** | Basic | $10 |
| **Monitoring** | Basic paid | $20 |
| **TOTAL** | | **$188/month** |

---

## Recommended Starting Configuration

**For 10K users MVP, I recommend Option 1 (All-in-One):**

```
Infrastructure:
├── Single Server (Hetzner CPX31)
│   ├── 4 vCPUs, 8 GB RAM, 80 GB SSD
│   ├── Application (Node.js + gRPC)
│   ├── PostgreSQL 15
│   ├── Redis 7
│   └── OSRM
├── Backblaze B2 (Object Storage)
│   └── 160 GB
└── CloudFlare Free (CDN)

Total Cost: $38/month
```

**Why this works:**
- ✅ Handles 10K users comfortably
- ✅ Room to grow to 25K users
- ✅ Simple to manage (one server)
- ✅ Easy to backup
- ✅ Can scale vertically first (upgrade server)
- ✅ 95% cheaper than 100K user setup

---

## Scaling Path

### At 5K users → Stay on single server
**Cost:** $38/month

### At 10K users → Current configuration
**Cost:** $38/month

### At 20K users → Upgrade server
```
Server: 8 vCPUs, 16 GB RAM
Cost: $70/month
Total: $73/month
```

### At 40K users → Split services
```
- App Server: 4 vCPUs, 8 GB ($48)
- PostgreSQL: 2 vCPUs, 4 GB ($40)
- Redis: 1 GB ($20)
- OSRM: 4 vCPUs, 8 GB ($48)
- Storage: 500 GB ($15)
Total: $171/month
```

### At 100K users → Full infrastructure
**Cost:** $670/month (as per 100K user estimate)

---

## Performance Expectations

### Response Times (95th percentile)
- API requests: < 100ms
- Database queries: < 20ms
- OSRM distance calculations: < 50ms
- Image loading (CDN): < 500ms
- Search results: < 500ms

### Capacity
- Current: 10K users, 3 RPS average
- Burst: 20K users, 10 RPS
- Max before scaling: 25K users

### Availability
- Single server: 99.5% (3.6 hours downtime/month)
- With redundancy: 99.9% (43 minutes downtime/month)

---

## Cost Comparison by Provider

### Single Server (4 vCPU, 8 GB RAM, 80 GB SSD)

| Provider | Monthly Cost | Notes |
|----------|--------------|-------|
| **Hetzner** | $35 | Best value, EU/US locations |
| **DigitalOcean** | $48 | Easy to use, good docs |
| **Linode** | $48 | Reliable, good support |
| **Vultr** | $48 | Global locations |
| **AWS EC2** | $85 | Expensive, but scalable |
| **Google Cloud** | $90 | Expensive, but scalable |

**Recommendation:** Start with Hetzner for best value, migrate to DigitalOcean/AWS later if needed.

---

## Summary

**For 10,000 users (MVP):**

✅ **Minimum Viable:** $38/month (all-in-one server)  
✅ **Managed Services:** $83/month (easier management)  
✅ **With Redundancy:** $188/month (production-ready)  

**Key Metrics:**
- Total vCPUs: 4
- Total RAM: 8 GB
- Total Storage: 240 GB (80 GB server + 160 GB object storage)
- Peak capacity: 3 RPS
- Burst capacity: 10 RPS

**Recommended Starting Point:**
- Start with all-in-one server ($38/month)
- Use free tiers for CDN, email, monitoring
- Scale vertically as user base grows
- Split services at 40K users

**This setup is 95% cheaper than the 100K user infrastructure while providing excellent performance for early-stage growth!**

---

## Migration Path to 100K Users

```
10K users → $38/month (single server)
    ↓
25K users → $73/month (bigger server)
    ↓
50K users → $171/month (split services)
    ↓
100K users → $670/month (full infrastructure)
```

Each step is a natural progression with minimal downtime during migration.
