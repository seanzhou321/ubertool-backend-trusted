# Ubertool RESTful API Specification

## 1. API Overview

### 1.1 Base Information
- **Base URL:** `https://api.ubertool.com/api/v1`
- **Protocol:** HTTPS only
- **Format:** JSON
- **Authentication:** JWT Bearer tokens
- **API Version:** v1

### 1.2 Common Headers
```
Content-Type: application/json
Authorization: Bearer <access_token>
X-Request-ID: <unique_request_id>
```

### 1.3 Standard Response Format

**Success Response:**
```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "requestId": "req_abc123",
    "timestamp": "2026-01-11T17:42:05Z"
  }
}
```

**Error Response:**
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input data",
    "details": [
      {
        "field": "email",
        "message": "Invalid email format"
      }
    ]
  },
  "meta": {
    "requestId": "req_abc123",
    "timestamp": "2026-01-11T17:42:05Z"
  }
}
```

### 1.4 HTTP Status Codes
- `200 OK` - Successful GET, PUT, DELETE
- `201 Created` - Successful POST
- `204 No Content` - Successful DELETE with no response body
- `400 Bad Request` - Invalid request format
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `422 Unprocessable Entity` - Validation errors
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service temporarily unavailable

---

## 2. Authentication Endpoints

### 2.1 Register User
**POST** `/auth/register`

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "name": "John Smith",
  "phone": "+1-555-0123"
}
```

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "userId": 123,
    "email": "user@example.com",
    "name": "John Smith",
    "emailVerificationSent": true
  }
}
```

**Validation Rules:**
- Email: Valid format, unique
- Password: Min 8 chars, 1 uppercase, 1 lowercase, 1 number, 1 special char
- Name: Required, max 255 chars
- Phone: Optional, valid format

---

### 2.2 Login
**POST** `/auth/login`

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expiresIn": 900,
    "user": {
      "id": 123,
      "email": "user@example.com",
      "name": "John Smith",
      "isVerified": true
    }
  }
}
```

---

### 2.3 Refresh Token
**POST** `/auth/refresh`

**Request Body:**
```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expiresIn": 900
  }
}
```

---

### 2.4 Logout
**POST** `/auth/logout`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `204 No Content`

---

### 2.5 Forgot Password
**POST** `/auth/forgot-password`

**Request Body:**
```json
{
  "email": "user@example.com"
}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "message": "Password reset email sent"
  }
}
```

---

### 2.6 Reset Password
**POST** `/auth/reset-password`

**Request Body:**
```json
{
  "token": "reset_token_from_email",
  "newPassword": "NewSecurePass123!"
}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "message": "Password reset successful"
  }
}
```

---

### 2.7 Verify Email
**POST** `/auth/verify-email`

**Request Body:**
```json
{
  "token": "verification_token_from_email"
}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "message": "Email verified successfully"
  }
}
```

---

## 3. User Endpoints

### 3.1 Get Current User Profile
**GET** `/users/me`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": 123,
    "email": "user@example.com",
    "name": "John Smith",
    "phone": "+1-555-0123",
    "bio": "DIY enthusiast",
    "profilePhotoUrl": "https://cdn.ubertool.com/users/123/profile.jpg",
    "location": {
      "lat": 47.6062,
      "lng": -122.3321
    },
    "aggregateRating": 4.8,
    "reviewCount": 24,
    "isVerified": true,
    "createdAt": "2024-01-15T10:30:00Z"
  }
}
```

---

### 3.2 Update Current User Profile
**PUT** `/users/me`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "name": "John Smith",
  "phone": "+1-555-0123",
  "bio": "Updated bio",
  "location": {
    "address": "123 Main St, Seattle, WA 98101"
  }
}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": 123,
    "name": "John Smith",
    "phone": "+1-555-0123",
    "bio": "Updated bio",
    "location": {
      "lat": 47.6062,
      "lng": -122.3321,
      "address": "123 Main St, Seattle, WA 98101"
    }
  }
}
```

---

### 3.3 Upload Profile Photo
**POST** `/users/me/photo`

**Headers:** 
- `Authorization: Bearer <access_token>`
- `Content-Type: multipart/form-data`

**Request Body:**
```
photo: <image_file>
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "profilePhotoUrl": "https://cdn.ubertool.com/users/123/profile.jpg"
  }
}
```

---

