# Product Requirements Document - Ubertool Backend

## 1. Executive Summary

The Ubertool backend provides a robust, scalable API service that powers the neighborhood tool sharing platform. It supports **gRPC for mobile clients** (Android/iOS) and **RESTful API for web browsers**, managing user authentication, tool inventory, rental workflows, ratings, and notifications while ensuring data integrity, security, and performance.

**MVP Scope:** Mobile platforms only (Android and iOS). Web browser support is planned for post-MVP.

## 2. Technical Vision

Build a secure, performant, and maintainable backend system using modern best practices, with PostgreSQL as the persistence layer, supporting multi-platform clients through dual API interfaces:
- **gRPC API** for mobile clients (Android, iOS) - high performance, efficient binary protocol
- **RESTful API** for web browsers - standard HTTP/JSON for broad compatibility

Both APIs share the same business logic layer, ensuring consistency across platforms.

## 3. System Architecture Overview

### 3.1 Architecture Style
- **Pattern:** Layered architecture with clear separation of concerns
- **API Style:** Dual API Gateway
  - **gRPC** for mobile clients (Android, iOS) - Protocol Buffers, HTTP/2, bidirectional streaming
  - **RESTful** for web browsers - JSON over HTTP/1.1, standard REST conventions
- **Database:** PostgreSQL with connection pooling
- **Authentication:** JWT-based token authentication (same tokens for both APIs)
- **Deployment:** Containerized microservices (Docker)

### 3.2 Core Components
1. **Dual API Gateway Layer:**
   - **gRPC Gateway:** For mobile clients (Android, iOS)
   - **REST Gateway:** For web browsers (post-MVP)
   - Shared: Request routing, rate limiting, authentication
2. **Authentication Service:** User auth, token management (JWT for both APIs)
3. **User Service:** Profile and account management
4. **Tool Service:** Listing CRUD operations
5. **Search Service:** Tool discovery and filtering
6. **Rental Service:** Request workflow management
7. **Review Service:** Ratings and reviews
8. **Notification Service:** Email and push notifications
9. **Storage Service:** Image upload and management
10. **Geolocation Service:** Distance calculations

**Note:** All services expose both gRPC and REST interfaces, but MVP focuses on gRPC implementation for mobile clients.

## 4. Core Features & Requirements

### 4.1 Authentication & Authorization
**Priority:** P0 (MVP)

**Requirements:**
- User registration with email verification
- Secure password hashing (bcrypt, min 12 rounds)
- JWT token generation and validation
- Token refresh mechanism
- Password reset flow
- Session management
- Role-based access control (RBAC)
- Rate limiting on auth endpoints

**API Endpoints:**
```
POST   /api/v1/auth/register
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/auth/refresh
POST   /api/v1/auth/forgot-password
POST   /api/v1/auth/reset-password
POST   /api/v1/auth/verify-email
```

**Security Requirements:**
- Password complexity: min 8 chars, uppercase, lowercase, number, special char
- Account lockout after 5 failed attempts (15-minute cooldown)
- Email verification required before full access
- Secure token storage (httpOnly cookies or secure storage)
- Token expiration: access token (15 min), refresh token (7 days)

### 4.2 User Management
**Priority:** P0 (MVP)

**Requirements:**
- CRUD operations for user profiles
- Profile photo upload and management
- Address geocoding for proximity calculations
- User verification status tracking
- Privacy settings management
- Account deactivation/deletion
- User search (admin only)

**API Endpoints:**
```
GET    /api/v1/users/me
PUT    /api/v1/users/me
DELETE /api/v1/users/me
POST   /api/v1/users/me/photo
GET    /api/v1/users/:userId
GET    /api/v1/users/:userId/ratings
```

**Data Validation:**
- Email format validation
- Phone number format validation
- Address validation and geocoding
- Image size limits (max 5MB)
- Supported image formats: JPEG, PNG, WebP

### 4.3 Tool Management
**Priority:** P0 (MVP)

**Requirements:**
- CRUD operations for tool listings
- Multi-image upload (1-10 images per tool)
- Category management
- Rental schedule management (day-based)
- Tool status tracking (active, inactive, rented, archived)
- Ownership verification
- Soft delete for data retention

**API Endpoints:**
```
POST   /api/v1/tools
GET    /api/v1/tools/:toolId
PUT    /api/v1/tools/:toolId
DELETE /api/v1/tools/:toolId
GET    /api/v1/tools/my-listings
POST   /api/v1/tools/:toolId/images
DELETE /api/v1/tools/:toolId/images/:imageId
PUT    /api/v1/tools/:toolId/schedule/block
DELETE /api/v1/tools/:toolId/schedule/unblock
GET    /api/v1/tools/:toolId/schedule
GET    /api/v1/tools/:toolId/schedule/availability
```

