# gRPC Mobile Architecture - Implementation Summary

## Overview

Ubertool backend has been updated to support a **dual API architecture**:
- **gRPC API** for mobile clients (Android, iOS) - **MVP Focus**
- **RESTful API** for web browsers - **Post-MVP**

This approach provides optimal performance for mobile apps while maintaining flexibility for future web platform support.

---

## Key Changes Made

### 1. Backend PRD (`docs/backend-design/PRD-Backend.md`)

**Updated Sections:**
- **Executive Summary:** Clarified dual API support with MVP focus on mobile
- **Technical Vision:** Explained gRPC for mobile, REST for web, shared business logic
- **Architecture Style:** Dual API Gateway with Protocol Buffers (HTTP/2) and JSON (HTTP/1.1)
- **Core Components:** Separated gRPC Gateway (MVP) and REST Gateway (post-MVP)

**Key Points:**
- Both APIs share the same business logic layer
- JWT authentication works for both APIs
- All services expose both gRPC and REST interfaces
- MVP implementation focuses on gRPC only

---

### 2. Architecture Design (`docs/backend-design/Architecture-Design.md`)

**Updated Architecture Diagram:**
```
Client Layer:
  - Android App (gRPC Client) ──────► gRPC Gateway (MVP)
  - iOS App (gRPC Client) ──────────► gRPC Gateway (MVP)
  - Web App (REST Client) ─ ─ ─ ─ ─► REST Gateway (post-MVP)
                                      ↓
                            Application Services
                                      ↓
                              Data Layer (PostgreSQL, Redis, S3)
```

**Legend:**
- Solid lines: MVP implementation (gRPC)
- Dashed lines: Post-MVP (REST)

**New Sections Added:**
- **3.1 gRPC Request Flow (Mobile - MVP)**
  - Protocol Buffers over HTTP/2
  - JWT in metadata for authentication
  - Binary serialization for efficiency
  
- **3.2 RESTful Request Flow (Web - Post-MVP)**
  - JSON over HTTP/1.1
  - Standard REST conventions
  - CORS support for browsers

- **Benefits of gRPC for Mobile:**
  - Smaller payload size (Protocol Buffers vs JSON)
  - Faster serialization (binary vs text)
  - HTTP/2 multiplexing (multiple requests over single connection)
  - Bidirectional streaming (future real-time features)
  - Strong typing (auto-generated client code)
  - Better mobile performance (lower battery consumption)

**Updated Data Flow Patterns:**
- Asynchronous notifications now include Firebase Cloud Messaging for push notifications
- Image upload flow supports gRPC streaming for smaller images (<1MB)

---

### 3. Protocol Buffer Definitions (`proto/ubertool.proto`)

**Created comprehensive .proto file with 8 services:**

1. **AuthService** - Registration, login, logout, token refresh, password reset, email verification
2. **UserService** - Profile management, photo upload (streaming), ratings
3. **ToolService** - Tool CRUD, image upload (streaming), listings management
4. **SearchService** - Tool search, nearby tools, categories
5. **RentalScheduleService** - Schedule viewing, availability check, date blocking, statistics
6. **RentalRequestService** - Request workflow (create, accept, reject, finalize, cancel)
7. **ReviewService** - Review CRUD, flagging, user/tool reviews
8. **NotificationService** - Notifications, preferences, push token registration

**Key Features:**
- **Protocol Buffers v3** syntax
- **Java package** configuration for Android (`com.ubertool.api.v1`)
- **Swift prefix** configuration for iOS (`UBT`)
- **Streaming support** for image uploads
- **Pagination** support for list endpoints
- **Error handling** with detailed error messages
- **Type safety** with strong typing for all fields

**Common Types:**
- `Location` (latitude, longitude, address)
- `PaginationRequest/Response`
- `ErrorDetail` for validation errors

---

### 4. Frontend PRD (`docs/frontend-design/PRD-Frontend.md`)

**Updated to Mobile-First MVP:**

**Executive Summary:**
- Changed from "multi-platform" to "mobile-first interface for Android and iOS"
- Added explicit MVP scope: Native mobile apps only, web is post-MVP

**Compatibility:**
- Android: 8.0 (API level 26) and above
- iOS: 13.0 and above
- Web: Post-MVP

**New Section - Mobile-Specific Requirements:**
- Offline support (cache tool listings and user data)
- Push notifications (Firebase Cloud Messaging)
- Location services (GPS for proximity search)
- Camera integration (direct photo capture)
- Deep linking (share tool listings)
- App size target: < 50MB