### 3.4 Get User by ID
**GET** `/users/:userId`

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": 123,
    "name": "John Smith",
    "bio": "DIY enthusiast",
    "profilePhotoUrl": "https://cdn.ubertool.com/users/123/profile.jpg",
    "aggregateRating": 4.8,
    "reviewCount": 24,
    "isVerified": true,
    "memberSince": "2024-01-15T10:30:00Z"
  }
}
```

---

### 3.5 Get User Ratings
**GET** `/users/:userId/ratings`

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "aggregateRating": 4.8,
    "totalReviews": 24,
    "distribution": {
      "5": 18,
      "4": 4,
      "3": 2,
      "2": 0,
      "1": 0
    }
  }
}
```

---

### 3.6 Delete Account
**DELETE** `/users/me`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `204 No Content`

---

## 4. Tool Endpoints

### 4.1 Create Tool Listing
**POST** `/tools`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "name": "DeWalt 20V Cordless Drill",
  "categoryId": 1,
  "description": "Professional-grade cordless drill...",
  "condition": "excellent",
  "dailyPrice": 15.00,
  "replacementValue": 120.00,
  "repairCosts": [
    { "part": "Chuck", "cost": 50.00 },
    { "part": "Battery", "cost": 30.00 }
  ],
  "location": {
    "address": "123 Main St, Seattle, WA 98101"
  },
  "termsConditions": "Must return cleaned..."
}
```

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": 456,
    "name": "DeWalt 20V Cordless Drill",
    "status": "active",
    "createdAt": "2026-01-11T17:42:05Z"
  }
}
```

---

### 4.2 Get Tool by ID
**GET** `/tools/:toolId`

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": 456,
    "name": "DeWalt 20V Cordless Drill",
    "categoryId": 1,
    "categoryName": "Power Tools",
    "description": "Professional-grade cordless drill...",
    "condition": "excellent",
    "dailyPrice": 15.00,
    "replacementValue": 120.00,
    "repairCosts": [
      { "part": "Chuck", "cost": 50.00 }
    ],
    "location": {
      "lat": 47.6062,
      "lng": -122.3321
    },
    "termsConditions": "Must return cleaned...",
    "status": "active",
    "viewCount": 42,
    "images": [
      {
        "id": 1,
        "url": "https://cdn.ubertool.com/tools/456/img1.jpg",
        "thumbnailUrl": "https://cdn.ubertool.com/tools/456/img1_thumb.jpg",
        "displayOrder": 0
      }
    ],
    "owner": {
      "id": 123,
      "name": "John Smith",
      "profilePhotoUrl": "https://cdn.ubertool.com/users/123/profile.jpg",
      "aggregateRating": 4.8,
      "reviewCount": 24,
      "isVerified": true
    },
    "createdAt": "2026-01-10T10:00:00Z",
    "updatedAt": "2026-01-11T15:00:00Z"
  }
}
```

---

### 4.3 Update Tool Listing
**PUT** `/tools/:toolId`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:** (partial update supported)
```json
{
  "dailyPrice": 18.00,
  "description": "Updated description..."
}
```

**Response:** `200 OK`

---

### 4.4 Delete Tool Listing
**DELETE** `/tools/:toolId`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `204 No Content`

**Error:** `400 Bad Request` if active rentals exist

---

### 4.5 Get My Listings
**GET** `/tools/my-listings`

**Headers:** `Authorization: Bearer <access_token>`

**Query Parameters:**
- `status` (optional): `active`, `inactive`, `rented`, `archived`
- `page` (default: 1)
- `limit` (default: 20, max: 100)

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "tools": [
      { /* tool object */ },
      { /* tool object */ }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 8,
      "totalPages": 1
    }
  }
}
```

---

### 4.6 Upload Tool Images
**POST** `/tools/:toolId/images`

**Headers:** 
- `Authorization: Bearer <access_token>`
- `Content-Type: multipart/form-data`