**Business Rules:**
- Users can only edit/delete their own listings
- Deletion requires no active rental requests
- Minimum required fields: name, category, description, price, location, condition
- Price must be positive number
- Replacement value must be positive number

### 4.4 Rental Schedule Management
**Priority:** P0 (MVP)

**Overview:**
The rental schedule system manages tool availability using day-based rental periods. The minimum rental duration is one full day (24 hours), simplifying scheduling logic and preventing complex hourly conflicts.

**Requirements:**

**Day-Based Scheduling:**
- All rental periods are measured in full days
- Minimum rental period: 1 day
- Maximum rental period: 30 days (configurable)
- Dates are inclusive (start date and end date both count as rental days)
- Time component is normalized to midnight UTC for consistency

**Availability Calendar:**
- View tool availability for date ranges (default: next 90 days)
- Display blocked dates (owner-blocked or rented)
- Show pending requests (not yet confirmed)
- Calendar updates in real-time when requests are accepted/finalized

**Owner Blocking:**
- Owners can manually block dates when tool is unavailable
- Block reasons: maintenance, personal use, vacation, other
- Blocked dates prevent new rental requests
- Owners can unblock dates if no confirmed rentals exist
- Bulk blocking for date ranges

**Overlap Prevention:**
- System validates no overlapping rentals when accepting requests
- Overlap check includes:
  - Finalized rentals (confirmed bookings)
  - Accepted rentals (pending finalization)
  - Owner-blocked dates
- Pending requests do NOT block dates (multiple pending allowed)
- When request is accepted, system re-validates no conflicts occurred

**Conflict Detection Algorithm:**
```
For new rental request [start_date, end_date]:
  Check if ANY existing rental/block satisfies:
    (existing.start_date <= new.end_date) AND 
    (existing.end_date >= new.start_date)
  If match found → CONFLICT
  Else → AVAILABLE
```

**API Endpoints:**
```
# View schedule
GET    /api/v1/tools/:toolId/schedule
       Returns: blocked dates, finalized rentals, accepted rentals
       Query params: startDate, endDate (default: today + 90 days)

# Check availability for specific dates
GET    /api/v1/tools/:toolId/schedule/availability?startDate=2026-01-15&endDate=2026-01-17
       Returns: { available: true/false, conflicts: [...] }

# Block dates (owner only)
PUT    /api/v1/tools/:toolId/schedule/block
       Body: { startDate, endDate, reason }
       Returns: blocked date range

# Unblock dates (owner only)
DELETE /api/v1/tools/:toolId/schedule/unblock
       Body: { startDate, endDate }
       Returns: success if no confirmed rentals

# Get rental statistics
GET    /api/v1/tools/:toolId/schedule/stats
       Returns: utilization rate, total rental days, revenue estimate
```

**Database Schema Requirements:**
- `rental_requests` table:
  - `start_date`: DATE (not TIMESTAMP)
  - `end_date`: DATE (not TIMESTAMP)
  - Composite index on (tool_id, start_date, end_date, status)
- `blocked_dates` table (new):
  - `tool_id`: Foreign key to tools
  - `start_date`: DATE
  - `end_date`: DATE
  - `reason`: VARCHAR
  - `created_by`: Foreign key to users (owner)

**Business Rules:**
1. **Request Creation:**
   - Validate start_date >= today
   - Validate end_date > start_date
   - Validate rental period <= 30 days
   - Check availability (advisory, not blocking)
   - Allow multiple pending requests for same dates

2. **Request Acceptance:**
   - Re-validate availability (strict check)
   - If conflict detected → reject with error
   - If available → mark as accepted, dates are now reserved
   - Notify renter of acceptance

3. **Request Finalization:**
   - Only accepted requests can be finalized
   - Auto-cancel all other pending/accepted requests for same tool
   - Dates are now confirmed and cannot be unblocked

4. **Owner Blocking:**
   - Can only block future dates
   - Cannot block dates with finalized/accepted rentals
   - Can block dates with pending requests (requests remain pending)

5. **Cancellation:**
   - Renter can cancel pending/accepted requests anytime
   - Owner can cancel accepted requests (not finalized)
   - Cancelled requests free up the dates immediately
   - Finalized requests cannot be cancelled (out of scope for MVP)

**Validation Examples:**

*Example 1: Valid Request*
```
Tool has no bookings
Request: Jan 15-17 → ALLOWED (pending)
Request: Jan 20-22 → ALLOWED (pending)
Owner accepts Jan 15-17 → SUCCESS
Request: Jan 16-18 → REJECTED (overlaps with accepted)
```