**MVP Scope Updated:**
- ✅ Native mobile apps (Android and iOS)
- ✅ gRPC API integration
- ✅ Camera integration for photos
- ✅ Offline data caching
- ✅ Push notifications
- ❌ Web browser platform (moved to post-MVP)

**Dependencies Updated:**
- **Backend:** gRPC API (primary), JWT tokens
- **Third-Party:** Firebase Cloud Messaging, Mapbox SDK (Android/iOS), OSRM (routing)
- **Development Tools:**
  - Android: Kotlin, Android Studio, Jetpack Compose
  - iOS: Swift, Xcode, SwiftUI
  - gRPC: Auto-generated client code from Protocol Buffers
  - State Management: MVVM architecture
  - Image Loading: Coil (Android), Kingfisher (iOS)
  - Local Storage: Room (Android), Core Data (iOS)

---

## Technology Stack Summary

### Mobile Clients

**Android:**
- Language: Kotlin
- IDE: Android Studio
- UI Framework: Jetpack Compose
- gRPC Client: Auto-generated from .proto files
- State Management: ViewModel (MVVM)
- Image Loading: Coil
- Local Database: Room
- Minimum SDK: API 26 (Android 8.0)

**iOS:**
- Language: Swift
- IDE: Xcode
- UI Framework: SwiftUI
- gRPC Client: Auto-generated from .proto files
- State Management: ObservableObject (MVVM)
- Image Loading: Kingfisher
- Local Database: Core Data
- Minimum Version: iOS 13.0

### Backend (Shared by Both APIs)

**Recommended Stack:**
- Language: Node.js + TypeScript
- gRPC Framework: @grpc/grpc-js
- REST Framework: Express.js (post-MVP)
- ORM: Prisma
- Database: PostgreSQL 15+ with PostGIS
- Cache: Redis 7+
- Queue: Bull (Redis-based)
- Storage: AWS S3 or MinIO
- Push Notifications: Firebase Admin SDK
- Email: SendGrid or AWS SES

---

## gRPC vs REST Comparison

| Feature | gRPC (Mobile MVP) | REST (Web Post-MVP) |
|---------|-------------------|---------------------|
| **Protocol** | HTTP/2 | HTTP/1.1 |
| **Format** | Protocol Buffers (binary) | JSON (text) |
| **Payload Size** | ~30% smaller | Larger |
| **Serialization** | Faster (binary) | Slower (text parsing) |
| **Type Safety** | Strong (auto-generated) | Weak (manual validation) |
| **Streaming** | Bidirectional | Limited (SSE) |
| **Browser Support** | Requires grpc-web | Native |
| **Mobile Performance** | Excellent | Good |
| **Battery Consumption** | Lower | Higher |
| **Development** | Code generation | Manual implementation |

---

## Implementation Roadmap

### Phase 1: gRPC Backend (Weeks 1-2)
- [ ] Set up gRPC server with Protocol Buffer compilation
- [ ] Implement AuthService with JWT authentication
- [ ] Implement UserService with profile management
- [ ] Set up PostgreSQL database with migrations
- [ ] Configure Redis for caching and sessions

### Phase 2: Core Services (Weeks 3-4)
- [ ] Implement ToolService with image upload streaming
- [ ] Implement SearchService with geospatial queries
- [ ] Implement RentalScheduleService with date management
- [ ] Set up Firebase Cloud Messaging for push notifications

### Phase 3: Rental Workflow (Weeks 5-6)
- [ ] Implement RentalRequestService with state machine
- [ ] Implement ReviewService with bidirectional ratings
- [ ] Implement NotificationService with FCM integration
- [ ] Set up message queue for async processing

### Phase 4: Android App (Weeks 7-9)
- [ ] Generate Kotlin gRPC client code from .proto files
- [ ] Implement authentication flow with JWT storage
- [ ] Implement tool listing and search with Mapbox maps
- [ ] Implement rental request workflow
- [ ] Implement camera integration for photo capture
- [ ] Implement push notifications with FCM
- [ ] Implement offline caching with Room

### Phase 5: iOS App (Weeks 10-12)
- [ ] Generate Swift gRPC client code from .proto files
- [ ] Implement authentication flow with Keychain storage
- [ ] Implement tool listing and search with MapKit
- [ ] Implement rental request workflow
- [ ] Implement camera integration for photo capture
- [ ] Implement push notifications with FCM
- [ ] Implement offline caching with Core Data

### Phase 6: Testing & Launch (Week 13)
- [ ] Integration testing (backend + mobile)
- [ ] Performance testing (load, stress)
- [ ] Security testing (penetration, vulnerability)
- [ ] Beta testing with real users
- [ ] App Store submission (iOS)
- [ ] Google Play submission (Android)
- [ ] Production deployment