**Request Body:**
```
images: <image_file_1>
images: <image_file_2>
...
```

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "images": [
      {
        "id": 1,
        "url": "https://cdn.ubertool.com/tools/456/img1.jpg",
        "thumbnailUrl": "https://cdn.ubertool.com/tools/456/img1_thumb.jpg",
        "displayOrder": 0
      }
    ]
  }
}
```

---

### 4.7 Delete Tool Image
**DELETE** `/tools/:toolId/images/:imageId`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `204 No Content`

---

## 5. Search Endpoints

### 5.1 Search Tools
**GET** `/search/tools`

**Query Parameters:**
- `q` (optional): Search query
- `categoryId` (optional): Category filter
- `lat` (required): User latitude
- `lng` (required): User longitude
- `maxDistance` (optional, default: 10): Max distance in miles
- `minPrice` (optional): Minimum price
- `maxPrice` (optional): Maximum price
- `condition` (optional): `excellent`, `good`, `fair`, `poor`
- `sortBy` (optional, default: `distance`): `distance`, `price_asc`, `price_desc`, `rating`, `newest`
- `page` (default: 1)
- `limit` (default: 20, max: 100)

**Example:**
```
GET /search/tools?q=drill&categoryId=1&lat=47.6062&lng=-122.3321&maxDistance=10&sortBy=distance&page=1&limit=20
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "tools": [
      {
        "id": 456,
        "name": "DeWalt 20V Cordless Drill",
        "categoryName": "Power Tools",
        "condition": "excellent",
        "dailyPrice": 15.00,
        "primaryImageUrl": "https://cdn.ubertool.com/tools/456/img1_thumb.jpg",
        "distance": 0.8,
        "owner": {
          "name": "John Smith",
          "aggregateRating": 4.8
        }
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 45,
      "totalPages": 3
    }
  }
}
```

---

### 5.2 Get Nearby Tools
**GET** `/search/nearby`

**Query Parameters:**
- `lat` (required): Latitude
- `lng` (required): Longitude
- `radius` (optional, default: 10): Radius in miles

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "tools": [
      {
        "id": 456,
        "name": "DeWalt 20V Cordless Drill",
        "location": {
          "lat": 47.6062,
          "lng": -122.3321
        },
        "distance": 0.8,
        "dailyPrice": 15.00,
        "primaryImageUrl": "https://cdn.ubertool.com/tools/456/img1_thumb.jpg"
      }
    ]
  }
}
```

---

### 5.3 Get Categories
**GET** `/categories`

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "categories": [
      {
        "id": 1,
        "name": "Power Tools",
        "slug": "power-tools",
        "description": "Electric and battery-powered tools"
      }
    ]
  }
}
```

---

## 5A. Rental Schedule Endpoints

### 5A.1 Get Tool Schedule
**GET** `/tools/:toolId/schedule`

**Query Parameters:**
- `startDate` (optional, default: today): Start date in YYYY-MM-DD format
- `endDate` (optional, default: today + 90 days): End date in YYYY-MM-DD format

**Example:**
```
GET /tools/456/schedule?startDate=2026-01-15&endDate=2026-02-15
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "toolId": 456,
    "schedule": [
      {
        "type": "rental",
        "id": 789,
        "startDate": "2026-01-15",
        "endDate": "2026-01-17",
        "status": "finalized",
        "renterName": "Jane Doe"
      },
      {
        "type": "rental",
        "id": 790,
        "startDate": "2026-01-20",
        "endDate": "2026-01-22",
        "status": "accepted",
        "renterName": "Mike Smith"
      },
      {
        "type": "blocked",
        "id": 12,
        "startDate": "2026-01-25",
        "endDate": "2026-01-27",
        "reason": "maintenance",
        "notes": "Annual servicing"
      },
      {
        "type": "rental",
        "id": 791,
        "startDate": "2026-01-30",
        "endDate": "2026-01-31",
        "status": "pending",
        "renterName": "Sarah Johnson"
      }
    ]
  }
}
```

---

### 5A.2 Check Availability
**GET** `/tools/:toolId/schedule/availability`

**Query Parameters:**
- `startDate` (required): Start date in YYYY-MM-DD format
- `endDate` (required): End date in YYYY-MM-DD format

**Example:**
```
GET /tools/456/schedule/availability?startDate=2026-01-15&endDate=2026-01-17
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "available": false,
    "conflicts": [
      {
        "type": "rental",
        "id": 789,
        "startDate": "2026-01-15",
        "endDate": "2026-01-17",
        "status": "finalized"
      }
    ]
  }
}
```

**Available Response (no conflicts):**
```json
{
  "success": true,
  "data": {
    "available": true,
    "conflicts": []
  }
}
```

---

### 5A.3 Block Dates (Owner Only)
**PUT** `/tools/:toolId/schedule/block`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "startDate": "2026-01-25",
  "endDate": "2026-01-27",
  "reason": "maintenance",
  "notes": "Annual servicing"
}
```