*Example 2: Blocking Dates*
```
Owner blocks: Jan 10-12 (maintenance)
Request: Jan 11-13 → REJECTED (overlaps blocked dates)
Request: Jan 13-15 → ALLOWED (no overlap)
```

*Example 3: Multiple Pending*
```
Request A: Jan 15-17 (pending)
Request B: Jan 15-17 (pending) → ALLOWED
Request C: Jan 16-18 (pending) → ALLOWED
Owner accepts Request A → SUCCESS
Request B auto-rejected (conflict)
Request C auto-rejected (conflict)
```

**Error Codes:**
- `RENTAL_DATES_OVERLAP`: Requested dates conflict with existing rental
- `RENTAL_DATES_BLOCKED`: Requested dates are blocked by owner
- `RENTAL_PERIOD_INVALID`: Invalid date range (end before start, too long, etc.)
- `RENTAL_PAST_DATES`: Cannot request dates in the past
- `RENTAL_CANNOT_UNBLOCK`: Cannot unblock dates with confirmed rentals

**Performance Considerations:**
- Index on (tool_id, start_date, end_date, status) for fast conflict checks
- Cache tool schedules for frequently viewed tools (TTL: 5 minutes)
- Use database-level date range queries (PostgreSQL DATERANGE type)
- Batch conflict checks for multiple tools in search results

---

### 4.5 Search & Discovery
**Priority:** P0 (MVP)

**Requirements:**
- Full-text search on tool name and description
- Geospatial queries for proximity search
- Multi-criteria filtering:
  - Category
  - Price range
  - Distance radius
  - Condition
  - Availability dates (day-based)
  - Rating threshold
- Sorting options:
  - Distance (ascending)
  - Price (ascending/descending)
  - Rating (descending)
  - Date listed (newest first)
- Pagination support
- Search result caching
- Search analytics tracking

**API Endpoints:**
```
GET    /api/v1/search/tools?q=drill&category=power-tools&maxDistance=10&minPrice=5&maxPrice=50&condition=excellent&availableFrom=2026-01-15&availableTo=2026-01-17&sortBy=distance&page=1&limit=20
GET    /api/v1/search/nearby?lat=37.7749&lng=-122.4194&radius=10
GET    /api/v1/categories
```

**Performance Requirements:**
- Search response time < 500ms for 95th percentile
- Support for 1000+ concurrent searches
- Database indexing on searchable fields
- Geospatial indexing for location queries

### 4.6 Rental Request Management
**Priority:** P0 (MVP)

**Requirements:**
- Create rental requests with day-based date ranges
- Request status tracking (pending, accepted, rejected, cancelled, finalized)
- Multiple requests per tool per user
- Request acceptance/rejection by owner
- Finalization workflow (auto-cancel other requests)
- Request history and audit trail
- Notification triggers on status changes

**API Endpoints:**
```
POST   /api/v1/rentals/request
GET    /api/v1/rentals/requests/:requestId
PUT    /api/v1/rentals/requests/:requestId/accept
PUT    /api/v1/rentals/requests/:requestId/reject
PUT    /api/v1/rentals/requests/:requestId/finalize
PUT    /api/v1/rentals/requests/:requestId/cancel
GET    /api/v1/rentals/my-requests
GET    /api/v1/rentals/incoming-requests
GET    /api/v1/rentals/history
```

**Business Rules:**
- Renters must agree to terms before requesting
- All rental periods are day-based (minimum 1 day)
- Owners cannot accept overlapping rental periods (validated against schedule)
- Finalization cancels all other pending requests for same tool
- Only pending requests can be accepted/rejected
- Only accepted requests can be finalized
- Requests can be cancelled before finalization

**State Machine:**
```
pending → accepted → finalized
pending → rejected
pending → cancelled
accepted → cancelled
```

### 4.7 Ratings & Reviews
**Priority:** P0 (MVP)

**Requirements:**
- Bidirectional rating system
- 5-star rating scale
- Optional written review (max 500 chars)
- Review submission only after rental completion
- One review per rental per user
- 2-year rolling window enforcement
- Aggregate rating calculation
- Review moderation (flag inappropriate content)
- Review edit/delete (within 24 hours)

**API Endpoints:**
```
POST   /api/v1/reviews
GET    /api/v1/reviews/:reviewId
PUT    /api/v1/reviews/:reviewId
DELETE /api/v1/reviews/:reviewId
GET    /api/v1/reviews/user/:userId
GET    /api/v1/reviews/tool/:toolId
POST   /api/v1/reviews/:reviewId/flag
```