### Post-MVP: Web Platform (Weeks 14+)
- [ ] Implement RESTful API gateway
- [ ] Create OpenAPI/Swagger documentation
- [ ] Build web frontend with React
- [ ] Implement grpc-web for browser compatibility (alternative)

---

## Code Generation Commands

### Generate gRPC Code

**For Backend (Node.js + TypeScript):**
```bash
npm install -g grpc-tools
grpc_tools_node_protoc \
  --js_out=import_style=commonjs,binary:./src/generated \
  --grpc_out=grpc_js:./src/generated \
  --plugin=protoc-gen-grpc=`which grpc_tools_node_protoc_plugin` \
  proto/ubertool.proto
```

**For Android (Kotlin):**
```bash
protoc \
  --java_out=app/src/main/java \
  --grpc-java_out=app/src/main/java \
  --plugin=protoc-gen-grpc-java=/path/to/protoc-gen-grpc-java \
  proto/ubertool.proto
```

**For iOS (Swift):**
```bash
protoc \
  --swift_out=Sources/Generated \
  --grpc-swift_out=Sources/Generated \
  proto/ubertool.proto
```

---

## Authentication Flow (gRPC)

### JWT Token in Metadata

**Client sends:**
```
metadata: {
  "authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Server validates:**
1. Extract token from metadata
2. Verify JWT signature
3. Check expiration
4. Load user from database
5. Inject user context into request
6. Process request

**Token Refresh:**
1. Client detects token expiration (or proactively refreshes)
2. Calls `AuthService.RefreshToken` with refresh token
3. Server validates refresh token
4. Issues new access token
5. Client stores new token

---

## Push Notification Flow

### Setup
1. Mobile app requests FCM token on launch
2. App calls `NotificationService.RegisterPushToken` with device token
3. Backend stores token in database linked to user

### Sending Notifications
1. Event occurs (e.g., rental request accepted)
2. Backend publishes event to message queue
3. Notification worker consumes event
4. Worker retrieves user's FCM token from database
5. Worker sends push notification via Firebase Admin SDK
6. FCM delivers notification to device
7. App displays notification to user

---

## File Structure

```
ubertool-backend/
├── proto/
│   └── ubertool.proto              # Protocol Buffer definitions
├── src/
│   ├── generated/                  # Auto-generated gRPC code
│   ├── services/
│   │   ├── auth.service.ts
│   │   ├── user.service.ts
│   │   ├── tool.service.ts
│   │   ├── search.service.ts
│   │   ├── rental-schedule.service.ts
│   │   ├── rental-request.service.ts
│   │   ├── review.service.ts
│   │   └── notification.service.ts
│   ├── grpc/
│   │   ├── server.ts               # gRPC server setup
│   │   └── interceptors/           # Auth, logging, error handling
│   ├── repositories/               # Data access layer
│   ├── models/                     # Database models
│   └── utils/
├── docs/
│   ├── backend-design/
│   │   ├── PRD-Backend.md
│   │   ├── Architecture-Design.md
│   │   ├── Database-Schema.md
│   │   └── API-Specification.md
│   └── frontend-design/
│       ├── PRD-Frontend.md
│       ├── UI-Design.md
│       └── UseCases.md
└── package.json
```

---

## Testing Strategy

### Backend Testing
- **Unit Tests:** Test individual service methods
- **Integration Tests:** Test gRPC endpoints with test client
- **Performance Tests:** Load testing with ghz (gRPC benchmarking tool)
- **Security Tests:** Authentication, authorization, input validation

### Mobile Testing
- **Unit Tests:** Test ViewModels and business logic
- **UI Tests:** Espresso (Android), XCUITest (iOS)
- **Integration Tests:** Test gRPC client communication
- **Performance Tests:** Network performance, battery consumption

---

## Summary

✅ **Backend PRD** updated for dual API architecture  
✅ **Architecture Design** updated with gRPC flow and benefits  
✅ **Protocol Buffer definitions** created for all 8 services  
✅ **Frontend PRD** updated for mobile-first MVP  
✅ **Technology stack** defined for Android and iOS  
✅ **Implementation roadmap** created with 13-week timeline  

**Next Steps:**
1. Review and approve the gRPC architecture
2. Set up development environment with Protocol Buffer compiler
3. Generate gRPC code for backend (Node.js/TypeScript)
4. Begin Phase 1: gRPC Backend implementation
5. Set up Android and iOS projects with gRPC client generation

All design documents are now aligned with the mobile-first gRPC architecture!