**Validation:**
- `reason` must be one of: `maintenance`, `personal_use`, `vacation`, `other`
- `startDate` must be >= today
- `endDate` must be >= `startDate`
- Cannot block dates with finalized or accepted rentals

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": 12,
    "toolId": 456,
    "startDate": "2026-01-25",
    "endDate": "2026-01-27",
    "reason": "maintenance",
    "notes": "Annual servicing",
    "createdAt": "2026-01-11T17:42:05Z"
  }
}
```

**Error:** `409 Conflict` if dates overlap with existing rentals
```json
{
  "success": false,
  "error": {
    "code": "RENTAL_DATES_BLOCKED",
    "message": "Cannot block dates with existing rentals",
    "details": [
      {
        "rentalId": 789,
        "startDate": "2026-01-25",
        "endDate": "2026-01-26",
        "status": "accepted"
      }
    ]
  }
}
```

---

### 5A.4 Unblock Dates (Owner Only)
**DELETE** `/tools/:toolId/schedule/unblock`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "startDate": "2026-01-25",
  "endDate": "2026-01-27"
}
```

**Response:** `204 No Content`

**Error:** `409 Conflict` if dates have confirmed rentals
```json
{
  "success": false,
  "error": {
    "code": "RENTAL_CANNOT_UNBLOCK",
    "message": "Cannot unblock dates with confirmed rentals"
  }
}
```

---

### 5A.5 Get Rental Statistics
**GET** `/tools/:toolId/schedule/stats`

**Headers:** `Authorization: Bearer <access_token>` (owner only)

**Query Parameters:**
- `startDate` (optional, default: 90 days ago): Start date for stats
- `endDate` (optional, default: today): End date for stats

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "toolId": 456,
    "period": {
      "startDate": "2025-10-13",
      "endDate": "2026-01-11",
      "totalDays": 90
    },
    "utilization": {
      "rentedDays": 24,
      "blockedDays": 6,
      "availableDays": 60,
      "utilizationRate": 0.267
    },
    "revenue": {
      "totalRentals": 8,
      "estimatedRevenue": 360.00,
      "averageRentalDuration": 3.0
    }
  }
}
```

---

## 6. Rental Request Endpoints

### 6.1 Create Rental Request
**POST** `/rentals/request`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "toolId": 456,
  "startDate": "2026-01-15",
  "endDate": "2026-01-17",
  "renterMessage": "I need this for a weekend project",
  "termsAgreed": true
}
```

**Validation:**
- Dates must be in YYYY-MM-DD format (day-based scheduling)
- `startDate` must be >= today
- `endDate` must be > `startDate`
- Rental period must be <= 30 days
- `termsAgreed` must be `true`

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": 789,
    "toolId": 456,
    "status": "pending",
    "startDate": "2026-01-15",
    "endDate": "2026-01-17",
    "rentalDays": 3,
    "createdAt": "2026-01-11T17:42:05Z"
  }
}
```

**Error:** `422 Unprocessable Entity` if dates are invalid
```json
{
  "success": false,
  "error": {
    "code": "RENTAL_PERIOD_INVALID",
    "message": "Invalid rental period",
    "details": [
      {
        "field": "endDate",
        "message": "End date must be after start date"
      }
    ]
  }
}
```

---

### 6.2 Get Rental Request
**GET** `/rentals/requests/:requestId`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": 789,
    "tool": {
      "id": 456,
      "name": "DeWalt 20V Cordless Drill",
      "dailyPrice": 15.00
    },
    "renter": {
      "id": 124,
      "name": "Jane Doe",
      "aggregateRating": 4.9
    },
    "owner": {
      "id": 123,
      "name": "John Smith"
    },
    "startDate": "2026-01-15",
    "endDate": "2026-01-17",
    "rentalDays": 3,
    "status": "pending",
    "renterMessage": "I need this for a weekend project",
    "createdAt": "2026-01-11T17:42:05Z"
  }
}
```

---

### 6.3 Accept Rental Request
**PUT** `/rentals/requests/:requestId/accept`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "pickupAddress": "123 Main St, Seattle, WA 98101",
  "pickupPhone": "+1-555-0123",
  "pickupTime": "2026-01-15T09:00:00Z",
  "ownerResponse": "Looking forward to it!"
}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": 789,
    "status": "accepted",
    "pickupAddress": "123 Main St, Seattle, WA 98101",
    "pickupPhone": "+1-555-0123",
    "pickupTime": "2026-01-15T09:00:00Z"
  }
}
```

**Error:** `409 Conflict` if dates overlap with existing rentals
```json
{
  "success": false,
  "error": {
    "code": "RENTAL_DATES_OVERLAP",
    "message": "Requested dates conflict with existing rental",
    "details": [
      {
        "conflictingRentalId": 788,
        "startDate": "2026-01-16",
        "endDate": "2026-01-18",
        "status": "finalized"
      }
    ]
  }
}
```

---

### 6.4 Reject Rental Request
**PUT** `/rentals/requests/:requestId/reject`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "ownerResponse": "Sorry, tool is unavailable those dates"
}
```