**Business Rules:**
- Reviews only allowed after rental finalization
- Cannot review same rental twice
- Reviews older than 2 years excluded from calculations
- Aggregate rating recalculated on new review
- Flagged reviews require manual moderation

### 4.8 Notification Service
**Priority:** P0 (MVP)

**Requirements:**
- Email notifications
- Push notifications (post-MVP for mobile apps)
- Notification preferences management
- Notification templates
- Delivery tracking and retry logic
- Notification history

**Notification Types:**
- New rental request (to owner)
- Request accepted/rejected (to renter)
- Request finalized (to owner)
- Request cancelled (to both)
- New review received
- Email verification
- Password reset

**API Endpoints:**
```
GET    /api/v1/notifications
PUT    /api/v1/notifications/:notificationId/read
PUT    /api/v1/notifications/preferences
GET    /api/v1/notifications/preferences
```

### 4.9 Image Storage Service
**Priority:** P0 (MVP)

**Requirements:**
- Image upload with validation
- Multiple size generation (thumbnail, medium, full)
- Image optimization and compression
- CDN integration for fast delivery
- Secure signed URLs for uploads
- Image deletion and cleanup
- Storage quota management

**Technical Specs:**
- Max file size: 5MB
- Supported formats: JPEG, PNG, WebP
- Generated sizes: 150x150 (thumb), 800x600 (medium), original
- Storage: AWS S3 or compatible object storage
- CDN: CloudFront or similar

## 5. Data Model Overview

### 5.1 Core Entities
1. **Users:** User accounts and profiles
2. **Tools:** Tool listings
3. **ToolImages:** Images associated with tools
4. **Categories:** Tool categories
5. **RentalRequests:** Rental request records (day-based)
6. **BlockedDates:** Owner-blocked date ranges for tools
7. **Reviews:** Ratings and reviews
8. **Notifications:** User notifications
9. **Sessions:** Active user sessions

### 5.2 Key Relationships
- User → Tools (1:N) - A user owns multiple tools
- Tool → ToolImages (1:N) - A tool has multiple images
- Tool → Category (N:1) - Tools belong to categories
- Tool → BlockedDates (1:N) - A tool has multiple blocked date ranges
- User → RentalRequests (1:N) - Users make multiple requests
- Tool → RentalRequests (1:N) - Tools receive multiple requests
- User → Reviews (1:N) - Users write multiple reviews
- RentalRequest → Reviews (1:2) - Each rental has 2 reviews (bidirectional)

## 6. Non-Functional Requirements

### 6.1 Performance
- API response time: p95 < 500ms, p99 < 1s
- Database query time: < 100ms for indexed queries
- Support 1000 concurrent users
- Handle 10,000 requests per minute
- Image upload: < 5s for 5MB file

### 6.2 Scalability
- Horizontal scaling capability
- Database connection pooling (min 10, max 100)
- Caching layer (Redis) for frequent queries
- Asynchronous job processing for notifications
- CDN for static assets and images

### 6.3 Reliability
- 99.9% uptime SLA
- Automated health checks
- Database backups (daily full, hourly incremental)
- Point-in-time recovery capability
- Graceful degradation for non-critical features

### 6.4 Security
- HTTPS/TLS 1.3 only
- SQL injection prevention (parameterized queries)
- XSS prevention (input sanitization)
- CSRF protection
- Rate limiting (100 req/min per user, 1000 req/min per IP)
- API key rotation
- Secrets management (environment variables, vault)
- Regular security audits
- OWASP Top 10 compliance

### 6.5 Monitoring & Logging
- Structured logging (JSON format)
- Log levels: DEBUG, INFO, WARN, ERROR
- Centralized log aggregation
- Application performance monitoring (APM)
- Error tracking and alerting
- Metrics collection:
  - Request rate, latency, error rate
  - Database connection pool stats
  - Cache hit/miss rates
  - Queue depths

### 6.6 Data Privacy
- GDPR compliance
- Data encryption at rest (AES-256)
- Data encryption in transit (TLS 1.3)
- PII data anonymization for analytics
- Right to deletion (account removal)
- Data retention policies (2 years for reviews)

## 7. API Design Principles

### 7.1 RESTful Standards
- Resource-based URLs
- HTTP verbs: GET, POST, PUT, DELETE
- Proper status codes:
  - 200 OK, 201 Created, 204 No Content
  - 400 Bad Request, 401 Unauthorized, 403 Forbidden, 404 Not Found
  - 422 Unprocessable Entity, 429 Too Many Requests
  - 500 Internal Server Error, 503 Service Unavailable
- Consistent error response format
- API versioning (/api/v1/)

### 7.2 Request/Response Format
- Content-Type: application/json
- ISO 8601 date formats
- Pagination: limit/offset or cursor-based
- Filtering: query parameters
- Sorting: sortBy and order parameters
- Field selection: fields parameter (optional)

### 7.3 Error Handling
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input data",
    "details": [
      {
        "field": "email",
        "message": "Invalid email format"
      }
    ],
    "requestId": "req_abc123",
    "timestamp": "2026-01-11T17:42:05Z"
  }
}
```

## 8. Technology Stack

### 8.1 Recommended Stack
- **Language:** Node.js (TypeScript) or Python (FastAPI) or Go
- **Framework:** Express.js / FastAPI / Gin
- **Database:** PostgreSQL 15+
- **ORM:** Prisma / SQLAlchemy / GORM
- **Cache:** Redis 7+
- **Queue:** Bull (Redis-based) or RabbitMQ
- **Storage:** AWS S3 or MinIO
- **Email:** SendGrid or AWS SES
- **Monitoring:** Prometheus + Grafana
- **Logging:** ELK Stack or Loki

### 8.2 Development Tools
- **API Documentation:** OpenAPI/Swagger
- **Testing:** Jest/Pytest/Go Test
- **Linting:** ESLint/Pylint/golangci-lint
- **CI/CD:** GitHub Actions or GitLab CI
- **Containerization:** Docker + Docker Compose
- **Orchestration:** Kubernetes (production)

## 9. MVP Scope

**Included in MVP:**
- User authentication (JWT)
- User profile management
- Tool CRUD operations
- Search and filtering
- Rental request workflow
- Ratings and reviews
- Email notifications
- Image upload and storage
- Basic analytics

**Excluded from MVP:**
- Payment processing
- In-app messaging
- Advanced analytics dashboard
- Dispute resolution system
- Insurance integration
- Third-party integrations (social login)
- Mobile push notifications
- Real-time features (WebSockets)

## 10. Testing Strategy

### 10.1 Unit Tests
- 80%+ code coverage
- Test all business logic
- Mock external dependencies
- Fast execution (< 5 minutes)

### 10.2 Integration Tests
- API endpoint testing
- Database integration
- External service mocking
- Authentication flows

### 10.3 Performance Tests
- Load testing (1000 concurrent users)
- Stress testing (find breaking point)
- Endurance testing (sustained load)
- Database query optimization

### 10.4 Security Tests
- Penetration testing
- Vulnerability scanning
- Authentication/authorization testing
- Input validation testing

## 11. Deployment Strategy

### 11.1 Environments
- **Development:** Local Docker Compose
- **Staging:** Kubernetes cluster (mirrors production)
- **Production:** Kubernetes cluster with auto-scaling

### 11.2 CI/CD Pipeline
1. Code commit triggers pipeline
2. Run linting and unit tests
3. Build Docker image
4. Run integration tests
5. Push image to registry
6. Deploy to staging (auto)
7. Run smoke tests
8. Deploy to production (manual approval)

### 11.3 Database Migrations
- Version-controlled migration scripts
- Automated migration on deployment
- Rollback capability
- Zero-downtime migrations

## 12. Success Metrics

### 12.1 Technical Metrics
- API uptime: 99.9%
- Average response time: < 300ms
- Error rate: < 0.1%
- Database query time: < 100ms
- Cache hit rate: > 80%

### 12.2 Business Metrics
- User registrations
- Tools listed
- Rental requests created
- Request acceptance rate
- Review submission rate
- Active users (DAU/MAU)

## 13. Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Database performance degradation | High | Indexing, query optimization, read replicas |
| Image storage costs | Medium | Compression, CDN, lifecycle policies |
| API abuse/DDoS | High | Rate limiting, WAF, monitoring |
| Data breach | Critical | Encryption, security audits, compliance |
| Third-party service outage | Medium | Fallback mechanisms, retry logic |
| Scalability bottlenecks | High | Load testing, horizontal scaling, caching |

## 14. Timeline Estimate

- **Phase 1 (Weeks 1-2):** Project setup, database schema, authentication
- **Phase 2 (Weeks 3-4):** User and tool services
- **Phase 3 (Weeks 5-6):** Search service and geolocation
- **Phase 4 (Weeks 7-8):** Rental request workflow
- **Phase 5 (Weeks 9-10):** Reviews and notifications
- **Phase 6 (Weeks 11-12):** Testing, optimization, documentation
- **Phase 7 (Week 13):** Deployment and launch

**Total MVP Timeline:** 13 weeks (parallel with frontend)