**Response:** `200 OK`

---

### 6.5 Finalize Rental Request
**PUT** `/rentals/requests/:requestId/finalize`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": 789,
    "status": "finalized",
    "cancelledRequests": [790, 791]
  }
}
```

---

### 6.6 Cancel Rental Request
**PUT** `/rentals/requests/:requestId/cancel`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `200 OK`

---

### 6.7 Get My Requests (Renter)
**GET** `/rentals/my-requests`

**Headers:** `Authorization: Bearer <access_token>`

**Query Parameters:**
- `status` (optional): Filter by status
- `page`, `limit`

**Response:** `200 OK`

---

### 6.8 Get Incoming Requests (Owner)
**GET** `/rentals/incoming-requests`

**Headers:** `Authorization: Bearer <access_token>`

**Query Parameters:**
- `status` (optional): Filter by status
- `page`, `limit`

**Response:** `200 OK`

---

## 7. Review Endpoints

### 7.1 Create Review
**POST** `/reviews`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "rentalRequestId": 789,
  "rating": 5,
  "comment": "Great tool, worked perfectly!"
}
```

**Response:** `201 Created`

---

### 7.2 Get Review
**GET** `/reviews/:reviewId`

**Response:** `200 OK`

---

### 7.3 Update Review
**PUT** `/reviews/:reviewId`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "rating": 4,
  "comment": "Updated review text"
}
```

**Response:** `200 OK`

**Error:** `403 Forbidden` if more than 24 hours have passed

---

### 7.4 Delete Review
**DELETE** `/reviews/:reviewId`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `204 No Content`

---

### 7.5 Get Reviews for User
**GET** `/reviews/user/:userId`

**Query Parameters:**
- `page`, `limit`

**Response:** `200 OK`

---

### 7.6 Get Reviews for Tool
**GET** `/reviews/tool/:toolId`

**Query Parameters:**
- `page`, `limit`

**Response:** `200 OK`

---

### 7.7 Flag Review
**POST** `/reviews/:reviewId/flag`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "reason": "spam",
  "details": "This is clearly spam content"
}
```

**Response:** `200 OK`

---

## 8. Notification Endpoints

### 8.1 Get Notifications
**GET** `/notifications`

**Headers:** `Authorization: Bearer <access_token>`

**Query Parameters:**
- `isRead` (optional): `true` or `false`
- `page`, `limit`

**Response:** `200 OK`

---

### 8.2 Mark Notification as Read
**PUT** `/notifications/:notificationId/read`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `200 OK`

---

### 8.3 Get Notification Preferences
**GET** `/notifications/preferences`

**Headers:** `Authorization: Bearer <access_token>`

**Response:** `200 OK`

---

### 8.4 Update Notification Preferences
**PUT** `/notifications/preferences`

**Headers:** `Authorization: Bearer <access_token>`

**Request Body:**
```json
{
  "emailRentalRequest": true,
  "emailRentalAccepted": true,
  "pushRentalRequest": false
}
```

**Response:** `200 OK`

---

## 9. Rate Limiting

- **Per User:** 100 requests per minute
- **Per IP:** 1000 requests per minute
- **Headers:**
  - `X-RateLimit-Limit`: Total allowed requests
  - `X-RateLimit-Remaining`: Remaining requests
  - `X-RateLimit-Reset`: Timestamp when limit resets

**429 Response:**
```json
{
  "success": false,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests. Please try again later.",
    "retryAfter": 60
  }
}
```

---

## 10. Error Codes

| Code | Description |
|------|-------------|
| `VALIDATION_ERROR` | Input validation failed |
| `AUTHENTICATION_REQUIRED` | Missing or invalid auth token |
| `FORBIDDEN` | Insufficient permissions |
| `NOT_FOUND` | Resource not found |
| `CONFLICT` | Resource conflict (e.g., duplicate) |
| `RATE_LIMIT_EXCEEDED` | Too many requests |
| `INTERNAL_ERROR` | Server error |
| `SERVICE_UNAVAILABLE` | Service temporarily down |

---

This API specification provides a complete reference for integrating with the Ubertool backend platform.
